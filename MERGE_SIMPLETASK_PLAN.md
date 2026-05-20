# Balanciz + SimpleTask 合并方案

**日期**：2026-05-15
**目标**：将 SimpleTask 的功能（薪资、任务、支票、员工）整合到 Balanciz 中
**总工作量估算**：8-12 周（含测试、迁移、验证）

---

## 一、项目现状分析

### 1.1 Balanciz（母项目）
| 方面 | 详情 |
|-----|-----|
| **定位** | 多公司会计系统 + AI辅助层 |
| **数据库** | PostgreSQL + GORM（主）+ 显式SQL（报表） |
| **框架** | Fiber + Templ + HTMX + Alpine + React Islands |
| **架构** | 模块化单体 + DDD 核心域 |
| **核心引擎** | Posting、Tax、Reconciliation、FX、Permission、Audit |
| **关键特性** | 多公司隔离、审计日志、11 type 报表、搜索投影 |
| **已有模块** | AR（A/R 边）、AP（A/P 边）、General Ledger、Customer/Vendor/Product |

### 1.2 SimpleTask（子项目）
| 方面 | 详情 |
|-----|-----|
| **定位** | 任务与价目表管理 + 薪资系统 |
| **数据库** | SQLite（新建）+ migration 自动导入 |
| **框架** | net/http + 嵌入式 Web（5 个子应用） |
| **架构** | 功能模块化（无 DDD） |
| **特色模块** | Payroll（CRA 合规）、WriteCheque、Employee、ReportExport |
| **可复用资产** | 薪资计算、支票生成、员工管理、CSV 导出 |

### 1.3 合并的关键差异

| 维度 | Balanciz | SimpleTask | 解决方案 |
|-----|---------|-----------|--------|
| **数据库** | PostgreSQL | SQLite | 将 SimpleTask SQLite 结构迁移到 PostgreSQL schema |
| **认证** | 多用户、权限系统 | 单/多用户、SQLite users | 统一到 Balanciz auth + permission engine |
| **UI 框架** | Templ + HTMX + React | 嵌入式 HTML/CSS/JS | 将 SimpleTask 页面改写为 Templ + HTMX |
| **多公司** | native 多公司隔离 | 无多公司隔离 | Task/Payroll/Invoice 加 company_id 字段 |
| **会计双分录** | Yes（强制） | No（Task->Invoice 简化） | Task 生成的 Invoice 需通过 Posting Engine |

---

## 二、合并策略

### 选项对比

```
┌──────────────────────────────────────────────────────────┐
│                    合并选项对比                            │
├──────┬─────────────────┬─────────────────┬──────────────┤
│      │   完全融合      │    模块集成      │   微服务独立  │
├──────┼─────────────────┼─────────────────┼──────────────┤
│工作量│   12 周（高）   │   8 周（中）    │  5 周（低）  │
│维护性│   高（统一）    │   中（适度耦合）│  低（独立）  │
│复用性│   高            │   高            │  中          │
│可靠性│   中（改动大）  │   高（改动小）  │  高（独立）  │
└──────┴─────────────────┴─────────────────┴──────────────┘
```

### **推荐方案：选项 2 - 模块集成**

**理由**：
1. 充分利用 Balanciz 的会计严格性和权限系统
2. 将 SimpleTask 强功能模块（Payroll、WriteCheque）移植到 Balanciz
3. 降低代码重构压力，逐步迁移，便于验证
4. 保持 Balanciz 架构清洁

---

## 三、详细集成方案

### 阶段 1：数据库与表结构迁移（2 周）

#### 1.1 创建 SimpleTask 相关 PostgreSQL schema

```sql
-- 员工管理
CREATE TABLE employee (
    id SERIAL PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES company(id),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    sin VARCHAR(11),  -- Social Insurance Number
    effective_date DATE,
    termination_date DATE,
    salary_type VARCHAR(50),  -- SALARY | HOURLY
    base_salary DECIMAL(15,2),
    frequency VARCHAR(20),  -- WEEKLY | BIWEEKLY | MONTHLY
    status VARCHAR(50),  -- ACTIVE | INACTIVE | TERMINATED
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 薪资记录
CREATE TABLE payroll_run (
    id SERIAL PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES company(id),
    run_date DATE NOT NULL,
    pay_period_start DATE NOT NULL,
    pay_period_end DATE NOT NULL,
    status VARCHAR(50),  -- DRAFT | FINALIZED | ARCHIVED
    ytd_snapshot JSON,  -- 年初至今快照
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id)
);

CREATE TABLE payroll_detail (
    id SERIAL PRIMARY KEY,
    payroll_run_id INT NOT NULL REFERENCES payroll_run(id),
    employee_id INT NOT NULL REFERENCES employee(id),
    gross_amount DECIMAL(15,2),
    deductions JSON,  -- {cpp: 100, ei: 50, income_tax: 200}
    net_amount DECIMAL(15,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 支票记录
CREATE TABLE cheque (
    id SERIAL PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES company(id),
    cheque_number VARCHAR(20) NOT NULL,
    payee_name VARCHAR(255),
    amount DECIMAL(15,2),
    cheque_date DATE,
    bank_account_id INT,
    status VARCHAR(50),  -- DRAFT | ISSUED | CLEARED | VOIDED
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 任务管理
CREATE TABLE task (
    id SERIAL PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES company(id),
    task_date DATE NOT NULL,
    description TEXT,
    customer_id INT REFERENCES customer(id),
    service_id INT REFERENCES product(id),  -- 复用 product 作为服务
    quantity DECIMAL(10,2),
    unit_price DECIMAL(15,4),
    status VARCHAR(50),  -- OPEN | COMPLETED | BILLED
    completed_date DATE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 任务快速发票映射
CREATE TABLE task_invoice_mapping (
    task_id INT NOT NULL REFERENCES task(id),
    invoice_id INT NOT NULL REFERENCES invoice(id),
    PRIMARY KEY (task_id, invoice_id)
);

-- 创建索引以提升查询性能
CREATE INDEX idx_employee_company ON employee(company_id);
CREATE INDEX idx_payroll_run_company ON payroll_run(company_id);
CREATE INDEX idx_payroll_run_date ON payroll_run(run_date);
CREATE INDEX idx_cheque_company ON cheque(company_id);
CREATE INDEX idx_task_company ON task(company_id);
CREATE INDEX idx_task_date ON task(task_date);
```

#### 1.2 数据迁移脚本

创建 `migrations/020_add_simpletask_modules.sql`：
- 包含上述表创建 SQL
- 迁移 SimpleTask SQLite 数据到 PostgreSQL
- 创建数据验证视图

#### 1.3 Ent 模型生成

在 `ent/schema/` 下创建：
```
employee.go
payroll_run.go
payroll_detail.go
cheque.go
task.go
```

更新 `ent/generate.go` 并运行代码生成。

---

### 阶段 2：后端服务集成（3 周）

#### 2.1 创建核心服务模块

**目录结构**：
```
internal/services/
├── payroll.go          # 薪资计算（集成 CRA 规则）
├── employee.go         # 员工管理
├── cheque.go           # 支票生成
├── task.go             # 任务管理
└── task_to_invoice.go  # 任务生成发票工作流
```

#### 2.2 关键服务伪代码

**A. 员工服务** (`internal/services/employee.go`)
```go
type EmployeeService struct {
    db *ent.Client
}

// CreateEmployee 创建员工（含公司隔离）
func (es *EmployeeService) CreateEmployee(ctx context.Context, companyID string, emp *Employee) (*model.Employee, error) {
    // 验证 company_id 权限（通过 middleware）
    // 保存到 employee 表
}

// GetActiveEmployees 获取活跃员工列表
func (es *EmployeeService) GetActiveEmployees(ctx context.Context, companyID string) ([]*model.Employee, error)
```

**B. 薪资服务** (`internal/services/payroll.go`)

```go
// PayrollService 集成 CRA 规则
type PayrollService struct {
    db       *ent.Client
    taxRule  *TaxEngine  // 复用 Balanciz tax engine
}

// CalculatePayroll 计算薪资（含税收和 CPP/EI）
func (ps *PayrollService) CalculatePayroll(ctx context.Context, runID int, empID int) (*Payroll, error) {
    // 读取员工基础信息
    // 调用 CRA 税务规则（SimpleFax 已有 CRA T4001 参考）
    // 计算 gross / cpp / ei / tax / net
    // 生成 payroll_detail 记录
}

// FinalizePayroll 锁定薪资并生成会计分录
func (ps *PayrollService) FinalizePayroll(ctx context.Context, runID int) error {
    // 1. 检查 payroll_run 状态
    // 2. 批量生成 GL posting（调用 Posting Engine）
    //    - Debit: Payroll Expense
    //    - Credit: CPP/EI Payable, Income Tax Payable, Salary Payable
    // 3. 逐行存储 posting 记录
    // 4. 生成审计日志
    // 5. 更新 payroll_run status = FINALIZED
}

// ExportT4 生成 T4 表单数据
func (ps *PayrollService) ExportT4(ctx context.Context, companyID string, taxYear int) ([]map[string]interface{}, error)
```

**C. 任务转发票服务** (`internal/services/task_to_invoice.go`)

```go
type TaskToInvoiceService struct {
    db                  *ent.Client
    invoiceService      *InvoiceService      // Balanciz 现有发票服务
    postingEngine       PostingEngine        // 双分录
}

// QuickCreateInvoiceFromTask 从任务快速生成发票
func (s *TaskToInvoiceService) QuickCreateInvoiceFromTask(ctx context.Context, taskID int) (*Invoice, error) {
    // 1. 读取 task 记录
    // 2. 创建 invoice（调用 Balanciz invoiceService）
    // 3. 链接 task_invoice_mapping
    // 4. 生成双分录（收入 + AR）
    // 5. 标记任务 status = BILLED
    // 6. 返回 invoice record + posting result
}

// BatchCreateInvoicesFromTasks 批量生成
func (s *TaskToInvoiceService) BatchCreateInvoicesFromTasks(ctx context.Context, companyID string, taskIDs []int) error
```

**D. 支票服务** (`internal/services/cheque.go`)

```go
type ChequeService struct {
    db *ent.Client
}

// CreateCheque 创建支票草稿
func (cs *ChequeService) CreateCheque(ctx context.Context, companyID string, cheque *Cheque) (*model.Cheque, error) {
    // 1. 创建 cheque 记录（status = DRAFT）
    // 2. 触发支票号自动分配（或手动输入）
    // 3. 存储支票信息
}

// MarkChequeIssued 标记支票已签发并生成会计分录
func (cs *ChequeService) MarkChequeIssued(ctx context.Context, chequeID int) error {
    // 1. 检查支票草稿有效性
    // 2. 生成 GL posting（Debit: 应付款/费用，Credit: 银行账户）
    // 3. 更新 cheque status = ISSUED
    // 4. 生成审计日志
}

// PrintCheque 返回格式化支票数据（用于 PDF 渲染）
func (cs *ChequeService) PrintCheque(ctx context.Context, chequeID int) (map[string]interface{}, error)
```

#### 2.3 API 路由集成

在现有 `internal/web/` 路由中添加：

```go
// routes/payroll.go
func RegisterPayrollRoutes(router fiber.Router, svc *PayrollService) {
    router.Post("/payroll/runs", createPayrollRun)
    router.Get("/payroll/runs", listPayrollRuns)
    router.Get("/payroll/runs/:id", getPayrollRun)
    router.Post("/payroll/runs/:id/finalize", finalizePayroll)
    router.Get("/payroll/runs/:id/export-t4", exportT4)
}

// routes/employee.go
func RegisterEmployeeRoutes(router fiber.Router, svc *EmployeeService) {
    router.Post("/employees", createEmployee)
    router.Get("/employees", listEmployees)
    router.Get("/employees/:id", getEmployee)
    router.Put("/employees/:id", updateEmployee)
    router.Delete("/employees/:id", deleteEmployee)
}

// routes/cheque.go
func RegisterChequeRoutes(router fiber.Router, svc *ChequeService) {
    router.Post("/cheques", createCheque)
    router.Get("/cheques", listCheques)
    router.Post("/cheques/:id/issue", issueCheque)
    router.Get("/cheques/:id/print", printCheque)
}

// routes/task.go
func RegisterTaskRoutes(router fiber.Router, svc *TaskService, tti *TaskToInvoiceService) {
    router.Post("/tasks", createTask)
    router.Get("/tasks", listTasks)
    router.Post("/tasks/:id/invoice", quickCreateInvoice)
    router.Get("/tasks/monthly-report", monthlyReport)
}
```

---

### 阶段 3：前端集成（2.5 周）

#### 3.1 界面框架选择

SimpleTask 的当前 UI 使用：
- HTML/CSS 嵌入式
- 少量 JavaScript（Alpine 类似）

**迁移策略**：采用 **Templ + HTMX + Alpine**（与 Balanciz 现有框架统一）

#### 3.2 创建 Templ 组件

新增 `internal/web/templates/` 子目录：

```
templates/
├── payroll/
│   ├── run_list.templ        # 薪资运行列表
│   ├── run_detail.templ      # 薪资运行详情与编辑
│   ├── employee_selector.templ # 员工多选组件
│   └── payroll_pdf_export.templ # T4/T4A 导出预览
├── employee/
│   ├── list.templ            # 员工列表
│   ├── form.templ            # 员工表单（新建/编辑）
│   └── terminate.templ       # 终止员工对话框
├── cheque/
│   ├── list.templ            # 支票列表
│   ├── form.templ            # 支票表单
│   └── print_preview.templ    # 支票打印预览
└── task/
    ├── list.templ            # 任务列表（含日期/状态过滤）
    ├── form.templ            # 任务表单
    ├── quick_invoice.templ    # "生成发票"模态框
    └── monthly_report.templ   # 月度汇总报表
```

#### 3.3 示例 Templ 组件

**文件：`internal/web/templates/payroll/run_list.templ`**

```templ
package payroll

import "fmt"

templ PayrollRunList(companyID string, runs []*PayrollRun, currentUser *User) {
  <div class="container mx-auto p-4">
    <h1 class="text-2xl font-bold mb-4">薪资运行</h1>

    <!-- 新建按钮 -->
    <button hx-get="/payroll/runs/new"
            hx-trigger="click"
            hx-target="#modal"
            class="btn btn-primary mb-4">
      + 新建薪资运行
    </button>

    <!-- 薪资运行列表 -->
    <table class="table table-zebra w-full">
      <thead>
        <tr>
          <th>运行ID</th>
          <th>支付周期</th>
          <th>状态</th>
          <th>员工数</th>
          <th>总津贴</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        for _, run := range runs {
          <tr>
            <td>{ fmt.Sprintf("%d", run.ID) }</td>
            <td>
              { run.PayPeriodStart.Format("2006-01-02") } ~ { run.PayPeriodEnd.Format("2006-01-02") }
            </td>
            <td>
              <span class={ statusBadgeClass(run.Status) }>{ run.Status }</span>
            </td>
            <td>{ fmt.Sprintf("%d", run.EmployeeCount) }</td>
            <td>{ formatCAD(run.TotalGross) }</td>
            <td>
              <a href={ templ.SafeURL(fmt.Sprintf("/payroll/runs/%d", run.ID)) }
                 class="link link-primary">查看</a>
              if run.Status == "DRAFT" {
                <button hx-put={ fmt.Sprintf("/payroll/runs/%d/finalize", run.ID) }
                        hx-confirm="确认锁定此薪资运行？"
                        class="link link-success ml-2">锁定</button>
              }
            </td>
          </tr>
        }
      </tbody>
    </table>
  </div>
}

func statusBadgeClass(status string) string {
  switch status {
  case "DRAFT":
    return "badge badge-warning"
  case "FINALIZED":
    return "badge badge-success"
  default:
    return "badge"
  }
}

func formatCAD(amount float64) string {
  return fmt.Sprintf("$%.2f CAD", amount)
}
```

#### 3.4 HTMX 交互例子

**任务快速生成发票**（HTMX + Alpine）

```html
<!-- 任务列表行 -->
<tr x-data="{ showInvoiceForm: false }">
  <td>{{ task.Date }}</td>
  <td>{{ task.Description }}</td>
  <td>{{ task.Customer }}</td>
  <td>{{ formatPrice(task.Amount) }}</td>
  <td>
    <span class="badge badge-{{ task.Status === 'COMPLETED' ? 'success' : 'warning' }}">
      {{ task.Status }}
    </span>
  </td>
  <td>
    <button @click="showInvoiceForm = true"
            :disabled="task.Status === 'BILLED'"
            class="btn btn-sm btn-outline">
      生成发票
    </button>

    <!-- 模态框：确认生成发票 -->
    <div x-show="showInvoiceForm"
         @click.outside="showInvoiceForm = false"
         class="modal modal-open">
      <div class="modal-box">
        <h3 class="font-bold text-lg">快速生成发票</h3>
        <p class="py-2">
          将根据任务生成发票，自动创建双分录：
          <br>- Debit: A/R ({{ task.Customer }})
          <br>- Credit: Sales Revenue
        </p>
        <form hx-post="/api/tasks/{{ task.ID }}/invoice"
              hx-on::response-error="alert('生成失败')"
              @submit="showInvoiceForm = false">
          <button type="submit" class="btn btn-primary">确认生成</button>
          <button type="button"
                  @click="showInvoiceForm = false"
                  class="btn btn-ghost">取消</button>
        </form>
      </div>
    </div>
  </td>
</tr>
```

#### 3.5 React Island（可选复杂组件）

对于复杂的薪资计算表格，可选择用 React Island：

```tsx
// internal/web/components/payroll_calculator.tsx
import React, { useState } from 'react';

export function PayrollCalculator({ employees, yearToDate }) {
  const [rows, setRows] = useState(employees.map(e => ({
    employeeId: e.id,
    gross: 0,
    deductions: {},
    net: 0
  })));

  return (
    <table>
      <thead>
        <tr>
          <th>员工</th>
          <th>总津贴</th>
          <th>CPP</th>
          <th>EI</th>
          <th>Income Tax</th>
          <th>净额</th>
        </tr>
      </thead>
      <tbody>
        {rows.map(row => (
          <tr key={row.employeeId}>
            {/* ... */}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
```

---

### 阶段 4：认证与权限集成（1.5 周）

#### 4.1 权限模型扩展

Balanciz 已有权限系统，扩展为：

```go
// 新权限角色常量
const (
    RolePayrollAdmin    = "payroll_admin"    // 薪资管理员
    RolePayrollViewer   = "payroll_viewer"   // 薪资查看员
    RoleChequeSignor    = "cheque_signor"    // 支票签署人
    RoleTaskManager     = "task_manager"     // 任务管理员
)

// Balanciz 权限检查中间件应已支持细粒度权限
// 示例：在路由上应用权限检查
func RegisterPayrollRoutes(router fiber.Router, auth AuthMiddleware) {
    router.Post("/payroll/runs",
        auth.Require("payroll_admin"),  // 只允许 payroll_admin 创建
        createPayrollRun)

    router.Get("/payroll/runs",
        auth.Require("payroll_viewer"),  // payroll_viewer 或更高可查看
        listPayrollRuns)
}
```

#### 4.2 审计日志集成

所有 SimpleTask 模块的操作都需通过 Balanciz 的审计引擎记录：

```go
// 示例：薪资锁定时自动记录审计日志
func (ps *PayrollService) FinalizePayroll(ctx context.Context, runID int) error {
    // ... payroll 逻辑

    // 调用审计引擎
    ps.auditLog.RecordAction(ctx, &AuditEntry{
        CompanyID:    companyID,
        UserID:       userID,
        EntityType:   "PAYROLL_RUN",
        EntityID:     fmt.Sprintf("%d", runID),
        ActionType:   "FINALIZE",
        OldValue:     "DRAFT",
        NewValue:     "FINALIZED",
        Description:  fmt.Sprintf("Payroll run %d finalized", runID),
        Timestamp:    time.Now(),
    })
}
```

---

### 阶段 5：会计集成与 GL 映射（2 周）

#### 5.1 GL 账户设置

Balanciz 现有 GL 结构，额外需配置以下账户类别：

| 账户 | 代码示例 | 类型 |
|-----|--------|------|
| 薪资费用 | 5110 | Expense |
| 薪资应付款 | 2110 | Liability |
| CPP 应付款 | 2111 | Liability |
| EI 应付款 | 2112 | Liability |
| 所得税应付款 | 2113 | Liability |
| 支票账户 | 1010 | Asset |
| A/R（来自任务） | 1200 | Asset |
| 销售收入 | 4100 | Income |

#### 5.2 任务 -> 发票 -> GL 映射流程

```
Task (未结算)
  ↓
点击"生成发票"
  ↓
QuickCreateInvoiceFromTask()
  ├─ 创建 Invoice 记录
  ├─ 调用 PostingEngine 生成分录：
  │  └─ Debit 1200 A/R ($amount)
  │  └─ Credit 4100 Revenue ($amount)
  ├─ 创建 task_invoice_mapping
  ├─ 更新 task.status = BILLED
  └─ 返回 Invoice ID + GL posting result

Customer 支付
  ↓
记录收款
  ↓
PostingEngine 生成：
  └─ Debit 1010 Bank
  └─ Credit 1200 A/R
```

#### 5.3 薪资 -> GL 映射流程

```
创建 Payroll Run（草稿）
  ↓
员工薪资计算（service layer，无 GL）
  ↓
点击"锁定"
  ↓
FinalizePayroll()
  ├─ 计算总津贴 = $50,000
  ├─ 总扣款     = $12,000
  ├─ 净支付     = $38,000
  ├─ 调用 PostingEngine 生成分录：
  │  ├─ Debit 5110 薪资费用           $50,000
  │  ├─ Credit 2110 薪资应付（净）   $38,000
  │  ├─ Credit 2111 CPP 应付         $3,000
  │  ├─ Credit 2112 EI 应付          $1,500
  │  └─ Credit 2113 所得税应付       $7,500
  ├─ 存储 payroll_detail 记录
  └─ 更新 payroll_run.status = FINALIZED

签发支票
  ↓
MarkChequeIssued()
  └─ PostingEngine 生成分录：
     ├─ Debit 2110 薪资应付           $38,000
     └─ Credit 1010 银行账户          $38,000
```

---

### 阶段 6：混合测试与验证（1 周）

#### 6.1 单元测试

```go
// tests/payroll_service_test.go
func TestPayrollCalculation(t *testing.T) {
    // 创建测试员工与公司
    // 测试薪资计算逻辑
    // 验证 CPP、EI、税款计算正确
    // 验证最终 GL 分录生成
}

func TestTaskToInvoiceConversion(t *testing.T) {
    // 创建任务
    // 触发快速发票生成
    // 验证：
    // 1. Invoice 记录已创建
    // 2. task_invoice_mapping 正确
    // 3. GL 分录已生成（Debit A/R, Credit Revenue）
    // 4. task.status = BILLED
}

func TestChequeIssueFlow(t *testing.T) {
    // 创建支票草稿
    // 标记为已签发
    // 验证 GL 分录（Bank & Payable）
}
```

#### 6.2 集成测试

- 完整薪资周期测试（从创建到锁定到支票签发）
- 完整任务到发票到收款周期
- 多公司隔离验证（确保跨公司无数据泄露）
- 权限检查验证

#### 6.3 审计日志验证

```go
func TestAuditLogCompleteness(t *testing.T) {
    // 创建薪资运行、锁定、签发支票
    // 查询审计日志
    // 验证每个操作都被记录
    // 验证用户、时间戳、操作类型正确
}
```

---

### 阶段 7：迁移与生产部署（1 周）

#### 7.1 数据迁移清单

1. **导出 SimpleTask SQLite**
   ```bash
   # 在 SimpleTask 服务器上
   sqlite3 /opt/SimpleTask/data/SimpleTask.db ".mode csv" \
     "SELECT * FROM employee;" > employee.csv
   sqlite3 /opt/SimpleTask/data/SimpleTask.db ".mode csv" \
     "SELECT * FROM payroll_run;" > payroll_run.csv
   # 等等...
   ```

2. **导入到 Balanciz PostgreSQL**
   ```bash
   # 在 Balanciz 数据库主机上
   psql -U balanciz_user -d balanciz < migrations/020_add_simpletask_modules.sql
   # 导入 CSV（COPY 或应用脚本）
   ```

3. **数据验证脚本**
   ```sql
   -- 验证行数一致
   SELECT 'employee' as table_name, COUNT(*) FROM employee
   UNION ALL
   SELECT 'payroll_run', COUNT(*) FROM payroll_run
   UNION ALL
   SELECT 'cheque', COUNT(*) FROM cheque;

   -- 验证外键关系
   SELECT * FROM payroll_detail
   WHERE payroll_run_id NOT IN (SELECT id FROM payroll_run);
   ```

#### 7.2 部署清单

- [ ] Balanciz 新版本编译与测试
- [ ] PostgreSQL schema 更新
- [ ] 环境变量配置（权限、邮件等）
- [ ] Nginx 配置更新（新路由）
- [ ] 备份现有 Balanciz 数据
- [ ] 备份 SimpleTask SQLite 数据
- [ ] 灰度发布（金丝雀部署：10% 流量测试）
- [ ] 监控告警设置
- [ ] 回滚方案准备

#### 7.3 用户文档更新

- 薪资管理员手册（如何创建/锁定薪资运行）
- 支票签署流程文档
- 任务快速发票使用指南
- T4/T4A 导出与税务合规指南

---

## 四、技术栈统一方案

| 组件 | 当前 | 新增 | 统一方案 |
|-----|-----|-----|--------|
| **后端语言** | Go 1.23 | Go 1.22.2 | 升级到 Go 1.26+ |
| **Web 框架** | Fiber | net/http | 保持 Fiber（SimpleTask 部分迁移到 Fiber） |
| **ORM** | GORM + Ent | SQLite | Ent（统一代码生成） |
| **数据库** | PostgreSQL | SQLite | PostgreSQL（迁移 SimpleTask 数据） |
| **模板引擎** | Templ | 嵌入式 HTML | Templ（统一） |
| **前端交互** | HTMX + Alpine | 原生 JS | HTMX + Alpine（统一） |
| **复杂UI** | React Islands | 无 | React Islands（可选） |
| **认证** | 自研权限系统 | SQLite users | 统一到 Balanciz auth + company isolation |
| **部署** | Docker + systemd | install.sh + systemd | Docker Compose（统一） |

---

## 五、工作量估算与里程碑

```
┌─────────────────────────────────────────────────────────────────┐
│                     集成路线图（总计 8-12 周）                    │
├──────────┬──────────────────────┬──────────────┬───────────────┤
│  阶段    │    任务              │   工作量     │  里程碑        │
├──────────┼──────────────────────┼──────────────┼───────────────┤
│ 第 1 周  │ 数据库和表结构       │  2 人·周     │ Schema 完成   │
│ 第 2 周  │ 服务层核心逻辑       │  2.5 人·周   │ API 雏形     │
│ 第 3 周  │ 更多服务与 GL 映射   │  2.5 人·周   │ 后端 70% 完成 │
│ 第 4-5周 │ 前端 Templ 迁移      │  2 人·周     │ 前端完成      │
│ 第 6-7周 │ 认证权限集成         │  1.5 人·周   │审计 + 权限完成 |
│ 第 8 周  │ 单元/集成测试        │  1.5 人·周   │ 测试覆盖 85%  │
│ 第 9 周  │ 数据迁移验证         │  1 人·周     │ 迁移脚本完成  │
│ 第10-12周│ 灰度部署 + 优化      │  1 人·周     │ 生产就绪      │
└──────────┴──────────────────────┴──────────────┴───────────────┘

建议人力配置：
- 后端 Lead（全职）：架构设计、核心服务、GL 映射、测试
- 后端 Developer（全职）：服务实现、数据库、API 开发
- 前端 Developer（全职，第 3 周开始）：Templ 迁移、HTMX 集成、测试
- QA/DevOps（0.5 人·周）：测试计划、迁移脚本、部署验证
```

---

## 六、风险与缓解措施

| 风险 | 概率 | 影响 | 缓解措施 |
|-----|-----|-----|--------|
| **GL 映射逻辑错误** | 高 | 高 | 会计师审核分录规则，充分的单元测试 |
| **数据迁移丢失** | 中 | 高 | 多次备份，预演式迁移，数据验证脚本 |
| **多公司隔离泄露** | 中 | 极高 | 权限中间件严格检查，权限单元测试，审计日志 |
| **性能下降** | 中 | 中 | 索引优化，查询分析，分页实现 |
| **前端兼容性** | 低 | 中 | 浏览器测试矩阵，渐进增强策略 |
| **薪资计算错误** | 低 | 极高 | CRA 合规审查，与 SimpleTask 原逻辑对齐，专审 |

---

## 七、后续增强（第 13 周+）

1. **自动薪资周期管理**：根据员工合同自动创建 Payroll Run
2. **薪资预测**：基于历史数据预测下月薪资
3. **移动报告**：支票/薪资状态移动版
4. **与银行 API 集成**：自动导入支票清算状态
5. **员工自助门户**：员工可查看薪资单和 T4 副本
6. **税务合规报告**：自动生成和提交税务表单（T4、T4A、YTD）

---

## 八、相关文档清单

- [ ] 更新 `PROJECT_GUIDE.md`：新增 Payroll Module Authority 部分
- [ ] 更新 `AI_PRODUCT_ARCHITECTURE.md`：新增 Payroll 和 Task 组件
- [ ] 创建 `PAYROLL_IMPLEMENTATION_GUIDE.md`：薪资模块详细规范
- [ ] 创建 `DATABASE_MIGRATION_PLAN.md`：完整迁移步骤与脚本
- [ ] 创建 `PAYROLL_TESTING_PLAN.md`：薪资测试用例与 CRA 合规清单
- [ ] 更新部署文档：新增 SimpleTask 模块环境配置

---

## 九、决策检查清单

**在正式开始前确认**：

- [ ] 确认 SimpleTask 的薪资规则（CRA 合规性）已验证
- [ ] 获得财务/会计/合规人员对 GL 映射的批准
- [ ] 确认目标生产日期与资源可用性
- [ ] 完成 PostgreSQL 容量规划（预计 20-30GB 增长）
- [ ] 准备回滚方案与应急支持日程
- [ ] 获得 stakeholder 对 UI/UX 设计的批准

---

**建议下一步**：
1. 与财务/会计团队评审 GL 映射和薪资计算规则
2. 建立迁移项目管理结构和沟通计划
3. 详细规划第 1-2 阶段（数据库 + 服务层）
4. 准备开发环境与本地测试数据库
