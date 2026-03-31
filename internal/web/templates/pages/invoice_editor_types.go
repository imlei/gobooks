// 遵循project_guide.md
package pages

import "gobooks/internal/models"

// InvoiceEditorVM is the view-model for the invoice create/edit editor page.
type InvoiceEditorVM struct {
	HasCompany bool
	// IsEdit is true when editing an existing draft; false for new invoices.
	IsEdit    bool
	EditingID uint
	// ReviewLocked is true after a draft save when the editor re-opens in
	// review mode.
	ReviewLocked bool
	// SubmitPath is used by the locked-state Submit button.
	SubmitPath string

	// Header fields (form values).
	InvoiceNumber string
	CustomerID    string
	InvoiceDate   string
	Terms         string
	DueDate       string
	Memo          string

	// Header errors.
	InvoiceNumberError string
	CustomerError      string
	DateError          string
	LinesError         string
	FormError          string

	// Dropdown data.
	Customers []models.Customer
	// Products contains only active ProductServices for this company.
	// Serialised to ProductsJSON for Alpine.
	Products []models.ProductService
	// TaxCodes contains only active TaxCodes for this company.
	// Serialised to TaxCodesJSON for Alpine.
	TaxCodes []models.TaxCode

	// Alpine initialisation JSON (set by handler, consumed by invoice_editor.js).
	ProductsJSON     string
	TaxCodesJSON     string
	InitialLinesJSON string

	// Line rows — used when re-rendering after a validation error.
	Lines []InvoiceLineFormRow

	// Computed totals shown after server recalculation.
	Subtotal string
	TaxTotal string
	Total    string

	Saved bool
}

// InvoiceLineFormRow carries one line's form values (and optional error) for
// re-rendering after a validation failure or after a successful save.
type InvoiceLineFormRow struct {
	ProductServiceID string
	Description      string
	Qty              string
	UnitPrice        string
	TaxCodeID        string
	// Computed by server after save (shown read-only on re-render).
	LineNet   string
	LineTax   string
	LineTotal string
	Error     string
}

// InvoiceEditorTitle returns the page / drawer title.
func InvoiceEditorTitle(vm InvoiceEditorVM) string {
	if vm.IsEdit {
		return "Edit Invoice"
	}
	return "New Invoice"
}
