// 遵循project_guide.md
package services

import (
	"fmt"
	"time"

	"gobooks/internal/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// BillPayment is a single bill + the amount being paid against it.
type BillPayment struct {
	BillID uint
	Amount decimal.Decimal
}

// PayBillsInput is the data needed to record a vendor payment across one or more bills.
type PayBillsInput struct {
	CompanyID     uint
	EntryDate     time.Time
	BankAccountID uint
	APAccountID   uint
	Bills         []BillPayment // at least one entry required
	Memo          string
}

// RecordPayBills posts a 2-line journal entry:
//   - Debit  Accounts Payable (liability)  total amount
//   - Credit Bank (asset)                  total amount
//
// For each bill in Bills:
//   - Validates the bill is posted or partially_paid and belongs to this company.
//   - Applies the payment amount against balance_due.
//   - Sets status to partially_paid or paid depending on the remaining balance.
//
// Returns the new journal entry ID.
func RecordPayBills(tx *gorm.DB, in PayBillsInput) (uint, error) {
	if in.CompanyID == 0 {
		return 0, fmt.Errorf("company is required")
	}
	if in.BankAccountID == 0 || in.APAccountID == 0 {
		return 0, fmt.Errorf("bank and A/P accounts are required")
	}
	if len(in.Bills) == 0 {
		return 0, fmt.Errorf("at least one bill must be selected")
	}

	// Validate accounts exist and have expected types.
	var bank models.Account
	if err := tx.Where("id = ? AND company_id = ?", in.BankAccountID, in.CompanyID).First(&bank).Error; err != nil {
		return 0, fmt.Errorf("bank account not found")
	}
	var ap models.Account
	if err := tx.Where("id = ? AND company_id = ?", in.APAccountID, in.CompanyID).First(&ap).Error; err != nil {
		return 0, fmt.Errorf("A/P account not found")
	}
	if bank.ReportGroup() != models.AccountReportGroupAsset {
		return 0, fmt.Errorf("bank account must be an asset")
	}
	if ap.ReportGroup() != models.AccountReportGroupLiability {
		return 0, fmt.Errorf("A/P account must be a liability")
	}

	// Validate each bill and compute total.
	type billRecord struct {
		bill      models.Bill
		payAmount decimal.Decimal
	}
	records := make([]billRecord, 0, len(in.Bills))
	total := decimal.Zero

	for _, bp := range in.Bills {
		if bp.Amount.LessThanOrEqual(decimal.Zero) {
			return 0, fmt.Errorf("payment amount for bill %d must be > 0", bp.BillID)
		}
		var bill models.Bill
		if err := tx.Where("id = ? AND company_id = ?", bp.BillID, in.CompanyID).First(&bill).Error; err != nil {
			return 0, fmt.Errorf("bill %d not found", bp.BillID)
		}
		if bill.Status != models.BillStatusPosted && bill.Status != models.BillStatusPartiallyPaid {
			return 0, fmt.Errorf("bill %s is not open for payment (status: %s)", bill.BillNumber, bill.Status)
		}
		// Determine effective balance: use BalanceDue if positive, else fall back to Amount
		// (handles pre-migration bills where BalanceDue may be zero).
		balance := bill.BalanceDue
		if balance.LessThanOrEqual(decimal.Zero) {
			balance = bill.Amount
		}
		if bp.Amount.GreaterThan(balance) {
			return 0, fmt.Errorf("payment %s exceeds balance %s for bill %s",
				bp.Amount.StringFixed(2), balance.StringFixed(2), bill.BillNumber)
		}
		records = append(records, billRecord{bill: bill, payAmount: bp.Amount})
		total = total.Add(bp.Amount)
	}

	if total.LessThanOrEqual(decimal.Zero) {
		return 0, fmt.Errorf("total payment must be > 0")
	}

	// Create the journal entry.
	je := models.JournalEntry{
		CompanyID: in.CompanyID,
		EntryDate: in.EntryDate,
		JournalNo: "Pay Bills",
	}
	if err := tx.Create(&je).Error; err != nil {
		return 0, err
	}

	lines := []models.JournalLine{
		{
			CompanyID:      in.CompanyID,
			JournalEntryID: je.ID,
			AccountID:      in.APAccountID,
			Debit:          total,
			Credit:         decimal.Zero,
			Memo:           in.Memo,
		},
		{
			CompanyID:      in.CompanyID,
			JournalEntryID: je.ID,
			AccountID:      in.BankAccountID,
			Debit:          decimal.Zero,
			Credit:         total,
			Memo:           in.Memo,
		},
	}
	if err := tx.Create(&lines).Error; err != nil {
		return 0, err
	}

	// Update each bill's balance_due and status.
	for _, r := range records {
		balance := r.bill.BalanceDue
		if balance.LessThanOrEqual(decimal.Zero) {
			balance = r.bill.Amount
		}
		newBalance := balance.Sub(r.payAmount)
		var newStatus models.BillStatus
		if newBalance.LessThanOrEqual(decimal.Zero) {
			newStatus = models.BillStatusPaid
			newBalance = decimal.Zero
		} else {
			newStatus = models.BillStatusPartiallyPaid
		}
		if err := tx.Model(&r.bill).Updates(map[string]any{
			"balance_due": newBalance,
			"status":      newStatus,
		}).Error; err != nil {
			return 0, err
		}
	}

	return je.ID, nil
}
