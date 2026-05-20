// 遵循project_guide.md
package web

import "github.com/gofiber/fiber/v2"

func (s *Server) handleDashboardOverview(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no active company"})
	}
	overview, err := buildDashboardOverview(s.DB, companyID, smartPickerUserID(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "dashboard overview failed"})
	}
	overview = filterDashboardOverviewForVisibility(overview, dashboardVisibilityFromCtx(c))
	return c.JSON(overview)
}

func dashboardVisibilityFromCtx(c *fiber.Ctx) dashboardVisibility {
	return dashboardVisibility{
		CanViewReports:  CanFromCtx(c, ActionReportView),
		CanViewAP:       CanFromCtx(c, ActionBillView),
		CanViewSettings: CanFromCtx(c, ActionSettingsUpdate),
	}
}
