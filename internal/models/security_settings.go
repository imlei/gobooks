// 遵循project_guide.md
package models

import "time"

// AlertChannel is the delivery channel for security alert notifications.
type AlertChannel string

const (
	AlertChannelEmail AlertChannel = "email"
	AlertChannelSMS   AlertChannel = "sms"
	AlertChannelBoth  AlertChannel = "both"
)

// CompanySecuritySettings holds per-company security alert preferences.
// One row per company. Defaults mirror the system-level defaults at creation time.
type CompanySecuritySettings struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"uniqueIndex;not null"`

	UnusualIPLoginAlertEnabled  bool         `gorm:"not null;default:true"`
	UnusualIPLoginAlertChannel  AlertChannel `gorm:"type:text;not null;default:'email'"`
	NewDeviceLoginAlertEnabled  bool         `gorm:"not null;default:true"`
	PasswordResetAlertEnabled   bool         `gorm:"not null;default:true"`
	FailedLoginAlertEnabled     bool         `gorm:"not null;default:true"`
	FutureRulesJSON             *string      `gorm:"type:jsonb"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SystemSecuritySettings holds system-wide security alert defaults and override policy.
// This is a singleton table (only one row); the application layer enforces uniqueness.
type SystemSecuritySettings struct {
	ID uint `gorm:"primaryKey"`

	UnusualIPLoginAlertDefaultEnabled    bool    `gorm:"not null;default:true"`
	UnusualIPLoginCompanyOverrideAllowed bool    `gorm:"not null;default:true"`
	NewDeviceLoginAlertDefaultEnabled    bool    `gorm:"not null;default:true"`
	PasswordResetAlertDefaultEnabled     bool    `gorm:"not null;default:true"`
	FailedLoginAlertDefaultEnabled       bool    `gorm:"not null;default:true"`
	GlobalSecurityRulesJSON              *string `gorm:"type:jsonb"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SecurityEvent is an append-only record of notable authentication and security events.
// user_id is stored as text to accommodate both regular users (UUID) and sysadmin users
// without coupling this table to a specific user table via a foreign key.
// company_id is nullable: system-level events (e.g. sysadmin logins) have no company.
type SecurityEvent struct {
	ID           uint    `gorm:"primaryKey"`
	CompanyID    *uint   `gorm:"index"`
	UserID       *string `gorm:"type:text;index"`
	EventType    string  `gorm:"type:text;not null;index"`
	IPAddress    string  `gorm:"type:text"`
	UserAgent    string  `gorm:"type:text"`
	MetadataJSON *string `gorm:"type:jsonb"`
	CreatedAt    time.Time
}
