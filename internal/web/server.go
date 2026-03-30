// 遵循project_guide.md
package web

import (
	"gobooks/internal/config"
	"gobooks/internal/web/admin"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Server holds dependencies for handlers.
type Server struct {
	Cfg config.Config
	DB  *gorm.DB
}

// NewServer creates a Fiber app with basic middleware and routes.
func NewServer(cfg config.Config, db *gorm.DB) *fiber.App {
	s := &Server{Cfg: cfg, DB: db}

	app := fiber.New(fiber.Config{
		AppName:      "GoBooks",
		// 自定义错误处理器：5xx 持久化到 system_logs，4xx 仅 WARN 日志
		ErrorHandler: NewErrorHandler(db),
	})

	s.registerMiddleware(app)
	s.registerRoutes(app)

	// SysAdmin 路由：独立认证链，挂载在 /admin/* 下
	adminSrv := admin.NewServer(cfg, db)
	adminSrv.RegisterRoutes(app)

	return app
}

