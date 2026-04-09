// 遵循project_guide.md
package models

// customer_credit.go — Batch 16: Customer credit balance.
//
// A CustomerCredit represents money owed by the company back to the customer,
// arising from an overpayment.  It is NOT revenue and NOT a simple AR credit;
// it is a company obligation to the customer that can be applied to future invoices.
//
// Source types (SourceType):
//   overpayment — customer paid more than the invoice balance due; the excess
//                  is held as credit.  No further sources in this batch.
//
// Lifecycle:
//   active    — credit has remaining amount > 0; can be applied to invoices
//   exhausted — remaining_amount = 0; no further applications allowed
//
// Accounting note (Batch 16):
//   Overpayment credit creation does NOT generate a new JE.  The original
//   charge/capture posting already credited AR by the full payment amount
//   (Dr GW Clearing, Cr AR).  When the invoice is capped at its BalanceDue,
//   the excess AR credit is "unallocated" — the CustomerCredit record tracks
//   that unallocated position.  Credit application to a future invoice reduces
//   that invoice's BalanceDue (no new JE required) and decrements
//   RemainingAmount; the AR account remains in balance across the full chain.
//
// Idempotency:
//   unique index on (company_id, source_payment_txn_id, source_application_inv_id)
//   prevents duplicate credits from the same overpayment event.

import (
	"time"

	"github.com/shopspring/decimal"
)

// CustomerCreditSourceType describes how the credit originated.
type CustomerCreditSourceType string

const (
	// CreditSourceOverpayment arises when a payment amount exceeds the invoice balance due.
	CreditSourceOverpayment CustomerCreditSourceType = "overpayment"
)

// CustomerCreditStatus tracks whether the credit still has usable balance.
type CustomerCreditStatus string

const (
	CustomerCreditActive    CustomerCreditStatus = "active"
	CustomerCreditExhausted CustomerCreditStatus = "exhausted"
)

// CustomerCredit records a credit balance owed by the company to a specific customer.
// It is company-scoped and customer-scoped; cross-customer or cross-company use is forbidden.
type CustomerCredit struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	CustomerID uint `gorm:"not null;index"`

	// SourceType describes the business event that created the credit.
	SourceType CustomerCreditSourceType `gorm:"type:text;not null;default:'overpayment'"`

	// SourcePaymentTxnID is the charge/capture PaymentTransaction that caused the
	// overpayment.  Together with SourceApplicationInvID it forms the idempotency
	// key preventing duplicate credit creation.
	SourcePaymentTxnID *uint `gorm:"index;uniqueIndex:uq_customer_credit_source"`

	// SourceApplicationInvID is the invoice that was partially consumed by the
	// overpayment (i.e. the invoice whose BalanceDue was applied against).
	SourceApplicationInvID *uint `gorm:"index;uniqueIndex:uq_customer_credit_source"`

	// OriginalAmount is immutable after creation.
	OriginalAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// RemainingAmount decrements on each CustomerCreditApplication.
	// Must never go below 0.
	RemainingAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// CurrencyCode matches the invoice and payment currency.
	// Empty string means company base currency.
	// Cross-currency credit application is not supported in this batch.
	CurrencyCode string `gorm:"type:varchar(3);not null;default:''"`

	Status CustomerCreditStatus `gorm:"type:text;not null;default:'active'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// CustomerCreditApplication records a single use of a CustomerCredit against an invoice.
// Each application is immutable once created.
type CustomerCreditApplication struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// CustomerCreditID is the credit being consumed.
	CustomerCreditID uint `gorm:"not null;index"`

	// InvoiceID is the invoice whose BalanceDue is reduced.
	InvoiceID uint `gorm:"not null;index"`

	// Amount is the portion of the credit applied (> 0; ≤ credit.RemainingAmount; ≤ invoice.BalanceDue).
	Amount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	CreatedAt time.Time
}
