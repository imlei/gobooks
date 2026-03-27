// 遵循产品需求 v1.0
package web

import (
	"github.com/gofiber/fiber/v2"

	"gobooks/internal/models"
)

// AIConnectEditableFromCtx is true when the active membership may change AI Connect settings (owner or admin).
func AIConnectEditableFromCtx(c *fiber.Ctx) bool {
	m := MembershipFromCtx(c)
	if m == nil {
		return false
	}
	switch m.Role {
	case models.CompanyRoleOwner, models.CompanyRoleAdmin:
		return true
	default:
		return false
	}
}

// OwnerOrAdminFromCtx is true when the active membership is owner or admin (elevated company settings access).
func OwnerOrAdminFromCtx(c *fiber.Ctx) bool {
	return AIConnectEditableFromCtx(c)
}
