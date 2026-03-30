// 遵循project_guide.md
package pages

import "gobooks/internal/models"

type PayBillsVM struct {
	HasCompany bool

	Vendors   []models.Vendor
	Accounts  []models.Account
	OpenBills []models.Bill

	// Form values
	VendorID      string
	BillID        string
	EntryDate     string
	BankAccountID string
	APAccountID   string
	Amount        string
	Memo          string

	// Errors
	FormError   string
	VendorError string
	BillError   string
	DateError   string
	BankError   string
	APError     string
	AmountError string

	Saved bool
}
