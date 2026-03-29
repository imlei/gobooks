// 遵循产品需求 v1.0
package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

// ErrInvoiceNotVoidable is returned when voiding is attempted on an invoice that
// is not in the "sent" status.
var ErrInvoiceNotVoidable = errors.New("only posted (sent) invoices can be voided")

// VoidInvoice reverses the accounting for a posted invoice by:
//  1. Loading the original posting JournalEntry and its lines.
//  2. Creating a reversal JournalEntry with all debits and credits swapped.
//  3. Marking the invoice status as "voided".
//
// Only invoices with status = "sent" are voidable.
// Paid invoices require the payment to be reversed first.
func VoidInvoice(db *gorm.DB, companyID, invoiceID uint, actor string, userID *uuid.UUID) error {
	// ── Load invoice with original JE ────────────────────────────────────────
	var inv models.Invoice
	err := db.
		Preload("JournalEntry.Lines").
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&inv).Error
	if err != nil {
		return fmt.Errorf("load invoice: %w", err)
	}

	if inv.Status != models.InvoiceStatusSent {
		return ErrInvoiceNotVoidable
	}
	if inv.JournalEntryID == nil || inv.JournalEntry == nil {
		return errors.New("invoice has no linked journal entry — cannot void")
	}

	origJE := inv.JournalEntry
	if len(origJE.Lines) == 0 {
		return errors.New("original journal entry has no lines")
	}

	// ── Build reversal lines (swap debit / credit) ────────────────────────────
	type revLine struct {
		AccountID uint
		Debit     decimal.Decimal
		Credit    decimal.Decimal
		Memo      string
		PartyType models.PartyType
		PartyID   uint
	}
	var revLines []revLine
	for _, l := range origJE.Lines {
		revLines = append(revLines, revLine{
			AccountID: l.AccountID,
			Debit:     l.Credit, // swap
			Credit:    l.Debit,  // swap
			Memo:      "VOID: " + l.Memo,
			PartyType: l.PartyType,
			PartyID:   l.PartyID,
		})
	}

	// ── Transaction ───────────────────────────────────────────────────────────
	return db.Transaction(func(tx *gorm.DB) error {
		// Create reversal JE.
		reversalJE := models.JournalEntry{
			CompanyID:      companyID,
			EntryDate:      origJE.EntryDate,
			JournalNo:      "VOID-" + inv.InvoiceNumber,
			ReversedFromID: &origJE.ID,
		}
		if err := tx.Create(&reversalJE).Error; err != nil {
			return fmt.Errorf("create reversal journal entry: %w", err)
		}

		for _, rl := range revLines {
			line := models.JournalLine{
				CompanyID:      companyID,
				JournalEntryID: reversalJE.ID,
				AccountID:      rl.AccountID,
				Debit:          rl.Debit,
				Credit:         rl.Credit,
				Memo:           rl.Memo,
				PartyType:      rl.PartyType,
				PartyID:        rl.PartyID,
			}
			if err := tx.Create(&line).Error; err != nil {
				return fmt.Errorf("create reversal line: %w", err)
			}
		}

		// Mark invoice voided.
		if err := tx.Model(&inv).Updates(map[string]any{
			"status": string(models.InvoiceStatusVoided),
		}).Error; err != nil {
			return fmt.Errorf("update invoice status: %w", err)
		}

		cid := companyID
		return WriteAuditLogWithContextDetails(tx, "invoice.voided", "invoice", inv.ID, actor,
			map[string]any{"company_id": companyID},
			&cid, userID, nil,
			map[string]any{
				"invoice_number":    inv.InvoiceNumber,
				"reversal_entry_id": reversalJE.ID,
				"total":             inv.Amount.StringFixed(2),
			},
		)
	})
}
