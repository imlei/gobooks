// 遵循产品需求 v1.0
package models

import (
	"fmt"
	"time"
)

// AccountCodeMinDigits / AccountCodeMaxDigits bound chart-of-accounts codes (digits only).
const (
	AccountCodeMinDigits = 3
	AccountCodeMaxDigits = 12
)

// ValidateAccountCode returns nil if code is empty or valid (AccountCodeMinDigits..AccountCodeMaxDigits decimal digits).
// Callers should enforce "required" separately before or after this check.
func ValidateAccountCode(code string) error {
	if code == "" {
		return nil
	}
	if len(code) > AccountCodeMaxDigits {
		return fmt.Errorf("Account code must be at most %d digits.", AccountCodeMaxDigits)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return fmt.Errorf("Account code must contain only digits (no letters or symbols).")
		}
	}
	if len(code) < AccountCodeMinDigits {
		return fmt.Errorf("Account code must be at least %d digits.", AccountCodeMinDigits)
	}
	return nil
}

// AccountType is a strict enum (NOT a free-form string).
// This enforces the PROJECT_GUIDE requirement:
// accountType must match enum values exactly.
type AccountType string

const (
	// Legacy group-level types (kept for backward compatibility with existing DB rows).
	AccountTypeAsset       AccountType = "asset"
	AccountTypeLiability   AccountType = "liability"
	AccountTypeEquity      AccountType = "equity"
	AccountTypeRevenue     AccountType = "revenue"
	AccountTypeExpense     AccountType = "expense"
	AccountTypeCostOfSales AccountType = "cost_of_sales"

	// Detailed Chart of Accounts types.
	AccountTypeBank                   AccountType = "Bank"
	AccountTypeAccountsReceivable    AccountType = "Accounts Receivable"
	AccountTypeOtherCurrentAsset     AccountType = "Other Current Asset"
	AccountTypeFixedAsset            AccountType = "Fixed Asset"
	AccountTypeOtherAsset           AccountType = "Other Asset"
	AccountTypeAccountsPayable       AccountType = "Accounts Payable"
	AccountTypeCreditCard            AccountType = "Credit Card"
	AccountTypeOtherCurrentLiability AccountType = "Other Current Liability"
	AccountTypeLongTermLiability    AccountType = "Long Term Liability"
	AccountTypeEquityDetail          AccountType = "Equity"
	AccountTypeIncome                AccountType = "Income"
	AccountTypeCostOfGoodsSold      AccountType = "Cost of Goods Sold"
	AccountTypeExpenseDetail        AccountType = "Expense"
	AccountTypeOtherIncome          AccountType = "Other Income"
	AccountTypeOtherExpense         AccountType = "Other Expense"
)

func (t AccountType) Valid() bool {
	switch t {
	// Legacy
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeRevenue, AccountTypeExpense, AccountTypeCostOfSales,
		// Detailed
		AccountTypeBank, AccountTypeAccountsReceivable, AccountTypeOtherCurrentAsset, AccountTypeFixedAsset, AccountTypeOtherAsset,
		AccountTypeAccountsPayable, AccountTypeCreditCard, AccountTypeOtherCurrentLiability, AccountTypeLongTermLiability,
		AccountTypeEquityDetail,
		AccountTypeIncome, AccountTypeCostOfGoodsSold,
		AccountTypeExpenseDetail, AccountTypeOtherIncome, AccountTypeOtherExpense:
		return true
	default:
		return false
	}
}

func (t AccountType) String() string { return string(t) }

// ParseAccountType converts a user-facing string into a strict AccountType.
// This keeps validation centralized and beginner-friendly.
func ParseAccountType(s string) (AccountType, error) {
	t := AccountType(s)
	if !t.Valid() {
		return "", fmt.Errorf("invalid account type: %q", s)
	}
	return t, nil
}

type AccountReportGroup string

const (
	AccountReportGroupAsset           AccountReportGroup = "Asset"
	AccountReportGroupLiability      AccountReportGroup = "Liability"
	AccountReportGroupEquity         AccountReportGroup = "Equity"
	AccountReportGroupIncome         AccountReportGroup = "Income"
	AccountReportGroupCostOfGoodsSold AccountReportGroup = "Cost of Goods Sold"
	AccountReportGroupExpense        AccountReportGroup = "Expense"
)

// ReportGroup is the "big attribute" used by reports.
// It maps detailed COA types into report buckets.
func (t AccountType) ReportGroup() AccountReportGroup {
	switch t {
	// Asset group
	case AccountTypeBank,
		AccountTypeAccountsReceivable,
		AccountTypeOtherCurrentAsset,
		AccountTypeFixedAsset,
		AccountTypeOtherAsset,
		AccountTypeAsset:
		return AccountReportGroupAsset

	// Liability group
	case AccountTypeAccountsPayable,
		AccountTypeCreditCard,
		AccountTypeOtherCurrentLiability,
		AccountTypeLongTermLiability,
		AccountTypeLiability:
		return AccountReportGroupLiability

	// Equity group
	case AccountTypeEquityDetail,
		AccountTypeEquity:
		return AccountReportGroupEquity

	// Income group
	case AccountTypeIncome,
		AccountTypeOtherIncome,
		AccountTypeRevenue:
		return AccountReportGroupIncome

	// COGS group
	case AccountTypeCostOfGoodsSold,
		AccountTypeCostOfSales:
		return AccountReportGroupCostOfGoodsSold

	// Expense group
	case AccountTypeExpenseDetail,
		AccountTypeOtherExpense,
		AccountTypeExpense:
		return AccountReportGroupExpense

	default:
		return ""
	}
}

// Account is one row in the Chart of Accounts.
type Account struct {
	ID        uint        `gorm:"primaryKey"`
	Code      string      `gorm:"not null;uniqueIndex"`
	Name      string      `gorm:"not null"`
	Type      AccountType `gorm:"type:text;not null"`
	CreatedAt time.Time
}

