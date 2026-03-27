// 遵循产品需求 v1.0
package services

import (
	"time"

	"gobooks/internal/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TrialBalanceRow is one line in a Trial Balance report.
type TrialBalanceRow struct {
	Code  string
	Name  string
	Type  models.AccountType
	Debit decimal.Decimal
	Credit decimal.Decimal
}

// TrialBalance returns balances per account for a date range (inclusive).
//
// We calculate totals per account:
// - debit_sum
// - credit_sum
//
// Then we present a single balance as either a debit or credit based on sign:
// - net = debit_sum - credit_sum
// - if net >= 0 => Debit = net, Credit = 0
// - if net <  0 => Debit = 0,  Credit = -net
func TrialBalance(db *gorm.DB, fromDate, toDate time.Time) ([]TrialBalanceRow, decimal.Decimal, decimal.Decimal, error) {
	type row struct {
		Code   string
		Name   string
		Type   models.AccountType
		Debit  decimal.Decimal
		Credit decimal.Decimal
	}

	var sums []row
	err := db.Raw(
		`
SELECT
  a.code AS code,
  a.name AS name,
  a.type AS type,
  COALESCE(SUM(jl.debit), 0)  AS debit,
  COALESCE(SUM(jl.credit), 0) AS credit
FROM accounts a
LEFT JOIN journal_lines jl ON jl.account_id = a.id
LEFT JOIN journal_entries je ON je.id = jl.journal_entry_id
  AND je.entry_date >= ? AND je.entry_date < ?
GROUP BY a.code, a.name, a.type
ORDER BY a.code ASC
`,
		fromDate,
		toDate.AddDate(0, 0, 1), // < (toDate + 1 day) to make the range inclusive
	).Scan(&sums).Error
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, err
	}

	var out []TrialBalanceRow
	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, s := range sums {
		net := s.Debit.Sub(s.Credit)
		r := TrialBalanceRow{Code: s.Code, Name: s.Name, Type: s.Type, Debit: decimal.Zero, Credit: decimal.Zero}
		if net.GreaterThanOrEqual(decimal.Zero) {
			r.Debit = net
			totalDebits = totalDebits.Add(net)
		} else {
			r.Credit = net.Neg()
			totalCredits = totalCredits.Add(net.Neg())
		}
		out = append(out, r)
	}

	return out, totalDebits, totalCredits, nil
}

// IncomeStatementLine is one line item in Income Statement sections.
type IncomeStatementLine struct {
	Code   string
	Name   string
	Amount decimal.Decimal
}

type IncomeStatement struct {
	FromDate time.Time
	ToDate   time.Time

	Revenue      []IncomeStatementLine
	CostOfSales  []IncomeStatementLine
	Expenses     []IncomeStatementLine

	TotalRevenue     decimal.Decimal
	TotalCostOfSales decimal.Decimal
	TotalExpenses    decimal.Decimal

	GrossProfit decimal.Decimal
	NetIncome   decimal.Decimal
}

// IncomeStatement builds a simple income statement for a date range.
func IncomeStatementReport(db *gorm.DB, fromDate, toDate time.Time) (IncomeStatement, error) {
	report := IncomeStatement{FromDate: fromDate, ToDate: toDate}

	type row struct {
		Code   string
		Name   string
		Type   models.AccountType
		Debit  decimal.Decimal
		Credit decimal.Decimal
	}
	var sums []row

	err := db.Raw(
		`
SELECT
  a.code AS code,
  a.name AS name,
  a.type AS type,
  COALESCE(SUM(jl.debit), 0)  AS debit,
  COALESCE(SUM(jl.credit), 0) AS credit
FROM accounts a
LEFT JOIN journal_lines jl ON jl.account_id = a.id
LEFT JOIN journal_entries je ON je.id = jl.journal_entry_id
  AND je.entry_date >= ? AND je.entry_date < ?
WHERE a.type IN (
  'Income', 'Other Income', 'Expense', 'Other Expense', 'Cost of Goods Sold',
  'revenue', 'expense', 'cost_of_sales'
)
GROUP BY a.code, a.name, a.type
ORDER BY a.code ASC
`,
		fromDate,
		toDate.AddDate(0, 0, 1),
	).Scan(&sums).Error
	if err != nil {
		return IncomeStatement{}, err
	}

	for _, s := range sums {
		switch s.Type {
		case models.AccountTypeIncome, models.AccountTypeOtherIncome, models.AccountTypeRevenue:
			amt := s.Credit.Sub(s.Debit)
			if !amt.IsZero() {
				report.Revenue = append(report.Revenue, IncomeStatementLine{Code: s.Code, Name: s.Name, Amount: amt})
			}
			report.TotalRevenue = report.TotalRevenue.Add(amt)
		case models.AccountTypeCostOfGoodsSold, models.AccountTypeCostOfSales:
			amt := s.Debit.Sub(s.Credit)
			if !amt.IsZero() {
				report.CostOfSales = append(report.CostOfSales, IncomeStatementLine{Code: s.Code, Name: s.Name, Amount: amt})
			}
			report.TotalCostOfSales = report.TotalCostOfSales.Add(amt)
		case models.AccountTypeExpenseDetail, models.AccountTypeOtherExpense, models.AccountTypeExpense:
			amt := s.Debit.Sub(s.Credit)
			if !amt.IsZero() {
				report.Expenses = append(report.Expenses, IncomeStatementLine{Code: s.Code, Name: s.Name, Amount: amt})
			}
			report.TotalExpenses = report.TotalExpenses.Add(amt)
		}
	}

	report.GrossProfit = report.TotalRevenue.Sub(report.TotalCostOfSales)
	report.NetIncome = report.GrossProfit.Sub(report.TotalExpenses)

	return report, nil
}

type BalanceSheetLine struct {
	Code   string
	Name   string
	Amount decimal.Decimal
}

type BalanceSheet struct {
	AsOf time.Time

	Assets     []BalanceSheetLine
	Liabilities []BalanceSheetLine
	Equity     []BalanceSheetLine

	TotalAssets      decimal.Decimal
	TotalLiabilities decimal.Decimal
	TotalEquity      decimal.Decimal
}

// BalanceSheet builds a simple balance sheet as-of a date (inclusive).
func BalanceSheetReport(db *gorm.DB, asOf time.Time) (BalanceSheet, error) {
	report := BalanceSheet{AsOf: asOf}

	type row struct {
		Code   string
		Name   string
		Type   models.AccountType
		Debit  decimal.Decimal
		Credit decimal.Decimal
	}
	var sums []row

	err := db.Raw(
		`
SELECT
  a.code AS code,
  a.name AS name,
  a.type AS type,
  COALESCE(SUM(jl.debit), 0)  AS debit,
  COALESCE(SUM(jl.credit), 0) AS credit
FROM accounts a
LEFT JOIN journal_lines jl ON jl.account_id = a.id
LEFT JOIN journal_entries je ON je.id = jl.journal_entry_id
  AND je.entry_date < ?
WHERE a.type IN (
  'Bank', 'Accounts Receivable', 'Other Current Asset', 'Fixed Asset', 'Other Asset',
  'Accounts Payable', 'Credit Card', 'Other Current Liability', 'Long Term Liability',
  'Equity',
  'asset', 'liability', 'equity'
)
GROUP BY a.code, a.name, a.type
ORDER BY a.code ASC
`,
		asOf.AddDate(0, 0, 1), // inclusive as-of
	).Scan(&sums).Error
	if err != nil {
		return BalanceSheet{}, err
	}

	for _, s := range sums {
		switch s.Type {
		// Asset group: debit increases, credit decreases.
		case models.AccountTypeBank,
			models.AccountTypeAccountsReceivable,
			models.AccountTypeOtherCurrentAsset,
			models.AccountTypeFixedAsset,
			models.AccountTypeOtherAsset,
			models.AccountTypeAsset:
			amt := s.Debit.Sub(s.Credit)
			if !amt.IsZero() {
				report.Assets = append(report.Assets, BalanceSheetLine{Code: s.Code, Name: s.Name, Amount: amt})
			}
			report.TotalAssets = report.TotalAssets.Add(amt)
		// Liability group: credit increases, debit decreases.
		case models.AccountTypeAccountsPayable,
			models.AccountTypeCreditCard,
			models.AccountTypeOtherCurrentLiability,
			models.AccountTypeLongTermLiability,
			models.AccountTypeLiability:
			amt := s.Credit.Sub(s.Debit)
			if !amt.IsZero() {
				report.Liabilities = append(report.Liabilities, BalanceSheetLine{Code: s.Code, Name: s.Name, Amount: amt})
			}
			report.TotalLiabilities = report.TotalLiabilities.Add(amt)
		// Equity group: treat like liability side for sign consistency.
		case models.AccountTypeEquityDetail, models.AccountTypeEquity:
			amt := s.Credit.Sub(s.Debit)
			if !amt.IsZero() {
				report.Equity = append(report.Equity, BalanceSheetLine{Code: s.Code, Name: s.Name, Amount: amt})
			}
			report.TotalEquity = report.TotalEquity.Add(amt)
		}
	}

	return report, nil
}

