package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
)

func TestChequeFoundationCreatesAndListsCompanyScopedDrafts(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	companyA := models.Company{Name: "Cheque A", BaseCurrencyCode: "CAD", IsActive: true}
	companyB := models.Company{Name: "Cheque B", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&companyA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&companyB).Error; err != nil {
		t.Fatal(err)
	}

	accountA := models.ChequeBankAccount{
		CompanyID:             companyA.ID,
		Label:                 "Operating",
		BankName:              "Safe Bank",
		NextChequeNumber:      "1001",
		DefaultCurrencyCode:   "cad",
		BankAccountCiphertext: "encrypted-secret",
		BankAccountLast4:      "9876",
	}
	accountB := models.ChequeBankAccount{CompanyID: companyB.ID, Label: "Other", DefaultCurrencyCode: "CAD"}
	if err := CreateChequeBankAccount(db, &accountA); err != nil {
		t.Fatal(err)
	}
	if err := CreateChequeBankAccount(db, &accountB); err != nil {
		t.Fatal(err)
	}

	cheque := models.Cheque{
		CompanyID:     companyA.ID,
		BankAccountID: accountA.ID,
		PayeeName:     "Avery Vendor",
		Amount:        decimal.NewFromFloat(123.45),
		ChequeDate:    time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  "cad",
		Memo:          "Office supplies",
	}
	if err := CreateCheque(db, &cheque); err != nil {
		t.Fatal(err)
	}
	if cheque.ChequeNumber != "1001" {
		t.Fatalf("cheque number = %q, want account default 1001", cheque.ChequeNumber)
	}
	if cheque.Status != models.ChequeStatusDraft || cheque.CurrencyCode != "CAD" {
		t.Fatalf("unexpected cheque defaults: %+v", cheque)
	}

	cheques, err := ListCheques(db, ChequeListFilter{CompanyID: companyA.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(cheques) != 1 || cheques[0].CompanyID != companyA.ID || cheques[0].BankAccount.Label != "Operating" {
		t.Fatalf("company-scoped cheques not loaded correctly: %+v", cheques)
	}
	otherCheques, err := ListCheques(db, ChequeListFilter{CompanyID: companyB.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(otherCheques) != 0 {
		t.Fatalf("company B should not see company A cheques: %+v", otherCheques)
	}

	accounts, err := ListChequeBankAccounts(db, companyA.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || strings.Contains(accounts[0].Label, "9876") {
		t.Fatalf("unexpected account list: %+v", accounts)
	}
}

func TestCreateChequeValidatesRequiredFields(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	if err := CreateCheque(db, &models.Cheque{CompanyID: 1}); err == nil {
		t.Fatal("expected missing bank account to fail")
	}
	if err := CreateChequeBankAccount(db, &models.ChequeBankAccount{CompanyID: 1}); err == nil {
		t.Fatal("expected missing account label to fail")
	}
}

func TestGeneratePayrollChequeDraftsIsIdempotentAndAdvancesNumbers(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Cheque Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	account := models.ChequeBankAccount{
		CompanyID:           company.ID,
		Label:               "Payroll",
		NextChequeNumber:    "0098",
		DefaultCurrencyCode: "CAD",
		IsActive:            true,
	}
	if err := CreateChequeBankAccount(db, &account); err != nil {
		t.Fatal(err)
	}
	employeeA := models.Employee{CompanyID: company.ID, LegalName: "Avery One", Status: models.EmployeeStatusActive}
	employeeB := models.Employee{CompanyID: company.ID, LegalName: "Avery Two", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employeeA); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmployee(db, &employeeB); err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:   company.ID,
		RunNumber:   "PAY-FIN",
		PeriodStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunFinalized,
	}
	if err := CreatePayrollRun(db, &run); err != nil {
		t.Fatal(err)
	}
	entries := []models.PayrollEntry{
		{CompanyID: company.ID, PayrollRunID: run.ID, EmployeeID: employeeA.ID, NetPay: decimal.NewFromFloat(900.50), Status: models.PayrollEntryApproved},
		{CompanyID: company.ID, PayrollRunID: run.ID, EmployeeID: employeeB.ID, NetPay: decimal.NewFromFloat(1000.25), Status: models.PayrollEntryApproved},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}

	created, err := GeneratePayrollChequeDrafts(db, company.ID, run.ID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("created = %d, want 2", len(created))
	}
	if created[0].ChequeNumber != "0098" || created[1].ChequeNumber != "0099" {
		t.Fatalf("unexpected cheque numbers: %+v", created)
	}
	for _, cheque := range created {
		if cheque.PayeeType != "employee" || cheque.PayrollRunID == nil || cheque.PayrollEntryID == nil || cheque.Status != models.ChequeStatusDraft {
			t.Fatalf("unexpected payroll cheque: %+v", cheque)
		}
	}
	var updatedAccount models.ChequeBankAccount
	if err := db.First(&updatedAccount, account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updatedAccount.NextChequeNumber != "0100" {
		t.Fatalf("next cheque number = %q, want 0100", updatedAccount.NextChequeNumber)
	}

	createdAgain, err := GeneratePayrollChequeDrafts(db, company.ID, run.ID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(createdAgain) != 0 {
		t.Fatalf("idempotent generation created duplicate cheques: %+v", createdAgain)
	}
}

func TestGeneratePayrollChequeDraftsRequiresFinalizedRun(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Draft Payroll Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	account := models.ChequeBankAccount{CompanyID: company.ID, Label: "Payroll", DefaultCurrencyCode: "CAD", IsActive: true}
	if err := CreateChequeBankAccount(db, &account); err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:   company.ID,
		PeriodStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunCalculated,
	}
	if err := CreatePayrollRun(db, &run); err != nil {
		t.Fatal(err)
	}
	if _, err := GeneratePayrollChequeDrafts(db, company.ID, run.ID, account.ID); err == nil {
		t.Fatal("expected non-finalized payroll run to be rejected")
	}
}

func TestChequeStatusActionsPrintAndVoid(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Cheque Status Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	account := models.ChequeBankAccount{CompanyID: company.ID, Label: "Operating", DefaultCurrencyCode: "CAD", IsActive: true}
	if err := CreateChequeBankAccount(db, &account); err != nil {
		t.Fatal(err)
	}
	cheque := models.Cheque{
		CompanyID:     company.ID,
		BankAccountID: account.ID,
		PayeeName:     "Avery Payee",
		Amount:        decimal.NewFromInt(100),
		ChequeDate:    time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Status:        models.ChequeStatusDraft,
	}
	if err := CreateCheque(db, &cheque); err != nil {
		t.Fatal(err)
	}

	if err := MarkChequePrinted(db, company.ID, cheque.ID); err != nil {
		t.Fatal(err)
	}
	var printed models.Cheque
	if err := db.First(&printed, cheque.ID).Error; err != nil {
		t.Fatal(err)
	}
	if printed.Status != models.ChequeStatusPrinted || printed.PrintedAt == nil {
		t.Fatalf("printed state not applied: %+v", printed)
	}
	if err := MarkChequePrinted(db, company.ID, cheque.ID); err == nil {
		t.Fatal("expected printing a printed cheque to fail")
	}

	if err := VoidCheque(db, company.ID, cheque.ID); err != nil {
		t.Fatal(err)
	}
	var voided models.Cheque
	if err := db.First(&voided, cheque.ID).Error; err != nil {
		t.Fatal(err)
	}
	if voided.Status != models.ChequeStatusVoided || voided.VoidedAt == nil {
		t.Fatalf("voided state not applied: %+v", voided)
	}
	if err := VoidCheque(db, company.ID, cheque.ID); err == nil {
		t.Fatal("expected voiding a voided cheque to fail")
	}
}

func TestPayrollChequePrintPostsPaymentAndVoidReversesIt(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Cheque Payment Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	bank := payrollPostingAccount(t, db, company.ID, "1100", "Bank - Operating", models.RootAsset, models.DetailBank)
	wages := payrollPostingAccount(t, db, company.ID, "6000", "Salaries & Wages", models.RootExpense, models.DetailPayrollExpense)
	liability := payrollPostingAccount(t, db, company.ID, "2200", "Payroll Liabilities", models.RootLiability, models.DetailPayrollLiability)
	account := models.ChequeBankAccount{
		CompanyID:           company.ID,
		Label:               "Payroll",
		LedgerAccountID:     &bank.ID,
		NextChequeNumber:    "300",
		DefaultCurrencyCode: "CAD",
		IsActive:            true,
	}
	if err := CreateChequeBankAccount(db, &account); err != nil {
		t.Fatal(err)
	}
	employee := models.Employee{CompanyID: company.ID, LegalName: "Avery Paid", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employee); err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:       company.ID,
		RunNumber:       "PAY-CASH",
		PeriodStart:     time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:       time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		PayDate:         time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC),
		Status:          models.PayrollRunFinalized,
		TotalGross:      decimal.NewFromInt(1000),
		TotalDeductions: decimal.NewFromInt(100),
		TotalNetPay:     decimal.NewFromInt(900),
	}
	if err := CreatePayrollRun(db, &run); err != nil {
		t.Fatal(err)
	}
	entry := models.PayrollEntry{
		CompanyID:       company.ID,
		PayrollRunID:    run.ID,
		EmployeeID:      employee.ID,
		GrossPay:        decimal.NewFromInt(1000),
		TotalDeductions: decimal.NewFromInt(100),
		NetPay:          decimal.NewFromInt(900),
		Status:          models.PayrollEntryApproved,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID); err != nil {
		t.Fatal(err)
	}
	cheques, err := GeneratePayrollChequeDrafts(db, company.ID, run.ID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cheques) != 1 {
		t.Fatalf("cheques = %d, want 1", len(cheques))
	}

	if err := MarkChequePrinted(db, company.ID, cheques[0].ID); err != nil {
		t.Fatal(err)
	}
	paymentJE, found, err := chequePaymentJournalEntry(db, company.ID, cheques[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected cheque payment journal entry")
	}
	var lines []models.JournalLine
	if err := db.Where("company_id = ? AND journal_entry_id = ?", company.ID, paymentJE.ID).Find(&lines).Error; err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("payment lines = %d, want 2: %+v", len(lines), lines)
	}
	net := map[uint]decimal.Decimal{}
	for _, line := range lines {
		net[line.AccountID] = net[line.AccountID].Add(line.Debit).Sub(line.Credit)
	}
	if !net[liability.ID].Equal(decimal.NewFromInt(900)) {
		t.Fatalf("liability debit = %s, want 900", net[liability.ID])
	}
	if !net[bank.ID].Equal(decimal.NewFromInt(-900)) {
		t.Fatalf("bank credit = %s, want -900", net[bank.ID])
	}
	if !net[wages.ID].IsZero() {
		t.Fatalf("cheque payment should not hit wage expense, got %s", net[wages.ID])
	}

	if err := VoidCheque(db, company.ID, cheques[0].ID); err != nil {
		t.Fatal(err)
	}
	var originalPayment models.JournalEntry
	if err := db.First(&originalPayment, paymentJE.ID).Error; err != nil {
		t.Fatal(err)
	}
	if originalPayment.Status != models.JournalEntryStatusReversed {
		t.Fatalf("payment JE status = %q, want reversed", originalPayment.Status)
	}
	var reversalCount int64
	if err := db.Model(&models.JournalEntry{}).
		Where("company_id = ? AND source_type = ? AND source_id = ?", company.ID, models.LedgerSourceReversal, paymentJE.ID).
		Count(&reversalCount).Error; err != nil {
		t.Fatal(err)
	}
	if reversalCount != 1 {
		t.Fatalf("payment reversal count = %d, want 1", reversalCount)
	}
}

func TestPayrollChequePrintRequiresPostedRunAndLedgerBankAccount(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Cheque Guard Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	bank := payrollPostingAccount(t, db, company.ID, "1100", "Bank - Operating", models.RootAsset, models.DetailBank)
	account := models.ChequeBankAccount{
		CompanyID:           company.ID,
		Label:               "Payroll",
		LedgerAccountID:     &bank.ID,
		DefaultCurrencyCode: "CAD",
		IsActive:            true,
	}
	if err := CreateChequeBankAccount(db, &account); err != nil {
		t.Fatal(err)
	}
	employee := models.Employee{CompanyID: company.ID, LegalName: "Avery Guard", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employee); err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:       company.ID,
		RunNumber:       "PAY-GUARD",
		PeriodStart:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:       time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
		PayDate:         time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
		Status:          models.PayrollRunFinalized,
		TotalGross:      decimal.NewFromInt(100),
		TotalDeductions: decimal.NewFromInt(10),
		TotalNetPay:     decimal.NewFromInt(90),
	}
	if err := CreatePayrollRun(db, &run); err != nil {
		t.Fatal(err)
	}
	entry := models.PayrollEntry{
		CompanyID:       company.ID,
		PayrollRunID:    run.ID,
		EmployeeID:      employee.ID,
		GrossPay:        decimal.NewFromInt(100),
		TotalDeductions: decimal.NewFromInt(10),
		NetPay:          decimal.NewFromInt(90),
		Status:          models.PayrollEntryApproved,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	cheques, err := GeneratePayrollChequeDrafts(db, company.ID, run.ID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkChequePrinted(db, company.ID, cheques[0].ID); !errors.Is(err, ErrChequePayrollRunNotPosted) {
		t.Fatalf("unposted payroll print err = %v, want ErrChequePayrollRunNotPosted", err)
	}

	payrollPostingAccount(t, db, company.ID, "6000", "Salaries & Wages", models.RootExpense, models.DetailPayrollExpense)
	payrollPostingAccount(t, db, company.ID, "2200", "Payroll Liabilities", models.RootLiability, models.DetailPayrollLiability)
	if _, err := PostPayrollRunToJournalEntry(db, company.ID, run.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.ChequeBankAccount{}).
		Where("id = ? AND company_id = ?", account.ID, company.ID).
		Update("ledger_account_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := MarkChequePrinted(db, company.ID, cheques[0].ID); !errors.Is(err, ErrChequeBankLedgerRequired) {
		t.Fatalf("unlinked bank print err = %v, want ErrChequeBankLedgerRequired", err)
	}
}

func TestVoidedPayrollChequeCanBeRegenerated(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Regenerate Cheque Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	account := models.ChequeBankAccount{
		CompanyID:           company.ID,
		Label:               "Payroll",
		NextChequeNumber:    "200",
		DefaultCurrencyCode: "CAD",
		IsActive:            true,
	}
	if err := CreateChequeBankAccount(db, &account); err != nil {
		t.Fatal(err)
	}
	employee := models.Employee{CompanyID: company.ID, LegalName: "Avery Reissue", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employee); err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:   company.ID,
		RunNumber:   "PAY-REISSUE",
		PeriodStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunFinalized,
	}
	if err := CreatePayrollRun(db, &run); err != nil {
		t.Fatal(err)
	}
	entry := models.PayrollEntry{CompanyID: company.ID, PayrollRunID: run.ID, EmployeeID: employee.ID, NetPay: decimal.NewFromInt(800), Status: models.PayrollEntryApproved}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}

	first, err := GeneratePayrollChequeDrafts(db, company.ID, run.ID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ChequeNumber != "200" {
		t.Fatalf("first generated cheques = %+v", first)
	}
	if err := VoidCheque(db, company.ID, first[0].ID); err != nil {
		t.Fatal(err)
	}
	second, err := GeneratePayrollChequeDrafts(db, company.ID, run.ID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].ChequeNumber != "201" {
		t.Fatalf("regenerated cheques = %+v, want number 201", second)
	}
}
