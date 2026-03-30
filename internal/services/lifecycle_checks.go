// 遵循project_guide.md
package services

// lifecycle_checks.go — consistency checkers between business documents and
// their associated journal entries / ledger entries.
//
// These functions are read-only diagnostic helpers. They do NOT modify state.
// Call them from admin tools, audit reports, or test harnesses to detect
// records that were left in an inconsistent state (e.g. by an interrupted
// transaction or a bug in a legacy code path).
//
// Inconsistent states rejected:
//   - draft invoice/bill that has a posted JE     → ErrDraftWithActiveJournal
//   - sent/posted/paid doc with no JE             → ErrPostedWithoutJournal
//   - sent/posted/paid doc whose JE is not posted → ErrPostedWithUnpostedJournal
//   - voided doc whose JE still has active ledger entries → ErrVoidedWithActiveEntries

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"gobooks/internal/models"
)

// ── Error sentinels ───────────────────────────────────────────────────────────

// ErrDraftWithActiveJournal is returned when a draft document has a posted JE.
// A draft document must not have committed accounting entries.
var ErrDraftWithActiveJournal = errors.New("draft document has a posted journal entry — expected no active JE")

// ErrPostedWithoutJournal is returned when a posted/sent/paid document has no JE.
// Every posted document must have a corresponding journal entry.
var ErrPostedWithoutJournal = errors.New("posted document has no linked journal entry")

// ErrPostedWithUnpostedJournal is returned when a posted/sent/paid document's
// linked JE is not in the 'posted' state.
var ErrPostedWithUnpostedJournal = errors.New("posted document's journal entry is not in posted status")

// ErrVoidedWithActiveEntries is returned when a voided document's original JE
// still has active (non-reversed) ledger entries. After a successful void, all
// original ledger entries must be reversed.
var ErrVoidedWithActiveEntries = errors.New("voided document's journal entry still has active ledger entries")

// ── Invoice consistency ───────────────────────────────────────────────────────

// CheckInvoiceConsistency verifies that an invoice's status is consistent with
// its linked journal entry and ledger entries.
//
// Rules:
//   - draft:  JE nil → OK; JE.status='draft' → OK; JE.status='posted' → error
//   - sent:   JE must exist AND JE.status='posted'
//   - paid:   same as sent
//   - voided: JE must exist AND JE.status='reversed' AND no active ledger entries
func CheckInvoiceConsistency(db *gorm.DB, companyID, invoiceID uint) error {
	var inv models.Invoice
	if err := db.
		Preload("JournalEntry").
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&inv).Error; err != nil {
		return fmt.Errorf("load invoice: %w", err)
	}

	switch inv.Status {
	case models.InvoiceStatusDraft:
		if inv.JournalEntry != nil && inv.JournalEntry.Status == models.JournalEntryStatusPosted {
			return ErrDraftWithActiveJournal
		}

	case models.InvoiceStatusSent, models.InvoiceStatusPaid:
		if inv.JournalEntryID == nil || inv.JournalEntry == nil {
			return ErrPostedWithoutJournal
		}
		if inv.JournalEntry.Status != models.JournalEntryStatusPosted {
			return ErrPostedWithUnpostedJournal
		}

	case models.InvoiceStatusVoided:
		if inv.JournalEntryID == nil || inv.JournalEntry == nil {
			return ErrPostedWithoutJournal
		}
		if inv.JournalEntry.Status != models.JournalEntryStatusReversed {
			return fmt.Errorf("voided invoice's journal entry has status %q — expected 'reversed'", inv.JournalEntry.Status)
		}
		activeCount, err := countActiveLedgerEntries(db, companyID, inv.JournalEntry.ID)
		if err != nil {
			return fmt.Errorf("check active ledger entries: %w", err)
		}
		if activeCount > 0 {
			return ErrVoidedWithActiveEntries
		}
	}

	return nil
}

// ── Bill consistency ──────────────────────────────────────────────────────────

// CheckBillConsistency verifies that a bill's status is consistent with its
// linked journal entry and ledger entries.
//
// Rules:
//   - draft:  JE nil → OK; JE.status='draft' → OK; JE.status='posted' → error
//   - posted: JE must exist AND JE.status='posted'
//   - paid:   same as posted
//   - voided: JE must exist AND JE.status='reversed' AND no active ledger entries
func CheckBillConsistency(db *gorm.DB, companyID, billID uint) error {
	var bill models.Bill
	if err := db.
		Preload("JournalEntry").
		Where("id = ? AND company_id = ?", billID, companyID).
		First(&bill).Error; err != nil {
		return fmt.Errorf("load bill: %w", err)
	}

	switch bill.Status {
	case models.BillStatusDraft:
		if bill.JournalEntry != nil && bill.JournalEntry.Status == models.JournalEntryStatusPosted {
			return ErrDraftWithActiveJournal
		}

	case models.BillStatusPosted, models.BillStatusPaid:
		if bill.JournalEntryID == nil || bill.JournalEntry == nil {
			return ErrPostedWithoutJournal
		}
		if bill.JournalEntry.Status != models.JournalEntryStatusPosted {
			return ErrPostedWithUnpostedJournal
		}

	case models.BillStatusVoided:
		if bill.JournalEntryID == nil || bill.JournalEntry == nil {
			return ErrPostedWithoutJournal
		}
		if bill.JournalEntry.Status != models.JournalEntryStatusReversed {
			return fmt.Errorf("voided bill's journal entry has status %q — expected 'reversed'", bill.JournalEntry.Status)
		}
		activeCount, err := countActiveLedgerEntries(db, companyID, bill.JournalEntry.ID)
		if err != nil {
			return fmt.Errorf("check active ledger entries: %w", err)
		}
		if activeCount > 0 {
			return ErrVoidedWithActiveEntries
		}
	}

	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// countActiveLedgerEntries returns the number of active ledger entries for a
// given journal entry within a company.
func countActiveLedgerEntries(db *gorm.DB, companyID, journalEntryID uint) (int64, error) {
	var count int64
	err := db.Model(&models.LedgerEntry{}).
		Where("company_id = ? AND journal_entry_id = ? AND status = ?",
			companyID, journalEntryID, models.LedgerEntryStatusActive).
		Count(&count).Error
	return count, err
}
