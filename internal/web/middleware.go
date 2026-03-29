// 遵循产品需求 v1.0
package web

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"gobooks/internal/web/admin"
	"gobooks/internal/web/setup"
)

func (s *Server) registerMiddleware(app *fiber.App) {
	// 请求 ID：最先运行，确保后续所有中间件和日志都能读到 request_id。
	app.Use(RequestID())

	// Panic 恢复：替代 fiber recover.New()，slog ERROR + 持久化到 system_logs。
	app.Use(PanicRecovery(s.DB))

	// 请求日志：替代 fiber logger.New()，slog 结构化输出（JSON）。
	app.Use(RequestLogger())

	// CSRF 防护：对基于 cookie 的状态变更请求进行令牌校验。
	app.Use(CSRFMiddleware(s.Cfg))

	// Templ 页面均为 HTML 响应；静态资源保留各自的 Content-Type。
	app.Use(func(c *fiber.Ctx) error {
		if !strings.HasPrefix(c.Path(), "/static/") {
			c.Type("html", "utf-8")
		}
		return c.Next()
	})

	// 维护模式：当 SysAdmin 开启维护模式时，业务用户收到 503 提示。
	// /admin/*、/static/* 路径始终放行。
	app.Use(func(c *fiber.Ctx) error {
		if !admin.IsMaintenanceMode() {
			return c.Next()
		}
		p := c.Path()
		if strings.HasPrefix(p, "/admin") || strings.HasPrefix(p, "/static/") {
			return c.Next()
		}
		c.Status(fiber.StatusServiceUnavailable)
		return c.SendString("GoBooks is currently under maintenance. Please check back shortly.")
	})

	// Force first-time setup if no company exists yet.
	app.Use(setup.Guard(s.DB))
}
