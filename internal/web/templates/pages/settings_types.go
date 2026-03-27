// 遵循产品需求 v1.0
package pages

// SettingsBreadcrumbPart is a segment for settings sub-pages (Settings / Company / …).
type SettingsBreadcrumbPart struct {
	Label string
	Href  string // empty = current page (not a link)
}

// CompanyHubVM is the Company settings landing page.
type CompanyHubVM struct {
	HasCompany  bool
	Breadcrumb  []SettingsBreadcrumbPart
}

// CompanySubpageVM is a lightweight VM for placeholder company sub-pages (templates, sales tax).
type CompanySubpageVM struct {
	HasCompany bool
	Breadcrumb []SettingsBreadcrumbPart
}
