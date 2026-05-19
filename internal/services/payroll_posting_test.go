package services

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

func TestPostPayrollRunToJournalEntryCreatesBalancedLedgerProjection(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Posting Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	wages := payrollPostingAccount(t, db, company.ID, "6000", "Salaries & Wages", models.RootExpense, models.DetailPayrollExpense)
	employerTax := payrollPostingAccount(t, db, company.ID, "6010", "Employer Payroll Taxes", models.RootExpense, models.DetailPayrollExpense)
	liability := payrollPostingAccount(t, db, company.ID, "2200", "Payroll Liabilities", models.RootLiability, models.DetailPayrollLiability)
	cppLiability := payrollPostingAccount(t, db, company.ID, "2210", "CPP Payable", models.RootLiability, models.DetailPayrollLiability)
	eiLiability := payrollPostingAccount(t, db, company.ID, "2220", "EI Payable", models.RootLiability, models.DetailPayrollLiability)
	taxLiability := payrollPostingAccount(t, db, company.ID, "2230", "Income Tax Withheld Payable", models.RootLiability, models.DetailPayrollLiability)

	run := models.PayrollRun{
		CompanyID:        company.ID,
		RunNumber:        "2026-01",
		PeriodStart:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		PayDate:          time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
		Status:           models.PayrollRunFinalized,
		TotalGross:       decimal.NewFromInt(1000),
		TotalEmployeeTax: decimal.NewFromInt(180),
		TotalEmployeeCPP: decimal.NewFromInt(70),
		TotalEmployeeEI:  decimal.NewFromInt(50),
		TotalDeductions:  decimal.NewFromInt(300),
		TotalNetPay:      decimal.NewFromInt(700),
		TotalEmployerCPP: decimal.NewFromInt(60),
		TotalEmployerEI:  decimal.NewFromInt(40),
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.Employee{CompanyID: company.ID, LegalName: "Avery Payroll", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employee); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollEntry{
		CompanyID:       company.ID,
		PayrollRunID:    run.ID,
		EmployeeID:      employee.ID,
		GrossPay:        decimal.NewFromInt(1000),
		TotalDeductions: decimal.NewFromInt(300),
		NetPay:          decimal.NewFromInt(700),
		Status:          models.PayrollEntryApproved,
	}).Error; err != nil {
		t.Fatal(err)
	}

	je, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if je.SourceType != models.LedgerSourcePayrollRun || je.SourceID != run.ID || je.JournalNo != "PAYROLL-2026-01" {
		t.Fatalf("unexpected journal entry: %+v", je)
	}

	var lines []models.JournalLine
	if err := db.Where("company_id = ? AND journal_entry_id = ?", company.ID, je.ID).Order("id asc").Find(&lines).Error; err != nil {
		t.Fatal(err)
	}
	if len(lines) != 6 {
		t.Fatalf("lines = %d, want 6: %+v", len(lines), lines)
	}
	amountByAccount := map[uint]decimal.Decimal{}
	for _, line := range lines {
		amountByAccount[line.AccountID] = amountByAccount[line.AccountID].Add(line.Debit).Sub(line.Credit)
	}
	if !amountByAccount[wages.ID].Equal(decimal.NewFromInt(1000)) {
		t.Fatalf("wages debit = %s, want 1000", amountByAccount[wages.ID])
	}
	if !amountByAccount[employerTax.ID].Equal(decimal.NewFromInt(100)) {
		t.Fatalf("employer tax debit = %s, want 100", amountByAccount[employerTax.ID])
	}
	if !amountByAccount[liability.ID].Equal(decimal.NewFromInt(-700)) {
		t.Fatalf("net pay liability credit = %s, want -700", amountByAccount[liability.ID])
	}
	if !amountByAccount[cppLiability.ID].Equal(decimal.NewFromInt(-130)) {
		t.Fatalf("CPP payable credit = %s, want -130", amountByAccount[cppLiability.ID])
	}
	if !amountByAccount[eiLiability.ID].Equal(decimal.NewFromInt(-90)) {
		t.Fatalf("EI payable credit = %s, want -90", amountByAccount[eiLiability.ID])
	}
	if !amountByAccount[taxLiability.ID].Equal(decimal.NewFromInt(-180)) {
		t.Fatalf("tax payable credit = %s, want -180", amountByAccount[taxLiability.ID])
	}

	var ledgerCount int64
	if err := db.Model(&models.LedgerEntry{}).
		Where("company_id = ? AND source_type = ? AND source_id = ?", company.ID, models.LedgerSourcePayrollRun, run.ID).
		Count(&ledgerCount).Error; err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 6 {
		t.Fatalf("ledger entries = %d, want 6", ledgerCount)
	}

	again, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != je.ID {
		t.Fatalf("second post JE ID = %d, want %d", again.ID, je.ID)
	}
	var jeCount int64
	if err := db.Model(&models.JournalEntry{}).
		Where("company_id = ? AND source_type = ? AND source_id = ?", company.ID, models.LedgerSourcePayrollRun, run.ID).
		Count(&jeCount).Error; err != nil {
		t.Fatal(err)
	}
	if jeCount != 1 {
		t.Fatalf("journal entry count = %d, want 1", jeCount)
	}
}

func TestPostPayrollRunToJournalEntryFallsBackToGeneralPayrollLiability(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Posting Fallback Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	payrollPostingAccount(t, db, company.ID, "6000", "Salaries & Wages", models.RootExpense, models.DetailPayrollExpense)
	payrollPostingAccount(t, db, company.ID, "6010", "Employer Payroll Taxes", models.RootExpense, models.DetailPayrollExpense)
	liability := payrollPostingAccount(t, db, company.ID, "2200", "Payroll Liabilities", models.RootLiability, models.DetailPayrollLiability)

	run := models.PayrollRun{
		CompanyID:        company.ID,
		RunNumber:        "FALLBACK",
		PeriodStart:      time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		PayDate:          time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		Status:           models.PayrollRunFinalized,
		TotalGross:       decimal.NewFromInt(1000),
		TotalEmployeeTax: decimal.NewFromInt(180),
		TotalEmployeeCPP: decimal.NewFromInt(70),
		TotalEmployeeEI:  decimal.NewFromInt(50),
		TotalDeductions:  decimal.NewFromInt(300),
		TotalNetPay:      decimal.NewFromInt(700),
		TotalEmployerCPP: decimal.NewFromInt(60),
		TotalEmployerEI:  decimal.NewFromInt(40),
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.Employee{CompanyID: company.ID, LegalName: "Avery Fallback", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employee); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollEntry{
		CompanyID:       company.ID,
		PayrollRunID:    run.ID,
		EmployeeID:      employee.ID,
		GrossPay:        decimal.NewFromInt(1000),
		TotalDeductions: decimal.NewFromInt(300),
		NetPay:          decimal.NewFromInt(700),
		Status:          models.PayrollEntryApproved,
	}).Error; err != nil {
		t.Fatal(err)
	}

	je, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var lines []models.JournalLine
	if err := db.Where("company_id = ? AND journal_entry_id = ?", company.ID, je.ID).Find(&lines).Error; err != nil {
		t.Fatal(err)
	}
	liabilityNet := decimal.Zero
	for _, line := range lines {
		if line.AccountID == liability.ID {
			liabilityNet = liabilityNet.Add(line.Debit).Sub(line.Credit)
		}
	}
	if !liabilityNet.Equal(decimal.NewFromInt(-1100)) {
		t.Fatalf("fallback liability net = %s, want -1100", liabilityNet)
	}
}

func TestPostPayrollRunToJournalEntryRequiresFinalizedRunAndAccounts(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Posting Reject Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:   company.ID,
		RunNumber:   "DRAFT",
		PeriodStart: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunCalculated,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID); !errors.Is(err, ErrPayrollRunNotFinalized) {
		t.Fatalf("non-finalized post err = %v, want ErrPayrollRunNotFinalized", err)
	}

	if err := db.Model(&models.PayrollRun{}).Where("id = ?", run.ID).Updates(map[string]any{
		"status":           models.PayrollRunFinalized,
		"total_gross":      decimal.NewFromInt(100),
		"total_deductions": decimal.NewFromInt(10),
		"total_net_pay":    decimal.NewFromInt(90),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollEntry{
		CompanyID:       company.ID,
		PayrollRunID:    run.ID,
		EmployeeID:      1,
		GrossPay:        decimal.NewFromInt(100),
		TotalDeductions: decimal.NewFromInt(10),
		NetPay:          decimal.NewFromInt(90),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID); !errors.Is(err, ErrPayrollPostingAccountMissing) {
		t.Fatalf("missing account err = %v, want ErrPayrollPostingAccountMissing", err)
	}
}

func payrollPostingAccount(t *testing.T, db *gorm.DB, companyID uint, code, name string, root models.RootAccountType, detail models.DetailAccountType) models.Account {
	t.Helper()
	account := models.Account{
		CompanyID:         companyID,
		Code:              code,
		Name:              name,
		RootAccountType:   root,
		DetailAccountType: detail,
		IsActive:          true,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	return account
}
