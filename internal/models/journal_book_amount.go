// 遵循project_guide.md
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// JournalLineBookAmount stores per-secondary-book accounted amounts for a single
// journal line. Primary book amounts remain in JournalLine.Debit/Credit (fully
// backward compatible). This table is ONLY populated for secondary (non-primary)
// accounting books.
//
// Immutability contract: rows are written once at posting time and never updated.
// They are retained indefinitely for audit and reporting.
//
// Unique constraint: (journal_line_id, book_id) — exactly one row per line per book.
type JournalLineBookAmount struct {
	ID            uint `gorm:"primaryKey"`
	JournalLineID uint `gorm:"not null;uniqueIndex:idx_jlba_line_book"`
	BookID        uint `gorm:"not null;uniqueIndex:idx_jlba_line_book;index"`
	CompanyID     uint `gorm:"not null;index"`

	// AccountedDebit is the debit amount in the book's FunctionalCurrencyCode.
	AccountedDebit decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	// AccountedCredit is the credit amount in the book's FunctionalCurrencyCode.
	AccountedCredit decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	// FXSnapshotID links to the FXSnapshot used to convert this line's amounts.
	// Nil when the book's functional currency matches the transaction currency
	// (identity conversion — no rate snapshot needed).
	FXSnapshotID *uint `gorm:"index"`

	CreatedAt time.Time
}
