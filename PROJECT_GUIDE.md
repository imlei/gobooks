🧱 Gobooks Project Guide v3（基石版本 · 可执行版）

⚠️ 本文件为最高优先级约束
所有代码生成 / 修改 / AI 执行必须严格遵守
如有冲突，以本文件为准

一、产品定位（不变，但结构化）
🎯 核心目标

打造一个：

结构严谨、可审计、可扩展、AI增强的多公司会计系统

🔒 核心原则（强化）
Correctness > Flexibility
Backend Authority > Frontend Assumptions
Structure > Convenience
Auditability > Performance shortcuts
AI = Suggestion Layer ONLY
二、系统架构（强化为工程可执行）
🧩 双层架构
1️⃣ Business App
多公司隔离（company_id 强制）
Accounting / Sales / Expense / Reporting
2️⃣ SysAdmin
系统级管理（独立登录）
不参与业务数据写入
🔁 核心引擎：Posting Engine（必须统一）
Document → Validation → Tax → Posting Fragments → Aggregation → Journal Entry → Ledger Entries
❗硬性规则
所有会计数据必须带 company_id
Journal Entry 必须：
有 status
有 source_type + source_id
不允许跳过 Posting Engine
三、数据与标识系统
Entity Number（内部编号）
ENYYYY########

规则：

全局唯一
后端生成
不可修改
不可作为业务输入
Display Number（业务编号）
可配置
可重复检测
不参与 identity
四、Chart of Accounts（结构核心）
1️⃣ Account Type

Root：

asset
liability
equity
revenue
cost_of_sales
expense
2️⃣ Code 强约束
1xxxx → asset
2xxxx → liability
3xxxx → equity
4xxxx → revenue
5xxxx → cost_of_sales
6xxxx → expense

❗违反必须 reject

3️⃣ 删除规则
❌ delete
✅ inactive
4️⃣ COA Template（新增明确）
系统必须有 default template
创建 company 自动生成
标记：
is_system_default = true
五、Sales Tax（不变但强调）
核心原则
Tax 按 line 计算 → 按 account 聚合
Sales（固定）
Revenue → revenue
Tax → tax payable
Purchase（分 recoverability）
recoverable → recoverable tax account
non-recoverable → 并入 expense
六、Journal Entry（核心引擎）
强制规则
按 account 聚合
不允许碎片化
生命周期（强化）
draft → posted → voided / reversed
❗禁止
业务变动但 JE 不变
JE 独立存在无来源
七、Sidebar（新增 · 必须统一）
🎯 业务驱动结构（强制）
Core
Dashboard
Journal Entry
Invoices
Bills
Sales & Get Paid
Customers
Receive Payment
Expense & Bills
Vendors
Pay Bills
Accounting
Chart of Accounts
Reconciliation
Reports
Settings
不变
❗规则
❌ 不允许 Contacts 分组
❌ 不允许 Banking 残留
Reports 只能在 Accounting
八、Notifications（强化为系统依赖）
必须存储状态
config
test_status
last_tested_at
error
verification_ready
核心规则
SMTP 未 ready → 禁止发送验证码
config 变更
修改配置 → test 失效
九、User Security（不变但强化）
Profile
email change → 需验证
password change → 需验证
验证码
6位
case-insensitive
单次使用
有效期
十、Reconciliation（🔥核心模块 · 新增正式纳入）
🎯 定位
Reconciliation = Accounting Control Layer
核心模型
reconciliation_session
statement_lines
book_candidates
matches
exceptions
状态机
draft → in_progress → completed → reopened → cancelled
匹配支持（必须）
one-to-one
one-to-many
many-to-one
split
UI（强制）
QuickBooks-style
summary bar
payment / deposit 分列
checkbox 控制 match
完成规则
difference = 0 才允许 finish
十一、Void Reconciliation（🔥强规则）
核心限制
只允许 void 最后一个 completed reconciliation
行为
不删除
rollback matches
恢复状态
audit 必须记录
字段
is_voided
voided_by
voided_at
void_reason
十二、AI Auto Match（🔥新增核心层）
🎯 定位
AI = Suggestion Layer（不能改账）
三层结构
Rules → Scoring → AI Enhancement
Suggestion（新增实体）
reconciliation_match_suggestions
reconciliation_match_suggestion_lines
匹配类型
one_to_one
one_to_many
many_to_one
split
用户行为
Accept → 生成 match
Reject → 不改账
必须支持
Explainability（必须解释）
十三、Reconciliation Memory（新增）
reconciliation_memory

用途：

学习历史匹配
提升 future suggestion
❗限制
可解释
不允许黑盒
十四、AI 系统通用规则（强化）
❌ 禁止
AI 修改账务数据
AI 自动完成 reconciliation
AI 绕过 validation
✅ 允许
suggestion
ranking
explanation
十五、审计（Audit）
必须记录
match / unmatch
suggestion accept / reject
reconciliation finish
reconciliation void
auto match run
十六、数据原则（最终约束）
必须：
- company_id 隔离
- entity_number 不可变
- backend 为唯一规则执行者
- JE 可追溯

禁止：
- 删除历史数据
- AI 改账
- suggestion 绕过 validation
- JE 与业务脱节
🔚 最终总结（升级版）
Gobooks =
强规则会计系统（Posting Engine + COA + JE）
+ 控制层（Reconciliation + Audit）
+ 业务层（Sales / Expense）
+ AI 建议层（Auto Match + Recommendation）
+ 系统层（SysAdmin + Observability）