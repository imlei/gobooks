package services

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
)

func TestBuildPayrollRunEntriesCSVOmitsSensitiveEmployeeFields(t *testing.T) {
	run := models.PayrollRun{
		ID:          7,
		RunNumber:   "PAY 2026/01",
		PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		PayDate:     time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
	}
	entries := []models.PayrollEntry{
		{
			ID:              11,
			Employee:        models.Employee{EmployeeNo: "E-001", LegalName: "Avery Export", Email: "avery@example.com", Mobile: "555-1111", SINLast4: "1234", DateOfBirth: ptrTime(time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC))},
			Hours:           decimal.NewFromFloat(80),
			PayRate:         decimal.NewFromFloat(25),
			GrossPay:        decimal.NewFromFloat(2000),
			CPPEmployee:     decimal.NewFromFloat(100),
			CPP2Employee:    decimal.NewFromFloat(5),
			EIEmployee:      decimal.NewFromFloat(33),
			FederalTax:      decimal.NewFromFloat(120),
			ProvincialTax:   decimal.NewFromFloat(70),
			TotalDeductions: decimal.NewFromFloat(328),
			NetPay:          decimal.NewFromFloat(1672),
			CPPEmployer:     decimal.NewFromFloat(100),
			CPP2Employer:    decimal.NewFromFloat(5),
			EIEmployer:      decimal.NewFromFloat(46.20),
			PaymentType:     "cheque",
			Status:          models.PayrollEntryApproved,
		},
	}

	out, err := BuildPayrollRunEntriesCSV(run, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Avery Export") || !strings.Contains(out, "2000.00") || !strings.Contains(out, "1672.00") {
		t.Fatalf("expected payroll values in CSV, got:\n%s", out)
	}
	for _, forbidden := range []string{"avery@example.com", "555-1111", "1234", "1990-01-02"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("CSV leaked sensitive field %q:\n%s", forbidden, out)
		}
	}
	if got := PayrollRunExportFilename(run); got != "payroll-PAY-2026-01.csv" {
		t.Fatalf("filename = %q, want payroll-PAY-2026-01.csv", got)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
