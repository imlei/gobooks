// 遵循产品需求 v1.0
package web

import (
	"gobooks/internal/config"

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
		AppName: "GoBooks",
	})

	s.registerMiddleware(app)
	s.registerRoutes(app)

	return app
}

