package services

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
)

func TestPayrollRemittanceCreateAndPayPostsStatutoryLiabilities(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Remit Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	bank := payrollPostingAccount(t, db, company.ID, "1100", "Bank - Operating", models.RootAsset, models.DetailBank)
	payrollPostingAccount(t, db, company.ID, "6000", "Salaries & Wages", models.RootExpense, models.DetailPayrollExpense)
	payrollPostingAccount(t, db, company.ID, "6010", "Employer Payroll Taxes", models.RootExpense, models.DetailPayrollExpense)
	payrollPostingAccount(t, db, company.ID, "2200", "Payroll Liabilities", models.RootLiability, models.DetailPayrollLiability)
	cppLiability := payrollPostingAccount(t, db, company.ID, "2210", "CPP Payable", models.RootLiability, models.DetailPayrollLiability)
	eiLiability := payrollPostingAccount(t, db, company.ID, "2220", "EI Payable", models.RootLiability, models.DetailPayrollLiability)
	taxLiability := payrollPostingAccount(t, db, company.ID, "2230", "Income Tax Withheld Payable", models.RootLiability, models.DetailPayrollLiability)

	run := models.PayrollRun{
		CompanyID:        company.ID,
		RunNumber:        "2026-12",
		PeriodStart:      time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
		PayDate:          time.Date(2026, 12, 16, 0, 0, 0, 0, time.UTC),
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
	employee := models.Employee{CompanyID: company.ID, LegalName: "Avery Remit", Status: models.EmployeeStatusActive}
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
	if _, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID); err != nil {
		t.Fatal(err)
	}

	dueDate := time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)
	remittance, err := CreatePayrollRemittanceForRun(db, company.ID, run.ID, dueDate)
	if err != nil {
		t.Fatal(err)
	}
	if remittance.Status != models.PayrollRemittanceDraft || !remittance.TotalAmount.Equal(decimal.NewFromInt(400)) {
		t.Fatalf("unexpected remittance: %+v", remittance)
	}
	again, err := CreatePayrollRemittanceForRun(db, company.ID, run.ID, dueDate)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != remittance.ID {
		t.Fatalf("second remittance ID = %d, want %d", again.ID, remittance.ID)
	}

	paymentDate := time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)
	je, err := PayPayrollRemittance(db, company.ID, remittance.ID, bank.ID, paymentDate)
	if err != nil {
		t.Fatal(err)
	}
	if je.SourceType != models.LedgerSourcePayrollRemittance || je.SourceID != remittance.ID {
		t.Fatalf("unexpected remittance JE: %+v", je)
	}

	var paid models.PayrollRemittance
	if err := db.First(&paid, remittance.ID).Error; err != nil {
		t.Fatal(err)
	}
	if paid.Status != models.PayrollRemittancePaid || paid.JournalEntryID == nil || paid.BankLedgerAccountID == nil {
		t.Fatalf("remittance not marked paid: %+v", paid)
	}

	var lines []models.JournalLine
	if err := db.Where("company_id = ? AND journal_entry_id = ?", company.ID, je.ID).Find(&lines).Error; err != nil {
		t.Fatal(err)
	}
	if len(lines) != 4 {
		t.Fatalf("remittance lines = %d, want 4: %+v", len(lines), lines)
	}
	net := map[uint]decimal.Decimal{}
	for _, line := range lines {
		net[line.AccountID] = net[line.AccountID].Add(line.Debit).Sub(line.Credit)
	}
	if !net[cppLiability.ID].Equal(decimal.NewFromInt(130)) {
		t.Fatalf("CPP debit = %s, want 130", net[cppLiability.ID])
	}
	if !net[eiLiability.ID].Equal(decimal.NewFromInt(90)) {
		t.Fatalf("EI debit = %s, want 90", net[eiLiability.ID])
	}
	if !net[taxLiability.ID].Equal(decimal.NewFromInt(180)) {
		t.Fatalf("tax debit = %s, want 180", net[taxLiability.ID])
	}
	if !net[bank.ID].Equal(decimal.NewFromInt(-400)) {
		t.Fatalf("bank credit = %s, want -400", net[bank.ID])
	}

	paidAgain, err := PayPayrollRemittance(db, company.ID, remittance.ID, bank.ID, paymentDate)
	if err != nil {
		t.Fatal(err)
	}
	if paidAgain.ID != je.ID {
		t.Fatalf("second pay JE ID = %d, want %d", paidAgain.ID, je.ID)
	}

	voidResult, err := VoidPayrollRemittance(db, company.ID, remittance.ID, time.Date(2027, 1, 11, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if voidResult.OriginalJournalEntryID != je.ID || voidResult.ReversalJournalEntryID == 0 {
		t.Fatalf("unexpected void result: %+v", voidResult)
	}
	var voided models.PayrollRemittance
	if err := db.First(&voided, remittance.ID).Error; err != nil {
		t.Fatal(err)
	}
	if voided.Status != models.PayrollRemittanceVoided || voided.VoidedAt == nil || voided.ReversalJournalEntryID == nil {
		t.Fatalf("remittance not voided: %+v", voided)
	}
	var reversedJE models.JournalEntry
	if err := db.First(&reversedJE, je.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reversedJE.Status != models.JournalEntryStatusReversed {
		t.Fatalf("original remittance JE status = %s, want reversed", reversedJE.Status)
	}
	if _, err := PayPayrollRemittance(db, company.ID, remittance.ID, bank.ID, paymentDate); !errors.Is(err, ErrPayrollRemittanceCannotPay) {
		t.Fatalf("pay voided remittance err = %v, want ErrPayrollRemittanceCannotPay", err)
	}
}

func TestPayrollRemittanceRequiresPostedRunAndBankAccount(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Remit Guard Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	bank := payrollPostingAccount(t, db, company.ID, "1100", "Bank - Operating", models.RootAsset, models.DetailBank)
	run := models.PayrollRun{
		CompanyID:        company.ID,
		RunNumber:        "GUARD",
		PeriodStart:      time.Date(2026, 12, 16, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		PayDate:          time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC),
		Status:           models.PayrollRunFinalized,
		TotalGross:       decimal.NewFromInt(100),
		TotalEmployeeTax: decimal.NewFromInt(10),
		TotalDeductions:  decimal.NewFromInt(10),
		TotalNetPay:      decimal.NewFromInt(90),
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := CreatePayrollRemittanceForRun(db, company.ID, run.ID, time.Time{}); !errors.Is(err, ErrChequePayrollRunNotPosted) {
		t.Fatalf("unposted run remittance err = %v, want ErrChequePayrollRunNotPosted", err)
	}

	payrollPostingAccount(t, db, company.ID, "6000", "Salaries & Wages", models.RootExpense, models.DetailPayrollExpense)
	payrollPostingAccount(t, db, company.ID, "2200", "Payroll Liabilities", models.RootLiability, models.DetailPayrollLiability)
	if err := db.Create(&models.PayrollEntry{
		CompanyID:       company.ID,
		PayrollRunID:    run.ID,
		EmployeeID:      1,
		GrossPay:        decimal.NewFromInt(100),
		TotalDeductions: decimal.NewFromInt(10),
		NetPay:          decimal.NewFromInt(90),
		Status:          models.PayrollEntryApproved,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID); err != nil {
		t.Fatal(err)
	}
	remittance, err := CreatePayrollRemittanceForRun(db, company.ID, run.ID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PayPayrollRemittance(db, company.ID, remittance.ID, 0, time.Time{}); !errors.Is(err, ErrChequeBankLedgerRequired) {
		t.Fatalf("missing bank err = %v, want ErrChequeBankLedgerRequired", err)
	}
	if _, err := PayPayrollRemittance(db, company.ID, remittance.ID, bank.ID, time.Time{}); err != nil {
		t.Fatal(err)
	}
}

func TestPayrollRemittanceVoidDraftDoesNotCreateReversal(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Remit Draft Void Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	payrollPostingAccount(t, db, company.ID, "6000", "Salaries & Wages", models.RootExpense, models.DetailPayrollExpense)
	payrollPostingAccount(t, db, company.ID, "2200", "Payroll Liabilities", models.RootLiability, models.DetailPayrollLiability)

	run := models.PayrollRun{
		CompanyID:        company.ID,
		RunNumber:        "DRAFT-VOID",
		PeriodStart:      time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2027, 2, 15, 0, 0, 0, 0, time.UTC),
		PayDate:          time.Date(2027, 2, 16, 0, 0, 0, 0, time.UTC),
		Status:           models.PayrollRunFinalized,
		TotalGross:       decimal.NewFromInt(100),
		TotalEmployeeTax: decimal.NewFromInt(15),
		TotalDeductions:  decimal.NewFromInt(15),
		TotalNetPay:      decimal.NewFromInt(85),
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollEntry{
		CompanyID:       company.ID,
		PayrollRunID:    run.ID,
		EmployeeID:      1,
		GrossPay:        decimal.NewFromInt(100),
		TotalDeductions: decimal.NewFromInt(15),
		NetPay:          decimal.NewFromInt(85),
		Status:          models.PayrollEntryApproved,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID); err != nil {
		t.Fatal(err)
	}
	remittance, err := CreatePayrollRemittanceForRun(db, company.ID, run.ID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := VoidPayrollRemittance(db, company.ID, remittance.ID, time.Date(2027, 2, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.OriginalJournalEntryID != 0 || result.ReversalJournalEntryID != 0 {
		t.Fatalf("draft void result should not include journal entries: %+v", result)
	}
	var voided models.PayrollRemittance
	if err := db.First(&voided, remittance.ID).Error; err != nil {
		t.Fatal(err)
	}
	if voided.Status != models.PayrollRemittanceVoided || voided.VoidedAt == nil || voided.ReversalJournalEntryID != nil {
		t.Fatalf("draft remittance not voided cleanly: %+v", voided)
	}
}
