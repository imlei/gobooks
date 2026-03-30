// 遵循project_guide.md
package pages

import "strconv"

// NotifChannelStatusVM carries the delivery readiness state for one channel
// (email or SMS). All time strings are pre-formatted by the handler; empty
// string means the field has never been set.
type NotifChannelStatusVM struct {
	// TestStatus is "never", "success", or "failed".
	TestStatus string

	LastTestedAt  string // formatted UTC time or ""
	LastTestedBy  string
	LastSuccessAt string
	LastFailureAt string
	LastError     string

	// VerificationReady is true only when: enabled, config complete, last test
	// succeeded, and config has not changed since that test. This value comes
	// directly from the DB row — it is never derived on the frontend.
	VerificationReady bool
}

// CompanyNotificationSettingsVM is the view-model for Settings > Company > Notifications.
type CompanyNotificationSettingsVM struct {
	HasCompany bool
	Breadcrumb []SettingsBreadcrumbPart
	ReadOnly   bool

	Saved     bool
	FormError string

	// Test-send results (empty string = not yet attempted this load).
	TestEmailResult string // "ok" | "err" | ""
	TestEmailMsg    string
	TestSMSResult   string // "ok" | "err" | ""
	TestSMSMsg      string

	// Email / SMTP config
	EmailEnabled           bool
	SMTPHost               string
	SMTPPort               string // string for round-trip form safety
	SMTPUsername           string
	SMTPPasswordMaskedHint string // display-only; never plaintext
	SMTPFromEmail          string
	SMTPFromName           string
	SMTPEncryption         string // "none" | "ssl_tls" | "starttls"

	// SMS config
	SMSEnabled             bool
	SMSProvider            string
	SMSAPIKeyMaskedHint    string // display-only; never plaintext
	SMSAPISecretMaskedHint string // display-only; never plaintext
	SMSSenderID            string

	AllowSystemFallback bool

	// System policy: reflects system_notification_settings.allow_company_override
	SystemAllowsOverride bool

	// Delivery readiness state (from DB, not derived on frontend).
	EmailStatus NotifChannelStatusVM
	SMSStatus   NotifChannelStatusVM
}

// SystemNotificationSettingsVM is the view-model for SysAdmin > Settings > Notifications.
type SystemNotificationSettingsVM struct {
	AdminEmail      string
	MaintenanceMode bool
	Flash           string
	FormError       string

	TestEmailResult string
	TestEmailMsg    string
	TestSMSResult   string
	TestSMSMsg      string

	// Email / SMTP config
	EmailEnabled           bool
	SMTPHost               string
	SMTPPort               string
	SMTPUsername           string
	SMTPPasswordMaskedHint string
	SMTPFromEmail          string
	SMTPFromName           string
	SMTPEncryption         string

	// SMS config
	SMSEnabled             bool
	SMSProvider            string
	SMSAPIKeyMaskedHint    string
	SMSAPISecretMaskedHint string
	SMSSenderID            string

	AllowCompanyOverride bool

	// Delivery readiness state.
	EmailStatus NotifChannelStatusVM
	SMSStatus   NotifChannelStatusVM
}

// intStr converts an int to string for form display (port numbers etc.).
func intStr(n int) string {
	return strconv.Itoa(n)
}
