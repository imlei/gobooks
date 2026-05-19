# Balanciz 模块化集成方案 - SimpleTask 功能模块

**更新日期**：2026-05-15
**策略**：以 Balanciz 为母体框架，SimpleTask 功能作为 **可插拔模块**，支持后台独立启用/禁用

---

## 一、模块化架构设计

### 1.1 模块体系

```
┌─────────────────────────────────────────────────────────────┐
│                       Balanciz Core                           │
│  (Posting Engine + Tax Engine + Reconciliation + Reports)    │
├─────────────────────────────────────────────────────────────┤
│                    Feature Modules Layer                      │
│  ┌─────────────┬──────────────┬──────────────┬──────────────┐
│  │  Payroll    │   Employee   │   Cheque     │    Task      │
│  │  (SimpleTask)│ (SimpleTask) │ (SimpleTask) │ (SimpleTask) │
│  └─────────────┴──────────────┴──────────────┴──────────────┘
│  ┌─────────────┬──────────────┬──────────────┬──────────────┐
│  │  Dashboard   │  Reports     │  Maintenance │
│  │  (Balanciz)  │  (Balanciz)  │  (Balanciz)  │
│  └─────────────┴──────────────┴──────────────┴──────────────┘
├─────────────────────────────────────────────────────────────┤
│               Module Management Service                       │
│   (启用/禁用 + 权限控制 + FeatureFlag)                        │
├─────────────────────────────────────────────────────────────┤
│          Permission Engine + Audit Log + Auth                │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 模块注册表

```go
// internal/modules/registry.go
type ModuleRegistry struct {
    modules map[string]*Module
}

type Module struct {
    ID              string                   // "payroll" | "employee" | "cheque" | "task"
    Name            string                   // 显示名称
    Description     string                   // 模块描述
    Version         string                   // "1.0.0"
    Enabled         bool                     // 是否启用
    DefaultRoles    []string                 // 默认分配角色
    Dependencies    []string                 // 依赖的其他模块 e.g., ["employee"]
    RoutePrefix     string                   // "/payroll", "/employee"
    Init            func(context.Context) error // 初始化函数
    Routes          func(fiber.Router)       // 路由注册函数
    Migrations      []string                 // 关联的迁移文件
    Features        []Feature                // 子功能列表
}

type Feature struct {
    ID        string // "payroll.finalize" | "cheque.sign"
    Name      string
    RoleID    string // 所属角色 (e.g., "payroll_admin")
}

// 模块初始化示例
var PayrollModule = &Module{
    ID:          "payroll",
    Name:        "薪资管理",
    Description: "员工薪资处理、税务计算、T4 导出",
    Version:     "1.0.0",
    Enabled:     false,  // 初始禁用，由管理员启用
    DefaultRoles: []string{"payroll_admin", "payroll_viewer"},
    Dependencies: []string{"employee"},  // 依赖 employee 模块
    RoutePrefix: "/payroll",
    Migrations:  []string{
        "migrations/030_add_payroll_tables.sql",
        "migrations/031_add_employee_tables.sql",
    },
    Features: []Feature{
        {ID: "payroll.view", Name: "查看薪资运行"},
        {ID: "payroll.create", Name: "创建薪资运行"},
        {ID: "payroll.finalize", Name: "锁定薪资运行"},
        {ID: "payroll.export-t4", Name: "导出 T4"},
    },
}
```

---

## 二、数据库设计 - 模块管理表

### 2.1 模块管理表

```sql
-- 模块管理表
CREATE TABLE IF NOT EXISTS modules (
    id VARCHAR(50) PRIMARY KEY,          -- "payroll" | "employee" | "cheque" | "task"
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(20),
    enabled BOOLEAN DEFAULT FALSE,
    config JSONB,                        -- 模块级别配置
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID REFERENCES users(id)
);

-- 模块功能到角色的映射
CREATE TABLE IF NOT EXISTS module_features (
    id SERIAL PRIMARY KEY,
    module_id VARCHAR(50) NOT NULL REFERENCES modules(id),
    feature_id VARCHAR(100) NOT NULL,    -- "payroll.view" | "payroll.finalize"
    feature_name VARCHAR(255),
    role_id VARCHAR(50),                 -- 关联的角色 ID
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(module_id, feature_id)
);

-- 模块启用日志（审计）
CREATE TABLE IF NOT EXISTS module_audit_log (
    id SERIAL PRIMARY KEY,
    module_id VARCHAR(50) NOT NULL,
    action VARCHAR(50),                  -- "ENABLE" | "DISABLE" | "CONFIG_CHANGE"
    old_config JSONB,
    new_config JSONB,
    user_id UUID REFERENCES users(id),
    reason TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX idx_modules_enabled ON modules(enabled);
CREATE INDEX idx_module_features_module ON module_features(module_id);
CREATE INDEX idx_module_audit_module ON module_audit_log(module_id);
```

### 2.2 初始化模块数据

```sql
-- 插入 SimpleTask 功能模块
INSERT INTO modules (id, name, description, version, enabled) VALUES
('employee', '员工管理', '员工信息、档案、离职管理', '1.0.0', false),
('payroll', '薪资管理', '薪资计算、税务、T4 导出、YTD 管理', '1.0.0', false),
('cheque', '支票管理', '支票生成、签署、打印、对账', '1.0.0', false),
('task', '任务管理', '日常任务、快速发票、月度报表', '1.0.0', false);

-- 插入模块功能和角色映射
INSERT INTO module_features (module_id, feature_id, feature_name, role_id) VALUES
-- Employee
('employee', 'employee.view', '查看员工列表', 'employee_viewer'),
('employee', 'employee.create', '新建员工', 'employee_admin'),
('employee', 'employee.edit', '编辑员工信息', 'employee_admin'),
('employee', 'employee.terminate', '终止员工', 'employee_admin'),

-- Payroll
('payroll', 'payroll.view', '查看薪资运行', 'payroll_viewer'),
('payroll', 'payroll.create', '创建薪资运行', 'payroll_admin'),
('payroll', 'payroll.finalize', '锁定薪资运行（自动发帐）', 'payroll_admin'),
('payroll', 'payroll.export-t4', '导出 T4/T4A', 'payroll_admin'),

-- Cheque
('cheque', 'cheque.view', '查看支票', 'cheque_viewer'),
('cheque', 'cheque.create', '草稿支票', 'cheque_admin'),
('cheque', 'cheque.issue', '签署支票（自动发帐）', 'cheque_signor'),
('cheque.print', '打印支票', 'cheque_viewer'),

-- Task
('task', 'task.view', '查看任务', 'task_viewer'),
('task', 'task.create', '新建任务', 'task_admin'),
('task', 'task.complete', '完成任务', 'task_admin'),
('task', 'task.invoice', '快速生成发票（自动发帐）', 'task_admin'),
('task', 'task.report', '月度报表', 'task_viewer');
```

---

## 三、模块管理服务

### 3.1 核心接口

```go
// internal/services/module_service.go

type ModuleService struct {
    db         *ent.Client
    auditLog   AuditLogger
    cache      Cache  // 缓存已启用模块列表
}

// EnableModule 启用模块并执行迁移
func (ms *ModuleService) EnableModule(ctx context.Context, moduleID string, reason string) error {
    // 1. 验证权限 (SuperAdmin 或 SystemAdmin)
    user := ctx.Value("user").(*User)
    if !user.IsSuperAdmin() {
        return ErrUnauthorized
    }

    // 2. 获取模块定义
    module := ms.getModuleDefinition(moduleID)
    if module == nil {
        return ErrModuleNotFound
    }

    // 3. 检查依赖条件
    for _, dep := range module.Dependencies {
        depEnabled, _ := ms.db.Module.Query().
            Where(modulepredicate.ID(dep)).
            Only(ctx)
        if !depEnabled.Enabled {
            return fmt.Errorf("依赖模块 '%s' 未启用", dep)
        }
    }

    // 4. 执行迁移（如果之前未执行过）
    if err := ms.runMigrations(ctx, module.Migrations); err != nil {
        return fmt.Errorf("迁移失败: %w", err)
    }

    // 5. 创建默认角色和权限
    if err := ms.createDefaultRoles(ctx, module); err != nil {
        return fmt.Errorf("创建默认角色失败: %w", err)
    }

    // 6. 更新数据库
    err := ms.db.Module.UpdateOneID(moduleID).
        SetEnabled(true).
        SetUpdatedAt(time.Now()).
        SetUpdatedBy(user.ID).
        Exec(ctx)
    if err != nil {
        return fmt.Errorf("更新模块状态失败: %w", err)
    }

    // 7. 记录审计日志
    ms.auditLog.RecordModuleAction(ctx, &ModuleAuditEntry{
        ModuleID: moduleID,
        Action:   "ENABLE",
        Reason:   reason,
        UserID:   user.ID,
        Timestamp: time.Now(),
    })

    // 8. 清除缓存
    ms.cache.Delete("enabled_modules")

    // 9. 调用模块初始化钩子
    module.Init(ctx)

    return nil
}

// DisableModule 禁用模块（警告：会隐藏相关 UI）
func (ms *ModuleService) DisableModule(ctx context.Context, moduleID string, reason string) error {
    user := ctx.Value("user").(*User)
    if !user.IsSuperAdmin() {
        return ErrUnauthorized
    }

    // 1. 检查是否有其他模块依赖此模块
    dependents := ms.db.Module.Query().
        Where(module.DependenciesContains(moduleID)).
        AllX(ctx)
    if len(dependents) > 0 {
        depNames := make([]string, len(dependents))
        for i, d := range dependents {
            depNames[i] = d.Name
        }
        return fmt.Errorf("以下模块依赖此模块，无法禁用: %v", depNames)
    }

    // 2. 禁用数据库中的模块
    err := ms.db.Module.UpdateOneID(moduleID).
        SetEnabled(false).
        SetUpdatedAt(time.Now()).
        SetUpdatedBy(user.ID).
        Exec(ctx)
    if err != nil {
        return err
    }

    // 3. 记录审计日志
    ms.auditLog.RecordModuleAction(ctx, &ModuleAuditEntry{
        ModuleID: moduleID,
        Action:   "DISABLE",
        Reason:   reason,
        UserID:   user.ID,
        Timestamp: time.Now(),
    })

    // 4. 清除缓存
    ms.cache.Delete("enabled_modules")

    return nil
}

// IsModuleEnabled 检查模块是否启用（带缓存）
func (ms *ModuleService) IsModuleEnabled(ctx context.Context, moduleID string) bool {
    // 尝试从缓存读取
    if enabledModules, found := ms.cache.Get("enabled_modules"); found {
        return containsModule(enabledModules.([]string), moduleID)
    }

    // 从数据库查询
    module, err := ms.db.Module.Query().
        Where(modulepredicate.ID(moduleID)).
        Only(ctx)
    if err != nil {
        return false  // 模块不存在或查询出错，认为禁用
    }

    return module.Enabled
}

// GetModuleConfig 获取模块配置
func (ms *ModuleService) GetModuleConfig(ctx context.Context, moduleID string) (map[string]interface{}, error) {
    module, err := ms.db.Module.Query().
        Where(modulepredicate.ID(moduleID)).
        Only(ctx)
    if err != nil {
        return nil, err
    }

    var config map[string]interface{}
    if err := json.Unmarshal(module.Config, &config); err != nil {
        return nil, err
    }

    return config, nil
}

// SetModuleConfig 更新模块配置
func (ms *ModuleService) SetModuleConfig(ctx context.Context, moduleID string, config map[string]interface{}) error {
    user := ctx.Value("user").(*User)
    if !user.IsSuperAdmin() {
        return ErrUnauthorized
    }

    // 序列化新配置
    configJSON, err := json.Marshal(config)
    if err != nil {
        return err
    }

    // 获取旧配置用于审计
    oldModule, _ := ms.db.Module.Get(ctx, moduleID)
    oldConfig := oldModule.Config

    // 更新数据库
    err = ms.db.Module.UpdateOneID(moduleID).
        SetConfig(configJSON).
        SetUpdatedAt(time.Now()).
        SetUpdatedBy(user.ID).
        Exec(ctx)
    if err != nil {
        return err
    }

    // 记录审计日志
    ms.auditLog.RecordModuleAction(ctx, &ModuleAuditEntry{
        ModuleID:  moduleID,
        Action:    "CONFIG_CHANGE",
        OldConfig: oldConfig,
        NewConfig: configJSON,
        UserID:    user.ID,
        Timestamp: time.Now(),
    })

    ms.cache.Delete("module_config_" + moduleID)

    return nil
}

// 辅助函数：创建默认角色
func (ms *ModuleService) createDefaultRoles(ctx context.Context, module *Module) error {
    for _, roleName := range module.DefaultRoles {
        // 创建角色（如果不存在）
        _, err := ms.db.Role.Create().
            SetID(roleName).
            SetName(moduleName(roleName)).
            SetDescription(fmt.Sprintf("%s - %s 模块", module.Name, roleName)).
            OnConflict().
            DoNothing().
            Save(ctx)
        if err != nil {
            return err
        }
    }
    return nil
}
```

### 3.2 权限中间件扩展

```go
// internal/web/middleware/module_check.go

// RequireModuleEnabled 中间件：检查模块是否启用
func RequireModuleEnabled(moduleService *ModuleService) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // 从路由参数推断模块 ID
        moduleID := extractModuleID(c.Path())

        if !moduleService.IsModuleEnabled(c.Context(), moduleID) {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": fmt.Sprintf("模块 '%s' 未启用", moduleID),
            })
        }

        return c.Next()
    }
}

// RequireFeature 中间件：检查用户是否拥有特定功能权限
func RequireFeature(moduleService *ModuleService) fiber.Handler {
    return func(c *fiber.Ctx) error {
        featureID := c.Locals("requiredFeature").(string)  // 由路由定义
        user := c.Locals("user").(*User)
        companyID := c.Locals("company_id").(string)

        // 检查用户是否拥有此功能权限
        hasFeature, err := moduleService.UserHasFeature(c.Context(), user.ID, companyID, featureID)
        if err != nil {
            return err
        }

        if !hasFeature {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": fmt.Sprintf("您没有权限访问功能: %s", featureID),
            })
        }

        return c.Next()
    }
}

// 路由注册示例（应用中间件）
func RegisterPayrollRoutes(router fiber.Router, ms *ModuleService, ps *PayrollService) {
    payrollGroup := router.Group("/payroll", RequireModuleEnabled(ms))

    // 查看薪资运行（需要 payroll.view 权限）
    payrollGroup.Get("/runs",
        SetFeatureRequired("payroll.view"),
        RequireFeature(ms),
        ps.ListPayrollRuns)

    // 创建薪资运行（需要 payroll.create 权限）
    payrollGroup.Post("/runs",
        SetFeatureRequired("payroll.create"),
        RequireFeature(ms),
        ps.CreatePayrollRun)

    // 锁定薪资运行（需要 payroll.finalize 权限）
    payrollGroup.Post("/runs/:id/finalize",
        SetFeatureRequired("payroll.finalize"),
        RequireFeature(ms),
        ps.FinalizePayroll)
}
```

---

## 四、后台管理界面 - 模块配置

### 4.1 系统管理页面路由

```go
// internal/web/handlers/admin/modules.go

// GetModulesPage 返回模块管理页面
func GetModulesPage(c *fiber.Ctx, ms *ModuleService) error {
    user := c.Locals("user").(*User)

    // 仅 SuperAdmin 可访问
    if !user.IsSuperAdmin() {
        return c.Status(fiber.StatusForbidden).Render("error_forbidden", nil)
    }

    // 获取所有模块状态
    modules, err := ms.GetAllModules(c.Context())
    if err != nil {
        return err
    }

    // 获取审计日志
    auditLog, err := ms.GetModuleAuditLog(c.Context(), 20)
    if err != nil {
        return err
    }

    return c.Render("admin/modules_dashboard", fiber.Map{
        "modules": modules,
        "auditLog": auditLog,
    })
}

// EnableModule API 端点
func (h *AdminHandler) EnableModule(c *fiber.Ctx, ms *ModuleService) error {
    moduleID := c.Params("id")
    reason := c.FormValue("reason", "")

    if err := ms.EnableModule(c.Context(), moduleID, reason); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.JSON(fiber.Map{
        "message": fmt.Sprintf("模块 '%s' 已启用", moduleID),
    })
}

// DisableModule API 端点
func (h *AdminHandler) DisableModule(c *fiber.Ctx, ms *ModuleService) error {
    moduleID := c.Params("id")
    reason := c.FormValue("reason", "")

    if err := ms.DisableModule(c.Context(), moduleID, reason); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.JSON(fiber.Map{
        "message": fmt.Sprintf("模块 '%s' 已禁用", moduleID),
    })
}

// UpdateModuleConfig API 端点
func (h *AdminHandler) UpdateModuleConfig(c *fiber.Ctx, ms *ModuleService) error {
    moduleID := c.Params("id")
    var configData map[string]interface{}

    if err := c.BodyParser(&configData); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "配置格式错误",
        })
    }

    if err := ms.SetModuleConfig(c.Context(), moduleID, configData); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.JSON(fiber.Map{
        "message": "配置已更新",
    })
}
```

### 4.2 Templ UI 组件

**文件：`internal/web/templates/admin/modules_dashboard.templ`**

```templ
package admin

import "fmt"

templ ModulesDashboard(modules []*ModuleStatus, auditLog []*AuditEntry) {
  <div class="container mx-auto p-6">
    <h1 class="text-3xl font-bold mb-6">系统模块管理</h1>

    <!-- 模块启用状态卡片 -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
      for _, mod := range modules {
        @ModuleCard(mod)
      }
    </div>

    <!-- 模块操作日志 -->
    <div class="card bg-base-200 shadow-xl">
      <div class="card-body">
        <h2 class="card-title">最近操作日志</h2>
        <table class="table table-sm">
          <thead>
            <tr>
              <th>模块</th>
              <th>操作</th>
              <th>执行人</th>
              <th>原因</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            for _, entry := range auditLog {
              <tr>
                <td class="font-mono">{ entry.ModuleID }</td>
                <td>
                  <span class={ auditActionBadge(entry.Action) }>
                    { entry.Action }
                  </span>
                </td>
                <td>{ entry.UserName }</td>
                <td>{ entry.Reason }</td>
                <td>{ entry.Timestamp.Format("2006-01-02 15:04") }</td>
              </tr>
            }
          </tbody>
        </table>
      </div>
    </div>
  </div>
}

templ ModuleCard(mod *ModuleStatus) {
  <div class={ "card shadow-lg", statusCardClass(mod.Enabled) }>
    <div class="card-body">
      <!-- 模块标题和状态 -->
      <div class="flex justify-between items-start">
        <div>
          <h3 class="card-title text-lg">{ mod.Name }</h3>
          <p class="text-sm text-gray-600">{ mod.Description }</p>
          <p class="text-xs text-gray-500 mt-2">v{ mod.Version }</p>
        </div>
        <span class={ statusBadge(mod.Enabled) }>
          if mod.Enabled {
            已启用
          } else {
            已禁用
          }
        </span>
      </div>

      <!-- 依赖项展示 -->
      if len(mod.Dependencies) > 0 {
        <div class="mt-3 p-2 bg-blue-50 rounded text-sm">
          <p class="font-semibold mb-1">依赖模块：</p>
          <div class="flex gap-1 flex-wrap">
            for _, dep := range mod.Dependencies {
              <span class="badge badge-outline">{ dep }</span>
            }
          </div>
        </div>
      }

      <!-- 操作按钮 -->
      <div class="card-actions justify-end mt-4 gap-2">
        if mod.Enabled {
          <!-- 禁用按钮 -->
          <button hx-post={ "/admin/api/modules/" + mod.ID + "/disable" }
                  hx-prompt="禁用此模块的原因（可选）："
                  hx-confirm="确认禁用此模块？相关功能将被隐藏。"
                  class="btn btn-sm btn-warning">
            禁用
          </button>
        } else {
          <!-- 启用按钮 -->
          <button hx-post={ "/admin/api/modules/" + mod.ID + "/enable" }
                  hx-prompt="启用此模块的原因（可选）："
                  hx-confirm={ fmt.Sprintf("确认启用 '%s' 模块？", mod.Name) }
                  class="btn btn-sm btn-success">
            启用
          </button>
        }

        <!-- 配置按钮 -->
        <button @click={ fmt.Sprintf("showModuleConfig('%s')", mod.ID) }
                class="btn btn-sm btn-outline">
          配置
        </button>
      </div>
    </div>
  </div>
}

// 辅助函数
func statusBadge(enabled bool) string {
  if enabled {
    return "badge badge-success"
  }
  return "badge badge-warning"
}

func statusCardClass(enabled bool) string {
  if enabled {
    return "border-l-4 border-green-500"
  }
  return "border-l-4 border-yellow-500"
}

func auditActionBadge(action string) string {
  switch action {
  case "ENABLE":
    return "badge badge-success"
  case "DISABLE":
    return "badge badge-warning"
  case "CONFIG_CHANGE":
    return "badge badge-info"
  default:
    return "badge"
  }
}
```

---

## 五、集成流程与路由配置

### 5.1 应用启动时初始化模块

```go
// cmd/balanciz/main.go 的关键部分

func main() {
    // ... 初始化 Fiber、数据库等

    // 1. 初始化模块注册表
    moduleRegistry := modules.NewRegistry()
    moduleRegistry.Register(modules.EmployeeModule)
    moduleRegistry.Register(modules.PayrollModule)
    moduleRegistry.Register(modules.ChequeModule)
    moduleRegistry.Register(modules.TaskModule)

    // 2. 初始化模块服务
    moduleService := services.NewModuleService(db, auditLogger, cache)

    // 3. 仅注册启用的模块的路由
    enabledModules, _ := moduleService.GetEnabledModules(context.Background())
    for _, moduleID := range enabledModules {
        module := moduleRegistry.Get(moduleID)
        if module != nil {
            module.Routes(router)  // 动态注册路由
            log.Printf("✓ 模块 '%s' 路由已注册", moduleID)
        }
    }

    // 4. 注册系统管理路由（始终启用）
    admin.RegisterAdminRoutes(router, moduleService)

    // ... 启动服务器
    router.Listen(":3000")
}
```

### 5.2 模块的路由注册函数

```go
// internal/modules/payroll/module.go

func (m *Module) Routes(router fiber.Router) {
    payrollGroup := router.Group("/payroll")

    // 中间件：检查模块启用状态
    payrollGroup.Use(RequireModuleEnabled("payroll"))

    // 路由定义
    payrollGroup.Get("/runs", ListPayrollRuns)
    payrollGroup.Post("/runs", CreatePayrollRun)
    payrollGroup.Get("/runs/:id", GetPayrollRun)
    payrollGroup.Post("/runs/:id/finalize", FinalizePayroll)
    payrollGroup.Get("/runs/:id/export-t4", ExportT4)
}

// 模块初始化钩子（可选）
func (m *Module) Init(ctx context.Context) error {
    log.Printf("初始化 Payroll 模块")
    // 启动后台任务、预热缓存等
    return nil
}
```

---

## 六、前端导航与模块可见性

### 6.1 Templ 组件：动态菜单

```templ
// internal/web/templates/layout/sidebar.templ

package layout

import "context"

templ Sidebar(ctx context.Context, user *User, moduleService *ModuleService) {
  <aside class="w-64 bg-gray-900 text-white h-screen">
    <nav class="p-4">
      <!-- 核心菜单（始终显示） -->
      <div class="mb-6">
        <h2 class="text-sm uppercase font-semibold mb-3">核心模块</h2>
        <ul class="space-y-2">
          <li><a href="/dashboard" class="block p-2 hover:bg-gray-800 rounded">仪表板</a></li>
          <li><a href="/accounts" class="block p-2 hover:bg-gray-800 rounded">会计</a></li>
          <li><a href="/reports" class="block p-2 hover:bg-gray-800 rounded">报表</a></li>
        </ul>
      </div>

      <!-- 扩展功能（条件显示） -->
      if isModuleEnabled(ctx, moduleService, "employee") {
        <div class="mb-6 pb-6 border-b border-gray-700">
          <h2 class="text-sm uppercase font-semibold mb-3">人力资源</h2>
          <ul class="space-y-2">
            <li><a href="/employee" class="block p-2 hover:bg-gray-800 rounded">员工管理</a></li>
            if isModuleEnabled(ctx, moduleService, "payroll") {
              <li><a href="/payroll" class="block p-2 hover:bg-gray-800 rounded">薪资管理</a></li>
              <li><a href="/payroll/runs" class="block p-2 hover:bg-gray-800 rounded">薪资运行</a></li>
            }
          </ul>
        </div>
      }

      if isModuleEnabled(ctx, moduleService, "task") {
        <div class="mb-6 pb-6 border-b border-gray-700">
          <h2 class="text-sm uppercase font-semibold mb-3">业务</h2>
          <ul class="space-y-2">
            <li><a href="/task" class="block p-2 hover:bg-gray-800 rounded">任务管理</a></li>
          </ul>
        </div>
      }

      if isModuleEnabled(ctx, moduleService, "cheque") {
        <div class="mb-6 pb-6 border-b border-gray-700">
          <h2 class="text-sm uppercase font-semibold mb-3">现金</h2>
          <ul class="space-y-2">
            <li><a href="/cheque" class="block p-2 hover:bg-gray-800 rounded">支票管理</a></li>
          </ul>
        </div>
      }

      <!-- 系统管理（仅 SuperAdmin） -->
      if user.IsSuperAdmin() {
        <div class="mt-8 pt-6 border-t border-gray-700">
          <h2 class="text-sm uppercase font-semibold mb-3">系统</h2>
          <ul class="space-y-2">
            <li><a href="/admin/modules" class="block p-2 hover:bg-gray-800 rounded text-yellow-300">⚙️ 模块管理</a></li>
            <li><a href="/admin/users" class="block p-2 hover:bg-gray-800 rounded">👥 用户管理</a></li>
            <li><a href="/admin/audit" class="block p-2 hover:bg-gray-800 rounded">📋 审计日志</a></li>
          </ul>
        </div>
      }
    </nav>
  </aside>
}

func isModuleEnabled(ctx context.Context, ms *ModuleService, moduleID string) bool {
  return ms.IsModuleEnabled(ctx, moduleID)
}
```

### 6.2 前端权限检查（HTMX）

```html
<!-- 仅当用户拥有 "task.invoice" 权限且模块启用时显示按钮 -->
<button hx-post="/api/tasks/{{ task.ID }}/invoice"
        data-feature="task.invoice"
        class="btn btn-sm"
        :style="{ display: userHasFeature('task.invoice') ? 'inline-block' : 'none' }">
  生成发票
</button>

<script>
function userHasFeature(featureID) {
  // 前端缓存用户权限和模块状态
  const userFeatures = window.__USER_FEATURES || [];
  return userFeatures.includes(featureID);
}
</script>
```

---

## 七、数据库迁移文件

**文件：`migrations/030_add_simpletask_modules.sql`**

```sql
-- 模块管理表
CREATE TABLE IF NOT EXISTS modules (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(20),
    enabled BOOLEAN DEFAULT FALSE,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS module_features (
    id SERIAL PRIMARY KEY,
    module_id VARCHAR(50) NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    feature_id VARCHAR(100) NOT NULL,
    feature_name VARCHAR(255),
    role_id VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(module_id, feature_id)
);

CREATE TABLE IF NOT EXISTS module_audit_log (
    id SERIAL PRIMARY KEY,
    module_id VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    old_config JSONB,
    new_config JSONB,
    user_id UUID REFERENCES users(id),
    reason TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX idx_modules_enabled ON modules(enabled);
CREATE INDEX idx_module_features_module ON module_features(module_id);
CREATE INDEX idx_module_audit_log_module ON module_audit_log(module_id, timestamp DESC);

-- 初始化模块数据
INSERT INTO modules (id, name, description, version, enabled) VALUES
('employee', '员工管理', '员工信息、档案、离职管理', '1.0.0', false),
('payroll', '薪资管理', '薪资计算、税务、T4 导出', '1.0.0', false),
('cheque', '支票管理', '支票生成、签署打印', '1.0.0', false),
('task', '任务管理', '日常任务、快速发票、月度报表', '1.0.0', false)
ON CONFLICT DO NOTHING;

-- 插入模块功能
INSERT INTO module_features (module_id, feature_id, feature_name, role_id) VALUES
('employee', 'employee.view', '查看员工', 'employee_viewer'),
('employee', 'employee.create', '新建员工', 'employee_admin'),
('employee', 'employee.edit', '编辑员工', 'employee_admin'),
('employee', 'employee.terminate', '终止员工', 'employee_admin'),
('payroll', 'payroll.view', '查看薪资', 'payroll_viewer'),
('payroll', 'payroll.create', '创建薪资运行', 'payroll_admin'),
('payroll', 'payroll.finalize', '锁定薪资', 'payroll_admin'),
('payroll', 'payroll.export', '导出 T4', 'payroll_admin'),
('cheque', 'cheque.view', '查看支票', 'cheque_viewer'),
('cheque', 'cheque.create', '创建支票', 'cheque_admin'),
('cheque', 'cheque.issue', '签署支票', 'cheque_signor'),
('cheque', 'cheque.print', '打印支票', 'cheque_viewer'),
('task', 'task.view', '查看任务', 'task_viewer'),
('task', 'task.create', '新建任务', 'task_admin'),
('task', 'task.complete', '完成任务', 'task_admin'),
('task', 'task.invoice', '生成发票', 'task_admin'),
('task', 'task.report', '月度报表', 'task_viewer')
ON CONFLICT DO NOTHING;
```

---

## 八、模块依赖图

```
┌─────────────────┐
│   Employee      │  (独立)
└────────┬────────┘
         │ 依赖
    ┌────▼────────┐
    │   Payroll   │
    └─────────────┘

┌─────────────────┐
│     Cheque      │  (独立)
└─────────────────┘

┌──────────────┐
│     Task     │  (独立)
└──────┬───────┘
       │
    ┌──▼──────────┐
    └─────────────┘
```

---

## 九、启用模块的步骤

### 用户操作流程

1. **管理员访问系统管理 → 模块管理**
   ```
   https://your-balanciz.com/admin/modules
   ```

2. **查看模块列表**
   - 每个模块显示启用/禁用状态
   - 显示模块版本和描述
   - 显示依赖关系

3. **启用薪资模块（示例）**
   - 点击"薪资管理"卡片上的"启用"按钮
   - 系统提示：需要先启用"员工管理"模块
   - 点击"启用员工管理"
   - 再启用"薪资管理"
   - 后台执行：数据库迁移 → 创建角色 → 更新模块状态

4. **模块立即生效**
   - Sidebar 菜单中出现"薪资管理"菜单项
   - 用户可访问 `/payroll/*` 路由（验证权限）
   - API 端点可用

5. **禁用模块**
   - 点击"禁用"按钮（需输入原因）
   - 模块状态更新，菜单消失
   - 已有数据保留（可重新启用恢复）

---

## 十、权限场景示例

### 场景 1：小公司，仅启用 Task 模块

```
模块配置：
✓ Employee: 禁用
✓ Payroll: 禁用
✓ Cheque: 禁用
✓ Task: 启用

Sidebar 菜单仅显示：
- 仪表板
- 会计
- 报表
- 任务管理 ← 仅此项来自 SimpleTask

用户角色：
- task_admin (可创建任务、生成发票)
- task_viewer (可查看任务和报表)
```

### 场景 2：中等公司，启用 Payroll + Employee + Cheque

```
模块配置：
✓ Employee: 启用
✓ Payroll: 启用 (依赖 Employee)
✓ Cheque: 启用
✓ Task: 禁用

Sidebar 菜单：
- 仪表板
- 会计
- 报表
- 人力资源
  - 员工管理
  - 薪资管理
  - 薪资运行
- 现金
  - 支票管理

用户角色权限示例：
- payroll_admin: 可创建、锁定薪资，导出 T4
- payroll_viewer: 仅查看薪资
- cheque_signor: 可签署支票（触发 GL 分录）
- cheque_admin: 创建支票草稿
- employee_admin: 员工 CRUD
```

### 场景 3：模块禁用对功能的影响

当禁用 Payroll 模块时：
- ✗ `/payroll/*` 路由返回 403 Forbidden
- ✗ Sidebar"薪资管理"菜单消失
- ✗ API `/api/payroll/*` 端点不可用
- ✓ 数据库数据保留（不删除）
- ✓ 审计日志记录禁用事件
- ✓ 重新启用时数据恢复

---

## 十一、开发流程小结

```
1. 创建新模块
   ├─ 编写 Module 定义 (ID, Name, Dependencies, Routes)
   ├─ 编写迁移文件 (SQL schema)
   ├─ 编写 Service 层逻辑
   ├─ 编写 API handler
   └─ 写入 Templ 模板

2. 注册到系统
   ├─ ModuleRegistry.Register(MyModule)
   ├─ 在 main.go 中初始化
   └─ 系统启动时自动加载启用的模块

3. 管理员启用
   ├─ 访问 /admin/modules
   ├─ 检查依赖关系
   ├─ 点击"启用"
   └─ 自动执行迁移 + 角色创建

4. 运行时检查
   ├─ RequireModuleEnabled 中间件检查模块启用状态
   ├─ RequireFeature 中间件检查用户权限
   └─ 前端检查 window.__USER_FEATURES 显示/隐藏菜单
```

---

## 十二、成本与收益

| 方面 | 价值 |
|-----|-----|
| **灵活性** | ⭐⭐⭐⭐⭐ - 可按需启用/禁用功能模块 |
| **可维护性** | ⭐⭐⭐⭐ - 模块独立，清晰的依赖管理 |
| **数据安全** | ⭐⭐⭐⭐⭐ - 完整的审计日志，权限分离 |
| **升级风险** | ⭐⭐⭐⭐ - 单个模块升级独立，无全局影响 |
| **多租户支持** | ⭐⭐⭐⭐ - 可按租户启用模块组合 |

---

这个方案确保：
1. **Balanciz 核心不受影响** - 所有 SimpleTask 功能都是可选模块
2. **后台完全控制** - 管理员可随时启用/禁用任何模块
3. **权限粒度细** - 功能级别的权限控制
4. **审计可追溯** - 所有模块操作都有日志记录
5. **迁移风险最小** - 现有系统完全兼容
