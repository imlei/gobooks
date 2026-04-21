// 遵循project_guide.md
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// ExpenseStatus tracks the IN.2 lifecycle of an Expense.
//
//	draft   — created but not posted; no JE, no inventory effect
//	posted  — PostExpense wrote a JE (and, for stock lines under
//	          legacy mode, inventory movements)
//	voided  — post reversed; JE reversed + inventory restored
//
// Pre-IN.2 rows migrate in as 'draft'. Terminal state is voided.
type ExpenseStatus string

const (
	ExpenseStatusDraft  ExpenseStatus = "draft"
	ExpenseStatusPosted ExpenseStatus = "posted"
	ExpenseStatusVoided ExpenseStatus = "voided"
)

// AllExpenseStatuses returns expense statuses in logical order.
func AllExpenseStatuses() []ExpenseStatus {
	return []ExpenseStatus{
		ExpenseStatusDraft,
		ExpenseStatusPosted,
		ExpenseStatusVoided,
	}
}

// ExpenseLine is a single cost-category row within an Expense.
// An expense may have one or more lines; the sum of line amounts equals the
// parent Expense.Amount (maintained by the service layer).
type ExpenseLine struct {
	ID        uint `gorm:"primaryKey"`
	ExpenseID uint `gorm:"not null;index"`

	// LineOrder controls display ordering (0-based).
	LineOrder int `gorm:"not null;default:0"`

	Description string          `gorm:"type:text;not null;default:''"`
	Amount      decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// IN.2 (Rule #4): Qty + UnitPrice are authoritative when
	// ProductServiceID is set and the item is a stock item. They
	// drive the inventory.ReceiveStock call at post time. For
	// pure-expense (amount-only) lines these stay at the column
	// defaults (Qty=1, UnitPrice=0) and the service layer falls
	// back to Amount as the sole cost signal (legacy behavior).
	Qty       decimal.Decimal `gorm:"type:numeric(10,4);not null;default:1"`
	UnitPrice decimal.Decimal `gorm:"type:numeric(18,4);not null;default:0"`

	ExpenseAccountID *uint    `gorm:"index"`
	ExpenseAccount   *Account `gorm:"foreignKey:ExpenseAccountID"`

	// Optional link to the product/service catalog. Unlike
	// ExpenseAccountID which is a GL categorisation, this points at
	// a concrete catalog row so Task reinvoice and future catalog-
	// driven reports can see what the expense was actually for.
	// Nullable: many expenses remain pure cost-category with no
	// catalog item attached.
	//
	// IN.2: when ProductServiceID is set AND the linked item has
	// IsStockItem=true, this line becomes a Rule #4 stock line —
	// expense post forms an inventory movement (legacy mode) or
	// rejects the post (controlled mode).
	ProductServiceID *uint           `gorm:"index"`
	ProductService   *ProductService `gorm:"foreignKey:ProductServiceID"`

	// Optional per-line tax.
	TaxCodeID *uint     `gorm:"index"`
	TaxCode   *TaxCode  `gorm:"foreignKey:TaxCodeID"`
	LineTax   decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	LineTotal decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// Optional per-line task linkage.
	TaskID     *uint `gorm:"index"`
	Task       *Task `gorm:"foreignKey:TaskID"`
	IsBillable bool  `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Expense is a company-scoped standalone cost record, distinct from vendor Bills.
//
// It represents a direct expense entry (e.g. out-of-pocket spend, credit card
// charge, or any cost not tied to a formal vendor invoice) that needs to be
// tracked and, optionally, billed through to a customer.
//
// Task linkage rules (enforced by the service layer, not the DB schema):
//   - When task_id IS NULL:   a plain internal expense; task linkage fields are ignored.
//   - When task_id IS NOT NULL: the expense enters the Task body:
//   - billable_customer_id becomes required and must equal Task.customer_id.
//   - is_billable determines whether the expense is passed through to the customer.
//   - If is_billable = true:  reinvoice_status is set to "uninvoiced"; the expense
//     can be included in a billable Invoice Draft via the Draft Generator.
//   - If is_billable = false: reinvoice_status stays ""; the expense counts toward
//     the task's non-billable cost for margin analysis only.
//
// invoice_id / invoice_line_id are quick-lookup cache columns.
// The authoritative linkage record lives in task_invoice_sources.
//
// IN.2 lifecycle (migration 079): Status + JournalEntryID + WarehouseID
// + PostedAt/VoidedAt promote Expense from a save-only memo to a
// first-class posted business document. Status transitions:
//
//	draft    → posted  (via PostExpense; books JE + optional inventory)
//	posted   → voided  (via VoidExpense; reverses JE + inventory)
//
// Terminal: voided. Pre-IN.2 rows migrate in as status='draft'; they
// do not retroactively post.
type Expense struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// Status carries the IN.2 lifecycle. Default 'draft' on create
	// (column default matches migration 079). Only PostExpense flips
	// to posted; only VoidExpense flips to voided. Handlers do not
	// set this directly.
	Status ExpenseStatus `gorm:"type:text;not null;default:'draft';index"`

	// WarehouseID is the header-level warehouse used for routing
	// stock-line inventory movements at post time (Q3 decision:
	// header, defaulted but visible/editable). Nullable — for
	// pure-expense (no stock line) expenses it is ignored. For
	// expenses with stock lines, post-time validation requires it
	// (or falls back to the company default warehouse).
	WarehouseID *uint      `gorm:"index"`
	Warehouse   *Warehouse `gorm:"foreignKey:WarehouseID"`

	// JournalEntryID links the posted JE (nil = not yet posted, or
	// a pre-IN.2 record that never posted). Set once at post time;
	// cleared implicitly when VoidExpense reverses (the original JE
	// flips to status=reversed rather than being disassociated).
	JournalEntryID *uint         `gorm:"index"`
	JournalEntry   *JournalEntry `gorm:"foreignKey:JournalEntryID"`

	PostedAt *time.Time
	VoidedAt *time.Time

	// Task linkage (optional).
	TaskID   *uint `gorm:"index"`
	Task     *Task `gorm:"foreignKey:TaskID"`

	// BillableCustomerID identifies who this expense is billed to.
	// When TaskID is set, must equal Task.CustomerID (service-layer rule).
	BillableCustomerID *uint     `gorm:"index"`
	BillableCustomer   *Customer `gorm:"foreignKey:BillableCustomerID"`

	// IsBillable marks whether this expense should be passed through to the customer.
	// Only meaningful when TaskID is set.
	IsBillable bool `gorm:"not null;default:false"`

	// ReinvoiceStatus tracks the invoice lifecycle of this billable expense.
	// '' (none) | uninvoiced | invoiced | excluded
	// Managed by the service layer; not set directly by handlers.
	ReinvoiceStatus ReinvoiceStatus `gorm:"type:text;not null;default:''"`

	// Quick-lookup cache for current invoice linkage.
	// Authoritative source: task_invoice_sources.
	// Cleared to NULL by the service layer when the linked invoice is voided.
	InvoiceID     *uint        `gorm:"index"`
	Invoice       *Invoice     `gorm:"foreignKey:InvoiceID"`
	InvoiceLineID *uint        `gorm:"index"`
	InvoiceLine   *InvoiceLine `gorm:"foreignKey:InvoiceLineID"`

	// Core expense details.
	// ExpenseNumber is the user-visible reference string, auto-assigned
	// on create from the "expense" module in Settings → Company →
	// Numbering. Matches the pattern on PO / SO / Quote / Bill /
	// Invoice: one column per document, free-form text. The
	// (company_id, expense_number) compound index is created by
	// migration 074 at the SQL layer; no GORM index tag here to
	// avoid a duplicate single-column index being created by
	// AutoMigrate in test harnesses.
	ExpenseNumber string          `gorm:"type:text;not null;default:''"`
	ExpenseDate   time.Time       `gorm:"not null"`
	Description   string          `gorm:"type:text;not null;default:''"`
	Amount        decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`
	CurrencyCode  string          `gorm:"type:text;not null;default:''"`

	// Optional vendor and GL account references.
	VendorID *uint    `gorm:"index"`
	Vendor   *Vendor  `gorm:"foreignKey:VendorID"`

	ExpenseAccountID *uint    `gorm:"index"`
	ExpenseAccount   *Account `gorm:"foreignKey:ExpenseAccountID"`

	// Payment settlement fields (all optional).
	// PaymentAccountID points to the bank/credit-card/petty-cash account used to pay.
	// PaymentMethod records the payment instrument (check, wire, cash, credit_card, etc.).
	// PaymentReference is a user-supplied memo or cheque number.
	PaymentAccountID *uint         `gorm:"index"`
	PaymentAccount   *Account      `gorm:"foreignKey:PaymentAccountID"`
	PaymentMethod    PaymentMethod `gorm:"type:text;not null;default:''"`
	PaymentReference string        `gorm:"type:text;not null;default:''"`

	// MarkupPercent is reserved for future pass-through pricing support.
	// v1: always 0; UI does not expose this field.
	MarkupPercent decimal.Decimal `gorm:"type:numeric(8,4);not null;default:0"`

	Notes string `gorm:"type:text;not null;default:''"`

	// Lines holds the individual cost-category rows. Loaded on demand via Preload.
	Lines []ExpenseLine `gorm:"foreignKey:ExpenseID;constraint:OnDelete:CASCADE"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
