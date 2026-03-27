// 遵循产品需求 v1.0
package pages

import "gobooks/internal/models"

// AccountsVM is the view-model for the Chart of Accounts page.
type AccountsVM struct {
	HasCompany bool
	Active     string

	// Form fields
	Code string
	Name string
	Type string

	// Form-level + field-level errors
	FormError string
	CodeError string
	NameError string
	TypeError string

	// Success banner
	Created bool

	// Data to render the table
	Accounts []models.Account
}

