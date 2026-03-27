// 遵循产品需求 v1.0
package web

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"gobooks/internal/web/setup"
)

func (s *Server) registerMiddleware(app *fiber.App) {
	// Recover from panics so the server doesn't crash on unexpected errors.
	app.Use(recover.New())

	// Basic request logging.
	app.Use(logger.New())

	// Templ pages are HTML responses; make sure browsers render them as HTML.
	// Skip static assets so CSS/JS keep their own content types.
	app.Use(func(c *fiber.Ctx) error {
		if !strings.HasPrefix(c.Path(), "/static/") {
			c.Type("html", "utf-8")
		}
		return c.Next()
	})

	// Force first-time setup if no company exists yet.
	app.Use(setup.Guard(s.DB))
}

