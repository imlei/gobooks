// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// JournalEntry is the header for a double-entry transaction.
type JournalEntry struct {
	ID        uint      `gorm:"primaryKey"`
	CompanyID uint      `gorm:"not null;index"`
	EntryDate time.Time `gorm:"not null"`
	// JournalNo is an optional user-facing reference (e.g. JE-001); stored as journal_no.
	JournalNo string `gorm:"column:journal_no;not null;default:''"`
	// If set, this entry is a reversal of another entry.
	ReversedFromID *uint `gorm:"index"`
	CreatedAt   time.Time

	Lines []JournalLine `gorm:"foreignKey:JournalEntryID"`
}

// JournalLine is a single debit OR credit line in a journal entry.
//
// PROJECT_GUIDE rules (enforced in handlers/services):
// - Debit and Credit cannot both have values.
// - A saved Journal Entry must have at least 2 valid lines.
// - Total Debits must equal Total Credits.
type JournalLine struct {
	ID             uint `gorm:"primaryKey"`
	CompanyID      uint `gorm:"not null;index"`
	JournalEntryID uint `gorm:"not null;index"`

	AccountID uint   `gorm:"not null;index"`
	Account   Account `gorm:"foreignKey:AccountID"`

	Debit  decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	Credit decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	Memo string `gorm:"not null;default:''"`

	PartyType PartyType `gorm:"type:text;not null;default:''"`
	PartyID   uint      `gorm:"not null;default:0"`

	// Banking: reconciliation markers (optional).
	ReconciliationID *uint          `gorm:"index"`
	ReconciledAt     *time.Time     `gorm:""`
}

