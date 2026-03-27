// 遵循产品需求 v1.0
package pages

// CompanySettingsVM is used by the company settings page.
type CompanySettingsVM struct {
	HasCompany bool
	Values     SetupFormValues
	Errors     SetupFormErrors
	Saved      bool
}

