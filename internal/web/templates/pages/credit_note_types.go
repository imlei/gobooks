// 遵循project_guide.md
package pages

import "gobooks/internal/models"

// CreditNoteFormVM is the view-model for the /credit-notes/new form.
type CreditNoteFormVM struct {
	HasCompany bool
	CompanyID  uint
	CustomerID uint
	InvoiceID  uint
	Customers  []models.Customer
	Accounts   []models.Account // revenue + cost_of_sales accounts
	TaxCodes   []models.TaxCode
	FormError  string
	Reasons    []models.CreditNoteReason
}
