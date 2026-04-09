// 遵循project_guide.md
package models

// payment_reverse_exception.go — Batch 23: Payment-side reverse exception truth.
//
// A PaymentReverseException records a named, auditable anomaly in the payment
// reverse allocation workflow.  It is created automatically when a reverse
// allocation attempt fails for a structural reason that cannot be resolved
// without operator intervention.
//
// ─── Layer position ───────────────────────────────────────────────────────────
//
//   PaymentTransaction (charge/capture)       ← core payment truth
//   PaymentAllocation                         ← multi-invoice forward allocation truth (Batch 17)
//   PaymentReverseAllocation                  ← reverse allocation truth (Batch 22)
//   PaymentReverseException                   ← reverse anomaly / exception truth (THIS)
//
// ─── Exception types ──────────────────────────────────────────────────────────
//
//   Auto-created when the reverse allocation path rejects a structurally complex case:
//     reverse_allocation_ambiguous             — original charge cannot be resolved (no linkage)
//     reverse_amount_exceeds_supported_strategy— reversal amount exceeds remaining reversible capacity
//     reverse_over_credit_boundary             — reversal would restore invoice balance above total
//     reverse_requires_manual_split            — case cannot be handled automatically; operator must split
//     reverse_chain_conflict                   — conflicting reverse chain detected
//     unsupported_multi_layer_reversal         — multi-layer reversal chain is not supported
//
// ─── Status machine ───────────────────────────────────────────────────────────
//
//   open → reviewed → dismissed  (terminal)
//        → dismissed              (terminal)
//        → resolved               (terminal)
//   reviewed → resolved           (terminal)
//
// ─── Dedup ────────────────────────────────────────────────────────────────────
//   Before creating an exception the service checks for an existing open/reviewed
//   exception with the same normalized source key:
//     (company_id, type, reverse_txn_id, original_txn_id)
//   with nil references normalized to 0 in the internal dedup key.
//   A partial unique index on (company_id, dedup_key) for active statuses
//   ("open", "reviewed") acts as the database backstop against duplicate storms.
//
// ─── Separation from ReconciliationException ─────────────────────────────────
//   PaymentReverseException is intentionally a separate model from
//   ReconciliationException.  Reconciliation exceptions relate to payout ↔
//   bank-entry matching anomalies.  Payment reverse exceptions relate to
//   payment-side reversal structural failures.  Mixing them would create an
//   unmaintainable big-table pattern.

import "time"

// PaymentReverseExceptionType is the structured category of a payment reverse exception.
type PaymentReverseExceptionType string

const (
	// PRExceptionReverseAllocationAmbiguous is created when the reverse transaction
	// cannot be linked back to an original charge/capture (no OriginalTransactionID
	// and no PaymentRequest linkage).  Operator must manually identify the original.
	PRExceptionReverseAllocationAmbiguous PaymentReverseExceptionType = "reverse_allocation_ambiguous"

	// PRExceptionAmountExceedsStrategy is created when the reversal amount exceeds
	// the remaining reversible allocation capacity for the original charge.
	// This can occur when partial reversals have already been applied.
	PRExceptionAmountExceedsStrategy PaymentReverseExceptionType = "reverse_amount_exceeds_supported_strategy"

	// PRExceptionOverCreditBoundary is created when applying the reversal would
	// restore an invoice's BalanceDue above its total Amount — i.e. an over-credit.
	// This indicates a data-consistency issue that requires manual inspection.
	PRExceptionOverCreditBoundary PaymentReverseExceptionType = "reverse_over_credit_boundary"

	// PRExceptionRequiresManualSplit is created when the reversal scenario requires
	// the operator to manually specify how to split the reversal across invoices
	// (e.g. FX invoices, selective partial reversal of specific line items).
	PRExceptionRequiresManualSplit PaymentReverseExceptionType = "reverse_requires_manual_split"

	// PRExceptionChainConflict is created when a conflicting reverse chain is
	// detected — e.g. the original charge is itself already a reversal, or two
	// concurrent reverse attempts conflict with each other.
	PRExceptionChainConflict PaymentReverseExceptionType = "reverse_chain_conflict"

	// PRExceptionUnsupportedMultiLayerReversal is created when a multi-layer
	// reversal chain (reversal of a reversal) is detected and not supported.
	PRExceptionUnsupportedMultiLayerReversal PaymentReverseExceptionType = "unsupported_multi_layer_reversal"
)

// AllPaymentReverseExceptionTypes returns every supported exception type.
func AllPaymentReverseExceptionTypes() []PaymentReverseExceptionType {
	return []PaymentReverseExceptionType{
		PRExceptionReverseAllocationAmbiguous,
		PRExceptionAmountExceedsStrategy,
		PRExceptionOverCreditBoundary,
		PRExceptionRequiresManualSplit,
		PRExceptionChainConflict,
		PRExceptionUnsupportedMultiLayerReversal,
	}
}

// PaymentReverseExceptionTypeLabel returns a human-readable label.
func PaymentReverseExceptionTypeLabel(t PaymentReverseExceptionType) string {
	switch t {
	case PRExceptionReverseAllocationAmbiguous:
		return "Ambiguous Original Charge"
	case PRExceptionAmountExceedsStrategy:
		return "Amount Exceeds Reversible Capacity"
	case PRExceptionOverCreditBoundary:
		return "Would Over-Credit Invoice"
	case PRExceptionRequiresManualSplit:
		return "Requires Manual Split"
	case PRExceptionChainConflict:
		return "Reverse Chain Conflict"
	case PRExceptionUnsupportedMultiLayerReversal:
		return "Unsupported Multi-Layer Reversal"
	default:
		return string(t)
	}
}

// PaymentReverseExceptionStatus is the lifecycle state of a payment reverse exception.
type PaymentReverseExceptionStatus string

const (
	// PRExceptionStatusOpen is the initial state.  Requires attention.
	PRExceptionStatusOpen PaymentReverseExceptionStatus = "open"

	// PRExceptionStatusReviewed means the operator has acknowledged the exception
	// but has not taken a resolution action yet.  Not a terminal state.
	PRExceptionStatusReviewed PaymentReverseExceptionStatus = "reviewed"

	// PRExceptionStatusDismissed is a terminal state.  The operator has determined
	// this exception requires no further action.
	PRExceptionStatusDismissed PaymentReverseExceptionStatus = "dismissed"

	// PRExceptionStatusResolved is a terminal state.  The underlying issue has
	// been formally resolved.
	PRExceptionStatusResolved PaymentReverseExceptionStatus = "resolved"
)

// IsTerminalPRExceptionStatus returns true when the status is a final state.
func IsTerminalPRExceptionStatus(s PaymentReverseExceptionStatus) bool {
	return s == PRExceptionStatusDismissed || s == PRExceptionStatusResolved
}

// PaymentReverseException is the named, auditable record of a payment reverse
// allocation anomaly.
//
// It does NOT:
//   - create or modify any Journal Entry
//   - change the application state of any transaction or invoice
//   - constitute an override or force-apply
type PaymentReverseException struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index;uniqueIndex:uq_payment_rev_exc_active,where:status <> 'dismissed' AND status <> 'resolved'"`

	// ExceptionType is the structured category.
	ExceptionType PaymentReverseExceptionType `gorm:"type:text;not null;index"`

	// Status tracks the exception lifecycle.  Starts as "open".
	Status PaymentReverseExceptionStatus `gorm:"type:text;not null;default:'open';index"`

	// Optional payment transaction references.  At least one source reference
	// should be set so the exception is anchored to real truth.
	ReverseTxnID  *uint `gorm:"index"`
	OriginalTxnID *uint `gorm:"index"`

	// DedupKey is an internal normalized source key used only to prevent active
	// exception storms.  It is not user-facing truth.
	DedupKey string `gorm:"type:text;not null;default:'';uniqueIndex:uq_payment_rev_exc_active,where:status <> 'dismissed' AND status <> 'resolved'"`

	// Summary is a one-line human-readable description of why this exception exists.
	Summary string `gorm:"type:text;not null;default:''"`

	// Detail is optional additional context (amounts, IDs, reasons).
	Detail string `gorm:"type:text;not null;default:''"`

	// Resolution fields — populated when the exception is dismissed or resolved.
	ResolvedAt      *time.Time `gorm:"index"`
	ResolvedByActor string     `gorm:"type:text;not null;default:''"`
	ResolutionNote  string     `gorm:"type:text;not null;default:''"`

	// Who triggered the exception creation.
	CreatedByActor string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
