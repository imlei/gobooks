// 遵循project_guide.md
package services

import (
	"fmt"
	"time"

	"gobooks/internal/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PayBillsInput is the minimal data needed to record a vendor payment.
type PayBillsInput struct {
	CompanyID uint
	VendorID  uint
	EntryDate time.Time

	BankAccountID uint
	APAccountID   uint
	BillID        *uint

	Amount decimal.Decimal
	Memo   string
}

// RecordPayBills posts a 2-line journal entry:
// - Debit  Accounts Payable (liability) Amount
// - Credit Bank (asset)                 Amount
//
// This keeps the accounting logic simple:
// paying reduces A/P and reduces bank.
//
// If BillID is provided, the selected posted bill must belong to the same vendor
// and company, the payment amount must equal the bill total, and the bill will be
// marked paid in the same transaction.
// Returns the new journal entry id.
func RecordPayBills(tx *gorm.DB, in PayBillsInput) (uint, error) {
	if in.CompanyID == 0 {
		return 0, fmt.Errorf("company is required")
	}
	if in.VendorID == 0 || in.BankAccountID == 0 || in.APAccountID == 0 {
		return 0, fmt.Errorf("missing required ids")
	}
	if in.Amount.LessThanOrEqual(decimal.Zero) {
		return 0, fmt.Errorf("amount must be > 0")
	}

	// Load vendor for the journal line reference text (tenant-scoped).
	var vendor models.Vendor
	if err := tx.Where("id = ? AND company_id = ?", in.VendorID, in.CompanyID).First(&vendor).Error; err != nil {
		return 0, err
	}

	// Validate accounts exist and have expected types.
	var bank models.Account
	if err := tx.Where("id = ? AND company_id = ?", in.BankAccountID, in.CompanyID).First(&bank).Error; err != nil {
		return 0, err
	}
	var ap models.Account
	if err := tx.Where("id = ? AND company_id = ?", in.APAccountID, in.CompanyID).First(&ap).Error; err != nil {
		return 0, err
	}
	if bank.ReportGroup() != models.AccountReportGroupAsset {
		return 0, fmt.Errorf("bank account must be an asset")
	}
	if ap.ReportGroup() != models.AccountReportGroupLiability {
		return 0, fmt.Errorf("A/P account must be a liability")
	}
	if vendor.CompanyID != bank.CompanyID || vendor.CompanyID != ap.CompanyID || vendor.CompanyID != in.CompanyID {
		return 0, fmt.Errorf("vendor and accounts must belong to the same company")
	}

	companyID := in.CompanyID
	desc := fmt.Sprintf("Pay Bills - %s", vendor.Name)
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
			AccountID:      in.APAccountID,
			Debit:          in.Amount,
			Credit:         decimal.Zero,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeVendor,
			PartyID:        in.VendorID,
		},
		{
			CompanyID:      companyID,
			JournalEntryID: je.ID,
			AccountID:      in.BankAccountID,
			Debit:          decimal.Zero,
			Credit:         in.Amount,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeNone,
			PartyID:        0,
		},
	}

	if err := tx.Create(&lines).Error; err != nil {
		return 0, err
	}

	if in.BillID != nil && *in.BillID != 0 {
		var bill models.Bill
		if err := tx.Where("id = ? AND company_id = ?", *in.BillID, in.CompanyID).First(&bill).Error; err != nil {
			return 0, fmt.Errorf("linked bill not found")
		}
		if bill.VendorID != in.VendorID {
			return 0, fmt.Errorf("bill does not belong to the selected vendor")
		}
		if bill.Status != models.BillStatusPosted {
			return 0, fmt.Errorf("bill is not open for payment (status: %s)", bill.Status)
		}
		if !bill.Amount.Equal(in.Amount) {
			return 0, fmt.Errorf("payment amount (%s) must equal bill total (%s)",
				in.Amount.StringFixed(2), bill.Amount.StringFixed(2))
		}
		if err := tx.Model(&bill).Updates(map[string]any{
			"status": models.BillStatusPaid,
		}).Error; err != nil {
			return 0, err
		}
	}
	return je.ID, nil
}
