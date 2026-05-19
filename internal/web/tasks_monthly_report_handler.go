// filepath: internal/web/tasks_monthly_report_handler.go
package web

import (
	"fmt"
	"strconv"
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

	// Optional filters
	yearStr := c.Query("year", fmt.Sprintf("%d", time.Now().Year()))
	monthStr := c.Query("month", fmt.Sprintf("%d", int(time.Now().Month())))

	year, _ := strconv.Atoi(yearStr)
	month, _ := strconv.Atoi(monthStr)

	// Get monthly summary from service
	summary, err := services.GenerateMonthlyTaskReport(s.DB, companyID, year, month)
	if err != nil {
		return pages.TasksMonthlyReport(pages.TasksMonthlyReportVM{
			HasCompany: true,
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
	}).Render(c.Context(), c)
}
