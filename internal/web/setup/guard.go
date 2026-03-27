// 遵循产品需求 v1.0
package setup

import (
	"strings"

	"gobooks/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Guard redirects users to /setup when no company exists yet.
//
// This implements the PROJECT_GUIDE requirement:
// - Setup Wizard: 初次必须走
// - 后续可以在 Settings 修改
func Guard(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Allow static assets and the setup routes.
		if strings.HasPrefix(path, "/static/") || path == "/setup" {
			return c.Next()
		}

		// If no company exists, force the user into setup.
		var count int64
		if err := db.Model(&models.Company{}).Count(&count).Error; err != nil {
			// If DB check fails, return a safe error response.
			return fiber.NewError(fiber.StatusInternalServerError, "database error")
		}
		if count == 0 {
			return c.Redirect("/setup", fiber.StatusSeeOther)
		}

		return c.Next()
	}
}

