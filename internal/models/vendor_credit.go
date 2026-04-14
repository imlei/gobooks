// 遵循project_guide.md
package models

// vendor_credit.go — Vendor credit balance from bill overpayments.
//
// A VendorCredit represents a prepayment the company has made to a vendor
// that exceeds what was owed on a specific bill. The excess is held as an
// asset (DR Vendor Prepayments) until it is applied to a future bill.
//
// Lifecycle:
//   active    — credit has remaining amount > 0; can be applied to bills
//   exhausted — remaining_amount = 0; no further applications allowed

import (
	"time"

	"github.com/shopspring/decimal"
)

// VendorCreditStatus tracks whether the credit still has usable balance.
type VendorCreditStatus string

const (
	VendorCreditActive    VendorCreditStatus = "active"
	VendorCreditExhausted VendorCreditStatus = "exhausted"
)

// VendorCredit records a prepayment balance held on behalf of a vendor,
// arising when a bill payment exceeds the bill's balance due.
type VendorCredit struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	VendorID  uint `gorm:"not null;index"`

	// SourceJournalEntryID is the pay-bills JE that created this credit.
	SourceJournalEntryID uint `gorm:"not null;index"`

	// SourceBillID is the specific bill that was overpaid (nullable: future
	// sources may not be tied to a single bill).
	SourceBillID *uint `gorm:"index"`

	// OriginalAmount is immutable after creation (document currency).
	OriginalAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// RemainingAmount decrements on each application to a future bill.
	// Must never go below 0.
	RemainingAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// CurrencyCode matches the bill currency. Empty string = company base currency.
	CurrencyCode string `gorm:"type:varchar(3);not null;default:''"`

	Status VendorCreditStatus `gorm:"type:text;not null;default:'active'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
