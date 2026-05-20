package services

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"balanciz/internal/models"
)

func TestCalculatePayrollEntry_EmployeeAndContractor(t *testing.T) {
	run := models.PayrollRun{
		ID:           1,
		CompanyID:    1,
		PayDate:      time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
		PaysPerYear:  26,
		PayFrequency: models.PayFrequencyBiweekly,
	}
	employee := models.Employee{
		ID:                   1,
		CompanyID:            1,
		LegalName:            "Avery Payroll",
		ProvinceOfEmployment: "BC",
		MemberType:           models.EmployeeMemberEmployee,
		SalaryType:           models.EmployeeSalarySalaried,
		PayRate:              decimal.NewFromInt(52_000),
		PayRateUnit:          "annual",
		PaysPerYear:          26,
	}

	entry, err := CalculatePayrollEntry(PayrollEntryCalculationInput{
		Run:      run,
		Employee: employee,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !entry.GrossPay.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("GrossPay = %s, want 2000.00", entry.GrossPay)
	}
	if !entry.TotalDeductions.GreaterThan(decimal.Zero) {
		t.Fatalf("expected statutory deductions, got %+v", entry)
	}
	if !entry.NetPay.LessThan(entry.GrossPay) {
		t.Fatalf("net pay should be less than gross for employee: %+v", entry)
	}

	contractor := employee
	contractor.ID = 2
	contractor.MemberType = models.EmployeeMemberContractor
	contractorEntry, err := CalculatePayrollEntry(PayrollEntryCalculationInput{
		Run:      run,
		Employee: contractor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !contractorEntry.TotalDeductions.IsZero() {
		t.Fatalf("contractor should have no source deductions, got %s", contractorEntry.TotalDeductions)
	}
	if !contractorEntry.NetPay.Equal(contractorEntry.GrossPay) {
		t.Fatalf("contractor net should equal gross: %+v", contractorEntry)
	}
}

func TestGeneratePayrollEntriesForRun_UsesFinalizedYTDAndUpdatesRunTotals(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{
		Name:             "Payroll Co",
		EntityType:       models.EntityTypeIncorporated,
		BusinessType:     models.BusinessTypeProfessionalCorp,
		Industry:         models.IndustryServices,
		IncorporatedDate: "2020-01-01",
		FiscalYearEnd:    "12-31",
		BusinessNumber:   "123456789",
		AddressLine:      "1 Main",
		City:             "Vancouver",
		Province:         "BC",
		PostalCode:       "V6B 1A1",
		Country:          "CA",
		IsActive:         true,
	}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.Employee{
		CompanyID:            company.ID,
		LegalName:            "Avery Payroll",
		ProvinceOfEmployment: "BC",
		MemberType:           models.EmployeeMemberEmployee,
		Status:               models.EmployeeStatusActive,
		PayRate:              decimal.NewFromInt(52_000),
		PayRateUnit:          "annual",
		PaysPerYear:          26,
		PayFrequency:         models.PayFrequencyBiweekly,
	}
	if err := CreateEmployee(db, &employee); err != nil {
		t.Fatal(err)
	}

	priorRun := models.PayrollRun{
		CompanyID:    company.ID,
		RunNumber:    "PAY-OLD",
		PeriodStart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		PayDate:      time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
		PaysPerYear:  26,
		PayFrequency: models.PayFrequencyBiweekly,
		Status:       models.PayrollRunFinalized,
	}
	if err := db.Create(&priorRun).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollEntry{
		CompanyID:    company.ID,
		PayrollRunID: priorRun.ID,
		EmployeeID:   employee.ID,
		GrossPay:     decimal.NewFromInt(2000),
		CPPEmployee:  decimal.NewFromFloat(100.50),
		CPP2Employee: decimal.NewFromFloat(10.25),
		EIEmployee:   decimal.NewFromFloat(32.80),
		NetPay:       decimal.NewFromInt(1600),
		Status:       models.PayrollEntryApproved,
		PaymentType:  "cheque",
	}).Error; err != nil {
		t.Fatal(err)
	}

	currentRun := models.PayrollRun{
		CompanyID:    company.ID,
		RunNumber:    "PAY-CURRENT",
		PeriodStart:  time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
		PeriodEnd:    time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		PayDate:      time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		PaysPerYear:  26,
		PayFrequency: models.PayFrequencyBiweekly,
	}
	if err := CreatePayrollRun(db, &currentRun); err != nil {
		t.Fatal(err)
	}

	entries, err := GeneratePayrollEntriesForRun(db, company.ID, currentRun.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("generated entries = %d, want 1", len(entries))
	}
	if !entries[0].YTDGross.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("YTDGross = %s, want 2000", entries[0].YTDGross)
	}
	if !entries[0].YTDCPPEmployee.Equal(decimal.NewFromFloat(100.50)) {
		t.Fatalf("YTDCPPEmployee = %s, want 100.50", entries[0].YTDCPPEmployee)
	}

	var updated models.PayrollRun
	if err := db.First(&updated, currentRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != models.PayrollRunCalculated {
		t.Fatalf("run status = %q, want calculated", updated.Status)
	}
	if !updated.TotalGross.Equal(entries[0].GrossPay) || !updated.TotalNetPay.Equal(entries[0].NetPay) {
		t.Fatalf("run totals not updated from entry: run=%+v entry=%+v", updated, entries[0])
	}
}

func TestGetPayrollRunWithEntriesScopesToCompany(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	companyA := models.Company{Name: "Payroll A", BaseCurrencyCode: "CAD", IsActive: true}
	companyB := models.Company{Name: "Payroll B", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&companyA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&companyB).Error; err != nil {
		t.Fatal(err)
	}

	employeeA := models.Employee{CompanyID: companyA.ID, LegalName: "Avery A", Status: models.EmployeeStatusActive}
	employeeB := models.Employee{CompanyID: companyB.ID, LegalName: "Avery B", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employeeA); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmployee(db, &employeeB); err != nil {
		t.Fatal(err)
	}

	runA := models.PayrollRun{
		CompanyID:   companyA.ID,
		RunNumber:   "A-001",
		PeriodStart: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC),
	}
	runB := models.PayrollRun{
		CompanyID:   companyB.ID,
		RunNumber:   "B-001",
		PeriodStart: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC),
	}
	if err := CreatePayrollRun(db, &runA); err != nil {
		t.Fatal(err)
	}
	if err := CreatePayrollRun(db, &runB); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollEntry{
		CompanyID:    companyA.ID,
		PayrollRunID: runA.ID,
		EmployeeID:   employeeA.ID,
		GrossPay:     decimal.NewFromInt(1000),
		NetPay:       decimal.NewFromInt(900),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollEntry{
		CompanyID:    companyB.ID,
		PayrollRunID: runB.ID,
		EmployeeID:   employeeB.ID,
		GrossPay:     decimal.NewFromInt(2000),
		NetPay:       decimal.NewFromInt(1800),
	}).Error; err != nil {
		t.Fatal(err)
	}

	run, entries, err := GetPayrollRunWithEntries(db, companyA.ID, runA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != runA.ID {
		t.Fatalf("run ID = %d, want %d", run.ID, runA.ID)
	}
	if len(entries) != 1 || entries[0].CompanyID != companyA.ID || entries[0].Employee.SearchName() != employeeA.SearchName() {
		t.Fatalf("entries not scoped/preloaded correctly: %+v", entries)
	}
	if _, _, err := GetPayrollRunWithEntries(db, companyB.ID, runA.ID); err == nil {
		t.Fatal("expected cross-company run lookup to fail")
	}
}

func TestFinalizePayrollRunApprovesEntriesAndLocksRun(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Finalize Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.Employee{CompanyID: company.ID, LegalName: "Avery Final", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employee); err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:   company.ID,
		RunNumber:   "FIN-001",
		PeriodStart: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunCalculated,
	}
	if err := CreatePayrollRun(db, &run); err != nil {
		t.Fatal(err)
	}
	entry := models.PayrollEntry{
		CompanyID:    company.ID,
		PayrollRunID: run.ID,
		EmployeeID:   employee.ID,
		GrossPay:     decimal.NewFromInt(1000),
		NetPay:       decimal.NewFromInt(900),
		Status:       models.PayrollEntryDraft,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}

	actorID := uuid.New()
	if err := FinalizePayrollRun(db, company.ID, run.ID, &actorID); err != nil {
		t.Fatal(err)
	}

	var finalized models.PayrollRun
	if err := db.First(&finalized, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if finalized.Status != models.PayrollRunFinalized {
		t.Fatalf("run status = %q, want finalized", finalized.Status)
	}
	if finalized.FinalizedAt == nil || finalized.FinalizedByUserID == nil || *finalized.FinalizedByUserID != actorID {
		t.Fatalf("finalization metadata not set: %+v", finalized)
	}
	var approved models.PayrollEntry
	if err := db.First(&approved, entry.ID).Error; err != nil {
		t.Fatal(err)
	}
	if approved.Status != models.PayrollEntryApproved {
		t.Fatalf("entry status = %q, want approved", approved.Status)
	}
	if _, err := GeneratePayrollEntriesForRun(db, company.ID, run.ID, nil); err == nil {
		t.Fatal("expected finalized run recalculation to fail")
	}
}

func TestFinalizePayrollRunRejectsDraftOrEmptyRuns(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Reject Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	draftRun := models.PayrollRun{
		CompanyID:   company.ID,
		RunNumber:   "DRAFT",
		PeriodStart: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunDraft,
	}
	if err := CreatePayrollRun(db, &draftRun); err != nil {
		t.Fatal(err)
	}
	if err := FinalizePayrollRun(db, company.ID, draftRun.ID, nil); !errors.Is(err, ErrPayrollRunNotCalculated) {
		t.Fatalf("draft finalize err = %v, want ErrPayrollRunNotCalculated", err)
	}

	emptyRun := models.PayrollRun{
		CompanyID:   company.ID,
		RunNumber:   "EMPTY",
		PeriodStart: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunCalculated,
	}
	if err := CreatePayrollRun(db, &emptyRun); err != nil {
		t.Fatal(err)
	}
	if err := FinalizePayrollRun(db, company.ID, emptyRun.ID, nil); !errors.Is(err, ErrPayrollRunNoEntries) {
		t.Fatalf("empty finalize err = %v, want ErrPayrollRunNoEntries", err)
	}
}

func newPayrollCalcTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Company{},
		&models.Employee{},
		&models.PayrollRun{},
		&models.PayrollEntry{},
		&models.PayrollEarningCode{},
		&models.PayrollEntryEarning{},
		&models.Account{},
		&models.JournalEntry{},
		&models.JournalLine{},
		&models.LedgerEntry{},
		&models.ChequeBankAccount{},
		&models.Cheque{},
		&models.PayrollRemittance{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}
