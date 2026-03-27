// 遵循产品需求 v1.0
package services

import (
	"fmt"

	"gobooks/internal/models"

	"gorm.io/gorm"
)

// AccountTemplate is a simple shape for default Chart of Accounts entries.
type AccountTemplate struct {
	Code string
	Name string
	Type models.AccountType
}

// Default account templates keyed by (entity_type, business_type).
//
// Keep this intentionally small and obvious. We can expand it over time.
var defaultCOA = map[string][]AccountTemplate{
	key(models.EntityTypePersonal, models.BusinessTypeRetail): {
		{Code: "1000", Name: "Cash", Type: models.AccountTypeOtherCurrentAsset},
		{Code: "1100", Name: "Bank", Type: models.AccountTypeBank},
		{Code: "2000", Name: "Credit Card Payable", Type: models.AccountTypeCreditCard},
		{Code: "3000", Name: "Owner's Equity", Type: models.AccountTypeEquityDetail},
		{Code: "4000", Name: "Sales Revenue", Type: models.AccountTypeIncome},
		{Code: "5000", Name: "General Expenses", Type: models.AccountTypeExpenseDetail},
	},
	key(models.EntityTypeIncorporated, models.BusinessTypeRetail): {
		{Code: "1000", Name: "Cash", Type: models.AccountTypeOtherCurrentAsset},
		{Code: "1100", Name: "Bank - Operating", Type: models.AccountTypeBank},
		{Code: "1200", Name: "Inventory", Type: models.AccountTypeOtherCurrentAsset},
		{Code: "2100", Name: "Accounts Payable", Type: models.AccountTypeAccountsPayable},
		{Code: "3000", Name: "Share Capital", Type: models.AccountTypeEquityDetail},
		{Code: "3100", Name: "Retained Earnings", Type: models.AccountTypeEquityDetail},
		{Code: "4000", Name: "Sales Revenue", Type: models.AccountTypeIncome},
		{Code: "5000", Name: "Cost of Goods Sold", Type: models.AccountTypeCostOfGoodsSold},
		{Code: "5100", Name: "Rent Expense", Type: models.AccountTypeExpenseDetail},
	},
	key(models.EntityTypeIncorporated, models.BusinessTypeProfessionalCorp): {
		{Code: "1000", Name: "Cash", Type: models.AccountTypeOtherCurrentAsset},
		{Code: "1100", Name: "Bank - Operating", Type: models.AccountTypeBank},
		{Code: "1200", Name: "Accounts Receivable", Type: models.AccountTypeAccountsReceivable},
		{Code: "2100", Name: "Accounts Payable", Type: models.AccountTypeAccountsPayable},
		{Code: "2200", Name: "Payroll Liabilities", Type: models.AccountTypeOtherCurrentLiability},
		{Code: "3000", Name: "Share Capital", Type: models.AccountTypeEquityDetail},
		{Code: "3100", Name: "Retained Earnings", Type: models.AccountTypeEquityDetail},
		{Code: "4000", Name: "Professional Fees Revenue", Type: models.AccountTypeIncome},
		{Code: "5100", Name: "Salaries Expense", Type: models.AccountTypeExpenseDetail},
		{Code: "5200", Name: "Office Rent Expense", Type: models.AccountTypeExpenseDetail},
	},
	key(models.EntityTypeLLP, models.BusinessTypeProfessionalCorp): {
		{Code: "1000", Name: "Cash", Type: models.AccountTypeOtherCurrentAsset},
		{Code: "1100", Name: "Bank - Operating", Type: models.AccountTypeBank},
		{Code: "1200", Name: "Accounts Receivable", Type: models.AccountTypeAccountsReceivable},
		{Code: "2100", Name: "Accounts Payable", Type: models.AccountTypeAccountsPayable},
		{Code: "3000", Name: "Partners' Capital", Type: models.AccountTypeEquityDetail},
		{Code: "3100", Name: "Partners' Drawings", Type: models.AccountTypeEquityDetail},
		{Code: "4000", Name: "Professional Fees Revenue", Type: models.AccountTypeIncome},
		{Code: "5100", Name: "Salaries Expense", Type: models.AccountTypeExpenseDetail},
		{Code: "5200", Name: "Office Rent Expense", Type: models.AccountTypeExpenseDetail},
	},
}

func key(entity models.EntityType, business models.BusinessType) string {
	return fmt.Sprintf("%s|%s", entity, business)
}

// ImportDefaultChartOfAccounts inserts default accounts for a company.
//
// Rules:
// - Template depends on (entity_type, business_type).
// - Do not insert duplicates (accounts table has unique Code).
// - Keep it simple: if a code already exists, skip it.
func ImportDefaultChartOfAccounts(tx *gorm.DB, entity models.EntityType, business models.BusinessType) error {
	templates, ok := defaultCOA[key(entity, business)]
	if !ok {
		// No template defined yet; not an error, just nothing to import.
		return nil
	}

	// Fetch existing codes once, to avoid many DB calls.
	var existing []string
	if err := tx.Model(&models.Account{}).Pluck("code", &existing).Error; err != nil {
		return err
	}
	existingSet := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		existingSet[c] = struct{}{}
	}

	for _, t := range templates {
		if _, found := existingSet[t.Code]; found {
			continue
		}

		acc := models.Account{
			Code: t.Code,
			Name: t.Name,
			Type: t.Type,
		}

		if err := tx.Create(&acc).Error; err != nil {
			return err
		}

		existingSet[t.Code] = struct{}{}
	}

	return nil
}

