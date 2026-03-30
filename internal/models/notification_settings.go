// 遵循project_guide.md
package models

import "time"

// SMTPEncryption defines the transport-security mode for outbound SMTP.
type SMTPEncryption string

const (
	SMTPEncryptionNone     SMTPEncryption = "none"
	SMTPEncryptionSSLTLS   SMTPEncryption = "ssl_tls"
	SMTPEncryptionSTARTTLS SMTPEncryption = "starttls"
)

// NotifTestStatus records the outcome of the most recent channel test.
type NotifTestStatus string

const (
	NotifTestStatusNever   NotifTestStatus = "never"
	NotifTestStatusSuccess NotifTestStatus = "success"
	NotifTestStatusFailed  NotifTestStatus = "failed"
)

// CompanyNotificationSettings holds per-company SMTP and SMS configuration and
// delivery readiness state. Encrypted fields are never returned in plaintext to
// the browser; use the *MaskedHint fields for display.
//
// Readiness rule (enforced by the service layer, not the frontend):
//
//	EmailVerificationReady = EmailEnabled
//	  && config is complete (host + from_email + port > 0)
//	  && EmailTestStatus == "success"
//	  && EmailConfigHash == EmailTestedConfigHash   (config unchanged since last success)
//
// Same rule applies to SMS.
type CompanyNotificationSettings struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"uniqueIndex;not null"`

	// ── Email / SMTP ───────────────────────────────────────────────────────────

	EmailEnabled           bool           `gorm:"not null;default:false"`
	SMTPHost               string         `gorm:"type:text;not null;default:''"`
	SMTPPort               int            `gorm:"not null;default:587"`
	SMTPUsername           string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordEncrypted  string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordMaskedHint string         `gorm:"type:text;not null;default:''"`
	SMTPFromEmail          string         `gorm:"type:text;not null;default:''"`
	SMTPFromName           string         `gorm:"type:text;not null;default:''"`
	SMTPEncryption         SMTPEncryption `gorm:"type:text;not null;default:'starttls'"`

	// Email delivery readiness state.
	EmailTestStatus        NotifTestStatus `gorm:"type:text;not null;default:'never'"`
	EmailLastTestedAt      *time.Time
	EmailLastTestedBy      string    `gorm:"type:text;not null;default:''"`
	EmailLastSuccessAt     *time.Time
	EmailLastFailureAt     *time.Time
	EmailLastError         string    `gorm:"type:text;not null;default:''"`
	EmailConfigHash        string    `gorm:"type:text;not null;default:''"` // SHA-256 of current config
	EmailTestedConfigHash  string    `gorm:"type:text;not null;default:''"` // hash at time of last successful test
	EmailVerificationReady bool      `gorm:"not null;default:false"`

	// ── SMS ────────────────────────────────────────────────────────────────────

	SMSEnabled             bool   `gorm:"not null;default:false"`
	SMSProvider            string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyEncrypted     string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyMaskedHint    string `gorm:"type:text;not null;default:''"`
	SMSAPISecretEncrypted  string `gorm:"type:text;not null;default:''"`
	SMSAPISecretMaskedHint string `gorm:"type:text;not null;default:''"`
	SMSSenderID            string `gorm:"type:text;not null;default:''"`

	// SMS delivery readiness state.
	SMSTestStatus        NotifTestStatus `gorm:"type:text;not null;default:'never'"`
	SMSLastTestedAt      *time.Time
	SMSLastTestedBy      string    `gorm:"type:text;not null;default:''"`
	SMSLastSuccessAt     *time.Time
	SMSLastFailureAt     *time.Time
	SMSLastError         string    `gorm:"type:text;not null;default:''"`
	SMSConfigHash        string    `gorm:"type:text;not null;default:''"`
	SMSTestedConfigHash  string    `gorm:"type:text;not null;default:''"`
	SMSVerificationReady bool      `gorm:"not null;default:false"`

	AllowSystemFallback bool `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SystemNotificationSettings holds the system-wide SMTP and SMS configuration.
// This is a singleton table (only one row); the application layer enforces uniqueness.
// AllowCompanyOverride controls whether companies may supply their own credentials.
// Readiness state fields follow the same semantics as CompanyNotificationSettings.
type SystemNotificationSettings struct {
	ID uint `gorm:"primaryKey"`

	// ── Email / SMTP ───────────────────────────────────────────────────────────

	EmailEnabled           bool           `gorm:"not null;default:false"`
	SMTPHost               string         `gorm:"type:text;not null;default:''"`
	SMTPPort               int            `gorm:"not null;default:587"`
	SMTPUsername           string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordEncrypted  string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordMaskedHint string         `gorm:"type:text;not null;default:''"`
	SMTPFromEmail          string         `gorm:"type:text;not null;default:''"`
	SMTPFromName           string         `gorm:"type:text;not null;default:''"`
	SMTPEncryption         SMTPEncryption `gorm:"type:text;not null;default:'starttls'"`

	// Email delivery readiness state.
	EmailTestStatus        NotifTestStatus `gorm:"type:text;not null;default:'never'"`
	EmailLastTestedAt      *time.Time
	EmailLastTestedBy      string    `gorm:"type:text;not null;default:''"`
	EmailLastSuccessAt     *time.Time
	EmailLastFailureAt     *time.Time
	EmailLastError         string    `gorm:"type:text;not null;default:''"`
	EmailConfigHash        string    `gorm:"type:text;not null;default:''"`
	EmailTestedConfigHash  string    `gorm:"type:text;not null;default:''"`
	EmailVerificationReady bool      `gorm:"not null;default:false"`

	// ── SMS ────────────────────────────────────────────────────────────────────

	SMSEnabled             bool   `gorm:"not null;default:false"`
	SMSProvider            string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyEncrypted     string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyMaskedHint    string `gorm:"type:text;not null;default:''"`
	SMSAPISecretEncrypted  string `gorm:"type:text;not null;default:''"`
	SMSAPISecretMaskedHint string `gorm:"type:text;not null;default:''"`
	SMSSenderID            string `gorm:"type:text;not null;default:''"`

	// SMS delivery readiness state.
	SMSTestStatus        NotifTestStatus `gorm:"type:text;not null;default:'never'"`
	SMSLastTestedAt      *time.Time
	SMSLastTestedBy      string    `gorm:"type:text;not null;default:''"`
	SMSLastSuccessAt     *time.Time
	SMSLastFailureAt     *time.Time
	SMSLastError         string    `gorm:"type:text;not null;default:''"`
	SMSConfigHash        string    `gorm:"type:text;not null;default:''"`
	SMSTestedConfigHash  string    `gorm:"type:text;not null;default:''"`
	SMSVerificationReady bool      `gorm:"not null;default:false"`

	AllowCompanyOverride bool `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
