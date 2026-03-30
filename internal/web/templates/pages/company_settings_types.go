// 遵循project_guide.md
package pages

// CompanySettingsVM is used by the company profile (Settings > Company > Profile) page.
type CompanySettingsVM struct {
	HasCompany bool
	Breadcrumb []SettingsBreadcrumbPart
	Values     SetupFormValues
	Errors     SetupFormErrors
	Saved      bool

	// LogoPath is non-empty when a logo has been uploaded for this company.
	// Used to render a preview image on the profile page.
	LogoPath string
	// LogoError is a human-readable upload validation error (type, size, etc.).
	LogoError string
}

