// 遵循产品需求 v1.0
package web

import "github.com/gofiber/fiber/v2"

// AIConnectEditableFromCtx 返回当前成员是否有权修改 AI Connect 设置。
// 依赖 ActionSettingsUpdate → PermManageSettings（owner / admin）。
func AIConnectEditableFromCtx(c *fiber.Ctx) bool {
	return CanFromCtx(c, ActionSettingsUpdate)
}

// OwnerOrAdminFromCtx 返回当前成员是否有成员管理权限（邀请 / 角色调整）。
// 依赖 ActionMemberManage → PermManageMembers（owner / admin）。
// 仅用于 UI 可见性判断；路由层的强制检查由 RequirePermission(ActionMemberManage) 负责。
func OwnerOrAdminFromCtx(c *fiber.Ctx) bool {
	return CanFromCtx(c, ActionMemberManage)
}
