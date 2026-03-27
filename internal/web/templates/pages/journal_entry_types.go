// 遵循产品需求 v1.0
package pages

import "gobooks/internal/models"

// JournalEntryVM provides data for the Journal Entry page.
type JournalEntryVM struct {
	HasCompany bool

	// Dropdown data
	Accounts   []models.Account
	Customers  []models.Customer
	Vendors    []models.Vendor

	// UI messages
	FormError string
	Saved     bool
}

