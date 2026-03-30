// 遵循project_guide.md
package pages

// CompanySecuritySettingsVM is the view-model for Settings > Company > Security.
type CompanySecuritySettingsVM struct {
	HasCompany bool
	Breadcrumb []SettingsBreadcrumbPart
	ReadOnly   bool

	Saved     bool
	FormError string

	UnusualIPLoginAlertEnabled bool
	UnusualIPLoginAlertChannel string // "email" | "sms" | "both"
	NewDeviceLoginAlertEnabled bool
	PasswordResetAlertEnabled  bool
	FailedLoginAlertEnabled    bool

	// System policy: if false, company cannot override system defaults.
	CompanyOverrideAllowed bool
}

// SystemSecuritySettingsVM is the view-model for SysAdmin > Settings > Security.
type SystemSecuritySettingsVM struct {
	AdminEmail      string
	MaintenanceMode bool
	Flash           string
	FormError       string

	UnusualIPLoginAlertDefaultEnabled    bool
	UnusualIPLoginCompanyOverrideAllowed bool
	NewDeviceLoginAlertDefaultEnabled    bool
	PasswordResetAlertDefaultEnabled     bool
	FailedLoginAlertDefaultEnabled       bool
}
