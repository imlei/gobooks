// 遵循project_guide.md
package models

// reconciliation_exception.go — Batch 20: Reconciliation exception truth.
//
// A ReconciliationException records a named, auditable anomaly in the payout ↔
// bank-entry reconciliation workflow.  It is created when the matching path
// detects a structural problem that cannot be resolved automatically, or when
// an operator explicitly reports an unsupported scenario.
//
// ─── Layer position ───────────────────────────────────────────────────────────
//
//   GatewayPayout / BankEntry / GatewayPayoutComponent  ← core business truth
//   PayoutReconciliation                                 ← 1:1 match truth
//   ReconciliationException                              ← anomaly / exception truth (THIS)
//
// ─── Exception types ──────────────────────────────────────────────────────────
//
//   Auto-created when the matching path rejects a structurally complex case:
//     amount_mismatch         — expected net ≠ bank entry amount after components
//     account_mismatch        — bank accounts differ (structural, not a typo)
//     currency_mismatch       — currency codes differ (structural)
//     payout_conflict         — payout already matched, another match attempt made
//     bank_entry_conflict     — bank entry already matched, reuse attempted
//
//   Manually filed by the operator for unsupported scenarios:
//     unsupported_many_to_many    — one payout should match many bank entries (or vice versa)
//     unsupported_one_to_many     — single bank entry should cover multiple payouts
//     unsupported_many_to_one     — single payout should cover multiple bank entries
//     unknown_component_pattern   — delta cannot be explained by available components
//
// ─── Status machine ───────────────────────────────────────────────────────────
//
//   open → reviewed → dismissed  (terminal)
//        → dismissed              (terminal)
//        → resolved               (terminal)
//   reviewed → resolved           (terminal)
//
//   dismissed and resolved are terminal — no further transitions.
//
// ─── Exception is NOT ─────────────────────────────────────────────────────────
//   - Not a manual override / force-match path
//   - Not a JE creation trigger
//   - Not a replacement for the match truth (PayoutReconciliation)
//   - Not a free-text error bucket — every exception has a structured type
//
// ─── Dedup ────────────────────────────────────────────────────────────────────
//   Before creating an exception the service checks for an existing open/reviewed
//   exception with the same normalized source key:
//     (company_id, type, gateway_payout_id, bank_entry_id, payout_reconciliation_id)
//   with nil references normalized to 0 in the internal dedup key.
//   If found, the existing one is returned and no new record is created.
//   A partial unique index on (company_id, dedup_key) for active statuses
//   ("open", "reviewed") acts as the database backstop against duplicate storms.
//
// ─── Future scope (not in Batch 20) ──────────────────────────────────────────
//   - Manual override / forced match
//   - Assignment / SLA workflow
//   - Exception analytics / aging reports
//   - Notification triggers
//   - Bulk exception import

import "time"

// ReconciliationExceptionType is the structured category of an exception.
type ReconciliationExceptionType string

const (
	// ── Auto-created by the matching path ───────────────────────────────────

	// ExceptionAmountMismatch is created when the matching path fails because
	// the payout's expected net (after components) does not equal the bank entry
	// amount.  This is a structural mismatch, not a simple input error.
	ExceptionAmountMismatch ReconciliationExceptionType = "amount_mismatch"

	// ExceptionAccountMismatch is created when the payout's bank account does
	// not match the bank entry's bank account.
	ExceptionAccountMismatch ReconciliationExceptionType = "account_mismatch"

	// ExceptionCurrencyMismatch is created when the payout currency does not
	// match the bank entry currency.
	ExceptionCurrencyMismatch ReconciliationExceptionType = "currency_mismatch"

	// ExceptionPayoutConflict is created when a payout that is already matched
	// has another match attempt submitted.
	ExceptionPayoutConflict ReconciliationExceptionType = "payout_conflict"

	// ExceptionBankEntryConflict is created when a bank entry that is already
	// matched has a reuse attempt submitted.
	ExceptionBankEntryConflict ReconciliationExceptionType = "bank_entry_conflict"

	// ── Manually filed by the operator ─────────────────────────────────────

	// ExceptionUnsupportedManyToMany is filed when the operator identifies that
	// one payout corresponds to multiple bank entries or vice versa — a scenario
	// the current 1:1 engine does not support.
	ExceptionUnsupportedManyToMany ReconciliationExceptionType = "unsupported_many_to_many"

	// ExceptionUnsupportedOneToMany is filed when a single bank entry should
	// cover multiple payouts.
	ExceptionUnsupportedOneToMany ReconciliationExceptionType = "unsupported_one_to_many"

	// ExceptionUnsupportedManyToOne is filed when a single payout should cover
	// multiple bank entries.
	ExceptionUnsupportedManyToOne ReconciliationExceptionType = "unsupported_many_to_one"

	// ExceptionUnknownComponentPattern is filed when the delta between the
	// payout's expected net and the bank entry cannot be explained by any
	// supported component type.
	ExceptionUnknownComponentPattern ReconciliationExceptionType = "unknown_component_pattern"
)

// AllReconciliationExceptionTypes returns every supported exception type.
func AllReconciliationExceptionTypes() []ReconciliationExceptionType {
	return []ReconciliationExceptionType{
		ExceptionAmountMismatch,
		ExceptionAccountMismatch,
		ExceptionCurrencyMismatch,
		ExceptionPayoutConflict,
		ExceptionBankEntryConflict,
		ExceptionUnsupportedManyToMany,
		ExceptionUnsupportedOneToMany,
		ExceptionUnsupportedManyToOne,
		ExceptionUnknownComponentPattern,
	}
}

// ManuallyFilableExceptionTypes returns the exception types an operator may file
// manually (the structural mismatch types are auto-created by the matching path).
func ManuallyFilableExceptionTypes() []ReconciliationExceptionType {
	return []ReconciliationExceptionType{
		ExceptionUnsupportedManyToMany,
		ExceptionUnsupportedOneToMany,
		ExceptionUnsupportedManyToOne,
		ExceptionUnknownComponentPattern,
	}
}

// ReconciliationExceptionTypeLabel returns a human-readable label.
func ReconciliationExceptionTypeLabel(t ReconciliationExceptionType) string {
	switch t {
	case ExceptionAmountMismatch:
		return "Amount Mismatch"
	case ExceptionAccountMismatch:
		return "Account Mismatch"
	case ExceptionCurrencyMismatch:
		return "Currency Mismatch"
	case ExceptionPayoutConflict:
		return "Payout Already Matched"
	case ExceptionBankEntryConflict:
		return "Bank Entry Already Matched"
	case ExceptionUnsupportedManyToMany:
		return "Unsupported: Many-to-Many"
	case ExceptionUnsupportedOneToMany:
		return "Unsupported: One-to-Many"
	case ExceptionUnsupportedManyToOne:
		return "Unsupported: Many-to-One"
	case ExceptionUnknownComponentPattern:
		return "Unknown Component Pattern"
	default:
		return string(t)
	}
}

// ReconciliationExceptionStatus is the lifecycle state of an exception.
type ReconciliationExceptionStatus string

const (
	// ExceptionStatusOpen is the initial state.  Requires attention.
	ExceptionStatusOpen ReconciliationExceptionStatus = "open"

	// ExceptionStatusReviewed means the operator has acknowledged the exception
	// but has not taken a resolution action yet.  Not a terminal state.
	ExceptionStatusReviewed ReconciliationExceptionStatus = "reviewed"

	// ExceptionStatusDismissed is a terminal state.  The operator has determined
	// this exception requires no further action (e.g. the discrepancy was
	// expected, or it will be handled outside the system).
	ExceptionStatusDismissed ReconciliationExceptionStatus = "dismissed"

	// ExceptionStatusResolved is a terminal state.  The underlying issue has
	// been formally resolved (e.g. a subsequent successful match in a later
	// batch, or a manual correction outside the reconciliation engine).
	ExceptionStatusResolved ReconciliationExceptionStatus = "resolved"
)

// IsTerminalExceptionStatus returns true when the status is a final state and
// no further transitions are permitted.
func IsTerminalExceptionStatus(s ReconciliationExceptionStatus) bool {
	return s == ExceptionStatusDismissed || s == ExceptionStatusResolved
}

// ReconciliationException is the named, auditable record of a reconciliation
// anomaly.  It is the exception-layer truth — separate from matching truth
// (PayoutReconciliation) and core business truth (GatewayPayout / BankEntry).
//
// An exception does NOT:
//   - create or modify any Journal Entry
//   - change the matched/unmatched state of a payout or bank entry
//   - constitute an override or force-match
type ReconciliationException struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index;uniqueIndex:uq_recon_exception_active,where:status <> 'dismissed' AND status <> 'resolved'"`

	// ExceptionType is the structured category.  Must be one of the constants above.
	ExceptionType ReconciliationExceptionType `gorm:"type:text;not null;index"`

	// Status tracks the exception lifecycle.  Starts as "open".
	Status ReconciliationExceptionStatus `gorm:"type:text;not null;default:'open';index"`

	// Optional references to related business objects.  At least one source
	// reference is required so the exception stays anchored to real truth.
	GatewayPayoutID        *uint `gorm:"index"`
	BankEntryID            *uint `gorm:"index"`
	PayoutReconciliationID *uint `gorm:"index"` // set when the exception arose from a post-match issue

	// DedupKey is an internal normalized source key used only to prevent active
	// exception storms.  It is not user-facing truth.
	DedupKey string `gorm:"type:text;not null;default:'';uniqueIndex:uq_recon_exception_active,where:status <> 'dismissed' AND status <> 'resolved'"`

	// Summary is a one-line human-readable description of why this exception exists.
	Summary string `gorm:"type:text;not null;default:''"`

	// Detail is optional additional context (amounts, IDs, reasons).
	// May be a JSON string or free text.  Not required for display.
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
