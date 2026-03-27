// 遵循产品需求 v1.0
package web

import "gobooks/internal/web/templates/pages"

func breadcrumbSettingsCompanyHub() []pages.SettingsBreadcrumbPart {
	return []pages.SettingsBreadcrumbPart{
		{Label: "Settings", Href: "/settings/company"},
		{Label: "Company", Href: ""},
	}
}

func breadcrumbSettingsCompanyProfile() []pages.SettingsBreadcrumbPart {
	return []pages.SettingsBreadcrumbPart{
		{Label: "Settings", Href: "/settings/company"},
		{Label: "Company", Href: "/settings/company"},
		{Label: "Profile", Href: ""},
	}
}

func breadcrumbSettingsCompanyTemplates() []pages.SettingsBreadcrumbPart {
	return []pages.SettingsBreadcrumbPart{
		{Label: "Settings", Href: "/settings/company"},
		{Label: "Company", Href: "/settings/company"},
		{Label: "Templates", Href: ""},
	}
}

func breadcrumbSettingsCompanySalesTax() []pages.SettingsBreadcrumbPart {
	return []pages.SettingsBreadcrumbPart{
		{Label: "Settings", Href: "/settings/company"},
		{Label: "Company", Href: "/settings/company"},
		{Label: "Sales Tax", Href: ""},
	}
}

func breadcrumbSettingsCompanyNumbering() []pages.SettingsBreadcrumbPart {
	return []pages.SettingsBreadcrumbPart{
		{Label: "Settings", Href: "/settings/company"},
		{Label: "Company", Href: "/settings/company"},
		{Label: "Numbering", Href: ""},
	}
}

func breadcrumbSettingsAIConnect() []pages.SettingsBreadcrumbPart {
	return []pages.SettingsBreadcrumbPart{
		{Label: "Settings", Href: "/settings/company"},
		{Label: "AI Connect", Href: ""},
	}
}
