// 遵循产品需求 v1.0
package ui

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
	case "Company Settings", "Numbering Settings", "AI Connect Settings", "Audit Log":
		return "settings"
	default:
		return ""
	}
}

// BoolStr returns "true" or "false" for HTML data attributes.
func BoolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
