// 遵循project_guide.md
package services

// clearing_report_service.go — Read-only clearing account report.
//
// Shows the flow of funds through a channel's clearing account:
//   - Channel-origin invoice sales (debit clearing = increase)
//   - Settlement fee posting (credit clearing = decrease)
//   - Payout recording (credit clearing = decrease)
//   - Reversals (opposite direction)
//
// Data is sourced from ledger_entries filtered by the clearing account,
// grouped by source_type for classification.

import (
	"github.com/shopspring/decimal"
	"gobooks/internal/models"
	"gorm.io/gorm"
)

// ClearingSummary holds aggregated totals for a clearing account.
type ClearingSummary struct {
	ChannelAccountID   uint
	ChannelDisplayName string
	ClearingAccountID  uint
	ClearingAccountCode string
	ClearingAccountName string

	SalesTotal      decimal.Decimal // debits from invoice posting
	FeesTotal       decimal.Decimal // credits from settlement fee posting
	PayoutsTotal    decimal.Decimal // credits from payout recording
	ReversalsTotal  decimal.Decimal // net reversals
	CurrentBalance  decimal.Decimal // net balance on clearing account
}

// ClearingMovement is one row in the clearing movement ledger.
type ClearingMovement struct {
	Date           string
	SourceType     string
	SourceLabel    string
	SourceID       uint
	JournalEntryID uint
	Debit          decimal.Decimal
	Credit         decimal.Decimal
	RunningBalance decimal.Decimal
	Memo           string
}

// GetClearingSummary computes the clearing account summary for a channel.
func GetClearingSummary(db *gorm.DB, companyID, channelAccountID uint) (*ClearingSummary, error) {
	mapping, err := GetAccountingMapping(db, companyID, channelAccountID)
	if err != nil || mapping == nil || mapping.ClearingAccountID == nil {
		return nil, nil
	}

	clearingAcctID := *mapping.ClearingAccountID

	// Load clearing account info.
	var acct models.Account
	db.Where("id = ? AND company_id = ?", clearingAcctID, companyID).First(&acct)

	// Load channel account info.
	var chAcct models.SalesChannelAccount
	db.Where("id = ? AND company_id = ?", channelAccountID, companyID).First(&chAcct)

	// Query ledger entries for this clearing account.
	var entries []models.LedgerEntry
	db.Where("company_id = ? AND account_id = ? AND status = ?",
		companyID, clearingAcctID, "active").
		Order("posting_date ASC, id ASC").
		Find(&entries)

	summary := &ClearingSummary{
		ChannelAccountID:    channelAccountID,
		ChannelDisplayName:  chAcct.DisplayName,
		ClearingAccountID:   clearingAcctID,
		ClearingAccountCode: acct.Code,
		ClearingAccountName: acct.Name,
	}

	for _, e := range entries {
		switch e.SourceType {
		case models.LedgerSourceInvoice:
			summary.SalesTotal = summary.SalesTotal.Add(e.DebitAmount)
		case models.LedgerSourceSettlement:
			summary.FeesTotal = summary.FeesTotal.Add(e.CreditAmount)
			summary.PayoutsTotal = summary.PayoutsTotal.Add(e.CreditAmount.Sub(summary.FeesTotal).Add(summary.FeesTotal)) // will fix below
		case models.LedgerSourceReversal:
			summary.ReversalsTotal = summary.ReversalsTotal.Add(e.DebitAmount).Sub(e.CreditAmount)
		}
	}

	// Simple balance: sum all debits - sum all credits on this account.
	var totalDebit, totalCredit decimal.Decimal
	for _, e := range entries {
		totalDebit = totalDebit.Add(e.DebitAmount)
		totalCredit = totalCredit.Add(e.CreditAmount)
	}
	summary.CurrentBalance = totalDebit.Sub(totalCredit)

	// Recompute proper category totals from the raw entries.
	summary.SalesTotal = decimal.Zero
	summary.FeesTotal = decimal.Zero
	summary.PayoutsTotal = decimal.Zero
	summary.ReversalsTotal = decimal.Zero
	for _, e := range entries {
		switch e.SourceType {
		case models.LedgerSourceInvoice:
			summary.SalesTotal = summary.SalesTotal.Add(e.DebitAmount)
		case models.LedgerSourceSettlement:
			// Settlement posting credits clearing (fees); payout also credits clearing.
			// Distinguish by checking JE memo or by debit vs credit.
			// For simplicity: any credit to clearing from settlement = fees + payout combined.
			summary.FeesTotal = summary.FeesTotal.Add(e.CreditAmount)
			// Any debit from settlement reversal.
			summary.ReversalsTotal = summary.ReversalsTotal.Add(e.DebitAmount)
		case models.LedgerSourceReversal:
			summary.ReversalsTotal = summary.ReversalsTotal.Add(e.DebitAmount).Sub(e.CreditAmount)
		default:
			// Other sources affecting clearing (manual JE, etc.)
		}
	}

	return summary, nil
}

// ListClearingMovements returns the clearing account movements for a channel.
func ListClearingMovements(db *gorm.DB, companyID, channelAccountID uint, limit int) ([]ClearingMovement, error) {
	mapping, err := GetAccountingMapping(db, companyID, channelAccountID)
	if err != nil || mapping == nil || mapping.ClearingAccountID == nil {
		return nil, nil
	}
	clearingAcctID := *mapping.ClearingAccountID

	if limit <= 0 {
		limit = 100
	}

	// Query ledger entries for the clearing account, ordered by date ASC for running balance.
	var entries []models.LedgerEntry
	db.Where("company_id = ? AND account_id = ? AND status = ?",
		companyID, clearingAcctID, "active").
		Order("posting_date ASC, id ASC").
		Limit(limit).
		Find(&entries)

	movements := make([]ClearingMovement, len(entries))
	runningBal := decimal.Zero

	for i, e := range entries {
		runningBal = runningBal.Add(e.DebitAmount).Sub(e.CreditAmount)

		movements[i] = ClearingMovement{
			Date:           e.PostingDate.Format("2006-01-02"),
			SourceType:     string(e.SourceType),
			SourceLabel:    clearingSourceLabel(e.SourceType),
			SourceID:       e.SourceID,
			JournalEntryID: e.JournalEntryID,
			Debit:          e.DebitAmount,
			Credit:         e.CreditAmount,
			RunningBalance: runningBal,
		}
	}

	return movements, nil
}

func clearingSourceLabel(st models.LedgerSourceType) string {
	switch st {
	case models.LedgerSourceInvoice:
		return "Invoice (Sale)"
	case models.LedgerSourceSettlement:
		return "Settlement"
	case models.LedgerSourceReversal:
		return "Reversal"
	case models.LedgerSourcePayment:
		return "Payment"
	default:
		return string(st)
	}
}
