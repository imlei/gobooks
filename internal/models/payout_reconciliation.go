// 遵循project_guide.md
package models

// payout_reconciliation.go — Batch 18: Payout ↔ bank entry reconciliation.
//
// Two new models:
//
//   BankEntry — a manually-entered record of a bank-side credit (deposit).
//     The operator enters the entry after seeing it on their bank statement.
//     This is NOT a full bank statement import; it is the minimum anchor needed
//     to perform 1:1 payout matching without a bank data feed.
//
//   PayoutReconciliation — the 1:1 match record linking a GatewayPayout to
//     a BankEntry.  Both sides carry unique indexes so neither can be matched
//     more than once.
//
// ─── Accounting note ──────────────────────────────────────────────────────────
//   Matching does NOT create or modify any Journal Entry.
//   The GatewayPayout JE (Dr Bank, Cr Clearing) was posted at payout creation.
//   Reconciliation only asserts "this payout corresponds to this bank credit."
//
// ─── Status derivation ────────────────────────────────────────────────────────
//   GatewayPayout.IsMatched is NOT a stored field; callers JOIN or sub-query
//   PayoutReconciliation by gateway_payout_id to determine matched state.
//   BankEntry.IsMatched is similarly derived from PayoutReconciliation.
//
// ─── Future scope (not in Batch 18) ──────────────────────────────────────────
//   - Bank statement CSV import
//   - Multi-payout ↔ single bank batch matching
//   - Partial matching / tolerance
//   - Bank fee auto-split

import (
	"time"

	"github.com/shopspring/decimal"
)

// BankEntry represents a single manually-entered bank-side credit (deposit)
// as seen on the company's bank statement.  It is company-scoped and
// bank-account-scoped.  One BankEntry can be matched to at most one
// GatewayPayout via PayoutReconciliation.
type BankEntry struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// BankAccountID is the GL asset (bank) account the deposit was received into.
	BankAccountID uint    `gorm:"not null;index"`
	BankAccount   Account `gorm:"foreignKey:BankAccountID"`

	// EntryDate is the date shown on the bank statement for this deposit.
	EntryDate time.Time `gorm:"not null"`

	// Amount is the gross deposited amount (must be positive).
	Amount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// CurrencyCode is the currency of the deposit.
	// Empty string means company base currency.
	CurrencyCode string `gorm:"type:varchar(3);not null;default:''"`

	// Description is a free-text reference (bank memo, wire reference, etc.).
	Description string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// PayoutReconciliation is the 1:1 match record linking a GatewayPayout
// to a BankEntry.
//
// Unique indexes:
//   - uq_payout_recon_payout:    one GatewayPayout can be matched at most once
//   - uq_payout_recon_bank_entry: one BankEntry can be matched at most once
//
// These two constraints together guarantee strict 1:1 semantics and prevent
// both concurrent over-matching and duplicate submit replays.
type PayoutReconciliation struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// GatewayPayoutID is the payout being reconciled.
	// Unique: one payout → at most one reconciliation record.
	GatewayPayoutID uint          `gorm:"not null;uniqueIndex:uq_payout_recon_payout"`
	GatewayPayout   GatewayPayout `gorm:"foreignKey:GatewayPayoutID"`

	// BankEntryID is the bank deposit being matched.
	// Unique: one bank entry → at most one reconciliation record.
	BankEntryID uint      `gorm:"not null;uniqueIndex:uq_payout_recon_bank_entry"`
	BankEntry   BankEntry `gorm:"foreignKey:BankEntryID"`

	// MatchedAt is the wall-clock time the match was submitted.
	MatchedAt time.Time `gorm:"not null"`

	// Actor is the email/identifier of the user who performed the match.
	Actor string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
}
