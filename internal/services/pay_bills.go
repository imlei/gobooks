// 遵循产品需求 v1.0
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
	VendorID uint
	EntryDate time.Time

	BankAccountID uint
	APAccountID   uint

	Amount decimal.Decimal
	Memo   string
}

// RecordPayBills posts a 2-line journal entry:
// - Debit  Accounts Payable (liability) Amount
// - Credit Bank (asset)                 Amount
//
// This keeps the accounting logic simple:
// paying reduces A/P and reduces bank.
func RecordPayBills(tx *gorm.DB, in PayBillsInput) error {
	if in.VendorID == 0 || in.BankAccountID == 0 || in.APAccountID == 0 {
		return fmt.Errorf("missing required ids")
	}
	if in.Amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("amount must be > 0")
	}

	// Load vendor for the journal line reference text.
	var vendor models.Vendor
	if err := tx.First(&vendor, in.VendorID).Error; err != nil {
		return err
	}

	// Validate accounts exist and have expected types.
	var bank models.Account
	if err := tx.First(&bank, in.BankAccountID).Error; err != nil {
		return err
	}
	var ap models.Account
	if err := tx.First(&ap, in.APAccountID).Error; err != nil {
		return err
	}
	if bank.Type.ReportGroup() != models.AccountReportGroupAsset {
		return fmt.Errorf("bank account must be an asset")
	}
	if ap.Type.ReportGroup() != models.AccountReportGroupLiability {
		return fmt.Errorf("A/P account must be a liability")
	}

	desc := fmt.Sprintf("Pay Bills - %s", vendor.Name)
	je := models.JournalEntry{
		EntryDate: in.EntryDate,
		JournalNo: desc,
	}
	if err := tx.Create(&je).Error; err != nil {
		return err
	}

	lines := []models.JournalLine{
		{
			JournalEntryID: je.ID,
			AccountID:      in.APAccountID,
			Debit:          in.Amount,
			Credit:         decimal.Zero,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeVendor,
			PartyID:        in.VendorID,
		},
		{
			JournalEntryID: je.ID,
			AccountID:      in.BankAccountID,
			Debit:          decimal.Zero,
			Credit:         in.Amount,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeNone,
			PartyID:        0,
		},
	}

	return tx.Create(&lines).Error
}

