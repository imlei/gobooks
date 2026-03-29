// 遵循产品需求 v1.0
package ui

import "strings"

// SectionKeyForActivePage maps SidebarVM.Active to a collapsible group key.
// Used to keep the section containing the current route expanded on load.
func SectionKeyForActivePage(active string) string {
	switch active {
	case "Dashboard", "Accounts", "Journal Entry", "Invoices", "Bills", "Reports", "Setup":
		return "core"
	case "Customers", "Vendors":
		return "contacts"
	case "Bank Reconcile", "Receive Payment", "Pay Bills":
		return "banking"
	case "AI Connect Settings", "Members Settings", "Audit Log", "Products & Services":
		// "Products & Services" 已移至 Settings > Company，访问时保持 Settings 区块展开
		return "settings"
	default:
		// Any /settings/company/* page uses Active values like "Company Hub", "Company Profile", …
		if IsCompanySettingsNavActive(active) {
			return "settings"
		}
		return ""
	}
}

// IsCompanySettingsNavActive is true on Settings > Company hub and all company sub-pages.
// Active strings for those routes must start with "Company " (see layout SidebarVM on each page).
func IsCompanySettingsNavActive(active string) bool {
	return strings.HasPrefix(active, "Company ")
}

// BoolStr returns "true" or "false" for HTML data attributes.
func BoolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
