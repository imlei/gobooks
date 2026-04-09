// 遵循project_guide.md
package models

// payment_reverse_allocation.go — Batch 22: Multi-allocated payment reverse allocation truth.
//
// ─── Layer position ───────────────────────────────────────────────────────────
//
//   PaymentTransaction (charge/capture)          ← original payment event
//   PaymentAllocation ×N                         ← Batch 17 forward allocation truth
//   PaymentReverseAllocation ×N                  ← Batch 22 reverse allocation truth (THIS)
//
// ─── Purpose ─────────────────────────────────────────────────────────────────
//
//   When a charge/capture that was multi-allocated across N invoices is later
//   reversed (refund, chargeback, or dispute_lost), this table records exactly
//   how much was restored to each invoice.
//
//   One row is created per (reverse_txn_id, invoice_id) pair.  The unique index
//   uq_payment_rev_alloc prevents duplicate reverse allocations for the same
//   (reverse_txn_id, invoice_id) — both as an idempotency guard and as the
//   concurrent-submit race condition guard.
//
// ─── Invariants ───────────────────────────────────────────────────────────────
//
//   - Rows are immutable after creation.
//   - Σ Amount per reverse_txn_id ≤ Σ original PaymentAllocation.AllocatedAmount
//     (overpayment excess that became a CustomerCredit is never pushed back to
//      invoice via this path).
//   - Amount per row ≤ original PaymentAllocation.AllocatedAmount for that invoice.
//   - company_id on this row must equal company_id on both the reverse txn and
//     the original txn (enforced by service-layer company-scoped queries).
//
// ─── Not in Batch 22 ─────────────────────────────────────────────────────────
//   - Multi-allocated customer credit reverse allocation
//   - Multi-currency reverse allocation
//   - Manual override / forced invoice assignment
//   - Auto credit memo / writeoff on partial reversal

import (
	"time"

	"github.com/shopspring/decimal"
)

// ReverseAllocationType identifies the business event that triggered this
// reverse allocation.
type ReverseAllocationType string

const (
	// ReverseAllocRefund is a voluntary merchant-initiated refund.
	ReverseAllocRefund ReverseAllocationType = "refund"

	// ReverseAllocChargeback is a forcible card-network chargeback.
	ReverseAllocChargeback ReverseAllocationType = "chargeback"

	// ReverseAllocDisputeLost is a dispute that was decided against the merchant,
	// resulting in a chargeback-style restore.  Distinct from ReverseAllocChargeback
	// for audit-trail clarity even though the mechanics are identical.
	ReverseAllocDisputeLost ReverseAllocationType = "dispute_lost"
)

// AllReverseAllocationTypes returns every known type for validation.
func AllReverseAllocationTypes() []ReverseAllocationType {
	return []ReverseAllocationType{
		ReverseAllocRefund,
		ReverseAllocChargeback,
		ReverseAllocDisputeLost,
	}
}

// PaymentReverseAllocation is an immutable audit record of one invoice's share
// of a multi-allocated-payment reversal.
//
// One row is created per (reverse_txn_id, invoice_id) pair per reversal event.
// The combination uniquely identifies which reversal restored how much to which
// invoice.
//
// Both the forward allocation truth (PaymentAllocation) and the reversal source
// truth (PaymentTransaction of type refund/chargeback) are cross-referenced via
// OriginalTxnID and PaymentAllocationID so that auditors can trace the complete
// chain:
//
//	reverse txn → original charge → PaymentAllocation → invoice
type PaymentReverseAllocation struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// ReverseTxnID is the refund or chargeback PaymentTransaction that triggered
	// this reversal.
	ReverseTxnID uint `gorm:"not null;index;uniqueIndex:uq_payment_rev_alloc"`

	// OriginalTxnID is the original charge/capture PaymentTransaction whose
	// multi-invoice allocations are being reversed.
	OriginalTxnID uint `gorm:"not null;index"`

	// PaymentAllocationID links back to the specific PaymentAllocation row
	// (original forward allocation) that is being reversed by this record.
	PaymentAllocationID uint `gorm:"not null;index"`

	// InvoiceID is the invoice whose BalanceDue is restored by Amount.
	InvoiceID uint `gorm:"not null;index;uniqueIndex:uq_payment_rev_alloc"`

	// Amount is the portion of the reversal credited back to this invoice (> 0).
	Amount decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0"`

	// ReverseType distinguishes refund / chargeback / dispute_lost.
	ReverseType ReverseAllocationType `gorm:"type:text;not null"`

	CreatedAt time.Time
}
