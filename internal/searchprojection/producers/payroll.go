package producers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"balanciz/internal/logging"
	"balanciz/internal/models"
	"balanciz/internal/searchprojection"
)

const (
	EntityTypeEmployee          = "employee"
	EntityTypePayrollRun        = "payroll_run"
	EntityTypePayrollEntry      = "payroll_entry"
	EntityTypePayrollRemittance = "payroll_remittance"
	EntityTypeCheque            = "cheque"
)

func ProjectEmployee(ctx context.Context, db *gorm.DB, p searchprojection.Projector, companyID, employeeID uint) error {
	if p == nil {
		return nil
	}
	if companyID == 0 {
		return errors.New("producers.ProjectEmployee: companyID is required")
	}
	var employee models.Employee
	err := db.Where("id = ? AND company_id = ?", employeeID, companyID).First(&employee).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEntityNotInCompany
		}
		return fmt.Errorf("producers.ProjectEmployee: load employee %d for company %d: %w", employeeID, companyID, err)
	}
	doc := EmployeeDocument(employee)
	if err := p.Upsert(ctx, companyID, doc); err != nil {
		logging.L().Warn("searchprojection.ProjectEmployee upsert failed",
			"employee_id", employeeID, "company_id", companyID, "err", err)
		return err
	}
	return nil
}

func EmployeeDocument(employee models.Employee) searchprojection.Document {
	title := employee.SearchName()
	if title == "" {
		title = "Employee " + strconv.FormatUint(uint64(employee.ID), 10)
	}
	subtitleParts := []string{"Employee"}
	if employee.EmployeeNo != "" {
		subtitleParts = append(subtitleParts, employee.EmployeeNo)
	}
	if employee.Position != "" {
		subtitleParts = append(subtitleParts, employee.Position)
	}
	if employee.ProvinceOfEmployment != "" {
		subtitleParts = append(subtitleParts, employee.ProvinceOfEmployment)
	}
	return searchprojection.Document{
		CompanyID:  employee.CompanyID,
		EntityType: EntityTypeEmployee,
		EntityID:   employee.ID,
		DocNumber:  employee.EmployeeNo,
		Title:      title,
		Subtitle:   strings.Join(subtitleParts, " · "),
		Memo:       employee.Position,
		DocDate:    &employee.CreatedAt,
		Status:     string(employee.Status),
		URLPath:    "/employees?q=" + url.QueryEscape(employeeSearchQuery(employee)),
	}
}

func employeeSearchQuery(employee models.Employee) string {
	if employee.EmployeeNo != "" {
		return employee.EmployeeNo
	}
	if name := employee.SearchName(); name != "" {
		return name
	}
	return strconv.FormatUint(uint64(employee.ID), 10)
}

func ProjectPayrollRun(ctx context.Context, db *gorm.DB, p searchprojection.Projector, companyID, runID uint) error {
	if p == nil {
		return nil
	}
	if companyID == 0 {
		return errors.New("producers.ProjectPayrollRun: companyID is required")
	}
	var run models.PayrollRun
	err := db.Where("id = ? AND company_id = ?", runID, companyID).First(&run).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEntityNotInCompany
		}
		return fmt.Errorf("producers.ProjectPayrollRun: load payroll run %d for company %d: %w", runID, companyID, err)
	}
	doc := PayrollRunDocument(run)
	if err := p.Upsert(ctx, companyID, doc); err != nil {
		logging.L().Warn("searchprojection.ProjectPayrollRun upsert failed",
			"payroll_run_id", runID, "company_id", companyID, "err", err)
		return err
	}
	return nil
}

func PayrollRunDocument(run models.PayrollRun) searchprojection.Document {
	title := "Payroll Run"
	if run.RunNumber != "" {
		title = "Payroll Run " + run.RunNumber
	}
	subtitle := fmt.Sprintf("%s · %s to %s",
		run.PayDate.Format("2006-01-02"),
		run.PeriodStart.Format("2006-01-02"),
		run.PeriodEnd.Format("2006-01-02"),
	)
	return searchprojection.Document{
		CompanyID:  run.CompanyID,
		EntityType: EntityTypePayrollRun,
		EntityID:   run.ID,
		DocNumber:  run.RunNumber,
		Title:      title,
		Subtitle:   subtitle,
		DocDate:    &run.PayDate,
		Amount:     run.TotalNetPay.StringFixed(2),
		Status:     string(run.Status),
		URLPath:    "/payroll/runs/" + strconv.FormatUint(uint64(run.ID), 10),
	}
}

func PayrollEntryDocument(entry models.PayrollEntry) searchprojection.Document {
	title := "Payroll Entry"
	if entry.Employee.ID != 0 {
		if name := entry.Employee.SearchName(); name != "" {
			title = name
		}
	}
	docDate := entry.CreatedAt
	if entry.PayrollRun.ID != 0 {
		docDate = entry.PayrollRun.PayDate
	}
	subtitleParts := []string{"Payroll Entry"}
	if entry.PayrollRun.RunNumber != "" {
		subtitleParts = append(subtitleParts, entry.PayrollRun.RunNumber)
	}
	return searchprojection.Document{
		CompanyID:  entry.CompanyID,
		EntityType: EntityTypePayrollEntry,
		EntityID:   entry.ID,
		Title:      title,
		Subtitle:   strings.Join(subtitleParts, " · "),
		DocDate:    &docDate,
		Amount:     entry.NetPay.StringFixed(2),
		Status:     string(entry.Status),
		URLPath:    "/payroll/runs/" + strconv.FormatUint(uint64(entry.PayrollRunID), 10) + "#entry-" + strconv.FormatUint(uint64(entry.ID), 10),
	}
}

func ProjectPayrollEntry(ctx context.Context, db *gorm.DB, p searchprojection.Projector, companyID, entryID uint) error {
	if p == nil {
		return nil
	}
	if companyID == 0 {
		return errors.New("producers.ProjectPayrollEntry: companyID is required")
	}
	var entry models.PayrollEntry
	err := db.Preload("Employee").Preload("PayrollRun").
		Where("id = ? AND company_id = ?", entryID, companyID).
		First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEntityNotInCompany
		}
		return fmt.Errorf("producers.ProjectPayrollEntry: load payroll entry %d for company %d: %w", entryID, companyID, err)
	}
	doc := PayrollEntryDocument(entry)
	if err := p.Upsert(ctx, companyID, doc); err != nil {
		logging.L().Warn("searchprojection.ProjectPayrollEntry upsert failed",
			"payroll_entry_id", entryID, "company_id", companyID, "err", err)
		return err
	}
	return nil
}

func PayrollRemittanceDocument(remittance models.PayrollRemittance) searchprojection.Document {
	title := "Payroll Remittance"
	if remittance.RemittanceNumber != "" {
		title = "Payroll Remittance " + remittance.RemittanceNumber
	}
	subtitleParts := []string{"Payroll Remittance"}
	if remittance.PayrollRun.RunNumber != "" {
		subtitleParts = append(subtitleParts, remittance.PayrollRun.RunNumber)
	}
	if !remittance.DueDate.IsZero() {
		subtitleParts = append(subtitleParts, "Due "+remittance.DueDate.Format("2006-01-02"))
	}
	docDate := remittance.DueDate
	if docDate.IsZero() {
		docDate = remittance.CreatedAt
	}
	return searchprojection.Document{
		CompanyID:  remittance.CompanyID,
		EntityType: EntityTypePayrollRemittance,
		EntityID:   remittance.ID,
		DocNumber:  remittance.RemittanceNumber,
		Title:      title,
		Subtitle:   strings.Join(subtitleParts, " Â· "),
		DocDate:    &docDate,
		Amount:     remittance.TotalAmount.StringFixed(2),
		Status:     string(remittance.Status),
		URLPath:    "/payroll/remittances",
	}
}

func ProjectPayrollRemittance(ctx context.Context, db *gorm.DB, p searchprojection.Projector, companyID, remittanceID uint) error {
	if p == nil {
		return nil
	}
	if companyID == 0 {
		return errors.New("producers.ProjectPayrollRemittance: companyID is required")
	}
	var remittance models.PayrollRemittance
	err := db.Preload("PayrollRun").
		Where("id = ? AND company_id = ?", remittanceID, companyID).
		First(&remittance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEntityNotInCompany
		}
		return fmt.Errorf("producers.ProjectPayrollRemittance: load payroll remittance %d for company %d: %w", remittanceID, companyID, err)
	}
	doc := PayrollRemittanceDocument(remittance)
	if err := p.Upsert(ctx, companyID, doc); err != nil {
		logging.L().Warn("searchprojection.ProjectPayrollRemittance upsert failed",
			"payroll_remittance_id", remittanceID, "company_id", companyID, "err", err)
		return err
	}
	return nil
}

func ChequeDocument(cheque models.Cheque) searchprojection.Document {
	title := cheque.PayeeName
	if title == "" {
		title = "Cheque " + cheque.ChequeNumber
	}
	subtitle := "Cheque"
	if cheque.ChequeNumber != "" {
		subtitle = "Cheque · " + cheque.ChequeNumber
	}
	return searchprojection.Document{
		CompanyID:  cheque.CompanyID,
		EntityType: EntityTypeCheque,
		EntityID:   cheque.ID,
		DocNumber:  cheque.ChequeNumber,
		Title:      title,
		Subtitle:   subtitle,
		DocDate:    &cheque.ChequeDate,
		Amount:     cheque.Amount.StringFixed(2),
		Currency:   cheque.CurrencyCode,
		Status:     string(cheque.Status),
		URLPath:    "/cheques?q=" + url.QueryEscape(chequeSearchQuery(cheque)),
	}
}

func chequeSearchQuery(cheque models.Cheque) string {
	if cheque.ChequeNumber != "" {
		return cheque.ChequeNumber
	}
	if cheque.PayeeName != "" {
		return cheque.PayeeName
	}
	return strconv.FormatUint(uint64(cheque.ID), 10)
}

func ProjectCheque(ctx context.Context, db *gorm.DB, p searchprojection.Projector, companyID, chequeID uint) error {
	if p == nil {
		return nil
	}
	if companyID == 0 {
		return errors.New("producers.ProjectCheque: companyID is required")
	}
	var cheque models.Cheque
	err := db.Where("id = ? AND company_id = ?", chequeID, companyID).First(&cheque).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEntityNotInCompany
		}
		return fmt.Errorf("producers.ProjectCheque: load cheque %d for company %d: %w", chequeID, companyID, err)
	}
	doc := ChequeDocument(cheque)
	if err := p.Upsert(ctx, companyID, doc); err != nil {
		logging.L().Warn("searchprojection.ProjectCheque upsert failed",
			"cheque_id", chequeID, "company_id", companyID, "err", err)
		return err
	}
	return nil
}
