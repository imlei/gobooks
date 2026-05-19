package services

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

type PayrollEmployeeHistoryReport struct {
	FromDate   time.Time
	ToDate     time.Time
	EmployeeID uint

	Rows []PayrollEmployeeHistoryRow

	TotalGross              decimal.Decimal
	TotalDeductions         decimal.Decimal
	TotalNetPay             decimal.Decimal
	TotalEmployeeTax        decimal.Decimal
	TotalEmployeeCPP        decimal.Decimal
	TotalEmployeeEI         decimal.Decimal
	TotalEmployerCPP        decimal.Decimal
	TotalEmployerEI         decimal.Decimal
	TotalEmployerContribute decimal.Decimal
}

type PayrollEmployeeHistoryRow struct {
	PayrollRunID uint
	RunNumber    string
	PayDate      time.Time
	PeriodStart  time.Time
	PeriodEnd    time.Time

	PayrollEntryID uint
	EmployeeID     uint
	EmployeeNo     string
	EmployeeName   string
	Status         models.PayrollEntryStatus

	Hours           decimal.Decimal
	GrossPay        decimal.Decimal
	EmployeeTax     decimal.Decimal
	EmployeeCPP     decimal.Decimal
	EmployeeEI      decimal.Decimal
	TotalDeductions decimal.Decimal
	NetPay          decimal.Decimal
	EmployerCPP     decimal.Decimal
	EmployerEI      decimal.Decimal
}

func BuildPayrollEmployeeHistoryReport(db *gorm.DB, companyID, employeeID uint, from, to time.Time) (PayrollEmployeeHistoryReport, error) {
	if db == nil {
		return PayrollEmployeeHistoryReport{}, fmt.Errorf("BuildPayrollEmployeeHistoryReport: db is required")
	}
	if companyID == 0 {
		return PayrollEmployeeHistoryReport{}, fmt.Errorf("BuildPayrollEmployeeHistoryReport: companyID is required")
	}
	if from.IsZero() || to.IsZero() {
		return PayrollEmployeeHistoryReport{}, fmt.Errorf("BuildPayrollEmployeeHistoryReport: date range is required")
	}
	if to.Before(from) {
		from, to = to, from
	}

	q := db.Preload("Employee").
		Preload("PayrollRun").
		Joins("JOIN payroll_runs ON payroll_runs.id = payroll_entries.payroll_run_id AND payroll_runs.company_id = payroll_entries.company_id").
		Where("payroll_entries.company_id = ? AND payroll_runs.pay_date BETWEEN ? AND ?", companyID, from, to)
	if employeeID != 0 {
		q = q.Where("payroll_entries.employee_id = ?", employeeID)
	}

	var entries []models.PayrollEntry
	if err := q.Order("payroll_runs.pay_date asc, payroll_entries.employee_id asc, payroll_entries.id asc").Find(&entries).Error; err != nil {
		return PayrollEmployeeHistoryReport{}, err
	}

	report := PayrollEmployeeHistoryReport{
		FromDate:   from,
		ToDate:     to,
		EmployeeID: employeeID,
		Rows:       make([]PayrollEmployeeHistoryRow, 0, len(entries)),
	}
	for _, entry := range entries {
		row := PayrollEmployeeHistoryRow{
			PayrollRunID:    entry.PayrollRunID,
			RunNumber:       entry.PayrollRun.RunNumber,
			PayDate:         entry.PayrollRun.PayDate,
			PeriodStart:     entry.PayrollRun.PeriodStart,
			PeriodEnd:       entry.PayrollRun.PeriodEnd,
			PayrollEntryID:  entry.ID,
			EmployeeID:      entry.EmployeeID,
			EmployeeNo:      entry.Employee.EmployeeNo,
			EmployeeName:    entry.Employee.SearchName(),
			Status:          entry.Status,
			Hours:           entry.Hours.Round(4),
			GrossPay:        entry.GrossPay.Round(2),
			EmployeeTax:     entry.FederalTax.Add(entry.ProvincialTax).Round(2),
			EmployeeCPP:     entry.CPPEmployee.Add(entry.CPP2Employee).Round(2),
			EmployeeEI:      entry.EIEmployee.Round(2),
			TotalDeductions: entry.TotalDeductions.Round(2),
			NetPay:          entry.NetPay.Round(2),
			EmployerCPP:     entry.CPPEmployer.Add(entry.CPP2Employer).Round(2),
			EmployerEI:      entry.EIEmployer.Round(2),
		}
		report.Rows = append(report.Rows, row)
		report.TotalGross = report.TotalGross.Add(row.GrossPay)
		report.TotalDeductions = report.TotalDeductions.Add(row.TotalDeductions)
		report.TotalNetPay = report.TotalNetPay.Add(row.NetPay)
		report.TotalEmployeeTax = report.TotalEmployeeTax.Add(row.EmployeeTax)
		report.TotalEmployeeCPP = report.TotalEmployeeCPP.Add(row.EmployeeCPP)
		report.TotalEmployeeEI = report.TotalEmployeeEI.Add(row.EmployeeEI)
		report.TotalEmployerCPP = report.TotalEmployerCPP.Add(row.EmployerCPP)
		report.TotalEmployerEI = report.TotalEmployerEI.Add(row.EmployerEI)
		report.TotalEmployerContribute = report.TotalEmployerContribute.Add(row.EmployerCPP).Add(row.EmployerEI)
	}
	return report, nil
}

func ExportPayrollEmployeeHistoryCSV(report PayrollEmployeeHistoryReport, w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"Employee No",
		"Employee Name",
		"Run Number",
		"Period Start",
		"Period End",
		"Pay Date",
		"Entry Status",
		"Hours",
		"Gross Pay",
		"Employee Tax",
		"Employee CPP",
		"Employee EI",
		"Total Deductions",
		"Net Pay",
		"Employer CPP",
		"Employer EI",
	}); err != nil {
		return err
	}
	for _, row := range report.Rows {
		if err := cw.Write([]string{
			row.EmployeeNo,
			row.EmployeeName,
			row.RunNumber,
			formatPayrollDate(row.PeriodStart),
			formatPayrollDate(row.PeriodEnd),
			formatPayrollDate(row.PayDate),
			string(row.Status),
			row.Hours.StringFixed(4),
			row.GrossPay.StringFixed(2),
			row.EmployeeTax.StringFixed(2),
			row.EmployeeCPP.StringFixed(2),
			row.EmployeeEI.StringFixed(2),
			row.TotalDeductions.StringFixed(2),
			row.NetPay.StringFixed(2),
			row.EmployerCPP.StringFixed(2),
			row.EmployerEI.StringFixed(2),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func PayrollEmployeeHistoryExportFilename(from, to time.Time) string {
	return "payroll-employee-history-" + from.Format("2006-01-02") + "-to-" + to.Format("2006-01-02") + ".csv"
}
