package services

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

type PayrollSummaryReport struct {
	FromDate time.Time
	ToDate   time.Time

	Rows []PayrollSummaryRow

	TotalGross                  decimal.Decimal
	TotalEmployeeDeductions     decimal.Decimal
	TotalEmployerContributions  decimal.Decimal
	TotalNetPay                 decimal.Decimal
	TotalStatutoryRemittanceDue decimal.Decimal
	TotalRemitted               decimal.Decimal
	TotalOutstandingRemittance  decimal.Decimal
}

type PayrollSummaryRow struct {
	PayrollRunID uint
	RunNumber    string
	PeriodStart  time.Time
	PeriodEnd    time.Time
	PayDate      time.Time
	Status       models.PayrollRunStatus

	GrossPay                 decimal.Decimal
	EmployeeDeductions       decimal.Decimal
	EmployerContributions    decimal.Decimal
	NetPay                   decimal.Decimal
	StatutoryRemittanceDue   decimal.Decimal
	RemittanceID             uint
	RemittanceNumber         string
	RemittanceStatus         models.PayrollRemittanceStatus
	RemittanceTotal          decimal.Decimal
	OutstandingRemittanceDue decimal.Decimal
}

func BuildPayrollSummaryReport(db *gorm.DB, companyID uint, from, to time.Time) (PayrollSummaryReport, error) {
	if db == nil {
		return PayrollSummaryReport{}, fmt.Errorf("BuildPayrollSummaryReport: db is required")
	}
	if companyID == 0 {
		return PayrollSummaryReport{}, fmt.Errorf("BuildPayrollSummaryReport: companyID is required")
	}
	if from.IsZero() || to.IsZero() {
		return PayrollSummaryReport{}, fmt.Errorf("BuildPayrollSummaryReport: date range is required")
	}
	if to.Before(from) {
		from, to = to, from
	}

	var runs []models.PayrollRun
	if err := db.Where("company_id = ? AND pay_date BETWEEN ? AND ?", companyID, from, to).
		Order("pay_date asc, id asc").
		Find(&runs).Error; err != nil {
		return PayrollSummaryReport{}, err
	}

	remittances := map[uint]models.PayrollRemittance{}
	if len(runs) > 0 {
		runIDs := make([]uint, 0, len(runs))
		for _, run := range runs {
			runIDs = append(runIDs, run.ID)
		}
		var rows []models.PayrollRemittance
		if err := db.Where("company_id = ? AND payroll_run_id IN ?", companyID, runIDs).Find(&rows).Error; err != nil {
			return PayrollSummaryReport{}, err
		}
		for _, remittance := range rows {
			remittances[remittance.PayrollRunID] = remittance
		}
	}

	report := PayrollSummaryReport{FromDate: from, ToDate: to, Rows: make([]PayrollSummaryRow, 0, len(runs))}
	for _, run := range runs {
		row := PayrollSummaryRow{
			PayrollRunID:             run.ID,
			RunNumber:                run.RunNumber,
			PeriodStart:              run.PeriodStart,
			PeriodEnd:                run.PeriodEnd,
			PayDate:                  run.PayDate,
			Status:                   run.Status,
			GrossPay:                 run.TotalGross.Round(2),
			EmployeeDeductions:       run.TotalDeductions.Round(2),
			EmployerContributions:    payrollEmployerContributions(run),
			NetPay:                   run.TotalNetPay.Round(2),
			StatutoryRemittanceDue:   payrollStatutoryRemittanceDue(run),
			OutstandingRemittanceDue: payrollStatutoryRemittanceDue(run),
		}
		if remittance, ok := remittances[run.ID]; ok {
			row.RemittanceID = remittance.ID
			row.RemittanceNumber = remittance.RemittanceNumber
			row.RemittanceStatus = remittance.Status
			row.RemittanceTotal = remittance.TotalAmount.Round(2)
			if remittance.Status == models.PayrollRemittancePaid {
				row.OutstandingRemittanceDue = decimal.Zero
				report.TotalRemitted = report.TotalRemitted.Add(row.RemittanceTotal)
			} else if remittance.Status == models.PayrollRemittanceDraft {
				row.OutstandingRemittanceDue = row.RemittanceTotal
			}
		}

		report.Rows = append(report.Rows, row)
		report.TotalGross = report.TotalGross.Add(row.GrossPay)
		report.TotalEmployeeDeductions = report.TotalEmployeeDeductions.Add(row.EmployeeDeductions)
		report.TotalEmployerContributions = report.TotalEmployerContributions.Add(row.EmployerContributions)
		report.TotalNetPay = report.TotalNetPay.Add(row.NetPay)
		report.TotalStatutoryRemittanceDue = report.TotalStatutoryRemittanceDue.Add(row.StatutoryRemittanceDue)
		report.TotalOutstandingRemittance = report.TotalOutstandingRemittance.Add(row.OutstandingRemittanceDue)
	}

	return report, nil
}

func ExportPayrollSummaryCSV(report PayrollSummaryReport, w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"Run Number",
		"Period Start",
		"Period End",
		"Pay Date",
		"Run Status",
		"Gross Pay",
		"Employee Deductions",
		"Employer Contributions",
		"Net Pay",
		"Statutory Remittance Due",
		"Remittance Number",
		"Remittance Status",
		"Remittance Total",
		"Outstanding Remittance",
	}); err != nil {
		return err
	}
	for _, row := range report.Rows {
		if err := cw.Write([]string{
			row.RunNumber,
			formatPayrollDate(row.PeriodStart),
			formatPayrollDate(row.PeriodEnd),
			formatPayrollDate(row.PayDate),
			string(row.Status),
			row.GrossPay.StringFixed(2),
			row.EmployeeDeductions.StringFixed(2),
			row.EmployerContributions.StringFixed(2),
			row.NetPay.StringFixed(2),
			row.StatutoryRemittanceDue.StringFixed(2),
			row.RemittanceNumber,
			string(row.RemittanceStatus),
			row.RemittanceTotal.StringFixed(2),
			row.OutstandingRemittanceDue.StringFixed(2),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func PayrollSummaryExportFilename(from, to time.Time) string {
	return "payroll-summary-" + from.Format("2006-01-02") + "-to-" + to.Format("2006-01-02") + ".csv"
}

func payrollEmployerContributions(run models.PayrollRun) decimal.Decimal {
	return run.TotalEmployerCPP.Add(run.TotalEmployerCPP2).Add(run.TotalEmployerEI).Round(2)
}

func payrollStatutoryRemittanceDue(run models.PayrollRun) decimal.Decimal {
	return run.TotalEmployeeTax.
		Add(run.TotalEmployeeCPP).
		Add(run.TotalEmployeeCPP2).
		Add(run.TotalEmployeeEI).
		Add(run.TotalEmployerCPP).
		Add(run.TotalEmployerCPP2).
		Add(run.TotalEmployerEI).
		Round(2)
}

func PayrollSummaryDateLabel(from, to time.Time) string {
	parts := []string{}
	if !from.IsZero() {
		parts = append(parts, from.Format("2006-01-02"))
	}
	if !to.IsZero() {
		parts = append(parts, to.Format("2006-01-02"))
	}
	return strings.Join(parts, " to ")
}
