// 遵循project_guide.md
package pages

import "gobooks/internal/models"

// ── Quote VMs ─────────────────────────────────────────────────────────────────

// QuotesVM is the view model for the Quotes list page.
type QuotesVM struct {
	HasCompany     bool
	Quotes         []models.Quote
	Customers      []models.Customer
	FilterStatus   string
	FilterCustomer string
	Created        bool
	Saved          bool
	FormError      string
}

// QuoteDetailVM is the view model for a single Quote detail / edit page.
type QuoteDetailVM struct {
	HasCompany       bool
	Quote            models.Quote
	Customers        []models.Customer
	TaxCodes         []models.TaxCode
	ProductServices  []models.ProductService
	Accounts         []models.Account // revenue accounts
	FormError        string
	Saved            bool
	Sent             bool
	Accepted         bool
	Rejected         bool
	Converted        bool
	Cancelled        bool
}

// ── SalesOrder VMs ────────────────────────────────────────────────────────────

// SalesOrdersVM is the view model for the Sales Orders list page.
type SalesOrdersVM struct {
	HasCompany     bool
	Orders         []models.SalesOrder
	Customers      []models.Customer
	FilterStatus   string
	FilterCustomer string
	Created        bool
	Saved          bool
	FormError      string
}

// SalesOrderDetailVM is the view model for a single SalesOrder detail / edit page.
type SalesOrderDetailVM struct {
	HasCompany      bool
	Order           models.SalesOrder
	Customers       []models.Customer
	TaxCodes        []models.TaxCode
	ProductServices []models.ProductService
	Accounts        []models.Account // revenue accounts
	FormError       string
	Saved           bool
	Confirmed       bool
	Cancelled       bool
}
