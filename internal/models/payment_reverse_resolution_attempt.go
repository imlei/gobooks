// 遵循project_guide.md
package models

// payment_reverse_resolution_attempt.go — Batch 26: Payment reverse hook types and attempt truth.
//
// ─── Layer position ───────────────────────────────────────────────────────────
//
//   PaymentTransaction / PaymentAllocation / PaymentReverseAllocation  ← core truth
//   PaymentReverseException                                             ← exception truth
//   PaymentReverseResolutionAttempt                                     ← hook trace (THIS)
//
// ─── Hook types ───────────────────────────────────────────────────────────────
//
//   Navigation hooks (link only, NO attempt row):
//     open_original_charge       — navigate to the original charge/capture transaction
//     open_reverse_transaction   — navigate to the reverse (refund/chargeback) transaction
//     open_forward_allocations   — navigate to the transactions page filtered to the
//                                  original charge so forward allocations are visible
//
//   Execution hooks (server-side validation + attempt row):
//     retry_safe_reverse_check   — calls the existing reverse-allocation eligibility
//                                  validator (read-only).  Does NOT apply the reversal.
//                                  Records a succeeded/rejected attempt with the reason.
//                                  On success, transitions the exception to "reviewed"
//                                  so the operator knows the path is clear.
//
// ─── Invariants ───────────────────────────────────────────────────────────────
//
//   - Execution hooks record exactly one attempt row per trigger.
//   - Navigation hooks produce NO attempt rows — they are link-only.
//   - Attempt rows are NEVER modified after creation.
//   - All rows are company-scoped; cross-company reads are not permitted.
//   - Execution hooks NEVER directly modify JE / invoice / allocation truth.
//     They only call existing, strictly validated domain services.
//
// ─── Not in Batch 26 ─────────────────────────────────────────────────────────
//   - Auto-apply after successful validation (Batch 27+)
//   - Manual invoice restore editor
//   - Full guarded reverse execution engine
//   - Assignment / SLA / notification workflows

import "time"

// PRHookType identifies the action class for a payment reverse hook.
type PRHookType string

const (
	// PRHookOpenOriginalCharge is a navigation hook that directs the operator
	// to the transactions page anchored to the original charge/capture transaction.
	// Eligible when OriginalTxnID is not nil.
	PRHookOpenOriginalCharge PRHookType = "open_original_charge"

	// PRHookOpenReverseTransaction is a navigation hook that directs the operator
	// to the transactions page anchored to the reverse (refund/chargeback) transaction.
	// Eligible when ReverseTxnID is not nil.
	PRHookOpenReverseTransaction PRHookType = "open_reverse_transaction"

	// PRHookOpenForwardAllocations is a navigation hook that directs the operator
	// to the transactions page where forward allocations for the original charge
	// are visible.  Eligible when OriginalTxnID is not nil and the original charge
	// used the multi-invoice allocation path (has PaymentAllocation rows).
	PRHookOpenForwardAllocations PRHookType = "open_forward_allocations"

	// PRHookRetryCheck is an execution hook that calls the existing
	// reverse-allocation eligibility validator for the reverse transaction.
	//
	// It does NOT apply the reversal — it is a read-only safety check.
	// On success: records a succeeded attempt, transitions exception to "reviewed".
	// On rejection: records a rejected attempt with the blocking reason.
	//
	// Eligible when:
	//   - ReverseTxnID and OriginalTxnID are both non-nil
	//   - The original charge used the multi-invoice allocation path
	//   - Exception status is open or reviewed (not terminal)
	PRHookRetryCheck PRHookType = "retry_safe_reverse_check"
)

// AllPRHookTypes returns every known payment reverse hook type.
func AllPRHookTypes() []PRHookType {
	return []PRHookType{
		PRHookOpenOriginalCharge,
		PRHookOpenReverseTransaction,
		PRHookOpenForwardAllocations,
		PRHookRetryCheck,
	}
}

// IsPRExecutionHook returns true when the hook requires a server-side action
// and therefore produces a PaymentReverseResolutionAttempt record.
// Navigation hooks return false.
func IsPRExecutionHook(h PRHookType) bool {
	return h == PRHookRetryCheck
}

// PRAttemptStatus is the outcome of an execution hook attempt.
type PRAttemptStatus string

const (
	// PRAttemptSucceeded means the hook executed successfully and the
	// underlying domain validation/service call completed without error.
	PRAttemptSucceeded PRAttemptStatus = "succeeded"

	// PRAttemptRejected means the hook was rejected either by eligibility
	// checks (pre-conditions not met) or by the underlying domain service.
	PRAttemptRejected PRAttemptStatus = "rejected"
)

// PaymentReverseResolutionAttempt is the auditable record of one execution
// hook trigger on a payment reverse exception.
//
// One row is created per trigger of an execution hook, regardless of outcome.
// Navigation hooks do NOT produce attempt rows.
//
// Rows are immutable after creation — status, summary, and detail are written
// once and never updated.
type PaymentReverseResolutionAttempt struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// PaymentReverseExceptionID links this attempt to its parent exception.
	PaymentReverseExceptionID uint `gorm:"not null;index"`

	// HookType is the execution hook that was triggered.
	HookType PRHookType `gorm:"type:text;not null;index"`

	// Status is the outcome of the hook execution.
	Status PRAttemptStatus `gorm:"type:text;not null"`

	// Summary is a one-line human-readable description of the outcome.
	Summary string `gorm:"type:text;not null;default:''"`

	// Detail carries additional context (error messages, amounts, IDs).
	Detail string `gorm:"type:text;not null;default:''"`

	// Actor is the user email or "system" that triggered the hook.
	Actor string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
}
