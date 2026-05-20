package web

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"balanciz/internal/models"
	"balanciz/internal/searchprojection/producers"
	"balanciz/internal/services"
	"balanciz/internal/web/templates/pages"
)

func (s *Server) handleEmployees(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	q := strings.TrimSpace(c.Query("q"))
	statusRaw := strings.TrimSpace(c.Query("status"))
	filter := services.EmployeeListFilter{CompanyID: companyID, Query: q, Limit: 200}
	if statusRaw != "" {
		status := models.EmployeeStatus(statusRaw)
		if isEmployeeStatusAllowed(status) {
			filter.Status = &status
		} else {
			statusRaw = ""
		}
	}

	employees, err := services.ListEmployees(s.DB, filter)
	if err != nil {
		return pages.Employees(pages.EmployeesVM{
			HasCompany: true,
			FormError:  "Could not load employees.",
			CanManage:  CanFromCtx(c, ActionEmployeeManage),
		}).Render(c.Context(), c)
	}

	return pages.Employees(pages.EmployeesVM{
		HasCompany: true,
		Employees:  employees,
		Query:      q,
		Status:     statusRaw,
		Created:    c.Query("created") == "1",
		FormError:  strings.TrimSpace(c.Query("error")),
		CanManage:  CanFromCtx(c, ActionEmployeeManage),
	}).Render(c.Context(), c)
}

func (s *Server) handleEmployeeCreate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	employee := models.Employee{
		CompanyID:            companyID,
		EmployeeNo:           strings.TrimSpace(c.FormValue("employee_no")),
		LegalName:            strings.TrimSpace(c.FormValue("legal_name")),
		DisplayName:          strings.TrimSpace(c.FormValue("display_name")),
		Email:                strings.TrimSpace(c.FormValue("email")),
		Mobile:               strings.TrimSpace(c.FormValue("mobile")),
		Position:             strings.TrimSpace(c.FormValue("position")),
		ProvinceOfEmployment: strings.ToUpper(strings.TrimSpace(c.FormValue("province"))),
		MemberType:           parseEmployeeMemberType(c.FormValue("member_type")),
		SalaryType:           parseEmployeeSalaryType(c.FormValue("salary_type")),
		Status:               models.EmployeeStatusActive,
		PayRate:              decimalFormValue(c, "pay_rate", decimal.Zero),
		PayRateUnit:          parsePayRateUnit(c.FormValue("pay_rate_unit")),
		PaysPerYear:          intFormValue(c, "pays_per_year", 26),
		PayFrequency:         parsePayFrequency(c.FormValue("pay_frequency")),
		HoursPerWeek:         decimalFormValue(c, "hours_per_week", decimal.Zero),
		TD1Federal:           decimalFormValue(c, "td1_federal", decimal.Zero),
		TD1Provincial:        decimalFormValue(c, "td1_provincial", decimal.Zero),
		PaidYTDOtherPayroll:  false,
		AutoVacation:         false,
	}
	if employee.ProvinceOfEmployment == "" {
		employee.ProvinceOfEmployment = "BC"
	}

	if err := services.CreateEmployee(s.DB, &employee); err != nil {
		return redirectErr(c, "/employees", friendlyPayrollError(err, "Could not create employee."))
	}
	_ = producers.ProjectEmployee(c.Context(), s.DB, s.SearchProjector, companyID, employee.ID)

	return redirectTo(c, "/employees?created=1")
}

func (s *Server) handlePayrollRuns(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runs, err := services.ListPayrollRuns(s.DB, services.PayrollRunListFilter{CompanyID: companyID, Limit: 100})
	if err != nil {
		return pages.PayrollRuns(pages.PayrollRunsVM{
			HasCompany:     true,
			FormError:      "Could not load payroll runs.",
			CanRun:         CanFromCtx(c, ActionPayrollRun),
			CanViewDetails: CanFromCtx(c, ActionPayrollViewDetails),
		}).Render(c.Context(), c)
	}

	return pages.PayrollRuns(pages.PayrollRunsVM{
		HasCompany:     true,
		Runs:           runs,
		Created:        c.Query("created") == "1",
		Calculated:     c.Query("calculated") == "1",
		FormError:      strings.TrimSpace(c.Query("error")),
		CanRun:         CanFromCtx(c, ActionPayrollRun),
		CanViewDetails: CanFromCtx(c, ActionPayrollViewDetails),
	}).Render(c.Context(), c)
}

func (s *Server) handlePayrollRunCreate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	periodStart, err := dateFormValue(c, "period_start")
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Period start must use YYYY-MM-DD.")
	}
	periodEnd, err := dateFormValue(c, "period_end")
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Period end must use YYYY-MM-DD.")
	}
	payDate, err := dateFormValue(c, "pay_date")
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Pay date must use YYYY-MM-DD.")
	}
	if periodEnd.Before(periodStart) {
		return redirectErr(c, "/payroll/runs", "Period end cannot be before period start.")
	}

	run := models.PayrollRun{
		CompanyID:    companyID,
		RunNumber:    strings.TrimSpace(c.FormValue("run_number")),
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		PayDate:      payDate,
		PaysPerYear:  intFormValue(c, "pays_per_year", 26),
		PayFrequency: parsePayFrequency(c.FormValue("pay_frequency")),
		PayrollType:  "regular",
		Status:       models.PayrollRunDraft,
	}
	if err := services.CreatePayrollRun(s.DB, &run); err != nil {
		return redirectErr(c, "/payroll/runs", friendlyPayrollError(err, "Could not create payroll run."))
	}
	_ = producers.ProjectPayrollRun(c.Context(), s.DB, s.SearchProjector, companyID, run.ID)

	return redirectTo(c, "/payroll/runs?created=1")
}

func (s *Server) handlePayrollRunDetail(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runID, err := parsePayrollRunID(c)
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Invalid payroll run.")
	}

	run, entries, err := services.GetPayrollRunWithEntries(s.DB, companyID, runID)
	if err != nil {
		return redirectErr(c, "/payroll/runs", friendlyPayrollError(err, "Could not load payroll run."))
	}

	canViewDetails := CanFromCtx(c, ActionPayrollViewDetails)
	chequeFeatureEnabled, _ := services.IsCompanyFeatureEnabled(s.DB, companyID, models.FeatureKeyCheque)
	canCreateCheques := canViewDetails && chequeFeatureEnabled && CanFromCtx(c, ActionChequePrint)
	var chequeBankAccounts []models.ChequeBankAccount
	if canCreateCheques {
		chequeBankAccounts, _ = services.ListChequeBankAccounts(s.DB, companyID, false)
	}
	postedJE, posted, err := services.PayrollRunJournalEntry(s.DB, companyID, runID)
	if err != nil {
		return redirectErr(c, "/payroll/runs", friendlyPayrollError(err, "Could not load payroll posting status."))
	}
	postedJEID := uint(0)
	if posted {
		postedJEID = postedJE.ID
	}
	remittanceID := uint(0)
	if remittance, found, err := services.PayrollRemittanceForRun(s.DB, companyID, runID); err != nil {
		return redirectErr(c, "/payroll/runs", friendlyPayrollError(err, "Could not load payroll remittance status."))
	} else if found {
		remittanceID = remittance.ID
	}

	return pages.PayrollRunDetail(pages.PayrollRunDetailVM{
		HasCompany:           true,
		Run:                  run,
		Entries:              entries,
		ChequeBankAccounts:   chequeBankAccounts,
		Calculated:           c.Query("calculated") == "1",
		Finalized:            c.Query("finalized") == "1",
		Posted:               c.Query("posted") == "1",
		RemittanceCreated:    c.Query("remittance_created") == "1",
		ChequesCreated:       c.Query("cheques_created") == "1",
		FormError:            strings.TrimSpace(c.Query("error")),
		CanRun:               CanFromCtx(c, ActionPayrollRun),
		CanFinalize:          CanFromCtx(c, ActionPayrollFinalize),
		CanPost:              CanFromCtx(c, ActionPayrollFinalize),
		CanCreateRemittance:  CanFromCtx(c, ActionPayrollFinalize),
		CanExport:            CanFromCtx(c, ActionPayrollExport),
		CanCreateCheques:     canCreateCheques,
		CanViewDetails:       canViewDetails,
		PostedJournalEntryID: postedJEID,
		RemittanceID:         remittanceID,
	}).Render(c.Context(), c)
}

func (s *Server) handlePayrollRemittances(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	remittances, err := services.ListPayrollRemittances(s.DB, services.PayrollRemittanceListFilter{CompanyID: companyID, Limit: 100})
	if err != nil {
		return pages.PayrollRemittances(pages.PayrollRemittancesVM{
			HasCompany: true,
			FormError:  "Could not load payroll remittances.",
			CanPost:    CanFromCtx(c, ActionPayrollFinalize),
		}).Render(c.Context(), c)
	}
	bankAccounts, err := s.chequeLedgerBankAccounts(companyID)
	if err != nil {
		return pages.PayrollRemittances(pages.PayrollRemittancesVM{
			HasCompany:  true,
			Remittances: remittances,
			FormError:   "Could not load bank accounts.",
			CanPost:     CanFromCtx(c, ActionPayrollFinalize),
		}).Render(c.Context(), c)
	}

	return pages.PayrollRemittances(pages.PayrollRemittancesVM{
		HasCompany:   true,
		Remittances:  remittances,
		BankAccounts: bankAccounts,
		Created:      c.Query("created") == "1",
		Paid:         c.Query("paid") == "1",
		Voided:       c.Query("voided") == "1",
		FormError:    strings.TrimSpace(c.Query("error")),
		CanPost:      CanFromCtx(c, ActionPayrollFinalize),
	}).Render(c.Context(), c)
}

func (s *Server) handlePayrollRunCalculate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runID, err := parsePayrollRunID(c)
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Invalid payroll run.")
	}
	entries, err := services.GeneratePayrollEntriesForRun(s.DB, companyID, runID, nil)
	if err != nil {
		return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), friendlyPayrollError(err, "Could not calculate payroll run."))
	}
	_ = producers.ProjectPayrollRun(c.Context(), s.DB, s.SearchProjector, companyID, runID)
	for _, entry := range entries {
		_ = producers.ProjectPayrollEntry(c.Context(), s.DB, s.SearchProjector, companyID, entry.ID)
	}

	return redirectTo(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10)+"?calculated=1")
}

func (s *Server) handlePayrollRunFinalize(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runID, err := parsePayrollRunID(c)
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Invalid payroll run.")
	}

	var userIDPtr *uuid.UUID
	user := UserFromCtx(c)
	if user != nil {
		uid := user.ID
		userIDPtr = &uid
	}
	if err := services.FinalizePayrollRun(s.DB, companyID, runID, userIDPtr); err != nil {
		return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), friendlyPayrollError(err, "Could not finalize payroll run."))
	}
	_ = producers.ProjectPayrollRun(c.Context(), s.DB, s.SearchProjector, companyID, runID)

	return redirectTo(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10)+"?finalized=1")
}

func (s *Server) handlePayrollRunPostJournal(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runID, err := parsePayrollRunID(c)
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Invalid payroll run.")
	}
	je, err := services.PostPayrollRunToJournalEntry(s.DB, companyID, runID)
	if err != nil {
		return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), friendlyPayrollError(err, "Could not post payroll run to the journal."))
	}
	_ = producers.ProjectJournalEntry(c.Context(), s.DB, s.SearchProjector, companyID, je.ID)

	return redirectTo(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10)+"?posted=1")
}

func (s *Server) handlePayrollRunCreateRemittance(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runID, err := parsePayrollRunID(c)
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Invalid payroll run.")
	}
	var dueDate time.Time
	if raw := strings.TrimSpace(c.FormValue("due_date")); raw != "" {
		dueDate, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), "Due date must use YYYY-MM-DD.")
		}
	}
	remittance, err := services.CreatePayrollRemittanceForRun(s.DB, companyID, runID, dueDate)
	if err != nil {
		return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), friendlyPayrollError(err, "Could not create payroll remittance."))
	}
	_ = producers.ProjectPayrollRemittance(c.Context(), s.DB, s.SearchProjector, companyID, remittance.ID)
	return redirectTo(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10)+"?remittance_created=1")
}

func (s *Server) handlePayrollRemittancePay(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	remittanceID, err := parsePayrollRemittanceID(c)
	if err != nil {
		return redirectErr(c, "/payroll/remittances", "Invalid payroll remittance.")
	}
	bankAccountID64, err := strconv.ParseUint(strings.TrimSpace(c.FormValue("bank_ledger_account_id")), 10, 64)
	if err != nil || bankAccountID64 == 0 {
		return redirectErr(c, "/payroll/remittances", "Bank account is required.")
	}
	paymentDate := time.Now().UTC()
	if raw := strings.TrimSpace(c.FormValue("payment_date")); raw != "" {
		paymentDate, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return redirectErr(c, "/payroll/remittances", "Payment date must use YYYY-MM-DD.")
		}
	}
	je, err := services.PayPayrollRemittance(s.DB, companyID, remittanceID, uint(bankAccountID64), paymentDate)
	if err != nil {
		return redirectErr(c, "/payroll/remittances", friendlyPayrollError(err, "Could not pay payroll remittance."))
	}
	_ = producers.ProjectPayrollRemittance(c.Context(), s.DB, s.SearchProjector, companyID, remittanceID)
	_ = producers.ProjectJournalEntry(c.Context(), s.DB, s.SearchProjector, companyID, je.ID)

	return redirectTo(c, "/payroll/remittances?paid=1")
}

func (s *Server) handlePayrollRemittanceVoid(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	remittanceID, err := parsePayrollRemittanceID(c)
	if err != nil {
		return redirectErr(c, "/payroll/remittances", "Invalid payroll remittance.")
	}
	voidDate := time.Now().UTC()
	if raw := strings.TrimSpace(c.FormValue("void_date")); raw != "" {
		voidDate, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return redirectErr(c, "/payroll/remittances", "Void date must use YYYY-MM-DD.")
		}
	}
	result, err := services.VoidPayrollRemittance(s.DB, companyID, remittanceID, voidDate)
	if err != nil {
		return redirectErr(c, "/payroll/remittances", friendlyPayrollError(err, "Could not void payroll remittance."))
	}
	_ = producers.ProjectPayrollRemittance(c.Context(), s.DB, s.SearchProjector, companyID, remittanceID)
	if result.OriginalJournalEntryID != 0 {
		_ = producers.ProjectJournalEntry(c.Context(), s.DB, s.SearchProjector, companyID, result.OriginalJournalEntryID)
	}
	if result.ReversalJournalEntryID != 0 {
		_ = producers.ProjectJournalEntry(c.Context(), s.DB, s.SearchProjector, companyID, result.ReversalJournalEntryID)
	}

	return redirectTo(c, "/payroll/remittances?voided=1")
}

func (s *Server) handlePayrollSummaryReport(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	co := s.loadReportCompanyInfo(companyID)
	preset, fromStr, toStr := resolvePeriodDates(c.Query("period"), c.Query("from"), c.Query("to"), co.FiscalYearEnd)
	fromDate, toDate, fromStr, toStr, errMsg := parseReportRange(fromStr, toStr)

	toolbar := pages.ReportToolbarVM{
		Preset:        preset,
		From:          fromStr,
		To:            toStr,
		FiscalYearEnd: co.FiscalYearEnd,
		CompanyName:   co.Name,
		ReportTitle:   "Payroll Summary",
		FormAction:    "/payroll/reports/summary",
		CSVExportURL:  "/payroll/reports/summary/export.csv",
		Mode:          "period",
	}

	if errMsg != "" {
		return pages.PayrollSummaryReport(pages.PayrollSummaryReportVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			DateLabel:  services.PayrollSummaryDateLabel(fromDate, toDate),
			Report:     services.PayrollSummaryReport{FromDate: fromDate, ToDate: toDate},
			FormError:  errMsg,
			Toolbar:    toolbar,
		}).Render(c.Context(), c)
	}

	report, err := services.BuildPayrollSummaryReport(s.DB, companyID, fromDate, toDate)
	if err != nil {
		return pages.PayrollSummaryReport(pages.PayrollSummaryReportVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			DateLabel:  services.PayrollSummaryDateLabel(fromDate, toDate),
			Report:     services.PayrollSummaryReport{FromDate: fromDate, ToDate: toDate},
			FormError:  "Could not run payroll summary.",
			Toolbar:    toolbar,
		}).Render(c.Context(), c)
	}

	return pages.PayrollSummaryReport(pages.PayrollSummaryReportVM{
		HasCompany: true,
		From:       fromStr,
		To:         toStr,
		DateLabel:  services.PayrollSummaryDateLabel(fromDate, toDate),
		Report:     report,
		Toolbar:    toolbar,
	}).Render(c.Context(), c)
}

func (s *Server) handlePayrollSummaryExportCSV(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).SendString("company required")
	}
	fromDate, toDate, _, _, errMsg := parseReportRange(c.Query("from"), c.Query("to"))
	if errMsg != "" {
		return c.Status(fiber.StatusBadRequest).SendString(errMsg)
	}
	report, err := services.BuildPayrollSummaryReport(s.DB, companyID, fromDate, toDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("could not run payroll summary")
	}
	var buf bytes.Buffer
	if err := services.ExportPayrollSummaryCSV(report, &buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	setCsvHeaders(c, services.PayrollSummaryExportFilename(fromDate, toDate))
	_, err = c.Write(buf.Bytes())
	return err
}

func (s *Server) handlePayrollEmployeeHistoryReport(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	employeeID := uintQueryValue(c, "employee_id")
	employees, err := services.ListEmployees(s.DB, services.EmployeeListFilter{CompanyID: companyID, Limit: 500})
	if err != nil {
		employees = nil
	}

	co := s.loadReportCompanyInfo(companyID)
	preset, fromStr, toStr := resolvePeriodDates(c.Query("period"), c.Query("from"), c.Query("to"), co.FiscalYearEnd)
	fromDate, toDate, fromStr, toStr, errMsg := parseReportRange(fromStr, toStr)

	toolbar := pages.ReportToolbarVM{
		Preset:        preset,
		From:          fromStr,
		To:            toStr,
		FiscalYearEnd: co.FiscalYearEnd,
		CompanyName:   co.Name,
		ReportTitle:   "Employee Payroll History",
		FormAction:    "/payroll/reports/employee-history",
		CSVExportURL:  "/payroll/reports/employee-history/export.csv",
		Mode:          "period",
		HiddenInputs: []pages.ReportToolbarHiddenInput{
			{Name: "employee_id", Value: uintQueryString(employeeID)},
		},
	}

	if errMsg != "" {
		return pages.PayrollEmployeeHistoryReport(pages.PayrollEmployeeHistoryReportVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			EmployeeID: employeeID,
			Employees:  employees,
			Report:     services.PayrollEmployeeHistoryReport{FromDate: fromDate, ToDate: toDate, EmployeeID: employeeID},
			FormError:  errMsg,
			Toolbar:    toolbar,
		}).Render(c.Context(), c)
	}

	report, err := services.BuildPayrollEmployeeHistoryReport(s.DB, companyID, employeeID, fromDate, toDate)
	if err != nil {
		return pages.PayrollEmployeeHistoryReport(pages.PayrollEmployeeHistoryReportVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			EmployeeID: employeeID,
			Employees:  employees,
			Report:     services.PayrollEmployeeHistoryReport{FromDate: fromDate, ToDate: toDate, EmployeeID: employeeID},
			FormError:  "Could not run employee payroll history.",
			Toolbar:    toolbar,
		}).Render(c.Context(), c)
	}

	return pages.PayrollEmployeeHistoryReport(pages.PayrollEmployeeHistoryReportVM{
		HasCompany: true,
		From:       fromStr,
		To:         toStr,
		EmployeeID: employeeID,
		Employees:  employees,
		Report:     report,
		Toolbar:    toolbar,
	}).Render(c.Context(), c)
}

func (s *Server) handlePayrollEmployeeHistoryExportCSV(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).SendString("company required")
	}
	employeeID := uintQueryValue(c, "employee_id")
	fromDate, toDate, _, _, errMsg := parseReportRange(c.Query("from"), c.Query("to"))
	if errMsg != "" {
		return c.Status(fiber.StatusBadRequest).SendString(errMsg)
	}
	report, err := services.BuildPayrollEmployeeHistoryReport(s.DB, companyID, employeeID, fromDate, toDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("could not run employee payroll history")
	}
	var buf bytes.Buffer
	if err := services.ExportPayrollEmployeeHistoryCSV(report, &buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	setCsvHeaders(c, services.PayrollEmployeeHistoryExportFilename(fromDate, toDate))
	_, err = c.Write(buf.Bytes())
	return err
}

func (s *Server) handlePayrollRunExportCSV(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runID, err := parsePayrollRunID(c)
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Invalid payroll run.")
	}
	run, entries, err := services.GetPayrollRunWithEntries(s.DB, companyID, runID)
	if err != nil {
		return redirectErr(c, "/payroll/runs", friendlyPayrollError(err, "Could not load payroll run."))
	}
	body, err := services.BuildPayrollRunEntriesCSV(run, entries)
	if err != nil {
		return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), "Could not export payroll run.")
	}

	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, services.PayrollRunExportFilename(run)))
	c.Set("Content-Length", strconv.Itoa(len(body)))
	return c.SendString(body)
}

func (s *Server) handlePayrollRunCreateCheques(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	runID, err := parsePayrollRunID(c)
	if err != nil {
		return redirectErr(c, "/payroll/runs", "Invalid payroll run.")
	}
	bankAccountID64, err := strconv.ParseUint(strings.TrimSpace(c.FormValue("bank_account_id")), 10, 64)
	if err != nil || bankAccountID64 == 0 {
		return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), "Cheque bank account is required.")
	}

	cheques, err := services.GeneratePayrollChequeDrafts(s.DB, companyID, runID, uint(bankAccountID64))
	if err != nil {
		return redirectErr(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10), friendlyPayrollError(err, "Could not create payroll cheque drafts."))
	}
	for _, cheque := range cheques {
		_ = producers.ProjectCheque(c.Context(), s.DB, s.SearchProjector, companyID, cheque.ID)
	}

	return redirectTo(c, "/payroll/runs/"+strconv.FormatUint(uint64(runID), 10)+"?cheques_created=1")
}

func uintQueryValue(c *fiber.Ctx, key string) uint {
	id64, err := strconv.ParseUint(strings.TrimSpace(c.Query(key)), 10, 64)
	if err != nil {
		return 0
	}
	return uint(id64)
}

func uintQueryString(value uint) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(value), 10)
}

func parsePayrollRunID(c *fiber.Ctx) (uint, error) {
	id64, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id64 == 0 {
		return 0, fmt.Errorf("invalid payroll run ID")
	}
	return uint(id64), nil
}

func parsePayrollRemittanceID(c *fiber.Ctx) (uint, error) {
	id64, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id64 == 0 {
		return 0, fmt.Errorf("invalid payroll remittance ID")
	}
	return uint(id64), nil
}

func isEmployeeStatusAllowed(status models.EmployeeStatus) bool {
	switch status {
	case models.EmployeeStatusActive, models.EmployeeStatusInactive, models.EmployeeStatusTerminated:
		return true
	default:
		return false
	}
}

func parseEmployeeMemberType(value string) models.EmployeeMemberType {
	switch models.EmployeeMemberType(strings.TrimSpace(value)) {
	case models.EmployeeMemberContractor:
		return models.EmployeeMemberContractor
	case models.EmployeeMemberConstructionContractor:
		return models.EmployeeMemberConstructionContractor
	default:
		return models.EmployeeMemberEmployee
	}
}

func parseEmployeeSalaryType(value string) models.EmployeeSalaryType {
	switch models.EmployeeSalaryType(strings.TrimSpace(value)) {
	case models.EmployeeSalarySalaried:
		return models.EmployeeSalarySalaried
	default:
		return models.EmployeeSalaryTimeBased
	}
}

func parsePayFrequency(value string) models.PayFrequency {
	switch models.PayFrequency(strings.TrimSpace(value)) {
	case models.PayFrequencyWeekly:
		return models.PayFrequencyWeekly
	case models.PayFrequencySemiMonthly:
		return models.PayFrequencySemiMonthly
	case models.PayFrequencyMonthly:
		return models.PayFrequencyMonthly
	default:
		return models.PayFrequencyBiweekly
	}
}

func parsePayRateUnit(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "period", "annual", "monthly":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "hourly"
	}
}

func decimalFormValue(c *fiber.Ctx, key string, fallback decimal.Decimal) decimal.Decimal {
	raw := strings.TrimSpace(c.FormValue(key))
	if raw == "" {
		return fallback
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return fallback
	}
	return d
}

func intFormValue(c *fiber.Ctx, key string, fallback int) int {
	raw := strings.TrimSpace(c.FormValue(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func dateFormValue(c *fiber.Ctx, key string) (time.Time, error) {
	raw := strings.TrimSpace(c.FormValue(key))
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s is required", key)
	}
	return time.Parse("2006-01-02", raw)
}

func friendlyPayrollError(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return fallback
	}
	return msg
}
