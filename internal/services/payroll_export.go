package services

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"balanciz/internal/models"
)

func BuildPayrollRunEntriesCSV(run models.PayrollRun, entries []models.PayrollEntry) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)

	if err := w.Write([]string{
		"Run Number",
		"Period Start",
		"Period End",
		"Pay Date",
		"Entry ID",
		"Employee No",
		"Employee Name",
		"Entry Status",
		"Hours",
		"Pay Rate",
		"Gross Pay",
		"CPP Employee",
		"CPP2 Employee",
		"EI Employee",
		"Federal Tax",
		"Provincial Tax",
		"Total Deductions",
		"Net Pay",
		"CPP Employer",
		"CPP2 Employer",
		"EI Employer",
		"Payment Type",
	}); err != nil {
		return "", err
	}

	for _, entry := range entries {
		if err := w.Write([]string{
			run.RunNumber,
			formatPayrollDate(run.PeriodStart),
			formatPayrollDate(run.PeriodEnd),
			formatPayrollDate(run.PayDate),
			fmt.Sprintf("%d", entry.ID),
			entry.Employee.EmployeeNo,
			entry.Employee.SearchName(),
			string(entry.Status),
			entry.Hours.StringFixed(4),
			entry.PayRate.StringFixed(6),
			entry.GrossPay.StringFixed(2),
			entry.CPPEmployee.StringFixed(2),
			entry.CPP2Employee.StringFixed(2),
			entry.EIEmployee.StringFixed(2),
			entry.FederalTax.StringFixed(2),
			entry.ProvincialTax.StringFixed(2),
			entry.TotalDeductions.StringFixed(2),
			entry.NetPay.StringFixed(2),
			entry.CPPEmployer.StringFixed(2),
			entry.CPP2Employer.StringFixed(2),
			entry.EIEmployer.StringFixed(2),
			entry.PaymentType,
		}); err != nil {
			return "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func PayrollRunExportFilename(run models.PayrollRun) string {
	name := strings.TrimSpace(run.RunNumber)
	if name == "" {
		name = fmt.Sprintf("run-%d", run.ID)
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, name)
	return "payroll-" + strings.Trim(name, "-") + ".csv"
}

func formatPayrollDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
