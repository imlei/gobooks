// 遵循产品需求 v1.0
package pages

import "gobooks/internal/models"

type ReceivePaymentVM struct {
	HasCompany bool

	Customers []models.Customer
	Accounts  []models.Account

	// OpenInvoicesJSON is a JSON array of open (sent) invoices for Alpine.js filtering.
	// Each element: {id, customer_id, invoice_number, amount, due_date}
	OpenInvoicesJSON string

	// Form values (for re-render)
	CustomerID    string
	EntryDate     string
	BankAccountID string
	ARAccountID   string
	InvoiceID     string // optional — links payment to a specific invoice
	Amount        string
	Memo          string

	// Errors
	FormError     string
	CustomerError string
	DateError     string
	BankError     string
	ARError       string
	InvoiceError  string
	AmountError   string

	Saved bool
}

