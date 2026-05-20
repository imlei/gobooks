package services

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
)

func TestBuildPayrollSummaryReportAggregatesRunsAndRemittances(t *testing.T) {
	db := newPayrollCalcTestDB(t)
	company := models.Company{Name: "Payroll Report Co", BaseCurrencyCode: "CAD", IsActive: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}

	runA := models.PayrollRun{
		CompanyID:        company.ID,
		RunNumber:        "PAY-A",
		PeriodStart:      time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2027, 3, 15, 0, 0, 0, 0, time.UTC),
		PayDate:          time.Date(2027, 3, 16, 0, 0, 0, 0, time.UTC),
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
	runB := models.PayrollRun{
		CompanyID:        company.ID,
		RunNumber:        "PAY-B",
		PeriodStart:      time.Date(2027, 3, 16, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC),
		PayDate:          time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC),
		Status:           models.PayrollRunFinalized,
		TotalGross:       decimal.NewFromInt(500),
		TotalEmployeeTax: decimal.NewFromInt(60),
		TotalEmployeeCPP: decimal.NewFromInt(20),
		TotalEmployeeEI:  decimal.NewFromInt(10),
		TotalDeductions:  decimal.NewFromInt(90),
		TotalNetPay:      decimal.NewFromInt(410),
		TotalEmployerCPP: decimal.NewFromInt(20),
		TotalEmployerEI:  decimal.NewFromInt(10),
	}
	if err := db.Create(&runA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&runB).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollRemittance{
		CompanyID:        company.ID,
		PayrollRunID:     runA.ID,
		RemittanceNumber: "PAY-A",
		Status:           models.PayrollRemittancePaid,
		PeriodStart:      runA.PeriodStart,
		PeriodEnd:        runA.PeriodEnd,
		DueDate:          time.Date(2027, 4, 15, 0, 0, 0, 0, time.UTC),
		CPPAmount:        decimal.NewFromInt(130),
		EIAmount:         decimal.NewFromInt(90),
		TaxAmount:        decimal.NewFromInt(180),
		TotalAmount:      decimal.NewFromInt(400),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PayrollRemittance{
		CompanyID:        company.ID,
		PayrollRunID:     runB.ID,
		RemittanceNumber: "PAY-B",
		Status:           models.PayrollRemittanceDraft,
		PeriodStart:      runB.PeriodStart,
		PeriodEnd:        runB.PeriodEnd,
		DueDate:          time.Date(2027, 4, 30, 0, 0, 0, 0, time.UTC),
		CPPAmount:        decimal.NewFromInt(40),
		EIAmount:         decimal.NewFromInt(20),
		TaxAmount:        decimal.NewFromInt(60),
		TotalAmount:      decimal.NewFromInt(120),
	}).Error; err != nil {
		t.Fatal(err)
	}

	report, err := BuildPayrollSummaryReport(db, company.ID, time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 4, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(report.Rows))
	}
	if !report.TotalGross.Equal(decimal.NewFromInt(1500)) {
		t.Fatalf("gross = %s, want 1500", report.TotalGross)
	}
	if !report.TotalStatutoryRemittanceDue.Equal(decimal.NewFromInt(520)) {
		t.Fatalf("statutory due = %s, want 520", report.TotalStatutoryRemittanceDue)
	}
	if !report.TotalRemitted.Equal(decimal.NewFromInt(400)) {
		t.Fatalf("remitted = %s, want 400", report.TotalRemitted)
	}
	if !report.TotalOutstandingRemittance.Equal(decimal.NewFromInt(120)) {
		t.Fatalf("outstanding = %s, want 120", report.TotalOutstandingRemittance)
	}
}

func TestExportPayrollSummaryCSV(t *testing.T) {
	report := PayrollSummaryReport{
		Rows: []PayrollSummaryRow{{
			RunNumber:                "PAY-CSV",
			PeriodStart:              time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC),
			PeriodEnd:                time.Date(2027, 5, 15, 0, 0, 0, 0, time.UTC),
			PayDate:                  time.Date(2027, 5, 16, 0, 0, 0, 0, time.UTC),
			Status:                   models.PayrollRunFinalized,
			GrossPay:                 decimal.NewFromInt(100),
			EmployeeDeductions:       decimal.NewFromInt(20),
			EmployerContributions:    decimal.NewFromInt(5),
			NetPay:                   decimal.NewFromInt(80),
			StatutoryRemittanceDue:   decimal.NewFromInt(25),
			RemittanceNumber:         "REM-CSV",
			RemittanceStatus:         models.PayrollRemittancePaid,
			RemittanceTotal:          decimal.NewFromInt(25),
			OutstandingRemittanceDue: decimal.Zero,
		}},
	}
	var buf bytes.Buffer
	if err := ExportPayrollSummaryCSV(report, &buf); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{"Run Number", "PAY-CSV", "REM-CSV", "25.00"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected CSV to contain %q\n%s", want, body)
		}
	}
}
