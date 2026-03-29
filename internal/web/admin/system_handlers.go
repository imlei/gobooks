// 遵循产品需求 v1.0
package admin

import (
	"github.com/gofiber/fiber/v2"

	"gobooks/internal/services"
	"gobooks/internal/web/templates/admintmpl"
)

// handleAdminSystem 显示系统控制页面（维护模式开关、重启 stub）。
func (s *Server) handleAdminSystem(c *fiber.Ctx) error {
	return admintmpl.AdminSystem(admintmpl.AdminSystemVM{
		AdminEmail:      AdminUserFromCtx(c).Email,
		MaintenanceMode: IsMaintenanceMode(),
		Flash:           c.Query("flash"),
	}).Render(c.Context(), c)
}

// handleAdminMaintenanceEnable 开启维护模式（持久化到 DB 并更新内存缓存）。
func (s *Server) handleAdminMaintenanceEnable(c *fiber.Ctx) error {
	if err := s.setMaintenanceMode(true); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not persist maintenance mode")
	}

	services.TryWriteAuditLog(s.DB, "admin.system.maintenance_enabled", "system", 0,
		AdminUserFromCtx(c).Email,
		map[string]any{
			"maintenance_mode": true,
			"actor_type":       "sysadmin",
		},
	)

	return c.Redirect("/admin/system?flash=maintenance_on", fiber.StatusSeeOther)
}

// handleAdminMaintenanceDisable 关闭维护模式（持久化到 DB 并更新内存缓存）。
func (s *Server) handleAdminMaintenanceDisable(c *fiber.Ctx) error {
	if err := s.setMaintenanceMode(false); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not persist maintenance mode")
	}

	services.TryWriteAuditLog(s.DB, "admin.system.maintenance_disabled", "system", 0,
		AdminUserFromCtx(c).Email,
		map[string]any{
			"maintenance_mode": false,
			"actor_type":       "sysadmin",
		},
	)

	return c.Redirect("/admin/system?flash=maintenance_off", fiber.StatusSeeOther)
}

// handleAdminRestartStub 重启占位符（安全 stub，不执行任何危险操作）。
// 真实重启逻辑需在生产环境通过进程管理器（systemd / Docker）实现。
func (s *Server) handleAdminRestartStub(c *fiber.Ctx) error {
	services.TryWriteAuditLog(s.DB, "admin.system.restart_requested", "system", 0,
		AdminUserFromCtx(c).Email,
		map[string]any{
			"actor_type": "sysadmin",
			"note":       "stub only; actual restart handled by process manager",
		},
	)

	return c.Redirect("/admin/system?flash=restart_requested", fiber.StatusSeeOther)
}
