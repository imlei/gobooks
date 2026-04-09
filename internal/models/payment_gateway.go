// 遵循project_guide.md
package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

// ── Provider type ────────────────────────────────────────────────────────────

type PaymentProviderType string

const (
	ProviderStripe  PaymentProviderType = "stripe"
	ProviderPayPal  PaymentProviderType = "paypal"
	ProviderManual  PaymentProviderType = "manual"
	ProviderOther   PaymentProviderType = "other"
)

func AllPaymentProviderTypes() []PaymentProviderType {
	return []PaymentProviderType{ProviderStripe, ProviderPayPal, ProviderManual, ProviderOther}
}

func PaymentProviderLabel(t PaymentProviderType) string {
	switch t {
	case ProviderStripe:
		return "Stripe"
	case ProviderPayPal:
		return "PayPal"
	case ProviderManual:
		return "Manual"
	case ProviderOther:
		return "Other"
	default:
		return string(t)
	}
}

// ── Payment request status ───────────────────────────────────────────────────

type PaymentRequestStatus string

const (
	PaymentRequestDraft              PaymentRequestStatus = "draft"
	// PaymentRequestCreated is kept for backward compatibility with older rows
	// created before initial request status was unified to pending.
	PaymentRequestCreated            PaymentRequestStatus = "created"
	PaymentRequestPending            PaymentRequestStatus = "pending"
	PaymentRequestPaid               PaymentRequestStatus = "paid"
	PaymentRequestFailed             PaymentRequestStatus = "failed"
	PaymentRequestCancelled          PaymentRequestStatus = "cancelled"
	PaymentRequestRefunded           PaymentRequestStatus = "refunded"
	PaymentRequestPartiallyRefunded  PaymentRequestStatus = "partially_refunded"
)

func AllPaymentRequestStatuses() []PaymentRequestStatus {
	return []PaymentRequestStatus{
		PaymentRequestDraft, PaymentRequestCreated, PaymentRequestPending,
		PaymentRequestPaid, PaymentRequestFailed, PaymentRequestCancelled,
		PaymentRequestRefunded, PaymentRequestPartiallyRefunded,
	}
}

// ── Transaction type ─────────────────────────────────────────────────────────

type PaymentTransactionType string

const (
	TxnTypeCharge     PaymentTransactionType = "charge"
	TxnTypeCapture    PaymentTransactionType = "capture"
	TxnTypeRefund     PaymentTransactionType = "refund"
	TxnTypeFee        PaymentTransactionType = "fee"
	TxnTypePayout     PaymentTransactionType = "payout"
	TxnTypeDispute    PaymentTransactionType = "dispute"     // non-financial; kept for traceability
	TxnTypeChargeback PaymentTransactionType = "chargeback" // forcible reversal by card network
)

func AllPaymentTransactionTypes() []PaymentTransactionType {
	return []PaymentTransactionType{
		TxnTypeCharge, TxnTypeCapture, TxnTypeRefund,
		TxnTypeFee, TxnTypePayout, TxnTypeDispute, TxnTypeChargeback,
	}
}

func PaymentTransactionTypeLabel(t PaymentTransactionType) string {
	switch t {
	case TxnTypeCharge:
		return "Charge"
	case TxnTypeCapture:
		return "Capture"
	case TxnTypeRefund:
		return "Refund"
	case TxnTypeFee:
		return "Fee"
	case TxnTypePayout:
		return "Payout"
	case TxnTypeDispute:
		return "Dispute"
	case TxnTypeChargeback:
		return "Chargeback"
	default:
		return string(t)
	}
}

// ── Payment gateway account ──────────────────────────────────────────────────

// PaymentGatewayAccount represents a company's account with a payment processor
// (e.g. a Stripe account, PayPal merchant account). No credentials are stored
// here; actual API keys/tokens are deferred to a future secure vault layer.
type PaymentGatewayAccount struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	ProviderType       PaymentProviderType `gorm:"type:text;not null"`
	DisplayName        string              `gorm:"type:text;not null;default:''"`
	ExternalAccountRef string              `gorm:"type:text;not null;default:''"`
	AuthStatus         string              `gorm:"type:text;not null;default:'pending'"`
	WebhookStatus      string              `gorm:"type:text;not null;default:'not_configured'"`
	IsActive           bool                `gorm:"not null;default:true"`

	// WebhookSecret is the provider-provided endpoint signing secret used to verify
	// incoming webhook payloads. For Stripe this is the "whsec_..." endpoint secret
	// from the Stripe dashboard. Empty means webhook verification is not configured.
	WebhookSecret string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── Payment accounting mapping ───────────────────────────────────────────────

// PaymentAccountingMapping defines which GL accounts to use when posting
// gateway-originated transactions (charges, fees, refunds, payouts).
// One mapping per gateway account. All account FKs must be company-scoped.
//
// Typical gateway flow:
//   customer pays  → gateway clearing increases (Dr GW Clearing, Cr Revenue/AR)
//   gateway fee    → gateway clearing decreases (Dr Fee Expense, Cr GW Clearing)
//   payout to bank → gateway clearing decreases (Dr Bank, Cr GW Clearing)
type PaymentAccountingMapping struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	GatewayAccountID    uint                   `gorm:"not null;uniqueIndex:uq_payment_acct_mapping"`
	GatewayAccount      PaymentGatewayAccount  `gorm:"foreignKey:GatewayAccountID"`

	ClearingAccountID    *uint    `gorm:"index"`
	ClearingAccount      *Account `gorm:"foreignKey:ClearingAccountID"`
	FeeExpenseAccountID  *uint
	FeeExpenseAccount    *Account `gorm:"foreignKey:FeeExpenseAccountID"`
	RefundAccountID      *uint
	RefundAccount        *Account `gorm:"foreignKey:RefundAccountID"`
	PayoutBankAccountID  *uint
	PayoutBankAccount    *Account `gorm:"foreignKey:PayoutBankAccountID"`
	// ChargebackAccountID is the GL account debited when a chargeback is posted.
	// Typically an expense or receivable account representing a forced loss.
	// Required when posting TxnTypeChargeback.
	ChargebackAccountID  *uint
	ChargebackAccount    *Account `gorm:"foreignKey:ChargebackAccountID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── Payment request ──────────────────────────────────────────────────────────

// PaymentRequest represents a request for payment from a customer, linked to
// a gateway account and optionally to an invoice. In the future, a provider
// adapter creates a real checkout session from this request.
type PaymentRequest struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	GatewayAccountID uint                  `gorm:"not null;index"`
	GatewayAccount   PaymentGatewayAccount `gorm:"foreignKey:GatewayAccountID"`

	InvoiceID  *uint `gorm:"index"`
	CustomerID *uint `gorm:"index"`

	Amount       decimal.Decimal      `gorm:"type:numeric(18,2);not null;default:0"`
	CurrencyCode string               `gorm:"type:text;not null;default:''"`
	Status       PaymentRequestStatus `gorm:"type:text;not null;default:'pending'"`
	Description  string               `gorm:"type:text;not null;default:''"`
	ExternalRef  string               `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── Payment transaction ──────────────────────────────────────────────────────

// PaymentTransaction records a single event from the payment gateway (charge,
// refund, fee, payout, dispute). In the current phase these are entered manually;
// future provider webhooks will write directly to this table.
type PaymentTransaction struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	GatewayAccountID   uint                  `gorm:"not null;index"`
	GatewayAccount     PaymentGatewayAccount `gorm:"foreignKey:GatewayAccountID"`
	PaymentRequestID   *uint                 `gorm:"index"`

	TransactionType    PaymentTransactionType `gorm:"type:text;not null"`
	Amount             decimal.Decimal        `gorm:"type:numeric(18,2);not null;default:0"`
	CurrencyCode       string                 `gorm:"type:text;not null;default:''"`
	Status             string                 `gorm:"type:text;not null;default:'completed'"`
	ExternalTxnRef     string                 `gorm:"type:text;not null;default:''"`
	RawPayload         datatypes.JSON         `gorm:"not null"`

	// OriginalTransactionID links a refund or chargeback back to the original
	// charge/capture it reverses. Nil for charge/capture/fee/payout rows.
	OriginalTransactionID *uint `gorm:"index"`

	// Posting state: non-nil means the transaction has been posted to a JE.
	PostedJournalEntryID *uint      `gorm:"index"`
	PostedAt             *time.Time

	// Application state: non-nil means the transaction has been applied to an invoice.
	// charge/capture → reduces BalanceDue; refund/chargeback → restores BalanceDue.
	AppliedInvoiceID *uint            `gorm:"index"`
	AppliedAt        *time.Time
	// AppliedAmount is the portion of Amount actually applied to the invoice.
	// For normal payments this equals Amount; for overpayments it equals the invoice
	// BalanceDue at the time of application (the excess became a CustomerCredit).
	// Nil for rows created before Batch 16 — callers fall back to Amount in that case.
	AppliedAmount *decimal.Decimal `gorm:"type:numeric(18,2)"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── Payment allocation ────────────────────────────────────────────────────────

// PaymentAllocation records one invoice's share of a multi-invoice payment
// allocation.  One row is created per (payment_transaction_id, invoice_id) pair;
// the unique index prevents duplicate allocations to the same invoice.
//
// Multi-allocation path vs. single-invoice path:
//   - Single-invoice (pre-Batch-17): sets PaymentTransaction.AppliedInvoiceID directly.
//     No PaymentAllocation rows are created.
//   - Multi-invoice (Batch 17+): creates one or more PaymentAllocation rows.
//     PaymentTransaction.AppliedInvoiceID remains nil.
//   The two paths are mutually exclusive for a given transaction.
//
// Remaining allocatable amount = txn.Amount − Σ PaymentAllocation.AllocatedAmount.
// Additional allocations can be submitted as long as remaining > 0.
type PaymentAllocation struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// PaymentTransactionID must be a charge or capture transaction.
	PaymentTransactionID uint               `gorm:"not null;index;uniqueIndex:uq_payment_alloc"`
	PaymentTransaction   PaymentTransaction `gorm:"foreignKey:PaymentTransactionID"`

	// InvoiceID is the invoice whose BalanceDue is reduced by AllocatedAmount.
	InvoiceID uint    `gorm:"not null;index;uniqueIndex:uq_payment_alloc"`
	Invoice   Invoice `gorm:"foreignKey:InvoiceID"`

	// AllocatedAmount is the portion of the payment applied to this invoice (> 0).
	AllocatedAmount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
