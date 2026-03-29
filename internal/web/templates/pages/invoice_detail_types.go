// 遵循产品需求 v1.0
package pages

import "gobooks/internal/models"

// InvoiceDetailVM is the view-model for the read-only invoice detail page.
type InvoiceDetailVM struct {
	HasCompany bool

	// Invoice is fully preloaded:
	//   Invoice.Customer
	//   Invoice.Lines (sorted by sort_order)
	//   Invoice.Lines[i].ProductService
	//   Invoice.Lines[i].TaxCode
	//   Invoice.JournalEntry (nil if not yet posted)
	Invoice models.Invoice

	// JournalNo is the human-readable journal entry number (e.g. "INV-IN001").
	// Empty when the invoice has not been posted.
	JournalNo string

	// Banner flags set via query string on redirect.
	JustVoided bool   // ?voided=1
	VoidError  string // ?voiderror=...
}
