package services

import (
	"fmt"
	"time"

	"balanciz/internal/models"

	"gorm.io/gorm"
)

func ensureInvoicePostingSnapshotFresh(tx *gorm.DB, companyID, invoiceID uint, original models.Invoice) error {
	_, err := lockFreshInvoiceForPostingSnapshot(tx, companyID, invoiceID, original)
	return err
}

func ensureBillPostingSnapshotFresh(tx *gorm.DB, companyID, billID uint, original models.Bill) error {
	_, err := lockFreshBillForPostingSnapshot(tx, companyID, billID, original)
	return err
}

func lockFreshInvoiceForPostingSnapshot(tx *gorm.DB, companyID, invoiceID uint, original models.Invoice) (models.Invoice, error) {
	var current models.Invoice
	if err := applyLockForUpdate(tx.
		Where("id = ? AND company_id = ?", invoiceID, companyID),
	).First(&current).Error; err != nil {
		return current, fmt.Errorf("reload invoice snapshot: %w", err)
	}
	if timestampChanged(original.UpdatedAt, current.UpdatedAt) {
		return current, ErrPostingSourceChanged
	}

	var currentLines []models.InvoiceLine
	if err := applyLockForUpdate(
		tx.
			Preload("ProductService.RevenueAccount").
			Preload("TaxCode").
			Where("invoice_id = ? AND company_id = ?", invoiceID, companyID).
			Order("sort_order ASC, id ASC"),
	).Find(&currentLines).Error; err != nil {
		return current, fmt.Errorf("lock invoice lines: %w", err)
	}
	current.Lines = currentLines
	if lineSnapshotChanged(original.Lines, currentLines) {
		return current, ErrPostingSourceChanged
	}
	if invoicePostingSnapshotChanged(original, current) {
		return current, ErrPostingSourceChanged
	}
	return current, nil
}

func lockFreshBillForPostingSnapshot(tx *gorm.DB, companyID, billID uint, original models.Bill) (models.Bill, error) {
	var current models.Bill
	if err := applyLockForUpdate(tx.
		Where("id = ? AND company_id = ?", billID, companyID),
	).First(&current).Error; err != nil {
		return current, fmt.Errorf("reload bill snapshot: %w", err)
	}
	if timestampChanged(original.UpdatedAt, current.UpdatedAt) {
		return current, ErrPostingSourceChanged
	}

	var currentLines []models.BillLine
	if err := applyLockForUpdate(
		tx.
			Preload("TaxCode").
			Preload("ProductService").
			Where("bill_id = ? AND company_id = ?", billID, companyID).
			Order("sort_order ASC, id ASC"),
	).Find(&currentLines).Error; err != nil {
		return current, fmt.Errorf("lock bill lines: %w", err)
	}
	current.Lines = currentLines
	if billLineSnapshotChanged(original.Lines, currentLines) {
		return current, ErrPostingSourceChanged
	}
	if billPostingSnapshotChanged(original, current) {
		return current, ErrPostingSourceChanged
	}
	return current, nil
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

func invoicePostingSnapshotChanged(original, current models.Invoice) bool {
	if original.ID != current.ID ||
		original.CompanyID != current.CompanyID ||
		original.CustomerID != current.CustomerID ||
		original.InvoiceNumber != current.InvoiceNumber ||
		!sameTime(original.InvoiceDate, current.InvoiceDate) ||
		original.Status != current.Status ||
		!original.Amount.Equal(current.Amount) ||
		!original.Subtotal.Equal(current.Subtotal) ||
		!original.TaxTotal.Equal(current.TaxTotal) ||
		normalizeCurrencyCode(original.CurrencyCode) != normalizeCurrencyCode(current.CurrencyCode) ||
		!original.ExchangeRate.Equal(current.ExchangeRate) ||
		!sameUintPtr(original.WarehouseID, current.WarehouseID) ||
		!sameUintPtr(original.JournalEntryID, current.JournalEntryID) ||
		!sameUintPtr(original.ChannelOrderID, current.ChannelOrderID) ||
		!sameUintPtr(original.SalesOrderID, current.SalesOrderID) {
		return true
	}
	if len(original.Lines) != len(current.Lines) {
		return true
	}
	originalByID := make(map[uint]models.InvoiceLine, len(original.Lines))
	for _, line := range original.Lines {
		originalByID[line.ID] = line
	}
	for _, currentLine := range current.Lines {
		originalLine, ok := originalByID[currentLine.ID]
		if !ok || invoiceLinePostingSnapshotChanged(originalLine, currentLine) {
			return true
		}
	}
	return false
}

func invoiceLinePostingSnapshotChanged(original, current models.InvoiceLine) bool {
	return original.CompanyID != current.CompanyID ||
		original.InvoiceID != current.InvoiceID ||
		original.SortOrder != current.SortOrder ||
		!sameUintPtr(original.ProductServiceID, current.ProductServiceID) ||
		original.Description != current.Description ||
		!original.Qty.Equal(current.Qty) ||
		!original.UnitPrice.Equal(current.UnitPrice) ||
		original.LineUOM != current.LineUOM ||
		!original.LineUOMFactor.Equal(current.LineUOMFactor) ||
		!original.QtyInStockUOM.Equal(current.QtyInStockUOM) ||
		!sameUintPtr(original.TaxCodeID, current.TaxCodeID) ||
		!original.LineNet.Equal(current.LineNet) ||
		!original.LineTax.Equal(current.LineTax) ||
		!original.LineTotal.Equal(current.LineTotal) ||
		!sameUintPtr(original.ShipmentLineID, current.ShipmentLineID) ||
		!sameUintPtr(original.SalesOrderLineID, current.SalesOrderLineID)
}

func billPostingSnapshotChanged(original, current models.Bill) bool {
	if original.ID != current.ID ||
		original.CompanyID != current.CompanyID ||
		original.VendorID != current.VendorID ||
		original.BillNumber != current.BillNumber ||
		!sameTime(original.BillDate, current.BillDate) ||
		original.Status != current.Status ||
		!original.Amount.Equal(current.Amount) ||
		!original.Subtotal.Equal(current.Subtotal) ||
		!original.TaxTotal.Equal(current.TaxTotal) ||
		normalizeCurrencyCode(original.CurrencyCode) != normalizeCurrencyCode(current.CurrencyCode) ||
		!original.ExchangeRate.Equal(current.ExchangeRate) ||
		!sameUintPtr(original.WarehouseID, current.WarehouseID) ||
		!sameUintPtr(original.JournalEntryID, current.JournalEntryID) {
		return true
	}
	if len(original.Lines) != len(current.Lines) {
		return true
	}
	originalByID := make(map[uint]models.BillLine, len(original.Lines))
	for _, line := range original.Lines {
		originalByID[line.ID] = line
	}
	for _, currentLine := range current.Lines {
		originalLine, ok := originalByID[currentLine.ID]
		if !ok || billLinePostingSnapshotChanged(originalLine, currentLine) {
			return true
		}
	}
	return false
}

func billLinePostingSnapshotChanged(original, current models.BillLine) bool {
	return original.CompanyID != current.CompanyID ||
		original.BillID != current.BillID ||
		original.SortOrder != current.SortOrder ||
		!sameUintPtr(original.ProductServiceID, current.ProductServiceID) ||
		original.Description != current.Description ||
		!original.Qty.Equal(current.Qty) ||
		!original.UnitPrice.Equal(current.UnitPrice) ||
		original.LineUOM != current.LineUOM ||
		!original.LineUOMFactor.Equal(current.LineUOMFactor) ||
		!original.QtyInStockUOM.Equal(current.QtyInStockUOM) ||
		!sameUintPtr(original.TaxCodeID, current.TaxCodeID) ||
		!sameUintPtr(original.ExpenseAccountID, current.ExpenseAccountID) ||
		!original.LineNet.Equal(current.LineNet) ||
		!original.LineTax.Equal(current.LineTax) ||
		!original.LineTotal.Equal(current.LineTotal) ||
		!sameUintPtr(original.TaskID, current.TaskID) ||
		!sameUintPtr(original.BillableCustomerID, current.BillableCustomerID) ||
		original.IsBillable != current.IsBillable ||
		original.ReinvoiceStatus != current.ReinvoiceStatus ||
		!sameUintPtr(original.ReceiptLineID, current.ReceiptLineID) ||
		original.LotNumber != current.LotNumber ||
		!sameTimePtr(original.LotExpiryDate, current.LotExpiryDate)
}

func sameUintPtr(a, b *uint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameTime(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return a.IsZero() && b.IsZero()
	}
	return a.Equal(b)
}

func sameTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameTime(*a, *b)
}
