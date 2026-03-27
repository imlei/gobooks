// 遵循产品需求 v1.0
package services

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ReconcileCandidate is one unreconciled journal line for an account.
type ReconcileCandidate struct {
	LineID    uint
	EntryDate time.Time
	JournalNo string
	Memo      string
	Debit       decimal.Decimal
	Credit      decimal.Decimal

	// Amount is a convenience: for the bank account (asset),
	// amount = debit - credit.
	Amount decimal.Decimal
}

// ListReconcileCandidates returns unreconciled lines for a given account
// up to (and including) statementDate. companyID must match the account's company.
func ListReconcileCandidates(db *gorm.DB, companyID, accountID uint, statementDate time.Time) ([]ReconcileCandidate, error) {
	var out []ReconcileCandidate
	err := db.Raw(
		`
SELECT
  jl.id AS line_id,
  je.entry_date AS entry_date,
  je.journal_no AS journal_no,
  jl.memo AS memo,
  jl.debit AS debit,
  jl.credit AS credit,
  (jl.debit - jl.credit) AS amount
FROM journal_lines jl
JOIN journal_entries je ON je.id = jl.journal_entry_id
WHERE jl.account_id = ?
  AND jl.company_id = ?
  AND je.company_id = ?
  AND je.entry_date <= ?
  AND jl.reconciliation_id IS NULL
ORDER BY je.entry_date ASC, jl.id ASC
`,
		accountID, companyID, companyID, statementDate,
	).Scan(&out).Error
	return out, err
}

// ClearedBalance returns the sum of (debit - credit) for lines
// already reconciled for the account up to statementDate. companyID must match the account's company.
func ClearedBalance(db *gorm.DB, companyID, accountID uint, statementDate time.Time) (decimal.Decimal, error) {
	type row struct {
		Amount decimal.Decimal
	}
	var r row
	err := db.Raw(
		`
SELECT COALESCE(SUM(jl.debit - jl.credit), 0) AS amount
FROM journal_lines jl
JOIN journal_entries je ON je.id = jl.journal_entry_id
WHERE jl.account_id = ?
  AND jl.company_id = ?
  AND je.company_id = ?
  AND je.entry_date <= ?
  AND jl.reconciliation_id IS NOT NULL
`,
		accountID, companyID, companyID, statementDate,
	).Scan(&r).Error
	return r.Amount, err
}

