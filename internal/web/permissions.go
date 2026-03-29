// 遵循产品需求 v1.0
package web

import "github.com/gofiber/fiber/v2"

// ─────────────────────────────────────────────────────────────────────────────
// Action 常量
//
// Action 描述"用户想做什么"，是权限系统对外的语义接口。
// 路由通过 RequirePermission(ActionXxx) 声明所需操作；
// Handler 及模板通过 CanFromCtx(c, ActionXxx) 判断当前用户能否执行该操作。
// ─────────────────────────────────────────────────────────────────────────────

const (
	// 发票（应收账款端）
	ActionInvoiceView    = "invoice:view"    // 查看发票列表 / 详情（仅需成员资格）
	ActionInvoiceCreate  = "invoice:create"  // 创建 / 保存草稿
	ActionInvoiceUpdate  = "invoice:update"  // 编辑已有发票
	ActionInvoiceDelete  = "invoice:delete"  // 删除发票
	ActionInvoiceApprove = "invoice:approve" // 过账 / 冲销（需更高权限）

	// 账单（应付账款端）
	ActionBillView   = "bill:view"   // 查看账单列表 / 详情（仅需成员资格）
	ActionBillCreate = "bill:create" // 创建账单
	ActionBillUpdate = "bill:update" // 编辑账单
	ActionBillDelete = "bill:delete" // 删除账单
	ActionBillPay    = "bill:pay"    // 付款操作（含银行付款）

	// 日记账
	ActionJournalView   = "journal:view"   // 查看日记账（仅需成员资格）
	ActionJournalCreate = "journal:create" // 新建日记账 / 冲销分录
	ActionJournalUpdate = "journal:update" // 编辑草稿日记账
	ActionJournalDelete = "journal:delete" // 删除日记账

	// 会计科目（科目表）
	ActionAccountView   = "account:view"   // 查看科目表（仅需成员资格）
	ActionAccountCreate = "account:create" // 新增科目
	ActionAccountUpdate = "account:update" // 修改科目
	ActionAccountDelete = "account:delete" // 停用科目

	// 系统设置
	ActionSettingsView   = "settings:view"   // 查看设置页（仅需成员资格）
	ActionSettingsUpdate = "settings:update" // 修改公司设置 / AI 设置 / 产品目录

	// 报表
	ActionReportView = "report:view" // 查看财务报表

	// 审计日志
	ActionAuditView = "audit:view" // 查看审计日志

	// 成员管理
	ActionMemberView   = "member:view"   // 查看成员列表（仅需成员资格）
	ActionMemberManage = "member:manage" // 邀请成员 / 调整角色
)

// ─────────────────────────────────────────────────────────────────────────────
// Permission 常量
//
// Permission 是授予角色的能力单元。一个角色拥有若干 Permission；
// 每个 Action 需要对应的 Permission 才能执行。
// ─────────────────────────────────────────────────────────────────────────────

const (
	PermARAccess            = "ar_access"            // 应收账款写入（发票创建/编辑、日记账、银行收款、银行对账）
	PermAPAccess            = "ap_access"            // 应付账款写入（账单创建/编辑、付款）
	PermApproveTransactions = "approve_transactions" // 过账 / 冲销（需高于普通簿记权限）
	PermManageSettings      = "manage_settings"      // 公司设置、科目表、产品目录管理（owner / admin 专属）
	PermViewReports         = "view_reports"         // 查看财务报表
	PermViewAuditLog        = "view_audit_log"       // 查看审计日志
	PermManageMembers       = "manage_members"       // 邀请和管理公司成员
)

// ─────────────────────────────────────────────────────────────────────────────
// 角色 → 权限映射
//
// 设计原则：
//   - owner / admin：全部权限（owner 是唯一通过 bootstrap 创建的角色，admin 由 owner 邀请）
//   - accountant：AR + AP 写入、过账、查看报表和审计日志
//   - bookkeeper：AR + AP 写入、查看报表和审计日志（无过账权限）
//   - ap：仅应付账款写入（账单录入和付款）
//   - viewer：只读，仅有报表查看权；所有写操作由 RequireMembership 的 GET-only 规则兜底拦截
// ─────────────────────────────────────────────────────────────────────────────

var rolePermissions = map[string][]string{
	"owner": {
		PermARAccess, PermAPAccess, PermApproveTransactions,
		PermManageSettings, PermViewReports, PermViewAuditLog, PermManageMembers,
	},
	"admin": {
		PermARAccess, PermAPAccess, PermApproveTransactions,
		PermManageSettings, PermViewReports, PermViewAuditLog, PermManageMembers,
	},
	"accountant": {
		PermARAccess, PermAPAccess, PermApproveTransactions,
		PermViewReports, PermViewAuditLog,
	},
	"bookkeeper": {
		PermARAccess, PermAPAccess,
		PermViewReports, PermViewAuditLog,
	},
	"ap": {
		PermAPAccess,
	},
	"viewer": {
		PermViewReports,
		// viewer 无任何写权限；RequireMembership 已对非 GET 请求一律返回 403
	},
}

// ─────────────────────────────────────────────────────────────────────────────
// Action → Permission 映射
//
// 仅列出需要特定权限的写操作和受限查看操作。
// 未出现在此表中的 action 会被视为未配置并默认拒绝（fail closed），
// 防止新增路由遗漏映射时出现静默放权。
// ─────────────────────────────────────────────────────────────────────────────

var actionPermissions = map[string]string{
	// 发票写操作 ────────────────────────────────
	ActionInvoiceCreate:  PermARAccess,            // bookkeeper 及以上
	ActionInvoiceUpdate:  PermARAccess,            // bookkeeper 及以上
	ActionInvoiceDelete:  PermARAccess,            // bookkeeper 及以上
	ActionInvoiceApprove: PermApproveTransactions, // accountant 及以上（过账 / 冲销）

	// 账单写操作 ────────────────────────────────
	ActionBillCreate: PermAPAccess, // ap 及以上
	ActionBillUpdate: PermAPAccess, // ap 及以上
	ActionBillDelete: PermAPAccess, // ap 及以上
	ActionBillPay:    PermAPAccess, // ap 及以上（含银行付款）

	// 日记账写操作 ──────────────────────────────
	// 包含手动分录和冲销；银行对账、收款也归入 AR 操作
	ActionJournalCreate: PermARAccess, // bookkeeper 及以上
	ActionJournalUpdate: PermARAccess, // bookkeeper 及以上
	ActionJournalDelete: PermARAccess, // bookkeeper 及以上（冲销操作）

	// 科目表写操作 ──────────────────────────────
	// 科目表属于基础主数据，由 owner / admin 维护
	ActionAccountCreate: PermManageSettings,
	ActionAccountUpdate: PermManageSettings,
	ActionAccountDelete: PermManageSettings,

	// 系统设置写操作 ────────────────────────────
	// 包含公司档案、编号规则、AI 设置、产品目录
	ActionSettingsUpdate: PermManageSettings,

	// 受限查看操作 ──────────────────────────────
	// 报表和审计日志需要明确权限（AP / viewer 不具备审计日志访问权）
	ActionReportView: PermViewReports,
	ActionAuditView:  PermViewAuditLog,

	// 成员管理 ──────────────────────────────────
	ActionMemberManage: PermManageMembers,
}

// ─────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────────────────────────────────────

// GetPermissionsForRole 返回指定角色拥有的全部权限列表。
// 未知角色返回空切片（最小权限原则）。
func GetPermissionsForRole(role string) []string {
	perms, ok := rolePermissions[role]
	if !ok {
		return []string{}
	}
	return perms
}

// HasPermission 检查指定角色是否拥有某项具体权限。
func HasPermission(role string, permission string) bool {
	for _, p := range GetPermissionsForRole(role) {
		if p == permission {
			return true
		}
	}
	return false
}

// CanPerformAction 检查指定角色是否能执行某个操作。
//
// 若该操作未出现在 actionPermissions 映射中，则默认拒绝（返回 false）。
func CanPerformAction(role string, action string) bool {
	perm, required := actionPermissions[action]
	if !required {
		return false
	}
	return HasPermission(role, perm)
}

// CanFromCtx 从 Fiber 请求上下文读取当前成员角色，判断其是否能执行指定操作。
// 必须在 RequireMembership 之后的 handler 或模板辅助函数中调用。
func CanFromCtx(c *fiber.Ctx, action string) bool {
	m := MembershipFromCtx(c)
	if m == nil {
		return false
	}
	return CanPerformAction(string(m.Role), action)
}
