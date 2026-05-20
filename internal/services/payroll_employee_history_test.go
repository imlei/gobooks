package services

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
)

func TestBuildPayrollEmployeeHistoryReportFiltersEmployeeAndOmitsProfileSensitiveFields(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll History Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	employeeA := models.Employee{
		CompanyID:     company.ID,
		EmployeeNo:    "E-001",
		LegalName:     "Avery History",
		Email:         "avery.private@example.test",
		Mobile:        "555-0101",
		SINCiphertext: "encrypted-sin",
		Status:        models.EmployeeStatusActive,
	}
	employeeB := models.Employee{CompanyID: company.ID, EmployeeNo: "E-002", LegalName: "Blair History", Status: models.EmployeeStatusActive}
	if err := CreateEmployee(db, &employeeA); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmployee(db, &employeeB); err != nil {
		t.Fatal(err)
	}
	run := models.PayrollRun{
		CompanyID:   company.ID,
		RunNumber:   "HIST-1",
		PeriodStart: time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2027, 6, 16, 0, 0, 0, 0, time.UTC),
		Status:      models.PayrollRunFinalized,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.PayrollEntry{
		{
			CompanyID:       company.ID,
			PayrollRunID:    run.ID,
			EmployeeID:      employeeA.ID,
			Hours:           decimal.NewFromInt(80),
			GrossPay:        decimal.NewFromInt(1000),
			FederalTax:      decimal.NewFromInt(120),
			ProvincialTax:   decimal.NewFromInt(60),
			CPPEmployee:     decimal.NewFromInt(70),
			EIEmployee:      decimal.NewFromInt(50),
			TotalDeductions: decimal.NewFromInt(300),
			NetPay:          decimal.NewFromInt(700),
			CPPEmployer:     decimal.NewFromInt(60),
			EIEmployer:      decimal.NewFromInt(40),
			Status:          models.PayrollEntryApproved,
		},
		{
			CompanyID:       company.ID,
			PayrollRunID:    run.ID,
			EmployeeID:      employeeB.ID,
			Hours:           decimal.NewFromInt(40),
			GrossPay:        decimal.NewFromInt(500),
			TotalDeductions: decimal.NewFromInt(100),
			NetPay:          decimal.NewFromInt(400),
			Status:          models.PayrollEntryApproved,
		},
	}).Error; err != nil {
		t.Fatal(err)
	}

	report, err := BuildPayrollEmployeeHistoryReport(db, company.ID, employeeA.ID, time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(report.Rows))
	}
	if !report.TotalGross.Equal(decimal.NewFromInt(1000)) || !report.TotalEmployerContribute.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("unexpected totals: %+v", report)
	}

	var buf bytes.Buffer
	if err := ExportPayrollEmployeeHistoryCSV(report, &buf); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{"E-001", "Avery History", "HIST-1", "700.00"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected CSV to contain %q\n%s", want, body)
		}
	}
	for _, forbidden := range []string{"avery.private@example.test", "555-0101", "encrypted-sin"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("payroll history CSV leaked sensitive profile field %q\n%s", forbidden, body)
		}
	}
}
