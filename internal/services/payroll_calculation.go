package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"balanciz/internal/models"
	"balanciz/internal/payroll/calculator"
)

var (
	ErrPayrollEmployeeRequired = errors.New("payroll employee is required")
	ErrPayrollUnsupportedYear  = errors.New("payroll tax year is not supported")
)

type PayrollEntryCalculationInput struct {
	Run         models.PayrollRun
	Employee    models.Employee
	Hours       *decimal.Decimal
	GrossPay    *decimal.Decimal
	YTDGross    decimal.Decimal
	YTDCPP      decimal.Decimal
	YTDCPP2     decimal.Decimal
	YTDEI       decimal.Decimal
	PaymentType string
}

func CalculatePayrollEntry(in PayrollEntryCalculationInput) (models.PayrollEntry, error) {
	if in.Run.ID == 0 || in.Run.CompanyID == 0 {
		return models.PayrollEntry{}, ErrPayrollRunRequired
	}
	if in.Employee.ID == 0 || in.Employee.CompanyID == 0 {
		return models.PayrollEntry{}, ErrPayrollEmployeeRequired
	}
	if in.Run.CompanyID != in.Employee.CompanyID {
		return models.PayrollEntry{}, fmt.Errorf("CalculatePayrollEntry: run and employee company mismatch")
	}

	hours := defaultPayrollHours(in.Run, in.Employee)
	if in.Hours != nil {
		hours = in.Hours.Round(4)
	}
	gross := defaultPayrollGross(in.Run, in.Employee, hours)
	if in.GrossPay != nil {
		gross = in.GrossPay.Round(2)
	}

	entry := models.PayrollEntry{
		CompanyID:       in.Run.CompanyID,
		PayrollRunID:    in.Run.ID,
		EmployeeID:      in.Employee.ID,
		Hours:           hours,
		PayRate:         in.Employee.PayRate,
		GrossPay:        gross,
		YTDGross:        in.YTDGross.Round(2),
		YTDCPPEmployee:  in.YTDCPP.Round(2),
		YTDCPP2Employee: in.YTDCPP2.Round(2),
		YTDEIEmployee:   in.YTDEI.Round(2),
		PaymentType:     strings.TrimSpace(in.PaymentType),
		Status:          models.PayrollEntryDraft,
	}
	if entry.PaymentType == "" {
		entry.PaymentType = "cheque"
	}

	if in.Employee.MemberType != "" && in.Employee.MemberType != models.EmployeeMemberEmployee {
		entry.NetPay = gross
		entry.CalculationSnapshot = payrollSnapshot(in.Run, in.Employee, "contractor_no_source_deductions", nil)
		return entry, nil
	}

	rates, err := ratesForPayrollYear(in.Run.PayDate.Year())
	if err != nil {
		return models.PayrollEntry{}, err
	}
	province := strings.ToUpper(strings.TrimSpace(in.Employee.ProvinceOfEmployment))
	if province == "" {
		province = strings.ToUpper(strings.TrimSpace(in.Employee.AddrProvince))
	}
	if province == "" {
		province = "BC"
	}

	result := calculator.Calculate(calculator.Input{
		Province:   province,
		PayPeriods: in.Run.PaysPerYear,
		GrossPay:   decimalToFloat(gross),
		TD1Federal: decimalToFloat(in.Employee.TD1Federal),
		TD1Prov:    decimalToFloat(in.Employee.TD1Provincial),
		YTDGross:   decimalToFloat(in.YTDGross),
		YTDCPPEe:   decimalToFloat(in.YTDCPP),
		YTDCPP2Ee:  decimalToFloat(in.YTDCPP2),
		YTDEIEe:    decimalToFloat(in.YTDEI),
	}, rates)

	entry.CPPEmployee = money(result.CPPEmployee)
	entry.CPP2Employee = money(result.CPP2Employee)
	entry.EIEmployee = money(result.EIEmployee)
	entry.FederalTax = money(result.FederalTax)
	entry.ProvincialTax = money(result.ProvincialTax)
	entry.TotalDeductions = money(result.TotalDeductions)
	entry.NetPay = money(result.NetPay)
	entry.CPPEmployer = money(result.CPPEmployer)
	entry.CPP2Employer = money(result.CPP2Employer)
	entry.EIEmployer = money(result.EIEmployer)
	entry.CalculationSnapshot = payrollSnapshot(in.Run, in.Employee, "simpletask_internal_calculator", result)

	return entry, nil
}

func GeneratePayrollEntriesForRun(db *gorm.DB, companyID, runID uint, employeeIDs []uint) ([]models.PayrollEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("GeneratePayrollEntriesForRun: db is required")
	}
	if companyID == 0 || runID == 0 {
		return nil, ErrPayrollRunRequired
	}
	var run models.PayrollRun
	if err := db.Where("id = ? AND company_id = ?", runID, companyID).First(&run).Error; err != nil {
		return nil, err
	}
	if run.Status == models.PayrollRunFinalized || run.Status == models.PayrollRunVoided {
		return nil, fmt.Errorf("GeneratePayrollEntriesForRun: payroll run status %q cannot be recalculated", run.Status)
	}

	employeesQuery := db.Where("company_id = ?", companyID)
	if len(employeeIDs) > 0 {
		employeesQuery = employeesQuery.Where("id IN ?", employeeIDs)
	} else {
		employeesQuery = employeesQuery.Where("status = ?", models.EmployeeStatusActive)
	}
	var employees []models.Employee
	if err := employeesQuery.Order("legal_name asc, id asc").Find(&employees).Error; err != nil {
		return nil, err
	}

	entries := make([]models.PayrollEntry, 0, len(employees))
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ? AND payroll_run_id = ?", companyID, run.ID).
			Delete(&models.PayrollEntry{}).Error; err != nil {
			return err
		}

		totals := payrollRunTotals{}
		for _, employee := range employees {
			ytd, err := loadPayrollYTD(tx, companyID, employee.ID, run.PayDate)
			if err != nil {
				return err
			}
			entry, err := CalculatePayrollEntry(PayrollEntryCalculationInput{
				Run:      run,
				Employee: employee,
				YTDGross: ytd.Gross,
				YTDCPP:   ytd.CPP,
				YTDCPP2:  ytd.CPP2,
				YTDEI:    ytd.EI,
			})
			if err != nil {
				return err
			}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
			entries = append(entries, entry)
			totals.add(entry)
		}

		updates := map[string]any{
			"status":               models.PayrollRunCalculated,
			"total_gross":          totals.Gross,
			"total_employee_tax":   totals.EmployeeTax,
			"total_employee_cpp":   totals.EmployeeCPP,
			"total_employee_cpp2":  totals.EmployeeCPP2,
			"total_employee_ei":    totals.EmployeeEI,
			"total_employer_cpp":   totals.EmployerCPP,
			"total_employer_cpp2":  totals.EmployerCPP2,
			"total_employer_ei":    totals.EmployerEI,
			"total_deductions":     totals.Deductions,
			"total_net_pay":        totals.NetPay,
			"calculation_snapshot": payrollRunSnapshot(run, len(entries)),
		}
		return tx.Model(&models.PayrollRun{}).
			Where("id = ? AND company_id = ?", run.ID, companyID).
			Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

type payrollYTD struct {
	Gross decimal.Decimal
	CPP   decimal.Decimal
	CPP2  decimal.Decimal
	EI    decimal.Decimal
}

func loadPayrollYTD(db *gorm.DB, companyID, employeeID uint, payDate time.Time) (payrollYTD, error) {
	start := time.Date(payDate.Year(), 1, 1, 0, 0, 0, 0, payDate.Location())
	type row struct {
		Gross decimal.Decimal
		CPP   decimal.Decimal
		CPP2  decimal.Decimal
		EI    decimal.Decimal
	}
	var r row
	err := db.Table("payroll_entries").
		Select(`
			COALESCE(SUM(payroll_entries.gross_pay), 0) AS gross,
			COALESCE(SUM(payroll_entries.cpp_employee), 0) AS cpp,
			COALESCE(SUM(payroll_entries.cpp2_employee), 0) AS cpp2,
			COALESCE(SUM(payroll_entries.ei_employee), 0) AS ei`).
		Joins("JOIN payroll_runs ON payroll_runs.id = payroll_entries.payroll_run_id").
		Where("payroll_entries.company_id = ?", companyID).
		Where("payroll_entries.employee_id = ?", employeeID).
		Where("payroll_runs.status = ?", models.PayrollRunFinalized).
		Where("payroll_runs.pay_date >= ? AND payroll_runs.pay_date < ?", start, payDate).
		Scan(&r).Error
	if err != nil {
		return payrollYTD{}, err
	}
	return payrollYTD{Gross: r.Gross, CPP: r.CPP, CPP2: r.CPP2, EI: r.EI}, nil
}

type payrollRunTotals struct {
	Gross        decimal.Decimal
	EmployeeTax  decimal.Decimal
	EmployeeCPP  decimal.Decimal
	EmployeeCPP2 decimal.Decimal
	EmployeeEI   decimal.Decimal
	EmployerCPP  decimal.Decimal
	EmployerCPP2 decimal.Decimal
	EmployerEI   decimal.Decimal
	Deductions   decimal.Decimal
	NetPay       decimal.Decimal
}

func (t *payrollRunTotals) add(entry models.PayrollEntry) {
	t.Gross = t.Gross.Add(entry.GrossPay)
	t.EmployeeTax = t.EmployeeTax.Add(entry.FederalTax).Add(entry.ProvincialTax)
	t.EmployeeCPP = t.EmployeeCPP.Add(entry.CPPEmployee)
	t.EmployeeCPP2 = t.EmployeeCPP2.Add(entry.CPP2Employee)
	t.EmployeeEI = t.EmployeeEI.Add(entry.EIEmployee)
	t.EmployerCPP = t.EmployerCPP.Add(entry.CPPEmployer)
	t.EmployerCPP2 = t.EmployerCPP2.Add(entry.CPP2Employer)
	t.EmployerEI = t.EmployerEI.Add(entry.EIEmployer)
	t.Deductions = t.Deductions.Add(entry.TotalDeductions)
	t.NetPay = t.NetPay.Add(entry.NetPay)
}

func defaultPayrollHours(run models.PayrollRun, employee models.Employee) decimal.Decimal {
	if !employee.HoursPerWeek.IsZero() && run.PaysPerYear > 0 {
		return employee.HoursPerWeek.Mul(decimal.NewFromInt(52)).Div(decimal.NewFromInt(int64(run.PaysPerYear))).Round(4)
	}
	return decimal.Zero
}

func defaultPayrollGross(run models.PayrollRun, employee models.Employee, hours decimal.Decimal) decimal.Decimal {
	unit := strings.ToLower(strings.TrimSpace(employee.PayRateUnit))
	switch unit {
	case "annual", "annually", "year", "yearly":
		if run.PaysPerYear > 0 {
			return employee.PayRate.Div(decimal.NewFromInt(int64(run.PaysPerYear))).Round(2)
		}
	case "month", "monthly":
		if run.PaysPerYear > 0 {
			return employee.PayRate.Mul(decimal.NewFromInt(12)).Div(decimal.NewFromInt(int64(run.PaysPerYear))).Round(2)
		}
	case "period", "per_period":
		return employee.PayRate.Round(2)
	}
	return employee.PayRate.Mul(hours).Round(2)
}

func ratesForPayrollYear(year int) (calculator.TaxYear, error) {
	switch year {
	case 2025:
		return calculator.Rates2025(), nil
	case 2026:
		return calculator.Rates2026(), nil
	default:
		return calculator.TaxYear{}, fmt.Errorf("%w: %d", ErrPayrollUnsupportedYear, year)
	}
}

func money(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v).Round(2)
}

func decimalToFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

func payrollSnapshot(run models.PayrollRun, employee models.Employee, source string, result any) datatypes.JSON {
	payload := map[string]any{
		"source":        source,
		"tax_year":      run.PayDate.Year(),
		"pay_frequency": run.PayFrequency,
		"pays_per_year": run.PaysPerYear,
		"province":      employee.ProvinceOfEmployment,
		"member_type":   employee.MemberType,
		"salary_type":   employee.SalaryType,
	}
	if result != nil {
		payload["result"] = result
	}
	b, _ := json.Marshal(payload)
	return datatypes.JSON(b)
}

func payrollRunSnapshot(run models.PayrollRun, entryCount int) datatypes.JSON {
	payload := map[string]any{
		"source":        "balanciz_payroll_generation",
		"tax_year":      run.PayDate.Year(),
		"pay_frequency": run.PayFrequency,
		"pays_per_year": run.PaysPerYear,
		"entry_count":   entryCount,
	}
	b, _ := json.Marshal(payload)
	return datatypes.JSON(b)
}
