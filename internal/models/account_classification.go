// 遵循project_guide.md
package models

import (
	"fmt"
	"strings"
)

// RootAccountType is the primary classification for accounting rules and code prefix (1–6).
type RootAccountType string

const (
	RootAsset        RootAccountType = "asset"
	RootLiability    RootAccountType = "liability"
	RootEquity       RootAccountType = "equity"
	RootRevenue      RootAccountType = "revenue"
	RootCostOfSales  RootAccountType = "cost_of_sales"
	RootExpense      RootAccountType = "expense"
)

// DetailAccountType is the second-level classification (snake_case in DB).
type DetailAccountType string

// Asset details
const (
	DetailBank              DetailAccountType = "bank"
	DetailAccountsReceivable DetailAccountType = "accounts_receivable"
	DetailOtherCurrentAsset DetailAccountType = "other_current_asset"
	DetailFixedAsset        DetailAccountType = "fixed_asset"
	DetailOtherAsset        DetailAccountType = "other_asset"
	DetailInventory         DetailAccountType = "inventory"
	DetailPrepaidExpense    DetailAccountType = "prepaid_expense"
)

// Liability details
const (
	DetailAccountsPayable       DetailAccountType = "accounts_payable"
	DetailCreditCard            DetailAccountType = "credit_card"
	DetailOtherCurrentLiability DetailAccountType = "other_current_liability"
	DetailLongTermLiability     DetailAccountType = "long_term_liability"
	DetailSalesTaxPayable       DetailAccountType = "sales_tax_payable"
	DetailPayrollLiability      DetailAccountType = "payroll_liability"
)

// Equity details
const (
	DetailShareCapital     DetailAccountType = "share_capital"
	DetailRetainedEarnings DetailAccountType = "retained_earnings"
	DetailOwnerContribution DetailAccountType = "owner_contribution"
	DetailOwnerDrawings    DetailAccountType = "owner_drawings"
	DetailOtherEquity      DetailAccountType = "other_equity"
)

// Revenue details
const (
	DetailOperatingRevenue DetailAccountType = "operating_revenue"
	DetailServiceRevenue   DetailAccountType = "service_revenue"
	DetailSalesRevenue     DetailAccountType = "sales_revenue"
	DetailOtherIncome      DetailAccountType = "other_income"
)

// Cost of sales
const (
	DetailCostOfGoodsSold DetailAccountType = "cost_of_goods_sold"
)

// Expense details
const (
	DetailOperatingExpense   DetailAccountType = "operating_expense"
	DetailOfficeExpense      DetailAccountType = "office_expense"
	DetailRentExpense        DetailAccountType = "rent_expense"
	DetailUtilitiesExpense   DetailAccountType = "utilities_expense"
	DetailPayrollExpense     DetailAccountType = "payroll_expense"
	DetailProfessionalFees   DetailAccountType = "professional_fees"
	DetailBankCharges        DetailAccountType = "bank_charges"
	DetailAdvertisingExpense DetailAccountType = "advertising_expense"
	DetailInsuranceExpense   DetailAccountType = "insurance_expense"
	DetailOtherExpense       DetailAccountType = "other_expense"
)

var validRootDetails = map[RootAccountType]map[DetailAccountType]struct{}{
	RootAsset: {
		DetailBank: {}, DetailAccountsReceivable: {}, DetailOtherCurrentAsset: {},
		DetailFixedAsset: {}, DetailOtherAsset: {}, DetailInventory: {}, DetailPrepaidExpense: {},
	},
	RootLiability: {
		DetailAccountsPayable: {}, DetailCreditCard: {}, DetailOtherCurrentLiability: {},
		DetailLongTermLiability: {}, DetailSalesTaxPayable: {}, DetailPayrollLiability: {},
	},
	RootEquity: {
		DetailShareCapital: {}, DetailRetainedEarnings: {}, DetailOwnerContribution: {},
		DetailOwnerDrawings: {}, DetailOtherEquity: {},
	},
	RootRevenue: {
		DetailOperatingRevenue: {}, DetailServiceRevenue: {}, DetailSalesRevenue: {}, DetailOtherIncome: {},
	},
	RootCostOfSales: {
		DetailCostOfGoodsSold: {},
	},
	RootExpense: {
		DetailOperatingExpense: {}, DetailOfficeExpense: {}, DetailRentExpense: {},
		DetailUtilitiesExpense: {}, DetailPayrollExpense: {}, DetailProfessionalFees: {},
		DetailBankCharges: {}, DetailAdvertisingExpense: {}, DetailInsuranceExpense: {}, DetailOtherExpense: {},
	},
}

// ParseRootAccountType parses and validates a root value.
func ParseRootAccountType(s string) (RootAccountType, error) {
	r := RootAccountType(strings.TrimSpace(s))
	switch r {
	case RootAsset, RootLiability, RootEquity, RootRevenue, RootCostOfSales, RootExpense:
		return r, nil
	default:
		return "", fmt.Errorf("invalid root account type: %q", s)
	}
}

// ParseDetailAccountType parses a detail string (must pair with root via ValidateRootDetail).
func ParseDetailAccountType(s string) (DetailAccountType, error) {
	d := DetailAccountType(strings.TrimSpace(s))
	if d == "" {
		return "", fmt.Errorf("detail account type is required")
	}
	return d, nil
}

// ValidateRootDetail returns nil if detail is allowed for root.
func ValidateRootDetail(root RootAccountType, detail DetailAccountType) error {
	m, ok := validRootDetails[root]
	if !ok {
		return fmt.Errorf("invalid root account type")
	}
	if _, ok := m[detail]; !ok {
		return fmt.Errorf("detail %q is not valid for root %q", detail, root)
	}
	return nil
}

// RootRequiredPrefixDigit returns the first digit (1–6) for account codes under this root.
func RootRequiredPrefixDigit(root RootAccountType) (byte, error) {
	switch root {
	case RootAsset:
		return '1', nil
	case RootLiability:
		return '2', nil
	case RootEquity:
		return '3', nil
	case RootRevenue:
		return '4', nil
	case RootCostOfSales:
		return '5', nil
	case RootExpense:
		return '6', nil
	default:
		return 0, fmt.Errorf("unknown root account type")
	}
}

func accountCodePrefixMismatchMessage(want, got byte) string {
	switch want {
	case '1':
		return "Account code must start with 1 for asset accounts."
	case '2':
		return "Account code must start with 2 for liability accounts."
	case '3':
		return "Account code must start with 3 for equity accounts."
	case '4':
		return "Account code must start with 4 for revenue accounts."
	case '5':
		return "Account code must start with 5 for cost of sales accounts."
	case '6':
		return "Account code must start with 6 for expense accounts."
	default:
		return fmt.Sprintf("Account code must start with %c; got %c.", want, got)
	}
}

// ValidateAccountCodePrefixForRoot ensures the first digit matches the root (1–6 scheme).
func ValidateAccountCodePrefixForRoot(code string, root RootAccountType) error {
	if code == "" {
		return nil
	}
	want, err := RootRequiredPrefixDigit(root)
	if err != nil {
		return err
	}
	if code[0] != want {
		return fmt.Errorf("%s", accountCodePrefixMismatchMessage(want, code[0]))
	}
	return nil
}

// ValidateAccountCodeAndClassification applies strict code rules and prefix for root.
func ValidateAccountCodeAndClassification(code string, companyLength int, root RootAccountType) error {
	if err := ValidateAccountCodeStrict(code, companyLength); err != nil {
		return err
	}
	if code == "" {
		return nil
	}
	return ValidateAccountCodePrefixForRoot(code, root)
}

// ReportGroup maps root to financial statement groupings (same as prior AccountType.ReportGroup semantics).
func (root RootAccountType) ReportGroup() AccountReportGroup {
	switch root {
	case RootAsset:
		return AccountReportGroupAsset
	case RootLiability:
		return AccountReportGroupLiability
	case RootEquity:
		return AccountReportGroupEquity
	case RootRevenue:
		return AccountReportGroupIncome
	case RootCostOfSales:
		return AccountReportGroupCostOfGoodsSold
	case RootExpense:
		return AccountReportGroupExpense
	default:
		return ""
	}
}

// ClassificationDisplay returns a short label for tables (e.g. "Asset · Bank").
func ClassificationDisplay(root RootAccountType, detail DetailAccountType) string {
	return fmt.Sprintf("%s · %s", DetailSnakeToLabel(string(root)), DetailSnakeToLabel(string(detail)))
}

// DetailSnakeToLabel turns snake_case into a short Title Case label for display.
func DetailSnakeToLabel(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// LegacyTypeColumnToRootDetail maps the old single `type` column value to root + detail (migration + tests).
func LegacyTypeColumnToRootDetail(legacy string) (RootAccountType, DetailAccountType, error) {
	switch legacy {
	case "Bank":
		return RootAsset, DetailBank, nil
	case "Accounts Receivable":
		return RootAsset, DetailAccountsReceivable, nil
	case "Other Current Asset":
		return RootAsset, DetailOtherCurrentAsset, nil
	case "Fixed Asset":
		return RootAsset, DetailFixedAsset, nil
	case "Other Asset":
		return RootAsset, DetailOtherAsset, nil
	case "Accounts Payable":
		return RootLiability, DetailAccountsPayable, nil
	case "Credit Card":
		return RootLiability, DetailCreditCard, nil
	case "Other Current Liability":
		return RootLiability, DetailOtherCurrentLiability, nil
	case "Long Term Liability":
		return RootLiability, DetailLongTermLiability, nil
	case "Equity":
		return RootEquity, DetailOtherEquity, nil
	case "Income":
		return RootRevenue, DetailOperatingRevenue, nil
	case "Cost of Goods Sold":
		return RootCostOfSales, DetailCostOfGoodsSold, nil
	case "Expense":
		return RootExpense, DetailOperatingExpense, nil
	case "Other Income":
		return RootRevenue, DetailOtherIncome, nil
	case "Other Expense":
		return RootExpense, DetailOtherExpense, nil
	case "asset":
		return RootAsset, DetailOtherAsset, nil
	case "liability":
		return RootLiability, DetailOtherCurrentLiability, nil
	case "equity":
		return RootEquity, DetailOtherEquity, nil
	case "revenue":
		return RootRevenue, DetailOperatingRevenue, nil
	case "cost_of_sales":
		return RootCostOfSales, DetailCostOfGoodsSold, nil
	case "expense":
		return RootExpense, DetailOperatingExpense, nil
	default:
		return "", "", fmt.Errorf("unknown legacy account type: %q", legacy)
	}
}

// AllRootAccountTypes returns roots in stable UI order.
func AllRootAccountTypes() []RootAccountType {
	return []RootAccountType{
		RootAsset, RootLiability, RootEquity, RootRevenue, RootCostOfSales, RootExpense,
	}
}

// DetailsForRoot returns valid detail values for a root (stable order).
func DetailsForRoot(root RootAccountType) []DetailAccountType {
	switch root {
	case RootAsset:
		return []DetailAccountType{
			DetailBank, DetailAccountsReceivable, DetailOtherCurrentAsset, DetailFixedAsset,
			DetailOtherAsset, DetailInventory, DetailPrepaidExpense,
		}
	case RootLiability:
		return []DetailAccountType{
			DetailAccountsPayable, DetailCreditCard, DetailOtherCurrentLiability, DetailLongTermLiability,
			DetailSalesTaxPayable, DetailPayrollLiability,
		}
	case RootEquity:
		return []DetailAccountType{
			DetailShareCapital, DetailRetainedEarnings, DetailOwnerContribution, DetailOwnerDrawings, DetailOtherEquity,
		}
	case RootRevenue:
		return []DetailAccountType{
			DetailOperatingRevenue, DetailServiceRevenue, DetailSalesRevenue, DetailOtherIncome,
		}
	case RootCostOfSales:
		return []DetailAccountType{DetailCostOfGoodsSold}
	case RootExpense:
		return []DetailAccountType{
			DetailOperatingExpense, DetailOfficeExpense, DetailRentExpense, DetailUtilitiesExpense,
			DetailPayrollExpense, DetailProfessionalFees, DetailBankCharges, DetailAdvertisingExpense,
			DetailInsuranceExpense, DetailOtherExpense,
		}
	default:
		return nil
	}
}
