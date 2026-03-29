// 遵循产品需求 v1.0
package services

import (
	"fmt"
	"time"

	"gobooks/internal/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ReceivePaymentInput is the minimal data needed to record a customer payment.
type ReceivePaymentInput struct {
	CompanyID  uint
	CustomerID uint
	EntryDate  time.Time

	BankAccountID uint
	ARAccountID   uint

	// InvoiceID optionally links this payment to a specific posted invoice.
	// When set, the invoice's Amount must match the payment Amount,
	// and the invoice will be marked as paid within the same transaction.
	InvoiceID *uint

	Amount decimal.Decimal
	Memo   string
}

// RecordReceivePayment posts a 2-line journal entry:
// - Debit  Bank (asset)         Amount
// - Credit Accounts Receivable  Amount
//
// This keeps the accounting logic simple and consistent:
// receiving money increases bank and reduces A/R.
// Returns the new journal entry id.
func RecordReceivePayment(tx *gorm.DB, in ReceivePaymentInput) (uint, error) {
	if in.CompanyID == 0 {
		return 0, fmt.Errorf("company is required")
	}
	if in.CustomerID == 0 || in.BankAccountID == 0 || in.ARAccountID == 0 {
		return 0, fmt.Errorf("missing required ids")
	}
	if in.Amount.LessThanOrEqual(decimal.Zero) {
		return 0, fmt.Errorf("amount must be > 0")
	}

	// Load customer for the journal line reference text (tenant-scoped).
	var cust models.Customer
	if err := tx.Where("id = ? AND company_id = ?", in.CustomerID, in.CompanyID).First(&cust).Error; err != nil {
		return 0, err
	}

	// Basic account validation: both accounts must exist and be assets for MVP.
	var bank models.Account
	if err := tx.Where("id = ? AND company_id = ?", in.BankAccountID, in.CompanyID).First(&bank).Error; err != nil {
		return 0, err
	}
	var ar models.Account
	if err := tx.Where("id = ? AND company_id = ?", in.ARAccountID, in.CompanyID).First(&ar).Error; err != nil {
		return 0, err
	}
	if bank.ReportGroup() != models.AccountReportGroupAsset {
		return 0, fmt.Errorf("bank account must be an asset")
	}
	if ar.ReportGroup() != models.AccountReportGroupAsset {
		return 0, fmt.Errorf("A/R account must be an asset")
	}
	if cust.CompanyID != bank.CompanyID || cust.CompanyID != ar.CompanyID || cust.CompanyID != in.CompanyID {
		return 0, fmt.Errorf("customer and accounts must belong to the same company")
	}

	companyID := in.CompanyID
	desc := fmt.Sprintf("Receive Payment - %s", cust.Name)

	je := models.JournalEntry{
		CompanyID: companyID,
		EntryDate: in.EntryDate,
		JournalNo: desc,
	}
	if err := tx.Create(&je).Error; err != nil {
		return 0, err
	}

	lines := []models.JournalLine{
		{
			CompanyID:      companyID,
			JournalEntryID: je.ID,
			AccountID:      in.BankAccountID,
			Debit:          in.Amount,
			Credit:         decimal.Zero,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeNone,
			PartyID:        0,
		},
		{
			CompanyID:      companyID,
			JournalEntryID: je.ID,
			AccountID:      in.ARAccountID,
			Debit:          decimal.Zero,
			Credit:         in.Amount,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeCustomer,
			PartyID:        in.CustomerID,
		},
	}

	if err := tx.Create(&lines).Error; err != nil {
		return 0, err
	}

	// If linked to an invoice, validate amount and mark it paid.
	if in.InvoiceID != nil && *in.InvoiceID != 0 {
		var inv models.Invoice
		if err := tx.Where("id = ? AND company_id = ?", *in.InvoiceID, in.CompanyID).First(&inv).Error; err != nil {
			return 0, fmt.Errorf("linked invoice not found")
		}
		if inv.CustomerID != in.CustomerID {
			return 0, fmt.Errorf("invoice does not belong to the selected customer")
		}
		if inv.Status != models.InvoiceStatusSent {
			return 0, fmt.Errorf("invoice is not open for payment (status: %s)", inv.Status)
		}
		if !inv.Amount.Equal(in.Amount) {
			return 0, fmt.Errorf(
				"linked invoice payments currently support full settlement only: payment amount (%s) must equal invoice total (%s); leave the invoice blank to record a partial or unapplied receipt",
				in.Amount.StringFixed(2), inv.Amount.StringFixed(2),
			)
		}
		if err := tx.Model(&inv).Updates(map[string]any{
			"status": models.InvoiceStatusPaid,
		}).Error; err != nil {
			return 0, err
		}
	}

	return je.ID, nil
}
