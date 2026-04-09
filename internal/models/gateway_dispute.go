// 遵循project_guide.md
package models

// gateway_dispute.go — Dispute state tracking (Batch 15).
//
// ─── Conceptual position ─────────────────────────────────────────────────────
//
//	customer paid   →  GatewaySettlement (clearing truth)
//	dispute opened  →  GatewayDispute (status: dispute_opened)
//	dispute won     →  GatewayDispute (status: dispute_won)  — no financial effect
//	dispute lost    →  GatewayDispute (status: dispute_lost)
//	                   + PaymentTransaction (type: chargeback) created atomically
//	                     └→ operator must post + apply-chargeback to restore invoice
//
// ─── What this model does NOT do ─────────────────────────────────────────────
//   - Does not generate JEs directly.  That is the chargeback PaymentTransaction's job.
//   - Does not track provider evidence, document uploads, or case notes.
//   - Does not handle the "pre-arbitration" stage. dispute_lost is terminal.
//
// ─── Idempotency anchor ───────────────────────────────────────────────────────
// Unique index on (company_id, gateway_account_id, provider_dispute_id) prevents
// duplicate dispute records for the same external dispute event.
//
// ─── State machine ────────────────────────────────────────────────────────────
//
//	dispute_opened → dispute_won   (no chargeback created)
//	dispute_opened → dispute_lost  (chargeback PaymentTransaction created atomically)
//	Any other transition is rejected.

import (
	"time"

	"github.com/shopspring/decimal"
)

// GatewayDisputeStatus is the lifecycle status of a gateway dispute.
type GatewayDisputeStatus string

const (
	// DisputeStatusOpened is the initial state when a dispute is registered.
	DisputeStatusOpened GatewayDisputeStatus = "dispute_opened"
	// DisputeStatusWon means the dispute was decided in the company's favour.
	// No financial reversal; the original payment stands.
	DisputeStatusWon GatewayDisputeStatus = "dispute_won"
	// DisputeStatusLost means the dispute was decided in the cardholder's favour.
	// A chargeback PaymentTransaction is created atomically at this transition.
	DisputeStatusLost GatewayDisputeStatus = "dispute_lost"
)

// GatewayDispute tracks the lifecycle of a payment dispute raised by a cardholder.
// Created when a dispute is opened; status transitions to won or lost by operator action.
type GatewayDispute struct {
	ID uint `gorm:"primaryKey"`

	// CompanyID for isolation. Never 0.
	CompanyID uint `gorm:"not null;index:idx_gw_dispute_company;uniqueIndex:uq_gw_dispute_provider"`

	// GatewayAccountID identifies which gateway account this dispute belongs to.
	GatewayAccountID uint `gorm:"not null;index;uniqueIndex:uq_gw_dispute_provider"`

	// ProviderDisputeID is the processor-assigned dispute reference (e.g. Stripe dp_xxx).
	// Unique per (company_id, gateway_account_id).
	ProviderDisputeID string `gorm:"type:text;not null;uniqueIndex:uq_gw_dispute_provider"`

	// PaymentTransactionID is the original charge/capture that is being disputed.
	// Must exist, must belong to the same company.
	PaymentTransactionID uint `gorm:"not null;index"`

	// GatewaySettlementID is optionally set if the payment was already gateway-settled
	// (clearing bridge exists). Nil if the dispute precedes settlement.
	GatewaySettlementID *uint `gorm:"index"`

	// Amount is the disputed amount (may be ≤ original charge for partial disputes).
	Amount       decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	CurrencyCode string          `gorm:"type:text;not null;default:''"`

	// Status tracks the dispute lifecycle.
	Status GatewayDisputeStatus `gorm:"type:text;not null"`

	// OpenedAt is when the dispute was raised by the cardholder/processor.
	OpenedAt time.Time `gorm:"not null"`

	// ResolvedAt is set when the dispute transitions to won or lost.
	ResolvedAt *time.Time

	// ChargebackTransactionID is set when the dispute is lost and a chargeback
	// PaymentTransaction is created. Nil until then.
	ChargebackTransactionID *uint `gorm:"index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (GatewayDispute) TableName() string { return "gateway_disputes" }
