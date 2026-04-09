// 遵循project_guide.md
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// HostedPaymentAttemptSettlementStatus is the outcome of the last auto-settlement
// or manual retry for this attempt. It is distinct from the payment-side status
// (which tracks the provider-verified payment outcome) and from the invoice
// accounting status (which tracks the double-entry book state).
//
// Empty string means settlement has not been attempted yet.
type HostedPaymentAttemptSettlementStatus = string

const (
	// SettlementOutcomeApplied means the gateway settlement bridge completed
	// successfully: JE created, invoice balance zeroed, GatewaySettlement row exists.
	SettlementOutcomeApplied = "applied"

	// SettlementOutcomePendingReview means settlement was evaluated but found
	// ineligible (e.g. missing clearing account, amount mismatch). The verified
	// payment exists; the operator must fix the condition and retry.
	SettlementOutcomePendingReview = "pending_review"

	// SettlementOutcomeFailed means an unexpected execution error occurred during
	// the settlement transaction (not just an ineligibility condition).
	// Operator should investigate and retry.
	SettlementOutcomeFailed = "failed"
)

// HostedPaymentAttemptStatus tracks the lifecycle of a hosted payment attempt.
type HostedPaymentAttemptStatus string

const (
	// HostedPaymentAttemptCreated means the attempt row was created but no redirect yet.
	HostedPaymentAttemptCreated HostedPaymentAttemptStatus = "created"
	// HostedPaymentAttemptRedirected means the customer was sent to the provider checkout.
	HostedPaymentAttemptRedirected HostedPaymentAttemptStatus = "redirected"
	// HostedPaymentAttemptFailed means the attempt could not be created (provider error).
	HostedPaymentAttemptFailed HostedPaymentAttemptStatus = "failed"
	// HostedPaymentAttemptCancelled means the customer returned via the cancel URL.
	HostedPaymentAttemptCancelled HostedPaymentAttemptStatus = "cancelled"
	// HostedPaymentAttemptPaymentSucceeded means the payment provider confirmed receipt
	// of payment via a verified webhook event. This is the authoritative success state.
	HostedPaymentAttemptPaymentSucceeded HostedPaymentAttemptStatus = "payment_succeeded"
	// HostedPaymentAttemptPaymentFailed means the payment provider reported a failure
	// via a verified webhook event.
	HostedPaymentAttemptPaymentFailed HostedPaymentAttemptStatus = "payment_failed"
)

// HostedPaymentAttempt records a single pay-intent initiated from the hosted invoice page.
//
// Immutable trace design:
//   - One row per user-initiated attempt. Terminal status is set but rows are never deleted.
//   - ProviderRef is set when the provider creates a checkout session (Stripe session ID, etc.)
//   - RedirectURL is the provider-generated checkout URL the customer is sent to.
//   - This table does NOT record payment completion; completed payments are handled by the
//     existing payment application layer (PaymentReceipt / PaymentTransaction).
//
// Idempotency:
//   - A new attempt is blocked when an attempt for the same invoice_id with status
//     'created' or 'redirected' was created within the last 30 minutes.
//   - This prevents double-triggers from form re-submission or network retries.
type HostedPaymentAttempt struct {
	ID               uint `gorm:"primaryKey"`
	CompanyID        uint `gorm:"not null;index:idx_hpa_company"`
	InvoiceID        uint `gorm:"not null;index:idx_hpa_invoice"`
	HostedLinkID     uint `gorm:"not null;index:idx_hpa_link"`
	GatewayAccountID uint `gorm:"not null;index:idx_hpa_gateway"`

	ProviderType PaymentProviderType        `gorm:"type:text;not null"`
	Amount       decimal.Decimal            `gorm:"type:numeric(18,2);not null;default:0"`
	CurrencyCode string                     `gorm:"type:text;not null;default:''"`
	Status       HostedPaymentAttemptStatus `gorm:"type:text;not null;default:'created'"`

	// ProviderRef is set after the provider creates a checkout session.
	// Empty until the checkout session is successfully created.
	ProviderRef string `gorm:"type:text;not null;default:''"`

	// RedirectURL is the payment provider checkout page URL.
	// Set immediately after the provider creates the session.
	RedirectURL string `gorm:"type:text;not null;default:''"`

	// ── Settlement operability fields (Batch 12) ──────────────────────────────
	// These track the outcome of the last auto-settlement or manual retry.
	// They are updated by gateway_settlement_service and persist the result so
	// operators can see why settlement did not happen automatically.
	//
	// SettlementStatus: "" (not attempted) | "applied" | "pending_review" | "failed"
	// See SettlementOutcome* constants above.
	SettlementStatus string `gorm:"type:text;not null;default:''"`

	// SettlementReason is the human-readable reason for a non-applied outcome.
	// Empty when SettlementStatus is "" or "applied".
	SettlementReason string `gorm:"type:text;not null;default:''"`

	// SettlementLastAttemptedAt is when auto-settle or manual retry last ran.
	// Nil when settlement has never been attempted for this record.
	SettlementLastAttemptedAt *time.Time `gorm:"index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName returns the PostgreSQL table name for GORM.
func (HostedPaymentAttempt) TableName() string {
	return "hosted_payment_attempts"
}
