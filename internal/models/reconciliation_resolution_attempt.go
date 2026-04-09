// 遵循project_guide.md
package models

// reconciliation_resolution_attempt.go — Batch 21: Resolution hook types and attempt truth.
//
// ─── Layer position ───────────────────────────────────────────────────────────
//
//   GatewayPayout / BankEntry / GatewayPayoutComponent  ← core business truth
//   PayoutReconciliation                                 ← 1:1 match truth
//   ReconciliationException                              ← anomaly truth
//   ReconciliationResolutionAttempt                      ← hook execution trace (THIS)
//
// ─── Hook types ───────────────────────────────────────────────────────────────
//
//   Execution hooks (server-side action + attempt record):
//     retry_match           — calls MatchGatewayPayoutToBankEntry; eligible for
//                             amount_mismatch exceptions only.  Succeeds only when the
//                             current expected net (after components) equals the linked
//                             bank entry amount.
//
//   Navigation hooks (link only, no attempt record):
//     open_payout_components — directs the operator to the payout detail / component
//                              editor so they can correct the component set before
//                              retrying the match.
//
// ─── Invariants ───────────────────────────────────────────────────────────────
//
//   - Execution hooks record exactly one attempt row per trigger.
//   - Navigation hooks produce NO attempt rows — they are link-only.
//   - An attempt row is NEVER modified after creation.
//   - Attempt rows belong to a company and are scoped by company_id on all reads.
//   - Hook execution NEVER directly modifies JE / bank entry / payout core truth.
//     It only calls existing domain services that manage their own truth.
//
// ─── Not in Batch 21 ──────────────────────────────────────────────────────────
//   - Manual override / forced match
//   - Multi-to-multi resolution hooks
//   - SLA / assignment / notification workflows
//   - Auto-resolution suggestion engine

import "time"

// ResolutionHookType identifies the action class for a resolution hook.
type ResolutionHookType string

const (
	// HookTypeRetryMatch is an execution hook that calls the existing
	// 1:1 payout ↔ bank entry matching service.  Eligible only for
	// exceptions of type ExceptionAmountMismatch where both a payout and a
	// bank entry are linked and neither is already matched.
	HookTypeRetryMatch ResolutionHookType = "retry_match"

	// HookTypeOpenPayoutComponents is a navigation hook that links the operator
	// to the payout detail / component editor for the linked payout.
	// It does not execute any server-side action.
	HookTypeOpenPayoutComponents ResolutionHookType = "open_payout_components"
)

// AllResolutionHookTypes returns every known hook type for validation.
func AllResolutionHookTypes() []ResolutionHookType {
	return []ResolutionHookType{
		HookTypeRetryMatch,
		HookTypeOpenPayoutComponents,
	}
}

// IsExecutionHook returns true when the hook requires a server-side action and
// therefore produces a ResolutionAttempt record.
// Navigation hooks return false.
func IsExecutionHook(h ResolutionHookType) bool {
	return h == HookTypeRetryMatch
}

// ResolutionAttemptStatus is the outcome of an execution hook attempt.
type ResolutionAttemptStatus string

const (
	// AttemptStatusSucceeded means the hook executed successfully and the
	// domain service call completed without error.
	AttemptStatusSucceeded ResolutionAttemptStatus = "succeeded"

	// AttemptStatusRejected means the hook was rejected either by eligibility
	// checks (pre-conditions not met) or by the underlying domain service.
	AttemptStatusRejected ResolutionAttemptStatus = "rejected"
)

// ReconciliationResolutionAttempt is the auditable record of one execution
// hook trigger on a reconciliation exception.
//
// One row is created per trigger of an execution hook, regardless of outcome.
// Navigation hooks do NOT produce attempt rows.
//
// Rows are immutable after creation — status, summary, and detail are written
// once and never updated.
type ReconciliationResolutionAttempt struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// ReconciliationExceptionID links this attempt to its parent exception.
	ReconciliationExceptionID uint `gorm:"not null;index"`

	// HookType is the execution hook that was triggered.
	HookType ResolutionHookType `gorm:"type:text;not null;index"`

	// Status is the outcome of the hook execution.
	Status ResolutionAttemptStatus `gorm:"type:text;not null"`

	// Summary is a one-line human-readable description of the outcome.
	Summary string `gorm:"type:text;not null;default:''"`

	// Detail carries additional context (error messages, amounts, IDs).
	Detail string `gorm:"type:text;not null;default:''"`

	// Actor is the user email or "system" that triggered the hook.
	Actor string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
}
