## GoBooks - PROJECT_GUIDE

gobooks 硬性需求来源 v2（更新整合版）

⚠️ 本文件是本项目的最高优先级产品与架构约束来源
所有代码生成、修改、重构必须严格遵守本规范。

一、产品定位
🎯 核心目标

打造一个：

简洁、高效、结构清晰、可扩展，并融合 AI 的多用户、多公司会计系统

💡 核心原则
正确性 > 灵活性
结构一致性 > 用户随意性
后端规则 > 前端假设
可扩展架构 > 快速堆功能
AI = 辅助层（永远不能破坏会计逻辑）
二、系统结构
MVP 模块
Company Setup
Chart of Accounts
Journal Entry（核心）
Reports
Customers / Vendors
Banking
Settings
Settings 子模块（必须）
Company
Numbering
AI Connect
Audit Log
三、编号系统（双层）
1️⃣ 内部编号（entity_number）

格式：

ENYYYY########

规则：

全局唯一
后端生成
不可修改
不可暴露为可编辑字段
2️⃣ 业务编号（Display Number）

例如：

JE-0001
IN001

规则：

可配置
可重复检测
不作为系统身份
四、Account Code（结构性约束）
长度规则
Setup 时选择：4–12 位
一旦确定不可修改
格式规则（强约束）
必须为整数
不允许小数 / 字母
不允许前导 0
必须为正数
必须唯一（company 内）
自动扩展

4位模板 → 高位数通过右补 0 实现：

1000 → 10000 → 100000
五、Chart of Accounts（核心结构）
1️⃣ Account Type（重构）
Root Account Type（核心逻辑）

固定：

asset
liability
equity
revenue
cost_of_sales
expense
Detail Account Type（业务细分）

示例：

asset
bank
accounts_receivable
inventory
prepaid_expense
liability
accounts_payable
credit_card
sales_tax_payable
expense
office_expense
rent_expense
professional_fees

👉 detail 只用于：

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
1️⃣ 字段
gifi_code (optional)
2️⃣ 作用
CRA 报税映射
不参与系统 identity
3️⃣ 推荐方式
rule-based（必须）
AI-enhanced（可选）
七、Recommendation 系统（核心新增）
🎯 目标

减少：

account code 混乱
命名不一致
GIFI 填写成本
🧠 架构（两层）
Layer 1（必须）

Rule-based recommendation

Layer 2（可选）

AI-enhanced recommendation

1️⃣ Rule-based Recommendation（必须存在）

后端服务：

POST /api/accounts/recommendations

返回：

suggested_account_name
suggested_account_code
suggested_gifi_code
confidence
reason
source = "rule"
2️⃣ 推荐范围
A. Account Name
标准化名称
detail 驱动默认值
B. Account Code

必须：

符合长度
符合 prefix
不重复
优先使用现有分组
推荐“下一合理编号”

❗禁止随机生成

C. GIFI

使用本地 mapping 表

3️⃣ Confidence
high
medium
low
八、AI Recommendation（增强层）
原则

AI 只能建议，不能决定

Endpoint
POST /api/ai/recommend/account
触发方式
用户点击 Suggest with AI
不自动频繁调用
行为
提升 name / code / gifi 推荐质量
不可覆盖用户输入
必须 fallback 到 rule
安全
API key 仅后端
不暴露前端
九、Suggestion Strip（UI）
位置

Chart of Accounts → Drawer 内

功能

显示：

Suggested Name
Suggested Code
Suggested GIFI
行为
每项独立 Apply
不自动覆盖
loading / error 状态
fallback 安全
风格
不改变 UI
不做 AI 面板
轻量辅助区域
十、保存与验证（关键）
原则

Recommendation ≠ Validation

后端必须验证：
account_type
detail_type
account_code
prefix
唯一性
Normalization（可选）

允许：

trim
空格规范

不允许：

自动改 code
自动改 GIFI
十一、Recommendation Source Tracking（新增）
🎯 目的

用于：

产品分析
使用行为理解
Source 类型
manual
rule
ai
原则

Recommendation source ≠ 审计数据

关键规则
仅记录用户声明（apply vs manual）
后端不信任该数据
可被伪造（允许）
不用于：
权限
会计逻辑
合规判断
扩展（未来）

如需增强：

server-side correlation
signed recommendation

（当前不做）

十二、Journal Entry（核心）
规则
Debit XOR Credit
Debits == Credits
UI
95% 宽度
自动 total
保存
不平衡禁止提交
十三、Audit Log

必须记录：

entity_number
action
timestamp
十四、AI Connect
配置
API Key
Base URL
Model
当前范围
仅用于 recommendation
不参与核心账务
十五、数据与架构原则
必须
entity_number 不可变
backend 为最终规则执行者
recommendation 不参与核心逻辑
禁止
AI 修改账务数据
suggestion 绕过 validation
删除历史数据
✅ 最终总结

gobooks =
结构化会计系统（强规则） + 智能建议层（AI + Rule）
