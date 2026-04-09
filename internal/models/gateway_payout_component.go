// 遵循project_guide.md
package models

// gateway_payout_component.go — Batch 19: Payout component / composition truth.
//
// A GatewayPayoutComponent records one line in the composition of a payout's
// expected bank deposit.  Examples:
//
//   type=fee,            direction=debit,  amount=5.00   → processor deducted an extra $5 fee
//   type=reserve_hold,   direction=debit,  amount=20.00  → processor withheld $20 as rolling reserve
//   type=reserve_release,direction=credit, amount=15.00  → processor released $15 of prior reserve
//   type=adjustment,     direction=credit, amount=3.00   → positive adjustment (e.g. dispute won)
//
// ─── Expected net formula ─────────────────────────────────────────────────────
//
//   ExpectedNet = GatewayPayout.NetAmount
//               + Σ (amount of credit-direction components)
//               − Σ (amount of debit-direction components)
//
// This is the value compared against BankEntry.Amount during reconciliation.
//
// ─── Accounting note ──────────────────────────────────────────────────────────
//   Components are reconciliation-side explanatory truth only.
//   No Journal Entry is created or modified when components are added.
//   The original payout JE (Dr Bank / Dr Fee Expense / Cr Clearing) is the
//   accounting fact and is never touched by this layer.
//
// ─── Modification guard ───────────────────────────────────────────────────────
//   Components may only be added/deleted while the payout is unmatched.
//   Once a PayoutReconciliation record exists for a payout, the component set
//   is locked — changes would silently invalidate the match truth.
//
// ─── Direction constraints per type ──────────────────────────────────────────
//   fee            → direction must be debit  (reduces bank deposit)
//   reserve_hold   → direction must be debit  (reduces bank deposit)
//   reserve_release→ direction must be credit (increases bank deposit)
//   adjustment     → either direction (operator-specified)
//
// ─── Future scope (not in Batch 19) ──────────────────────────────────────────
//   - JE postings for reserve / adjustment components
//   - Reserve aging / analytics / forecast
//   - Adjustment origin tracing
//   - Bulk component import from processor settlement report

import (
	"time"

	"github.com/shopspring/decimal"
)

// PayoutComponentType identifies the business nature of a payout composition
// difference.  Only the four types below are supported in Batch 19.
type PayoutComponentType string

const (
	// PayoutComponentFee is an additional processor fee not captured in
	// GatewayPayout.FeeAmount (e.g. a monthly fee deducted from a payout).
	// Direction: always debit — reduces expected bank deposit.
	PayoutComponentFee PayoutComponentType = "fee"

	// PayoutComponentReserveHold is an amount the processor withholds as a
	// rolling reserve.  It is not lost; it should be released later.
	// Direction: always debit — reduces expected bank deposit.
	PayoutComponentReserveHold PayoutComponentType = "reserve_hold"

	// PayoutComponentReserveRelease is a prior reserve amount the processor
	// pays out in this cycle.
	// Direction: always credit — increases expected bank deposit.
	PayoutComponentReserveRelease PayoutComponentType = "reserve_release"

	// PayoutComponentAdjustment is a one-off positive or negative adjustment
	// by the processor (dispute win, correction, fee reversal, etc.).
	// Direction: either debit or credit — operator specifies.
	PayoutComponentAdjustment PayoutComponentType = "adjustment"
)

// AllPayoutComponentTypes returns the supported component types in Batch 19.
func AllPayoutComponentTypes() []PayoutComponentType {
	return []PayoutComponentType{
		PayoutComponentFee,
		PayoutComponentReserveHold,
		PayoutComponentReserveRelease,
		PayoutComponentAdjustment,
	}
}

// PayoutComponentTypeLabel returns a human-readable label for UI display.
func PayoutComponentTypeLabel(t PayoutComponentType) string {
	switch t {
	case PayoutComponentFee:
		return "Fee"
	case PayoutComponentReserveHold:
		return "Reserve Hold"
	case PayoutComponentReserveRelease:
		return "Reserve Release"
	case PayoutComponentAdjustment:
		return "Adjustment"
	default:
		return string(t)
	}
}

// PayoutComponentDirection defines whether a component increases or decreases
// the expected bank deposit relative to GatewayPayout.NetAmount.
type PayoutComponentDirection string

const (
	// PayoutComponentDebit reduces the expected bank deposit.
	// Amount is subtracted from ExpectedNet.
	PayoutComponentDebit PayoutComponentDirection = "debit"

	// PayoutComponentCredit increases the expected bank deposit.
	// Amount is added to ExpectedNet.
	PayoutComponentCredit PayoutComponentDirection = "credit"
)

// GatewayPayoutComponent records one element in the composition of a payout's
// expected bank deposit.  Multiple components per payout are allowed.
//
// Company isolation: all components must share CompanyID with their parent payout.
// Payout isolation: GatewayPayoutID scopes each component to its payout.
// Modification guard: components may only be changed on unmatched payouts.
type GatewayPayoutComponent struct {
	ID uint `gorm:"primaryKey"`

	// CompanyID mirrors the parent payout's company for isolation queries.
	CompanyID uint `gorm:"not null;index;uniqueIndex:uq_gw_payout_component_exact"`

	// GatewayPayoutID is the parent payout.
	GatewayPayoutID uint `gorm:"not null;index;uniqueIndex:uq_gw_payout_component_exact"`

	// ComponentType categorises the nature of this composition element.
	ComponentType PayoutComponentType `gorm:"type:text;not null;uniqueIndex:uq_gw_payout_component_exact"`

	// Direction determines whether this component adds to or subtracts from
	// the expected bank deposit.
	//   debit  → ExpectedNet decreases by Amount
	//   credit → ExpectedNet increases by Amount
	Direction PayoutComponentDirection `gorm:"type:text;not null;uniqueIndex:uq_gw_payout_component_exact"`

	// Amount is the absolute value of this component (must be positive).
	// The Direction field carries the sign semantics.
	Amount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0;uniqueIndex:uq_gw_payout_component_exact"`

	// Description is an optional free-text note (processor reference, memo, etc.).
	Description string `gorm:"type:text;not null;default:'';uniqueIndex:uq_gw_payout_component_exact"`

	CreatedAt time.Time
}
