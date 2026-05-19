// filepath: internal/web/tasks_monthly_report_handler.go
package web

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"balanciz/internal/services"
	"balanciz/internal/web/templates/pages"

	"github.com/gofiber/fiber/v2"
)

// Monthly task report: Group tasks by month, show billable hours/amounts
func (s *Server) handleTasksMonthlyReport(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	year, month, periodErr := taskMonthlyReportPeriodFromQuery(c, time.Now())
	if periodErr != nil {
		return pages.TasksMonthlyReport(pages.TasksMonthlyReportVM{
			HasCompany: true,
			Year:       year,
			Month:      month,
			FormError:  periodErr.Error(),
		}).Render(c.Context(), c)
	}

	// Get monthly summary from service
	summary, err := services.GenerateMonthlyTaskReport(s.DB, companyID, year, month)
	if err != nil {
		return pages.TasksMonthlyReport(pages.TasksMonthlyReportVM{
			HasCompany: true,
			Year:       year,
			Month:      month,
			FormError:  err.Error(),
		}).Render(c.Context(), c)
	}

	// Customers for filter dropdown
	customers, _ := s.customersForCompany(companyID)

	return pages.TasksMonthlyReport(pages.TasksMonthlyReportVM{
		HasCompany: true,
		Year:       year,
		Month:      month,
		Summary:    summary,
		Customers:  customers,
		CanExport:  CanFromCtx(c, ActionTaskExport),
	}).Render(c.Context(), c)
}

func taskMonthlyReportPeriodFromQuery(c *fiber.Ctx, now time.Time) (int, int, error) {
	year := now.Year()
	month := int(now.Month())

	if yearStr := strings.TrimSpace(c.Query("year")); yearStr != "" {
		parsed, err := strconv.Atoi(yearStr)
		if err != nil || parsed < 1900 || parsed > 9999 {
			return year, month, services.ErrTaskReportYearInvalid
		}
		year = parsed
	}
	if monthStr := strings.TrimSpace(c.Query("month")); monthStr != "" {
		parsed, err := strconv.Atoi(monthStr)
		if err != nil || parsed < 1 || parsed > 12 {
			return year, month, services.ErrTaskReportMonthInvalid
		}
		month = parsed
	}
	if (strings.TrimSpace(c.Query("year")) == "") != (strings.TrimSpace(c.Query("month")) == "") {
		return now.Year(), int(now.Month()), errors.New("year and month are required together")
	}

	return year, month, nil
}
