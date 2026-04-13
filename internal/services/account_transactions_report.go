// 遵循project_guide.md
package services

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

// AccountTransactionRow is one line in the account ledger.
type AccountTransactionRow struct {
	Date        string
	Description string // journal entry memo or journal number
	JournalNo   string
	Debit       decimal.Decimal
	Credit      decimal.Decimal
	Balance     decimal.Decimal // running credit-normal balance after this line
}

// AccountTransactionsReport is the full result for one account ledger view.
type AccountTransactionsReport struct {
	AccountID       uint
	AccountCode     string
	AccountName     string
	AccountRootType string // e.g. "liability"
	DetailType      string // e.g. "sales_tax_payable"
	StartingBalance decimal.Decimal // credit-normal balance before fromDate
	Rows            []AccountTransactionRow
	TotalDebits     decimal.Decimal
	TotalCredits    decimal.Decimal
	EndingBalance   decimal.Decimal // credit-normal balance at end of toDate
}

// BuildAccountTransactionsReport loads one account's transaction history for the
// given period. Returns an error when the account does not belong to companyID
// or does not exist.
//
// Balance convention: credit-normal for liability/equity/revenue accounts;
// debit-normal for asset/expense/cost_of_sales accounts. The sign is positive
// when the balance is in the account's normal direction, negative when abnormal.
func BuildAccountTransactionsReport(
	db *gorm.DB,
	companyID, accountID uint,
	fromDate, toDate time.Time,
) (*AccountTransactionsReport, error) {
	// ── 1. Load account ───────────────────────────────────────────────────────
	type accountRow struct {
		ID              uint
		Code            string
		Name            string
		RootAccountType string
		DetailAccountType string
	}
	var acc accountRow
	if err := db.Raw(`
		SELECT id, code, name, root_account_type, detail_account_type
		FROM accounts
		WHERE id = ? AND company_id = ?
		LIMIT 1
	`, accountID, companyID).Scan(&acc).Error; err != nil {
		return nil, err
	}
	if acc.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	// ── 2. Starting balance (all posted JEs before fromDate) ──────────────────
	type sumRow struct {
		TotalDebit  decimal.Decimal
		TotalCredit decimal.Decimal
	}
	var prePeriod sumRow
	if err := db.Raw(`
		SELECT COALESCE(SUM(jl.debit),  0) AS total_debit,
		       COALESCE(SUM(jl.credit), 0) AS total_credit
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE jl.account_id = ?
		  AND je.company_id = ?
		  AND je.status = 'posted'
		  AND je.entry_date < ?
	`, accountID, companyID, fromDate).Scan(&prePeriod).Error; err != nil {
		return nil, err
	}
	startingBalance := signedBalance(acc.RootAccountType, prePeriod.TotalDebit, prePeriod.TotalCredit)

	// ── 3. Period lines ───────────────────────────────────────────────────────
	type lineRow struct {
		EntryDate time.Time // scanned as time.Time; formatted to "2006-01-02" below
		JournalNo string
		Memo      string
		Debit     decimal.Decimal
		Credit    decimal.Decimal
	}
	var lines []lineRow
	if err := db.Raw(`
		SELECT je.entry_date,
		       je.journal_no,
		       jl.memo,
		       jl.debit,
		       jl.credit
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE jl.account_id = ?
		  AND je.company_id = ?
		  AND je.status = 'posted'
		  AND je.entry_date >= ?
		  AND je.entry_date <= ?
		ORDER BY je.entry_date ASC, je.id ASC, jl.id ASC
	`, accountID, companyID, fromDate, toDate).Scan(&lines).Error; err != nil {
		return nil, err
	}

	// ── 4. Build rows with running balance ────────────────────────────────────
	rows := make([]AccountTransactionRow, 0, len(lines))
	runningBalance := startingBalance
	var totalDebits, totalCredits decimal.Decimal

	for _, l := range lines {
		delta := signedBalance(acc.RootAccountType, l.Debit, l.Credit)
		runningBalance = runningBalance.Add(delta)
		totalDebits = totalDebits.Add(l.Debit)
		totalCredits = totalCredits.Add(l.Credit)

		desc := l.Memo
		if desc == "" {
			desc = l.JournalNo
		}
		rows = append(rows, AccountTransactionRow{
			Date:        l.EntryDate.Format("2006-01-02"),
			Description: desc,
			JournalNo:   l.JournalNo,
			Debit:       l.Debit,
			Credit:      l.Credit,
			Balance:     runningBalance,
		})
	}

	return &AccountTransactionsReport{
		AccountID:       acc.ID,
		AccountCode:     acc.Code,
		AccountName:     acc.Name,
		AccountRootType: acc.RootAccountType,
		DetailType:      acc.DetailAccountType,
		StartingBalance: startingBalance,
		Rows:            rows,
		TotalDebits:     totalDebits,
		TotalCredits:    totalCredits,
		EndingBalance:   runningBalance,
	}, nil
}

// signedBalance converts raw debit/credit sums to a signed balance following
// the account's normal balance convention. Delegates to normalBalance in reports.go.
func signedBalance(rootType string, debit, credit decimal.Decimal) decimal.Decimal {
	return normalBalance(models.RootAccountType(rootType), debit, credit)
}
