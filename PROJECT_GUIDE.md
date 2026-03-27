## GoBooks - PROJECT_GUIDE

> 本文件是本项目的**硬性需求来源**。后续每次生成/修改代码都必须对照本指南。

---

## 一、产品定位

目标：做一个“替代 QuickBooks 的简洁版会计系统”

### 🎯 核心定位

- **面向**：小企业 / Bookkeeper
- **特点**
  - 简单、干净、好用（不是复杂 ERP）
  - 核心功能够用即可（不堆功能）

---

## 二、整体系统结构

### 1️⃣ 基础模块（MVP 必须有）

- Company Setup（公司初始化）
- Chart of Accounts（科目表）
- Journal Entry（核心）
- Reports（报表）
- Customers / Vendors（名称选择）
- Banking（银行 + Reconciliation）
- Setting（初始化之后，可以修改公司的信息和其他设置）

---

## 三、Setup / 公司初始化

### 必填信息

- 公司名称
- 公司类型
  - Personal
  - Incorporated
  - LLP
- 地址（完整）
- Business Number
- 行业（industry）
- Fiscal Year End（年结）

### 逻辑

- 根据公司类型 👉 自动生成 Chart of Accounts
- Setup Wizard
  - 初次必须走
  - 后续可以在 Settings 修改（不是入口主功能）

---

## 四、Chart of Accounts（科目表）

### 功能要求

- 自动生成（基于公司类型）
- 必须支持
  - ➕ Add New Account

### Account Type 限制（必须严格）

Account Type 只能是：

- asset
- expense
- liability
- equity
- revenue
- cost_of_sales

### 验证逻辑（关键）

- `accountType` **必须严格匹配 enum**（不能用 string）

---

## 五、Journal Entry（🔥最核心模块）

### 1️⃣ UI 结构

- 表格宽度：95%
- 每一行字段
  - Account
  - Debit
  - Credit
  - Memo
  - Name（客户/供应商）
  - Action（+ / -）

### 2️⃣ 行级规则（非常重要）

每一行：

- Debit 和 Credit
  - ❌ 不能同时有值
  - ✅ 只能填一个

### 3️⃣ 自动计算

底部自动显示：

- Total Debits
- Total Credits

并且自动实时累加。

### 4️⃣ 保存逻辑（核心规则）

必须同时满足：

- 至少 2 行有效数据
- Debits == Credits

否则：

- ❌ Save 按钮禁用（灰色）
- ❌ 不允许提交
- ❌ 不允许跳转页面

### 5️⃣ 错误处理

- 不平衡：显示提示（Primary validation message）
- 未完成：行级错误提示（row status）

### 6️⃣ UX 细节

- 页面刷新不能丢数据
- 不允许误操作提交
- Save / Save & Close：只有平衡才可点击

### 7️⃣ Action 列

两个按钮：

- ➕（新增一行）
- ➖（删除一行）

### 8️⃣ Name 字段

必须支持选择：

- Customers
- Vendors

---

## 六、Invoices & Bills

### Invoice（销售）

- 自动填充基础字段
- Invoice Number
  - 自动递增（IN001, IN002）
  - 支持：字母、数字、`- # @`

#### 重复检测（重点）

- 不区分大小写：`IN001 == in001`

#### 冲突处理

弹窗：

- Cancel（不提交）
- Confirm（继续）

### Bills（采购）

- 检测逻辑：Vendor + Bill Number
- 注意 Sales Tax：
  - refundable
  - nonrefundable
  - refundable 的 tax 计入独立的 Tax payable account

---

## 七、Banking 模块

优先级：

1. Bank Reconciliation（最重要）
2. Receive Payment
3. Pay Bills

---

## 八、Reports（报表）

必须有：

- Income Statement
- Balance Sheet
- Trial Balance

要求：

- 支持日期筛选

---

## 九、UI / Sidebar（非常重要）

### Sidebar 要求

- 分组清晰
- Active 状态明显
- 支持折叠
- Create 区域要产品级重构

### UI 一致性（必须）

需要统一：

- spacing
- 字体
- 按钮风格
- 颜色体系

---

## 十、系统级能力（高级）

### 1️⃣ Audit Log

- 谁做了什么
- 什么时间

### 2️⃣ Reverse Entry

- 支持冲销分录

