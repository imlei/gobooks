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
	CustomerID uint
	EntryDate  time.Time

	BankAccountID uint
	ARAccountID   uint

	Amount decimal.Decimal
	Memo   string
}

// RecordReceivePayment posts a 2-line journal entry:
// - Debit  Bank (asset)         Amount
// - Credit Accounts Receivable  Amount
//
// This keeps the accounting logic simple and consistent:
// receiving money increases bank and reduces A/R.
func RecordReceivePayment(tx *gorm.DB, in ReceivePaymentInput) error {
	if in.CustomerID == 0 || in.BankAccountID == 0 || in.ARAccountID == 0 {
		return fmt.Errorf("missing required ids")
	}
	if in.Amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("amount must be > 0")
	}

	// Load customer for the journal line reference text.
	var cust models.Customer
	if err := tx.First(&cust, in.CustomerID).Error; err != nil {
		return err
	}

	// Basic account validation: both accounts must exist and be assets for MVP.
	var bank models.Account
	if err := tx.First(&bank, in.BankAccountID).Error; err != nil {
		return err
	}
	var ar models.Account
	if err := tx.First(&ar, in.ARAccountID).Error; err != nil {
		return err
	}
	if bank.Type.ReportGroup() != models.AccountReportGroupAsset {
		return fmt.Errorf("bank account must be an asset")
	}
	if ar.Type.ReportGroup() != models.AccountReportGroupAsset {
		return fmt.Errorf("A/R account must be an asset")
	}

	desc := fmt.Sprintf("Receive Payment - %s", cust.Name)

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
			AccountID:      in.BankAccountID,
			Debit:          in.Amount,
			Credit:         decimal.Zero,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeNone,
			PartyID:        0,
		},
		{
			JournalEntryID: je.ID,
			AccountID:      in.ARAccountID,
			Debit:          decimal.Zero,
			Credit:         in.Amount,
			Memo:           in.Memo,
			PartyType:      models.PartyTypeCustomer,
			PartyID:        in.CustomerID,
		},
	}

	return tx.Create(&lines).Error
}

