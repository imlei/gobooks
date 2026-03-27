// 遵循产品需求 v1.0
package setup

import (
	"strings"

	"gobooks/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Guard enforces first-run flows:
// - Empty database (no users and no companies): only /setup/bootstrap is reachable (plus static assets).
// - Company missing but users exist (legacy): redirect to /setup.
// - Company exists: allow normal navigation.
func Guard(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		if strings.HasPrefix(path, "/static/") {
			return c.Next()
		}
		if path == "/setup/bootstrap" {
			return c.Next()
		}

		var userCount int64
		if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "database error")
		}
		var companyCount int64
		if err := db.Model(&models.Company{}).Count(&companyCount).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "database error")
		}

		if userCount == 0 && companyCount == 0 {
			return c.Redirect("/setup/bootstrap", fiber.StatusSeeOther)
		}

		if companyCount == 0 {
			if path == "/setup" || path == "/login" || path == "/logout" || path == "/select-company" {
				return c.Next()
			}
			return c.Redirect("/setup", fiber.StatusSeeOther)
		}

		return c.Next()
	}
}

