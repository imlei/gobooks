// 遵循project_guide.md
// Package admin implements the SysAdmin subsystem.
//
// 认证流程与业务用户完全隔离：
//   - 独立的 cookie（gobooks_admin_session）
//   - 独立的数据库表（sysadmin_users, sysadmin_sessions）
//   - 独立的中间件（LoadAdminSession / RequireAdminAuth）
//   - 独立的 Locals 键（不与业务用户 Locals 冲突）
//
// 所有 /admin/* 路由均通过 RegisterRoutes 挂载到主 Fiber 应用。
package admin

import (
	"gobooks/internal/config"

	"gorm.io/gorm"
)

// Server 持有 SysAdmin 处理器所需的依赖。
type Server struct {
	DB  *gorm.DB
	Cfg config.Config
}

// NewServer 创建 SysAdmin 服务实例，并从数据库加载持久化状态（维护模式）。
func NewServer(cfg config.Config, db *gorm.DB) *Server {
	s := &Server{Cfg: cfg, DB: db}
	// 从 system_settings 表加载维护模式状态，确保重启后状态一致
	initMaintenanceMode(db)
	return s
}
