// 遵循project_guide.md
package models

// gateway_payout.go — Payout bridge: gateway clearing → bank + fee.
//
// ─── Position in the collection chain ────────────────────────────────────────
//
//	customer pay       →  HostedPaymentAttempt (payment-side truth)
//	gateway settlement →  GatewaySettlement    (clearing truth, Dr Clearing / Cr AR)
//	gateway payout     →  GatewayPayout        (THIS) Dr Bank / Dr Fee Expense / Cr Clearing
//
// GatewayPayout represents the moment the processor deducts its fee and
// deposits the net amount into the company's bank account. One payout may
// aggregate multiple prior GatewaySettlement clearing rows (a common pattern
// with Stripe's daily batch payouts).
//
// ─── Idempotency anchor ───────────────────────────────────────────────────────
// Unique index on (company_id, gateway_account_id, provider_payout_id) prevents
// a duplicate payout bridge for the same external payout event.
//
// ─── Join safety ─────────────────────────────────────────────────────────────
// GatewayPayoutSettlement.GatewaySettlementID carries its own unique index —
// a GatewaySettlement can appear in at most one active payout bridge. The
// constraint fires on concurrent bridging races.
//
// ─── Accounting entry ────────────────────────────────────────────────────────
//
//	Dr  Bank Account        (NetAmount)
//	Dr  Fee Expense Account (FeeAmount, only when FeeAmount > 0)
//	Cr  Gateway Clearing    (GrossAmount = NetAmount + FeeAmount)

import (
	"time"

	"github.com/shopspring/decimal"
)

// GatewayPayout is the accounting bridge between gateway clearing and bank.
// Created atomically alongside its JournalEntry and GatewayPayoutSettlement rows.
// JournalEntryID is always non-nil after successful creation (no draft state).
type GatewayPayout struct {
	ID uint `gorm:"primaryKey"`

	// CompanyID for isolation. Never 0.
	CompanyID uint `gorm:"not null;index:idx_gw_payout_company;uniqueIndex:uq_gw_payout_external"`

	// GatewayAccountID identifies which payment gateway this payout came from.
	GatewayAccountID uint `gorm:"not null;index;uniqueIndex:uq_gw_payout_external"`

	// ProviderPayoutID is the external payout identifier assigned by the processor
	// (e.g. Stripe's po_xxx). Unique per (company_id, gateway_account_id).
	ProviderPayoutID string `gorm:"type:text;not null;uniqueIndex:uq_gw_payout_external"`

	// PayoutDate is the date the processor sent funds (used as JE entry date).
	PayoutDate time.Time `gorm:"not null"`

	// CurrencyCode of all linked settlements (must be identical; stored for quick reporting).
	CurrencyCode string `gorm:"type:text;not null;default:''"`

	// GrossAmount = sum of all linked GatewaySettlement amounts.
	GrossAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// FeeAmount is the processor fee deducted from gross. May be zero.
	FeeAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// NetAmount = GrossAmount − FeeAmount. Deposited into BankAccount.
	NetAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// BankAccountID is the GL asset account that receives the net deposit.
	BankAccountID uint `gorm:"not null;index"`

	// JournalEntryID is set once the payout JE is posted. Always non-nil after creation.
	JournalEntryID *uint `gorm:"index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (GatewayPayout) TableName() string { return "gateway_payouts" }

// GatewayPayoutSettlement is the join record linking a GatewayPayout to one
// GatewaySettlement. The unique index on GatewaySettlementID enforces that a
// given clearing row can appear in at most one payout bridge.
type GatewayPayoutSettlement struct {
	ID uint `gorm:"primaryKey"`

	// CompanyID mirrors the payout/settlement company for quick isolation queries.
	CompanyID uint `gorm:"not null;index"`

	GatewayPayoutID uint `gorm:"not null;index:idx_gw_payout_settle_payout"`

	// GatewaySettlementID is the idempotency anchor — at most one payout per settlement.
	GatewaySettlementID uint `gorm:"not null;uniqueIndex:uq_gw_payout_settle_settlement"`

	CreatedAt time.Time
}

func (GatewayPayoutSettlement) TableName() string { return "gateway_payout_settlements" }
