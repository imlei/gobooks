// 遵循project_guide.md
package pages

import "gobooks/internal/models"

type ExpenseListVM struct {
	HasCompany bool

	FormError string
	Created   bool
	Updated   bool

	CanCreate bool
	CanUpdate bool

	Expenses []models.Expense
}

type ExpenseFormVM struct {
	HasCompany bool
	IsEdit     bool
	EditingID  uint

	ExpenseDate         string
	Description         string
	Amount              string
	CurrencyCode        string
	VendorID            string
	VendorLabel         string // human-readable label for SmartPicker rehydration; never a raw DB ID
	ExpenseAccountID    string
	ExpenseAccountLabel string // human-readable label for SmartPicker rehydration; never a raw DB ID
	TaskID              string
	IsBillable          bool
	Notes               string

	// Payment settlement fields (all optional).
	PaymentAccountID    string
	PaymentAccountLabel string // human-readable label for SmartPicker rehydration
	PaymentMethod       string
	PaymentReference    string

	ExpenseDateError      string
	DescriptionError      string
	AmountError           string
	CurrencyError         string
	VendorError           string
	ExpenseAccountError   string
	TaskError             string
	BillableCustomerError string
	PaymentAccountError   string
	PaymentMethodError    string
	FormError             string

	BaseCurrencyCode string
	MultiCurrency    bool
	CurrencyOptions  []string
	SelectableTasks  []models.Task
}
