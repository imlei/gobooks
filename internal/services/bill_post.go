// 遵循project_guide.md
package services

// bill_post.go — PostBill: posting pipeline for purchase bills.
//
// Posting pipeline (Phase 4 + Phase 6 concurrency controls):
//
//   1. Load bill + lines  (TaxCode + ProductService preloaded)
//   2. Pre-flight validation
//   3. Resolve Accounts Payable account
//   4. BuildBillFragments   → raw []PostingFragment per line (fragment_builder.go)
//   5. AggregateJournalLines → collapse by account + side (journal_aggregate.go)
//   6. Validate double-entry balance (ΣDebit == ΣCredit)
//   7. Transaction:
//        a. INSERT journal_entries header
//        b. INSERT journal_lines (one per aggregated fragment)
//        c. ProjectToLedger   → INSERT ledger_entries (ledger.go)
//        d. UPDATE bills      → status='posted', amount=total, journal_entry_id
//        e. WriteAuditLog
//
// Before vs after journal shape — example bill $1 000 net, 13% HST (full recovery):
//
//   Line 1: Office rent   $600.00  net, HST $78.00  → expense account 6100
//   Line 2: Office supply $400.00  net, HST $52.00  → expense account 6100  (same acct)
//
//   Raw fragments (pre-aggregation):
//     DR  6100 Office Expense   600.00   (rent net — ITC fully recoverable)
//     DR  1320 ITC Receivable    78.00   (rent HST)
//     DR  6100 Office Expense   400.00   (supplies net)
//     DR  1320 ITC Receivable    52.00   (supplies HST)
//     CR  2000 AP              1130.00
//
//   After AggregateJournalLines (merged by account + side):
//     DR  6100 Office Expense  1 000.00  ← two expense lines merged
//     DR  1320 ITC Receivable    130.00  ← two ITC lines merged
//     CR  2000 AP              1 130.00
//
//   Non-recoverable tax variant — same bill, TaxCode.RecoveryMode = none:
//     Raw fragments:
//       DR  6100 Office Expense   678.00  (600 + 78 embedded non-recoverable)
//       DR  6100 Office Expense   452.00  (400 + 52 embedded)
//       CR  2000 AP              1130.00
//     After aggregation:
//       DR  6100 Office Expense  1 130.00  ← net + full tax merged into expense
//       CR  2000 AP              1 130.00
//
//   Ledger entries (one per journal line, status=active):
//     company 1, account 6100, debit  1 000.00, credit      0
//     company 1, account 1320, debit    130.00, credit      0
//     company 1, account 2000, debit        0,  credit  1 130.00

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

// ErrBillNotDraft is returned when posting is attempted on a non-draft bill.
var ErrBillNotDraft = errors.New("only draft bills can be posted")

// ErrNoAPAccount is returned when no active Accounts Payable account exists for the company.
var ErrNoAPAccount = errors.New("no active Accounts Payable account found — create one in your Chart of Accounts first")

// PostBill transitions a draft bill to "posted" and generates a double-entry
// journal entry in a single database transaction.
//
// Recovery mode behaviour (from TaxCode.RecoveryMode):
//   - full:    entire tax → ITC Receivable debit; expense = lineNet only.
//   - partial: TaxCode.RecoveryRate % → ITC Receivable; remainder embedded in expense.
//   - none:    no ITC line; full tax embedded in expense debit.
//
// Returns ErrBillNotDraft, ErrNoAPAccount, or a descriptive error on failure.
func PostBill(db *gorm.DB, companyID, billID uint, actor string, userID *uuid.UUID) error {
	// ── 1. Load bill with full line detail ────────────────────────────────────
	var bill models.Bill
	if err := db.
		Preload("Lines.TaxCode").
		Preload("Lines.ProductService").
		Where("id = ? AND company_id = ?", billID, companyID).
		First(&bill).Error; err != nil {
		return fmt.Errorf("load bill: %w", err)
	}

	// ── 2. Pre-flight checks ──────────────────────────────────────────────────
	if bill.Status != models.BillStatusDraft {
		return ErrBillNotDraft
	}
	if len(bill.Lines) == 0 {
		return errors.New("bill has no line items")
	}
	for i, l := range bill.Lines {
		if l.ExpenseAccountID == nil {
			return fmt.Errorf("line %d (%q): expense account is required before posting", i+1, l.Description)
		}
	}

	// ── 3. Resolve AP account ─────────────────────────────────────────────────
	var apAccount models.Account
	if err := db.
		Where("company_id = ? AND detail_account_type = ? AND is_active = true",
			companyID, string(models.DetailAccountsPayable)).
		Order("code asc").
		First(&apAccount).Error; err != nil {
		return ErrNoAPAccount
	}

	// ── 4. Build posting fragments ────────────────────────────────────────────
	// Pure function: one DR per line (expense ± embedded tax), one DR per
	// recoverable-tax line (ITC), and one CR (AP) for the gross total.
	frags, err := BuildBillFragments(bill, apAccount.ID)
	if err != nil {
		return fmt.Errorf("build bill fragments: %w", err)
	}

	// ── 5. Aggregate by account + side ────────────────────────────────────────
	// Lines sharing the same expense account are merged. ITC lines sharing the
	// same recoverable-tax account are merged. AP credit is always a single line.
	// Debit and credit sides are never merged together.
	jeLines, err := AggregateJournalLines(frags)
	if err != nil {
		return fmt.Errorf("aggregate journal lines: %w", err)
	}

	// ── 6. Double-entry balance check ─────────────────────────────────────────
	debitSum := sumPostingDebits(jeLines)
	creditSum := sumPostingCredits(jeLines)
	if !debitSum.Equal(creditSum) {
		return fmt.Errorf(
			"journal entry imbalance: debit sum %s, credit sum %s — check line totals",
			debitSum.StringFixed(2), creditSum.StringFixed(2),
		)
	}

	// ── 7. Transaction ────────────────────────────────────────────────────────
	return db.Transaction(func(tx *gorm.DB) error {
		// a. Lock bill row and re-validate status inside the lock.
		var locked models.Bill
		if err := applyLockForUpdate(
			tx.Select("id", "company_id", "status").
				Where("id = ? AND company_id = ?", billID, companyID),
		).First(&locked).Error; err != nil {
			return fmt.Errorf("lock bill: %w", err)
		}
		if locked.Status != models.BillStatusDraft {
			return ErrAlreadyPosted
		}

		// b. Journal entry header.
		je := models.JournalEntry{
			CompanyID:  companyID,
			EntryDate:  bill.BillDate,
			JournalNo:  bill.BillNumber,
			Status:     models.JournalEntryStatusPosted,
			SourceType: models.LedgerSourceBill,
			SourceID:   bill.ID,
		}
		if err := wrapUniqueViolation(tx.Create(&je).Error, "create journal entry"); err != nil {
			return fmt.Errorf("create journal entry: %w", err)
		}

		// c. Journal lines — one per aggregated fragment.
		//    Collect created rows for the ledger projection step.
		createdLines := make([]models.JournalLine, 0, len(jeLines))
		for _, jl := range jeLines {
			line := models.JournalLine{
				CompanyID:      companyID,
				JournalEntryID: je.ID,
				AccountID:      jl.AccountID,
				Debit:          jl.Debit,
				Credit:         jl.Credit,
				Memo:           jl.Memo,
				PartyType:      models.PartyTypeVendor,
				PartyID:        bill.VendorID,
			}
			if err := tx.Create(&line).Error; err != nil {
				return fmt.Errorf("create journal line: %w", err)
			}
			createdLines = append(createdLines, line)
		}

		// d. Ledger projection — one ledger_entry per journal_line, status=active.
		if err := ProjectToLedger(tx, companyID, LedgerPostInput{
			JournalEntry: je,
			Lines:        createdLines,
			SourceType:   models.LedgerSourceBill,
			SourceID:     bill.ID,
		}); err != nil {
			return fmt.Errorf("project to ledger: %w", err)
		}

		// e. Update bill: mark posted, cache grand total (AP credit = gross payable),
		//    link journal entry.
		if err := tx.Model(&bill).Updates(map[string]any{
			"status":           string(models.BillStatusPosted),
			"amount":           creditSum,
			"journal_entry_id": je.ID,
		}).Error; err != nil {
			return fmt.Errorf("update bill status: %w", err)
		}

		// f. Audit log.
		cid := companyID
		return WriteAuditLogWithContextDetails(tx, "bill.posted", "bill", bill.ID, actor,
			map[string]any{"company_id": companyID},
			&cid, userID, nil,
			map[string]any{
				"bill_number":      bill.BillNumber,
				"journal_entry_id": je.ID,
				"total":            creditSum.StringFixed(2),
			},
		)
	})
}
