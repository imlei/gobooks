// 遵循产品需求 v1.0
package models

import "time"

// SMTPEncryption defines the transport-security mode for outbound SMTP.
type SMTPEncryption string

const (
	SMTPEncryptionNone     SMTPEncryption = "none"
	SMTPEncryptionSSLTLS   SMTPEncryption = "ssl_tls"
	SMTPEncryptionSTARTTLS SMTPEncryption = "starttls"
)

// CompanyNotificationSettings holds per-company SMTP and SMS configuration.
// Encrypted fields (SMTPPasswordEncrypted, SMSAPIKeyEncrypted, SMSAPISecretEncrypted)
// are never returned in plaintext to the browser; use the *MaskedHint fields for display.
type CompanyNotificationSettings struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"uniqueIndex;not null"`

	// Email / SMTP
	EmailEnabled           bool           `gorm:"not null;default:false"`
	SMTPHost               string         `gorm:"type:text;not null;default:''"`
	SMTPPort               int            `gorm:"not null;default:587"`
	SMTPUsername           string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordEncrypted  string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordMaskedHint string         `gorm:"type:text;not null;default:''"`
	SMTPFromEmail          string         `gorm:"type:text;not null;default:''"`
	SMTPFromName           string         `gorm:"type:text;not null;default:''"`
	SMTPEncryption         SMTPEncryption `gorm:"type:text;not null;default:'starttls'"`

	// SMS
	SMSEnabled             bool   `gorm:"not null;default:false"`
	SMSProvider            string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyEncrypted     string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyMaskedHint    string `gorm:"type:text;not null;default:''"`
	SMSAPISecretEncrypted  string `gorm:"type:text;not null;default:''"`
	SMSAPISecretMaskedHint string `gorm:"type:text;not null;default:''"`
	SMSSenderID            string `gorm:"type:text;not null;default:''"`

	AllowSystemFallback bool `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SystemNotificationSettings holds the system-wide SMTP and SMS configuration.
// This is a singleton table (only one row); the application layer enforces uniqueness.
// AllowCompanyOverride controls whether companies may supply their own credentials.
type SystemNotificationSettings struct {
	ID uint `gorm:"primaryKey"`

	// Email / SMTP
	EmailEnabled           bool           `gorm:"not null;default:false"`
	SMTPHost               string         `gorm:"type:text;not null;default:''"`
	SMTPPort               int            `gorm:"not null;default:587"`
	SMTPUsername           string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordEncrypted  string         `gorm:"type:text;not null;default:''"`
	SMTPPasswordMaskedHint string         `gorm:"type:text;not null;default:''"`
	SMTPFromEmail          string         `gorm:"type:text;not null;default:''"`
	SMTPFromName           string         `gorm:"type:text;not null;default:''"`
	SMTPEncryption         SMTPEncryption `gorm:"type:text;not null;default:'starttls'"`

	// SMS
	SMSEnabled             bool   `gorm:"not null;default:false"`
	SMSProvider            string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyEncrypted     string `gorm:"type:text;not null;default:''"`
	SMSAPIKeyMaskedHint    string `gorm:"type:text;not null;default:''"`
	SMSAPISecretEncrypted  string `gorm:"type:text;not null;default:''"`
	SMSAPISecretMaskedHint string `gorm:"type:text;not null;default:''"`
	SMSSenderID            string `gorm:"type:text;not null;default:''"`

	AllowCompanyOverride bool `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
