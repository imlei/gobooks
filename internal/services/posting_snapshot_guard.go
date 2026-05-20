package services

import (
	"fmt"
	"time"

	"balanciz/internal/models"

	"gorm.io/gorm"
)

func ensureInvoicePostingSnapshotFresh(tx *gorm.DB, companyID, invoiceID uint, original models.Invoice) error {
	var current models.Invoice
	if err := tx.Select("id", "updated_at").
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&current).Error; err != nil {
		return fmt.Errorf("reload invoice snapshot: %w", err)
	}
	if timestampChanged(original.UpdatedAt, current.UpdatedAt) {
		return ErrPostingSourceChanged
	}

	var currentLines []models.InvoiceLine
	if err := applyLockForUpdate(
		tx.Select("id", "updated_at").
			Where("invoice_id = ? AND company_id = ?", invoiceID, companyID).
			Order("id ASC"),
	).Find(&currentLines).Error; err != nil {
		return fmt.Errorf("lock invoice lines: %w", err)
	}
	if lineSnapshotChanged(original.Lines, currentLines) {
		return ErrPostingSourceChanged
	}
	return nil
}

func ensureBillPostingSnapshotFresh(tx *gorm.DB, companyID, billID uint, original models.Bill) error {
	var current models.Bill
	if err := tx.Select("id", "updated_at").
		Where("id = ? AND company_id = ?", billID, companyID).
		First(&current).Error; err != nil {
		return fmt.Errorf("reload bill snapshot: %w", err)
	}
	if timestampChanged(original.UpdatedAt, current.UpdatedAt) {
		return ErrPostingSourceChanged
	}

	var currentLines []models.BillLine
	if err := applyLockForUpdate(
		tx.Select("id", "updated_at").
			Where("bill_id = ? AND company_id = ?", billID, companyID).
			Order("id ASC"),
	).Find(&currentLines).Error; err != nil {
		return fmt.Errorf("lock bill lines: %w", err)
	}
	if billLineSnapshotChanged(original.Lines, currentLines) {
		return ErrPostingSourceChanged
	}
	return nil
}

func lineSnapshotChanged(original []models.InvoiceLine, current []models.InvoiceLine) bool {
	if len(original) != len(current) {
		return true
	}
	seen := make(map[uint]time.Time, len(original))
	for _, line := range original {
		seen[line.ID] = line.UpdatedAt
	}
	for _, line := range current {
		originalUpdatedAt, ok := seen[line.ID]
		if !ok || timestampChanged(originalUpdatedAt, line.UpdatedAt) {
			return true
		}
	}
	return false
}

func billLineSnapshotChanged(original []models.BillLine, current []models.BillLine) bool {
	if len(original) != len(current) {
		return true
	}
	seen := make(map[uint]time.Time, len(original))
	for _, line := range original {
		seen[line.ID] = line.UpdatedAt
	}
	for _, line := range current {
		originalUpdatedAt, ok := seen[line.ID]
		if !ok || timestampChanged(originalUpdatedAt, line.UpdatedAt) {
			return true
		}
	}
	return false
}

func timestampChanged(original, current time.Time) bool {
	if original.IsZero() || current.IsZero() {
		return false
	}
	return !original.Equal(current)
}
