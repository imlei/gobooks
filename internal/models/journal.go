// 遵循project_guide.md
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// ── Journal entry status ──────────────────────────────────────────────────────

// JournalEntryStatus describes the lifecycle state of a journal entry.
//
// Storage rule: status is stored independently from the source document's own
// status field. The posting engine synchronises both inside a single transaction.
// Neither field drives the other directly — only the engine coordinates them.
type JournalEntryStatus string

const (
	// JournalEntryStatusDraft means the entry has been created but not committed
	// to books. Lines may still change. No active ledger entries exist.
	// The current engine does not produce draft JEs; reserved for future
	// approval workflows.
	JournalEntryStatusDraft JournalEntryStatus = "draft"

	// JournalEntryStatusPosted means the entry is committed to books.
	// Lines are immutable. Ledger entries are active (status=active).
	JournalEntryStatusPosted JournalEntryStatus = "posted"

	// JournalEntryStatusVoided means the entry was cancelled before posting.
	// No ledger entries exist for a voided JE.
	JournalEntryStatusVoided JournalEntryStatus = "voided"

	// JournalEntryStatusReversed means a reversal JE has been created and posted.
	// This entry's ledger entries have been marked reversed.
	// The entry itself is retained permanently for audit traceability.
	JournalEntryStatusReversed JournalEntryStatus = "reversed"
)

// ── Journal entry ─────────────────────────────────────────────────────────────

// JournalEntry is the header for a double-entry transaction.
//
// Lifecycle synchronisation (enforced by the posting engine, not DB triggers):
//
//	PostInvoice / PostBill       → Status = posted
//	VoidInvoice / VoidBill       → original JE Status = reversed
//	                               reversal JE Status = posted
//	ReverseJournalEntry          → same as above
//
// ReversedFromID links a reversal JE back to the original it cancels.
// The original JE's Status is set to 'reversed' when a reversal is posted.
//
// Concurrency / uniqueness:
//
//	SourceType + SourceID identify the originating business document.
//	A unique partial index (status='posted', source_type != '', source_id > 0)
//	enforces that at most one active JE exists per (company, source, document).
//	Manual entries (SourceType = '') are excluded from the index.
type JournalEntry struct {
	ID        uint               `gorm:"primaryKey"`
	CompanyID uint               `gorm:"not null;index"`
	EntryDate time.Time          `gorm:"not null"`
	JournalNo string             `gorm:"column:journal_no;not null;default:''"`
	Status    JournalEntryStatus `gorm:"type:text;not null;default:'posted'"`

	// SourceType identifies the originating business document.
	// Empty string for manual journal entries; excluded from the uniqueness index.
	SourceType LedgerSourceType `gorm:"type:text;not null;default:''"`
	// SourceID is the PK of the originating document. 0 for manual entries.
	SourceID uint `gorm:"not null;default:0"`

	// ReversedFromID is set on the reversal JE; nil on normal entries.
	ReversedFromID *uint `gorm:"index"`

	CreatedAt time.Time

	Lines []JournalLine `gorm:"foreignKey:JournalEntryID"`
}

// ── Journal line ──────────────────────────────────────────────────────────────

// JournalLine is a single debit OR credit line in a journal entry.
//
// PROJECT_GUIDE rules (enforced in handlers/services):
//   - Debit and Credit cannot both have values.
//   - A saved Journal Entry must have at least 2 valid lines.
//   - Total Debits must equal Total Credits.
type JournalLine struct {
	ID             uint `gorm:"primaryKey"`
	CompanyID      uint `gorm:"not null;index"`
	JournalEntryID uint `gorm:"not null;index"`

	AccountID uint    `gorm:"not null;index"`
	Account   Account `gorm:"foreignKey:AccountID"`

	Debit  decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	Credit decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	Memo string `gorm:"not null;default:''"`

	PartyType PartyType `gorm:"type:text;not null;default:''"`
	PartyID   uint      `gorm:"not null;default:0"`

	// Banking: reconciliation markers (optional).
	ReconciliationID *uint      `gorm:"index"`
	ReconciledAt     *time.Time `gorm:""`
}
