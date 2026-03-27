// 遵循产品需求 v1.0
package pages

// CompanySettingsVM is used by the company profile (Settings > Company > Profile) page.
type CompanySettingsVM struct {
	HasCompany bool
	Breadcrumb []SettingsBreadcrumbPart
	Values     SetupFormValues
	Errors     SetupFormErrors
	Saved      bool
}

