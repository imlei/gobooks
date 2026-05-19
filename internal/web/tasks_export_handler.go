// filepath: internal/web/tasks_export_handler.go
package web

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"balanciz/internal/models"
	"balanciz/internal/services"

	"github.com/gofiber/fiber/v2"
)

// Export tasks as CSV
func (s *Server) handleTasksExportCSV(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "company context required",
		})
	}

	// Optional filters from query params
	filterCustomerID := c.Query("customer_id")
	filterStatus := c.Query("status")
	filterFrom := c.Query("from")
	filterTo := c.Query("to")

	filter := services.TaskListFilter{CompanyID: companyID}
	if filterCustomerID != "" {
		if id64, err := services.ParseUint(filterCustomerID); err == nil && id64 > 0 {
			id := uint(id64)
			filter.CustomerID = &id
		}
	}
	if filterStatus != "" {
		status := models.TaskStatus(filterStatus)
		for _, allowed := range models.AllTaskStatuses() {
			if allowed == status {
				filter.Status = &status
				break
			}
		}
	}
	if filterFrom != "" {
		if from, err := time.Parse("2006-01-02", filterFrom); err == nil {
			filter.From = &from
		}
	}
	if filterTo != "" {
		if to, err := time.Parse("2006-01-02", filterTo); err == nil {
			filter.To = &to
		}
	}

	// Get tasks matching filters
	tasks, err := services.ListTasks(s.DB, filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to export tasks",
		})
	}

	// Create CSV in memory
	csvBuf := &strings.Builder{}
	w := csv.NewWriter(csvBuf)

	// Write header
	w.Write([]string{
		"Task ID",
		"Date",
		"Customer",
		"Description",
		"Quantity",
		"Unit Type",
		"Rate (" + c.Query("currency", "CAD") + ")",
		"Amount",
		"Billable",
		"Status",
		"Notes",
	})

	// Write data rows
	for _, task := range tasks {
		customerName := task.Customer.Name

		w.Write([]string{
			fmt.Sprintf("%d", task.ID),
			task.TaskDate.Format("2006-01-02"),
			customerName,
			task.Title,
			task.Quantity.String(),
			task.UnitType,
			task.Rate.String(),
			task.BillableAmount().String(),
			fmt.Sprintf("%v", task.IsBillable),
			string(task.Status),
			task.Notes,
		})
	}

	w.Flush()

	// Set response headers for CSV download
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"tasks-%s.csv\"", time.Now().Format("2006-01-02")))
	c.Set("Content-Length", fmt.Sprintf("%d", len(csvBuf.String())))

	return c.SendString(csvBuf.String())
}
