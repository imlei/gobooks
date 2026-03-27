// 遵循产品需求 v1.0
package pages

import "gobooks/internal/models"

type InvoicesVM struct {
	HasCompany bool

	Customers []models.Customer
	Invoices  []models.Invoice

	InvoiceNumber string
	CustomerID    string
	InvoiceDate   string
	Amount        string
	Memo          string

	InvoiceNumberError string
	CustomerError      string
	DateError          string
	AmountError        string
	FormError          string

	DuplicateWarning bool
	DuplicateMessage string

	Created bool

	FilterQ         string
	FilterCustomerID string
	FilterFrom      string
	FilterTo        string
}

type BillsVM struct {
	HasCompany bool

	Vendors []models.Vendor
	Bills   []models.Bill

	BillNumber string
	VendorID   string
	BillDate   string
	Amount     string
	Memo       string

	BillNumberError string
	VendorError     string
	DateError       string
	AmountError     string
	FormError       string

	DuplicateWarning bool
	DuplicateMessage string

	Created bool

	FilterQ       string
	FilterVendorID string
	FilterFrom    string
	FilterTo      string
}

