// 遵循产品需求 v1.0
package pages

import "gobooks/internal/models"

// ProductServicesVM is the view-model for the Products & Services page.
type ProductServicesVM struct {
	HasCompany bool

	// Dropdown data
	RevenueAccounts []models.Account  // accounts where root_account_type = 'revenue'
	TaxCodes        []models.TaxCode  // active tax codes for this company

	// Form fields
	Name               string
	Type               string
	Description        string
	DefaultPrice       string
	RevenueAccountID   string
	DefaultTaxCodeID   string

	// Field-level errors
	NameError             string
	TypeError             string
	DefaultPriceError     string
	RevenueAccountIDError string

	// Form-level error
	FormError string

	// Success banners
	Created    bool
	Updated    bool
	InactiveOK bool

	// DrawerMode is "create", "edit", or empty (drawer closed on first paint).
	DrawerMode string
	// EditingID is set when DrawerMode == "edit".
	EditingID uint

	// DrawerOpen opens the slide-over when true.
	DrawerOpen bool

	// Data to render the table
	Items []models.ProductService
}
