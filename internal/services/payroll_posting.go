package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

var (
	ErrPayrollRunNotFinalized          = errors.New("payroll run must be finalized before posting")
	ErrPayrollPostingAccountMissing    = errors.New("payroll posting account is missing")
	ErrPayrollRunNoPostableTotals      = errors.New("payroll run has no postable totals")
	ErrPayrollPostingJournalUnbalanced = errors.New("payroll posting journal is unbalanced")
)

func PayrollRunJournalEntry(db *gorm.DB, companyID, runID uint) (models.JournalEntry, bool, error) {
	if db == nil {
		return models.JournalEntry{}, false, fmt.Errorf("PayrollRunJournalEntry: db is required")
	}
	if companyID == 0 || runID == 0 {
		return models.JournalEntry{}, false, ErrPayrollRunRequired
	}

	var je models.JournalEntry
	err := db.Where("company_id = ? AND source_type = ? AND source_id = ? AND status = ?",
		companyID, models.LedgerSourcePayrollRun, runID, models.JournalEntryStatusPosted).
		First(&je).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.JournalEntry{}, false, nil
		}
		return models.JournalEntry{}, false, err
	}
	return je, true, nil
}

func PostPayrollRunToJournalEntry(db *gorm.DB, companyID, runID uint) (models.JournalEntry, error) {
	if db == nil {
		return models.JournalEntry{}, fmt.Errorf("PostPayrollRunToJournalEntry: db is required")
	}
	if companyID == 0 || runID == 0 {
		return models.JournalEntry{}, ErrPayrollRunRequired
	}

	var posted models.JournalEntry
	err := db.Transaction(func(tx *gorm.DB) error {
		var run models.PayrollRun
		if err := tx.Where("id = ? AND company_id = ?", runID, companyID).First(&run).Error; err != nil {
			return err
		}
		if run.Status != models.PayrollRunFinalized {
			return ErrPayrollRunNotFinalized
		}

		if existing, found, err := PayrollRunJournalEntry(tx, companyID, runID); err != nil {
			return err
		} else if found {
			posted = existing
			return nil
		}

		var entryCount int64
		if err := tx.Model(&models.PayrollEntry{}).
			Where("company_id = ? AND payroll_run_id = ?", companyID, runID).
			Count(&entryCount).Error; err != nil {
			return err
		}
		if entryCount == 0 {
			return ErrPayrollRunNoEntries
		}

		wagesExpense, err := findPayrollPostingAccount(tx, companyID, models.RootExpense, models.DetailPayrollExpense, "6000")
		if err != nil {
			return err
		}
		employerExpense, err := findPayrollPostingAccount(tx, companyID, models.RootExpense, models.DetailPayrollExpense, "6010")
		if err != nil {
			employerExpense = wagesExpense
		}
		liability, err := findPayrollPostingAccount(tx, companyID, models.RootLiability, models.DetailPayrollLiability, "2200")
		if err != nil {
			return err
		}
		cppLiability, err := findPayrollPostingAccountOrDefault(tx, companyID, liability, models.RootLiability, models.DetailPayrollLiability, "2210")
		if err != nil {
			return err
		}
		eiLiability, err := findPayrollPostingAccountOrDefault(tx, companyID, liability, models.RootLiability, models.DetailPayrollLiability, "2220")
		if err != nil {
			return err
		}
		taxLiability, err := findPayrollPostingAccountOrDefault(tx, companyID, liability, models.RootLiability, models.DetailPayrollLiability, "2230")
		if err != nil {
			return err
		}

		employerContributions := run.TotalEmployerCPP.Add(run.TotalEmployerCPP2).Add(run.TotalEmployerEI).Round(2)
		cppPayable := run.TotalEmployeeCPP.Add(run.TotalEmployeeCPP2).Add(run.TotalEmployerCPP).Add(run.TotalEmployerCPP2).Round(2)
		eiPayable := run.TotalEmployeeEI.Add(run.TotalEmployerEI).Round(2)
		taxPayable := run.TotalEmployeeTax.Round(2)
		statutoryDeductions := run.TotalEmployeeTax.Add(run.TotalEmployeeCPP).Add(run.TotalEmployeeCPP2).Add(run.TotalEmployeeEI).Round(2)
		otherDeductions := run.TotalDeductions.Sub(statutoryDeductions).Round(2)
		if otherDeductions.LessThan(decimal.Zero) {
			return ErrPayrollPostingJournalUnbalanced
		}
		creditTotal := run.TotalNetPay.Add(cppPayable).Add(eiPayable).Add(taxPayable).Add(otherDeductions).Round(2)
		debitTotal := run.TotalGross.Add(employerContributions).Round(2)
		if debitTotal.IsZero() || creditTotal.IsZero() {
			return ErrPayrollRunNoPostableTotals
		}
		if !debitTotal.Equal(creditTotal) {
			return ErrPayrollPostingJournalUnbalanced
		}

		company, err := loadPostingCompany(tx, companyID)
		if err != nil {
			return err
		}
		currency := strings.ToUpper(strings.TrimSpace(company.BaseCurrencyCode))
		if currency == "" {
			currency = "CAD"
		}

		je := models.JournalEntry{
			CompanyID:               companyID,
			EntryDate:               run.PayDate,
			JournalNo:               payrollJournalNo(run),
			Status:                  models.JournalEntryStatusPosted,
			TransactionCurrencyCode: currency,
			ExchangeRate:            decimal.NewFromInt(1),
			ExchangeRateDate:        run.PayDate,
			ExchangeRateSource:      "identity",
			SourceType:              models.LedgerSourcePayrollRun,
			SourceID:                run.ID,
		}
		if err := tx.Create(&je).Error; err != nil {
			return err
		}

		lines := []models.JournalLine{
			payrollJournalLine(companyID, je.ID, wagesExpense.ID, run.TotalGross.Round(2), decimal.Zero, "Gross wages"),
		}
		if !employerContributions.IsZero() {
			lines = append(lines, payrollJournalLine(companyID, je.ID, employerExpense.ID, employerContributions, decimal.Zero, "Employer CPP/EI expense"))
		}
		lines = appendPayrollCreditLine(lines, companyID, je.ID, liability.ID, run.TotalNetPay.Round(2), "Net pay payable")
		lines = appendPayrollCreditLine(lines, companyID, je.ID, cppLiability.ID, cppPayable, "CPP payable")
		lines = appendPayrollCreditLine(lines, companyID, je.ID, eiLiability.ID, eiPayable, "EI payable")
		lines = appendPayrollCreditLine(lines, companyID, je.ID, taxLiability.ID, taxPayable, "Income tax withheld payable")
		lines = appendPayrollCreditLine(lines, companyID, je.ID, liability.ID, otherDeductions, "Other payroll deductions payable")
		if err := tx.Create(&lines).Error; err != nil {
			return err
		}
		if err := WriteSecondaryBookAmounts(tx, companyID, lines, currency, run.PayDate, models.FXPostingReasonTransaction); err != nil {
			return err
		}
		if err := ProjectToLedger(tx, companyID, LedgerPostInput{
			JournalEntry: je,
			Lines:        lines,
			SourceType:   models.LedgerSourcePayrollRun,
			SourceID:     run.ID,
		}); err != nil {
			return err
		}
		posted = je
		return nil
	})
	if err != nil {
		return models.JournalEntry{}, err
	}
	return posted, nil
}

func findPayrollPostingAccount(db *gorm.DB, companyID uint, root models.RootAccountType, detail models.DetailAccountType, preferredCodes ...string) (models.Account, error) {
	var accounts []models.Account
	if err := db.Where("company_id = ? AND root_account_type = ? AND detail_account_type = ? AND is_active = ?",
		companyID, root, detail, true).
		Order("code asc, id asc").
		Find(&accounts).Error; err != nil {
		return models.Account{}, err
	}
	if len(accounts) == 0 {
		return models.Account{}, fmt.Errorf("%w: %s/%s", ErrPayrollPostingAccountMissing, root, detail)
	}
	for _, code := range preferredCodes {
		code = strings.TrimSpace(code)
		for _, account := range accounts {
			if account.Code == code {
				return account, nil
			}
		}
	}
	return accounts[0], nil
}

func findPayrollPostingAccountOrDefault(db *gorm.DB, companyID uint, fallback models.Account, root models.RootAccountType, detail models.DetailAccountType, preferredCodes ...string) (models.Account, error) {
	account, err := findPayrollPostingAccount(db, companyID, root, detail, preferredCodes...)
	if err != nil {
		if errors.Is(err, ErrPayrollPostingAccountMissing) {
			return fallback, nil
		}
		return models.Account{}, err
	}
	return account, nil
}

func loadPostingCompany(db *gorm.DB, companyID uint) (models.Company, error) {
	var company models.Company
	if err := db.Select("id", "base_currency_code").First(&company, companyID).Error; err != nil {
		return models.Company{}, err
	}
	return company, nil
}

func payrollJournalLine(companyID, journalEntryID, accountID uint, debit, credit decimal.Decimal, memo string) models.JournalLine {
	return models.JournalLine{
		CompanyID:      companyID,
		JournalEntryID: journalEntryID,
		AccountID:      accountID,
		Debit:          debit.Round(2),
		Credit:         credit.Round(2),
		TxDebit:        debit.Round(2),
		TxCredit:       credit.Round(2),
		Memo:           memo,
	}
}

func appendPayrollCreditLine(lines []models.JournalLine, companyID, journalEntryID, accountID uint, amount decimal.Decimal, memo string) []models.JournalLine {
	amount = amount.Round(2)
	if amount.IsZero() {
		return lines
	}
	return append(lines, payrollJournalLine(companyID, journalEntryID, accountID, decimal.Zero, amount, memo))
}

func payrollJournalNo(run models.PayrollRun) string {
	number := strings.TrimSpace(run.RunNumber)
	if number != "" {
		return "PAYROLL-" + number
	}
	return fmt.Sprintf("PAYROLL-%d", run.ID)
}
