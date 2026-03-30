// 遵循project_guide.md
package pages

import "gobooks/internal/models"

// JournalEntryVM provides data for the Journal Entry page.
type JournalEntryVM struct {
	HasCompany      bool
	ActiveCompanyID uint // scopes client-side recent-account localStorage

	// Dropdown data
	Accounts         []models.Account
	AccountsDataJSON string // script-safe JSON for account combobox (id, code, name, class)
	Customers        []models.Customer
	Vendors          []models.Vendor

	// UI messages
	FormError string
	Saved     bool
}
