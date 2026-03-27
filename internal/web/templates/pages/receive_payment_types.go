// 遵循产品需求 v1.0
package pages

import "gobooks/internal/models"

type ReceivePaymentVM struct {
	HasCompany bool

	Customers []models.Customer
	Accounts  []models.Account

	// Form values (for re-render)
	CustomerID string
	EntryDate  string
	BankAccountID string
	ARAccountID string
	Amount string
	Memo   string

	// Errors
	FormError string
	CustomerError string
	DateError string
	BankError string
	ARError string
	AmountError string

	Saved bool
}

