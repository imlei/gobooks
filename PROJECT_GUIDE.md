Gobooks 硬性需求来源 v2（更新整合版 · 强化版）

⚠️ 本文件是本项目的最高优先级产品与架构约束来源
所有代码生成、修改、重构必须严格遵守本规范

一、产品定位
🎯 核心目标：打造一个：简洁、高效、结构清晰、可扩展，并融合 AI 的多用户、多公司会计系统

💡 核心原则
正确性 > 灵活性
结构一致性 > 用户随意性
后端规则 > 前端假设
可扩展架构 > 快速堆功能
AI = 辅助层（永远不能破坏会计逻辑）

二、系统结构
🧩 双层架构
1️⃣ Business App（用户侧）每个 company 独立 Accounting / Invoicing / Billing / Reporting
2️⃣ SysAdmin（系统侧）：管理所有公司，管理所有用户，控制系统状态，查看日志与系统运行状态
📦 MVP 模块
Company Setup
Chart of Accounts
Journal Entry（核心）
Reports
Customers / Vendors
Banking
Settings
⚙️ Settings 子模块（必须）
Company
Numbering
Sales Tax
AI Connect
Audit Log

三、编号系统（双层）
1️⃣ 内部编号（entity_number）

格式：ENYYYY########

规则：
*全局唯一
*后端生成
*不可修改
*不可暴露为可编辑字段

2️⃣ 业务编号（Display Number）

例如：

JE-0001
IN001

规则：

可配置
可重复检测
不作为系统 identity

四、Account Code（结构性约束）
📏 长度规则
Setup 时选择：4–12 位
一旦确定不可修改
🔒 格式规则（强约束）
必须为整数
不允许小数 / 字母
不允许前导 0
必须为正数
必须唯一（company 内）
🔄 自动扩展
1000 → 10000 → 100000

五、Chart of Accounts（核心结构）
1️⃣ Account Type（重构）
Root Account Type（核心逻辑）
asset
liability
equity
revenue
cost_of_sales
expense
Detail Account Type（业务细分）

示例：

asset
- bank
- accounts_receivable
- inventory
- prepaid_expense

liability
- accounts_payable
- credit_card
- sales_tax_payable

expense
- office_expense
- rent_expense
- professional_fees

👉 detail 仅用于：

UI
默认值
recommendation
2️⃣ Code 与 Type 关系（强校验）
1xxxx → asset
2xxxx → liability
3xxxx → equity
4xxxx → revenue
5xxxx → cost_of_sales
6xxxx → expense

❗违反必须拒绝保存

3️⃣ 删除策略
❌ 不允许删除
✅ 使用 inactive
六、GIFI 体系（CRA 映射）
字段gifi_code (optional)
作用CRA 报税映射 不参与系统 identity. 但是可以导出作为报税依据（未完成）。

推荐方式
Rule-based（必须）
AI-enhanced（可选）

七、Sales Tax 系统（核心模块 🔥）
🎯 核心原则

Tax 按 line 计算，按 account 汇总入账

1️⃣ Tax Code 结构

字段：

name
rate
scope（sales / purchase / both）
recovery_mode（full / partial / none）
recovery_rate
accounts：
tax payable
recoverable tax

2️⃣ Sales 规则（固定）

无论 recoverable / non-recoverable：

Revenue → revenue account
Tax → tax payable

👉 Sales 永远统一逻辑

3️⃣ Bill 规则（关键差异）
✅ Recoverable Tax
Net → expense / asset
Tax → recoverable tax account
✅ Non-recoverable Tax
Tax 并入原 account
不进入 recoverable account
示例
Office Expense 1
Office Expense 10
Equipment 1000
Tax 7%

最终：

Dr Office Expense 11.77
Dr Equipment 1070
Cr Cash 1081.77
八、Journal Entry 核心规则（核心引擎 🔥）
1️⃣ 分层原则

业务层 ≠ 会计分录层

2️⃣ 分录生成流程（固定）
Line → Tax Calculation → Posting Fragments → Aggregation → Journal Entry
3️⃣ 聚合规则（强制）
按 account_id + debit/credit 聚合
同一 account 必须合并
不允许碎片化分录
示例（Sales）
Dr AR 1155
Cr Sales Revenue 1100
Cr GST Payable 55
九、Transaction 生命周期（必须一致 🔥）
核心原则

业务状态变化必须与 Journal Entry 同步

POST
→ 生成 JE

VOID
→ 生成冲销 / reversal
→ 保留 audit

REVERSE
→ 创建反向 JE

DELETE
draft → 可删除
posted → ❌ 禁止删除
❗禁止
业务变动但 JE 不变
JE 孤立存在
十、SysAdmin 系统（必须产品化）
功能范围
公司
deactivate / reactivate
delete（安全）
用户
edit
reset password
change role
deactivate
系统
maintenance mode
restart（placeholder）
日志
audit logs
runtime logs
UI 硬性要求
Layout（强制）
左侧 Sidebar
顶部 Top Bar
右侧 Content
Sidebar
Dashboard
Companies
Users
Audit Logs
Runtime Logs
System Control
SysAdmin Accounts
Dashboard 必须包含
KPI（companies / users / CPU / memory / DB / storage）
Quick actions
Recent activity
十一、系统指标（Observability）

必须支持：

CPU usage
Memory usage
DB size
Storage usage
Storage 设计
attachments 表
file_size_bytes
可按 company 聚合
十二、Recommendation 系统（核心）
🎯 目标

减少：
account code 混乱
命名不一致
GIFI 成本
架构
Layer 1（必须）

Rule-based

Layer 2（可选）

AI-enhanced

API
POST /api/accounts/recommendations

返回：

suggested_account_name
suggested_account_code
suggested_gifi_code
confidence
reason
source
Confidence
high
medium
low
十三、AI Recommendation（增强层）

原则：

AI 只能建议，不能决定

Endpoint
POST /api/ai/recommend/account
限制
不自动调用
不覆盖用户输入
必须 fallback
十四、Suggestion Strip（UI）

位置：

Chart of Accounts Drawer

行为
每项独立 Apply
不自动覆盖
有 loading / error
十五、保存与验证（关键）
原则

Recommendation ≠ Validation

后端必须验证：
account_type
detail_type
account_code
prefix
唯一性
❌ 不允许
自动改 code
自动改 GIFI
十六、Recommendation Source Tracking
Source 类型
manual
rule
ai
原则
不可信
不参与权限
不参与会计逻辑
十七、数据与架构原则（最终约束）
必须
entity_number 不可变
backend 为唯一规则执行者
recommendation 不参与核心逻辑
journal entry 可追溯
❌ 禁止
AI 修改账务数据
suggestion 绕过 validation
删除历史数据
JE 与业务脱节

## Journal Entry 状态独立与联动规则（强制）

### 核心原则
`journal_entries.status` 必须独立存在，但不能脱离业务单据状态独立运行。

业务单据（invoice / bill / payment 等）是业务意图的来源，journal entry 是其会计结果。

### 规则
1. journal_entries 必须有独立 status 字段
2. source document 也必须有独立 status 字段
3. source document 状态变化必须驱动 journal entry 状态和会计结果同步变化
4. 不允许出现业务状态已变化但 journal entry 未同步的情况
5. 对于自动生成的 journal entry，必须保存 source_type 和 source_id
6. posted 的业务单据不可物理删除
7. void / reverse 必须通过联动机制处理，不允许只改业务层或只改 journal 层

### 建议状态
- draft
- posted
- voided
- reversed

### 生命周期要求
- draft document 可编辑、可删除，不应产生正式 ledger entries
- posted document 必须产生 posted journal entry 和 ledger entries
- voided / reversed document 必须有对应的会计反映

Notifications infrastructure must store not only configuration values, but also delivery readiness state.

For SMTP and SMS, the system must track:
- whether the channel is enabled
- whether required configuration fields are present
- whether a test has ever been executed
- the last test status (never / success / failed)
- the last tested time
- the user who executed the last test
- the last success time
- the last failure time
- the last error message
- whether the channel is verification-ready

A channel is verification-ready only when:
- it is enabled
- configuration is complete
- the latest test succeeded
- the configuration has not changed since that successful test

Changing SMTP or SMS configuration must invalidate previous test success until a new successful test is completed.

Email/password security flows must rely on backend verification-ready checks, not frontend assumptions.

✅ 最终总结

Gobooks = 结构化会计系统（强规则） + 智能建议层（AI + Rule） + 系统级管理能力（SysAdmin + Observability）