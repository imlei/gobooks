package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

var (
	ErrEmployeeNameRequired        = errors.New("employee legal or display name is required")
	ErrPayrollRunRequired          = errors.New("payroll run is required")
	ErrPayrollRunNotCalculated     = errors.New("payroll run must be calculated before finalization")
	ErrPayrollRunNoEntries         = errors.New("payroll run has no entries to finalize")
	ErrPayrollRunCannotBeFinalized = errors.New("payroll run cannot be finalized")
)

type EmployeeListFilter struct {
	CompanyID uint
	Status    *models.EmployeeStatus
	Query     string
	Limit     int
}

func CreateEmployee(db *gorm.DB, employee *models.Employee) error {
	if db == nil {
		return fmt.Errorf("CreateEmployee: db is required")
	}
	if employee == nil {
		return fmt.Errorf("CreateEmployee: employee is required")
	}
	if employee.CompanyID == 0 {
		return fmt.Errorf("CreateEmployee: CompanyID is required")
	}
	employee.LegalName = strings.TrimSpace(employee.LegalName)
	employee.DisplayName = strings.TrimSpace(employee.DisplayName)
	employee.EmployeeNo = strings.TrimSpace(employee.EmployeeNo)
	employee.Email = strings.TrimSpace(employee.Email)
	employee.Mobile = strings.TrimSpace(employee.Mobile)
	employee.Position = strings.TrimSpace(employee.Position)
	if employee.LegalName == "" && employee.DisplayName == "" {
		return ErrEmployeeNameRequired
	}
	if employee.Status == "" {
		employee.Status = models.EmployeeStatusActive
	}
	if employee.MemberType == "" {
		employee.MemberType = models.EmployeeMemberEmployee
	}
	if employee.SalaryType == "" {
		employee.SalaryType = models.EmployeeSalaryTimeBased
	}
	if employee.PayFrequency == "" {
		employee.PayFrequency = models.PayFrequencyBiweekly
	}
	if employee.PaysPerYear == 0 {
		employee.PaysPerYear = 26
	}
	return db.Create(employee).Error
}

func ListEmployees(db *gorm.DB, filter EmployeeListFilter) ([]models.Employee, error) {
	if db == nil {
		return nil, fmt.Errorf("ListEmployees: db is required")
	}
	if filter.CompanyID == 0 {
		return nil, fmt.Errorf("ListEmployees: CompanyID is required")
	}
	q := db.Where("company_id = ?", filter.CompanyID)
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if s := strings.TrimSpace(filter.Query); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where(
			"LOWER(employee_no) LIKE ? OR LOWER(legal_name) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(position) LIKE ?",
			like, like, like, like,
		)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var employees []models.Employee
	if err := q.Order("status asc, legal_name asc, display_name asc, id asc").Limit(limit).Find(&employees).Error; err != nil {
		return nil, err
	}
	return employees, nil
}

type PayrollRunListFilter struct {
	CompanyID uint
	Status    *models.PayrollRunStatus
	Limit     int
}

func CreatePayrollRun(db *gorm.DB, run *models.PayrollRun) error {
	if db == nil {
		return fmt.Errorf("CreatePayrollRun: db is required")
	}
	if run == nil {
		return fmt.Errorf("CreatePayrollRun: run is required")
	}
	if run.CompanyID == 0 {
		return fmt.Errorf("CreatePayrollRun: CompanyID is required")
	}
	if run.PeriodStart.IsZero() || run.PeriodEnd.IsZero() || run.PayDate.IsZero() {
		return ErrPayrollRunRequired
	}
	if run.Status == "" {
		run.Status = models.PayrollRunDraft
	}
	if run.PayFrequency == "" {
		run.PayFrequency = models.PayFrequencyBiweekly
	}
	if run.PaysPerYear == 0 {
		run.PaysPerYear = 26
	}
	if run.PayrollType == "" {
		run.PayrollType = "regular"
	}
	return db.Create(run).Error
}

func ListPayrollRuns(db *gorm.DB, filter PayrollRunListFilter) ([]models.PayrollRun, error) {
	if db == nil {
		return nil, fmt.Errorf("ListPayrollRuns: db is required")
	}
	if filter.CompanyID == 0 {
		return nil, fmt.Errorf("ListPayrollRuns: CompanyID is required")
	}
	q := db.Where("company_id = ?", filter.CompanyID)
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var runs []models.PayrollRun
	if err := q.Order("pay_date desc, id desc").Limit(limit).Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

func GetPayrollRunWithEntries(db *gorm.DB, companyID, runID uint) (models.PayrollRun, []models.PayrollEntry, error) {
	if db == nil {
		return models.PayrollRun{}, nil, fmt.Errorf("GetPayrollRunWithEntries: db is required")
	}
	if companyID == 0 || runID == 0 {
		return models.PayrollRun{}, nil, ErrPayrollRunRequired
	}

	var run models.PayrollRun
	if err := db.Where("id = ? AND company_id = ?", runID, companyID).First(&run).Error; err != nil {
		return models.PayrollRun{}, nil, err
	}

	var entries []models.PayrollEntry
	if err := db.Preload("Employee").
		Where("company_id = ? AND payroll_run_id = ?", companyID, runID).
		Order("employee_id asc, id asc").
		Find(&entries).Error; err != nil {
		return models.PayrollRun{}, nil, err
	}
	return run, entries, nil
}

func FinalizePayrollRun(db *gorm.DB, companyID, runID uint, actorUserID *uuid.UUID) error {
	if db == nil {
		return fmt.Errorf("FinalizePayrollRun: db is required")
	}
	if companyID == 0 || runID == 0 {
		return ErrPayrollRunRequired
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var run models.PayrollRun
		if err := tx.Where("id = ? AND company_id = ?", runID, companyID).First(&run).Error; err != nil {
			return err
		}
		switch run.Status {
		case models.PayrollRunFinalized:
			return nil
		case models.PayrollRunCalculated:
		case models.PayrollRunVoided:
			return ErrPayrollRunCannotBeFinalized
		default:
			return ErrPayrollRunNotCalculated
		}

		var entryCount int64
		if err := tx.Model(&models.PayrollEntry{}).
			Where("company_id = ? AND payroll_run_id = ?", companyID, runID).
			Count(&entryCount).Error; err != nil {
			return err
		}
		if entryCount == 0 {
			return ErrPayrollRunNoEntries
		}

		if err := tx.Model(&models.PayrollEntry{}).
			Where("company_id = ? AND payroll_run_id = ?", companyID, runID).
			Update("status", models.PayrollEntryApproved).Error; err != nil {
			return err
		}

		now := time.Now().UTC()
		updates := map[string]any{
			"status":       models.PayrollRunFinalized,
			"finalized_at": &now,
		}
		if actorUserID != nil {
			updates["finalized_by_user_id"] = *actorUserID
		}
		return tx.Model(&models.PayrollRun{}).
			Where("id = ? AND company_id = ?", runID, companyID).
			Updates(updates).Error
	})
}
