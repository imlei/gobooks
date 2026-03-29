// 遵循产品需求 v1.0
package pages

import "strconv"

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

	// Email / SMTP
	EmailEnabled           bool
	SMTPHost               string
	SMTPPort               string // string for round-trip form safety
	SMTPUsername           string
	SMTPPasswordMaskedHint string // display-only; never plaintext
	SMTPFromEmail          string
	SMTPFromName           string
	SMTPEncryption         string // "none" | "ssl_tls" | "starttls"

	// SMS
	SMSEnabled             bool
	SMSProvider            string
	SMSAPIKeyMaskedHint    string // display-only; never plaintext
	SMSAPISecretMaskedHint string // display-only; never plaintext
	SMSSenderID            string

	AllowSystemFallback bool

	// System policy: reflects system_notification_settings.allow_company_override
	SystemAllowsOverride bool
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

	// Email / SMTP
	EmailEnabled           bool
	SMTPHost               string
	SMTPPort               string
	SMTPUsername           string
	SMTPPasswordMaskedHint string
	SMTPFromEmail          string
	SMTPFromName           string
	SMTPEncryption         string

	// SMS
	SMSEnabled             bool
	SMSProvider            string
	SMSAPIKeyMaskedHint    string
	SMSAPISecretMaskedHint string
	SMSSenderID            string

	AllowCompanyOverride bool
}

// intStr converts an int to string for form display (port numbers etc.).
func intStr(n int) string {
	return strconv.Itoa(n)
}
