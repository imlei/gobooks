// 遵循project_guide.md
package pages

import "gobooks/internal/models"

// BillEditorVM is the view-model for the bill create/edit editor page.
type BillEditorVM struct {
	HasCompany bool
	// IsEdit is true when editing an existing draft; false for new bills.
	IsEdit    bool
	EditingID uint

	// Header fields (form values).
	BillNumber string
	VendorID   string
	BillDate   string
	Terms      string
	DueDate    string
	Memo       string

	// Header errors.
	BillNumberError string
	VendorError     string
	DateError       string
	LinesError      string
	FormError       string

	// Dropdown data.
	Vendors  []models.Vendor
	Accounts []models.Account
	TaxCodes []models.TaxCode

	// Alpine initialisation JSON (set by handler, consumed by bill_editor.js).
	AccountsJSON     string
	TaxCodesJSON     string
	InitialLinesJSON string

	// Line rows — used when re-rendering after a validation error.
	Lines []BillLineFormRow

	Saved bool
}

// BillLineFormRow carries one line's form values (and optional error) for
// re-rendering after a validation failure.
type BillLineFormRow struct {
	ExpenseAccountID string
	Description      string
	Amount           string
	TaxCodeID        string
	// Computed by server after save.
	LineNet   string
	LineTax   string
	LineTotal string
	Error     string
}

// BillEditorTitle returns the page title.
func BillEditorTitle(vm BillEditorVM) string {
	if vm.IsEdit {
		return "Edit Bill"
	}
	return "New Bill"
}
