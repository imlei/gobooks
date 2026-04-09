// 遵循project_guide.md
package models

import (
	"time"

	"gorm.io/datatypes"
)

// WebhookEvent records each verified webhook event received from a payment provider.
//
// Purpose: deduplication (replay protection) and traceability.
//
// The unique index on external_event_id ensures each provider event is processed
// at most once even when the provider retries delivery after a temporary failure.
// A duplicate insert attempt will fail with a unique-constraint violation, which
// the ingestion layer treats as "already processed" and returns 200 to the provider.
//
// Raw payload is stored as delivered (after signature verification) for audit and
// debugging. It must not be treated as authoritative business data — normalised
// status transitions live in HostedPaymentAttempt / PaymentTransaction.
type WebhookEvent struct {
	ID               uint                `gorm:"primaryKey"`
	GatewayAccountID uint                `gorm:"not null;index:idx_webhook_event_gateway"`
	ProviderType     PaymentProviderType `gorm:"type:text;not null"`

	// ExternalEventID is the provider's event identifier (e.g. Stripe "evt_...").
	// Unique across all events to prevent double-processing on retried deliveries.
	ExternalEventID string `gorm:"type:text;not null;uniqueIndex:uq_webhook_event_ext_id"`

	// EventType is the provider's event type string (e.g. "checkout.session.completed").
	EventType string `gorm:"type:text;not null;default:''"`

	// RawPayload stores the raw event body after signature verification.
	RawPayload datatypes.JSON `gorm:"not null"`

	ProcessedAt time.Time
	CreatedAt   time.Time
}

// TableName returns the PostgreSQL table name for GORM.
func (WebhookEvent) TableName() string {
	return "webhook_events"
}
