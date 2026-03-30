// 遵循project_guide.md
package services

import (
	"fmt"
	"strings"
	"time"

	"gobooks/internal/models"

	"gorm.io/gorm"
)

// ReverseJournalEntry creates a new entry with debit/credit swapped for each line.
// It returns the new reversed journal entry ID. originalID must belong to companyID.
//
// Phase 5 lifecycle transitions (all within the caller-provided transaction):
//   - reversal JE is created with status=posted
//   - original JE status is updated to reversed
//   - original JE's ledger entries are marked reversed
//   - reversal JE's lines are projected to ledger as new active entries
//
// Phase 6 concurrency control:
//   - SELECT FOR UPDATE is acquired on the original JE row before any writes.
//     A concurrent reversal of the same JE blocks at the lock; when it unblocks
//     it will find reversed_from_id already set and return "already reversed".
func ReverseJournalEntry(tx *gorm.DB, companyID uint, originalID uint, reverseDate time.Time) (uint, error) {
	if originalID == 0 {
		return 0, fmt.Errorf("invalid journal entry id")
	}

	// Load the original JE with a row-level lock to serialise concurrent reversals.
	// applyLockForUpdate is a no-op for SQLite; Postgres gets SELECT ... FOR UPDATE.
	var original models.JournalEntry
	if err := applyLockForUpdate(
		tx.Preload("Lines").Where("id = ? AND company_id = ?", originalID, companyID),
	).First(&original).Error; err != nil {
		return 0, err
	}
	if original.ReversedFromID != nil {
		return 0, fmt.Errorf("cannot reverse a reversal entry")
	}
	if len(original.Lines) < 2 {
		return 0, fmt.Errorf("journal entry must have at least 2 lines")
	}

	var existing models.JournalEntry
	if err := tx.Where("reversed_from_id = ? AND company_id = ?", originalID, companyID).First(&existing).Error; err == nil {
		return 0, fmt.Errorf("journal entry already reversed")
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return 0, err
	}

	revDesc := fmt.Sprintf("Reversal of JE #%d", original.ID)
	if s := strings.TrimSpace(original.JournalNo); s != "" {
		revDesc = fmt.Sprintf("%s: %s", revDesc, s)
	}
	// SourceType=reversal, SourceID=original.ID (the JE being reversed).
	// The unique partial index (status='posted', source_type != '', source_id > 0)
	// ensures at most one posted reversal per original JE, acting as DB backstop.
	reversed := models.JournalEntry{
		CompanyID:      companyID,
		EntryDate:      reverseDate,
		JournalNo:      revDesc,
		ReversedFromID: &original.ID,
		Status:         models.JournalEntryStatusPosted,
		SourceType:     models.LedgerSourceReversal,
		SourceID:       original.ID,
	}
	if err := wrapUniqueViolation(tx.Create(&reversed).Error, "create reversal entry"); err != nil {
		return 0, err
	}

	lines := make([]models.JournalLine, 0, len(original.Lines))
	for _, l := range original.Lines {
		lines = append(lines, models.JournalLine{
			CompanyID:      companyID,
			JournalEntryID: reversed.ID,
			AccountID:      l.AccountID,
			Debit:          l.Credit,
			Credit:         l.Debit,
			Memo:           l.Memo,
			PartyType:      l.PartyType,
			PartyID:        l.PartyID,
		})
	}

	if err := tx.Create(&lines).Error; err != nil {
		return 0, err
	}

	// Mark original JE as reversed.
	if err := tx.Model(&models.JournalEntry{}).
		Where("id = ? AND company_id = ?", original.ID, companyID).
		Update("status", models.JournalEntryStatusReversed).Error; err != nil {
		return 0, fmt.Errorf("mark original journal entry reversed: %w", err)
	}

	// Mark original JE's ledger entries as reversed.
	if err := MarkLedgerEntriesReversed(tx, companyID, original.ID); err != nil {
		return 0, fmt.Errorf("mark ledger entries reversed: %w", err)
	}

	// Project reversal JE lines to ledger as new active entries.
	if err := ProjectToLedger(tx, companyID, LedgerPostInput{
		JournalEntry: reversed,
		Lines:        lines,
		SourceType:   models.LedgerSourceReversal,
		SourceID:     original.ID,
	}); err != nil {
		return 0, fmt.Errorf("project reversal to ledger: %w", err)
	}

	return reversed.ID, nil
}
