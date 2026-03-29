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

// ErrInvoiceNotDraft is returned when posting is attempted on a non-draft invoice.
var ErrInvoiceNotDraft = errors.New("only draft invoices can be posted")

// ErrNoARAccount is returned when no active Accounts Receivable account exists for the company.
var ErrNoARAccount = errors.New("no active Accounts Receivable account found — create one in your Chart of Accounts first")

// PostInvoice transitions a draft invoice to "sent" and generates a double-entry
// journal entry in a single transaction.
//
// Accounting entries produced:
//
//	Dr  Accounts Receivable    invoice.Amount    (full gross total)
//	Cr  Revenue account        line.LineNet      (one credit per line)
//	Cr  Sales Tax Payable      tax amount        (one credit per taxed line, via SalesTaxPostingLine)
//
// All lines must have a ProductService (for the revenue account).
// The AR account is the first active account with detail_type = accounts_receivable.
//
// Returns ErrInvoiceNotDraft, ErrNoARAccount, or a descriptive error on failure.
func PostInvoice(db *gorm.DB, companyID, invoiceID uint, actor string, userID *uuid.UUID) error {
	// ── Load invoice with full line detail ───────────────────────────────────
	var inv models.Invoice
	err := db.
		Preload("Lines.ProductService.RevenueAccount").
		Preload("Lines.TaxCode").
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&inv).Error
	if err != nil {
		return fmt.Errorf("load invoice: %w", err)
	}

	// ── Pre-flight checks ────────────────────────────────────────────────────
	if inv.Status != models.InvoiceStatusDraft {
		return ErrInvoiceNotDraft
	}
	if len(inv.Lines) == 0 {
		return errors.New("invoice has no line items")
	}
	for i, l := range inv.Lines {
		if l.ProductServiceID == nil {
			return fmt.Errorf("line %d (%q) has no product/service — assign one before posting", i+1, l.Description)
		}
		if l.ProductService.RevenueAccountID == 0 {
			return fmt.Errorf("line %d (%q): product/service has no revenue account configured", i+1, l.Description)
		}
		if !l.ProductService.IsActive {
			return fmt.Errorf("line %d (%q): product/service is inactive", i+1, l.Description)
		}
	}

	// ── Find AR account ───────────────────────────────────────────────────────
	var arAccount models.Account
	if err := db.
		Where("company_id = ? AND detail_account_type = ? AND is_active = true", companyID, string(models.DetailAccountsReceivable)).
		Order("code asc").
		First(&arAccount).Error; err != nil {
		return ErrNoARAccount
	}

	// ── Build JE lines ────────────────────────────────────────────────────────
	// Verify totals balance before writing anything.
	creditSum := decimal.Zero
	type jeLine struct {
		AccountID uint
		Debit     decimal.Decimal
		Credit    decimal.Decimal
		Memo      string
	}
	var jeLines []jeLine

	// Debit: Accounts Receivable = full gross total.
	jeLines = append(jeLines, jeLine{
		AccountID: arAccount.ID,
		Debit:     inv.Amount,
		Credit:    decimal.Zero,
		Memo:      "Invoice " + inv.InvoiceNumber,
	})

	for _, l := range inv.Lines {
		// Credit revenue account with the pre-tax line amount.
		jeLines = append(jeLines, jeLine{
			AccountID: l.ProductService.RevenueAccountID,
			Debit:     decimal.Zero,
			Credit:    l.LineNet,
			Memo:      l.Description,
		})
		creditSum = creditSum.Add(l.LineNet)

		// Credit Sales Tax Payable — use SalesTaxPostingLine for the flat-rate tax code.
		if l.TaxCodeID != nil && l.TaxCode != nil && l.TaxCode.Scope != models.TaxScopePurchase {
			taxResult := ComputeTax(l.LineNet, *l.TaxCode)
			if taxResult.TaxAmount.IsPositive() {
				posting := SalesTaxPostingLine(taxResult, *l.TaxCode)
				jeLines = append(jeLines, jeLine{
					AccountID: posting.AccountID,
					Debit:     decimal.Zero,
					Credit:    posting.Amount,
					Memo:      "Tax on " + l.Description,
				})
				creditSum = creditSum.Add(posting.Amount)
			}
		}
	}

	// Sanity check: credits must equal the AR debit (= inv.Amount).
	if !creditSum.Equal(inv.Amount) {
		return fmt.Errorf("journal entry imbalance: AR debit %s ≠ credit sum %s — check line totals",
			inv.Amount.StringFixed(2), creditSum.StringFixed(2))
	}

	// ── Transaction ───────────────────────────────────────────────────────────
	return db.Transaction(func(tx *gorm.DB) error {
		je := models.JournalEntry{
			CompanyID: companyID,
			EntryDate: inv.InvoiceDate,
			JournalNo: "INV-" + inv.InvoiceNumber,
		}
		if err := tx.Create(&je).Error; err != nil {
			return fmt.Errorf("create journal entry: %w", err)
		}

		for _, jl := range jeLines {
			line := models.JournalLine{
				CompanyID:      companyID,
				JournalEntryID: je.ID,
				AccountID:      jl.AccountID,
				Debit:          jl.Debit,
				Credit:         jl.Credit,
				Memo:           jl.Memo,
				PartyType:      models.PartyTypeCustomer,
				PartyID:        inv.CustomerID,
			}
			if err := tx.Create(&line).Error; err != nil {
				return fmt.Errorf("create journal line: %w", err)
			}
		}

		// Update invoice: mark sent, link journal entry.
		if err := tx.Model(&inv).Updates(map[string]any{
			"status":           string(models.InvoiceStatusSent),
			"journal_entry_id": je.ID,
		}).Error; err != nil {
			return fmt.Errorf("update invoice status: %w", err)
		}

		cid := companyID
		return WriteAuditLogWithContextDetails(tx, "invoice.posted", "invoice", inv.ID, actor,
			map[string]any{"company_id": companyID},
			&cid, userID, nil,
			map[string]any{
				"invoice_number":   inv.InvoiceNumber,
				"journal_entry_id": je.ID,
				"total":            inv.Amount.StringFixed(2),
			},
		)
	})
}
