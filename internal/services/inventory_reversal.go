// 遵循project_guide.md
package services

// inventory_reversal.go — Inventory movement reversal for voided documents.
//
// When an invoice or bill is voided, the JE reversal is handled by VoidInvoice/VoidBill.
// This file handles the corresponding inventory movement reversals that must occur
// in the SAME transaction as the JE reversal.
//
// Reversal movement design:
//   - source_type = "invoice_reversal" or "bill_reversal" (clearly distinguishable)
//   - source_id = original document ID (traceability to source document)
//   - journal_entry_id = reversal JE ID (links to the accounting side)
//   - Reversal movements are new rows — original movements are never modified/deleted
//
// Invoice void (sale reversal):
//   Original: sale movement (qty_delta = -N)
//   Reversal: inbound via CostingEngine.ApplyInbound (qty_delta = +N, restores stock)
//   Cost: uses the original sale movement's unit_cost (what was charged as COGS)
//
// Bill void (purchase reversal):
//   Original: purchase movement (qty_delta = +N)
//   Reversal: outbound via CostingEngine.ApplyOutbound (qty_delta = -N, reduces stock)
//   Cost: uses the original purchase movement's unit_cost
//   Blocked if insufficient stock (negative inventory not allowed)

import (
	"fmt"

	"github.com/shopspring/decimal"
	"gobooks/internal/models"
	"gorm.io/gorm"
)

// ReverseSaleMovements creates reversal inventory movements for a voided invoice.
// Restores stock for each stock item line using CostingEngine.ApplyInbound.
// Must be called inside the same transaction as the JE reversal.
func ReverseSaleMovements(tx *gorm.DB, companyID uint, inv models.Invoice, reversalJEID uint) error {
	// Load original sale movements for this invoice.
	var origMovements []models.InventoryMovement
	if err := tx.Where("company_id = ? AND source_type = ? AND source_id = ?",
		companyID, "invoice", inv.ID).
		Find(&origMovements).Error; err != nil {
		return fmt.Errorf("load sale movements: %w", err)
	}

	if len(origMovements) == 0 {
		return nil // no stock movements to reverse (service-only invoice)
	}

	engine, err := ResolveCostingEngineForCompany(tx, companyID)
	if err != nil {
		return fmt.Errorf("resolve costing engine: %w", err)
	}

	for _, orig := range origMovements {
		// Original sale movement has negative qty_delta; we restore the absolute qty.
		restoreQty := orig.QuantityDelta.Abs()
		if restoreQty.IsZero() {
			continue
		}

		// Use the original unit cost (what was recorded as COGS at posting time).
		unitCost := decimal.Zero
		if orig.UnitCost != nil {
			unitCost = *orig.UnitCost
		}

		// Apply inbound to restore stock and update weighted average cost.
		result, err := engine.ApplyInbound(tx, InboundRequest{
			CompanyID:    companyID,
			ItemID:       orig.ItemID,
			Quantity:     restoreQty,
			UnitCost:     unitCost,
			MovementType: models.MovementTypeAdjustment, // engine doesn't care about type
			LocationType: models.LocationTypeInternal,
		})
		if err != nil {
			return fmt.Errorf("restore stock for item %d: %w", orig.ItemID, err)
		}

		sourceID := inv.ID
		mov := models.InventoryMovement{
			CompanyID:      companyID,
			ItemID:         orig.ItemID,
			MovementType:   models.MovementTypeSale, // keeps the business context
			QuantityDelta:  restoreQty,              // positive = stock returned
			UnitCost:       &unitCost,
			TotalCost:      &result.TotalCost,
			SourceType:     "invoice_reversal",
			SourceID:       &sourceID,
			JournalEntryID: &reversalJEID,
			ReferenceNote:  "Void: " + inv.InvoiceNumber,
			MovementDate:   inv.InvoiceDate,
		}
		if err := tx.Create(&mov).Error; err != nil {
			return fmt.Errorf("create reversal movement for item %d: %w", orig.ItemID, err)
		}
	}

	return nil
}

// ReversePurchaseMovements creates reversal inventory movements for a voided bill.
// Reduces stock for each stock item line using CostingEngine.ApplyOutbound.
// Returns an error if insufficient stock exists (negative inventory not allowed).
// Must be called inside the same transaction as the JE reversal.
func ReversePurchaseMovements(tx *gorm.DB, companyID uint, bill models.Bill, reversalJEID uint) error {
	// Load original purchase movements for this bill.
	var origMovements []models.InventoryMovement
	if err := tx.Where("company_id = ? AND source_type = ? AND source_id = ?",
		companyID, "bill", bill.ID).
		Find(&origMovements).Error; err != nil {
		return fmt.Errorf("load purchase movements: %w", err)
	}

	if len(origMovements) == 0 {
		return nil // no stock movements to reverse (non-inventory bill)
	}

	engine, err := ResolveCostingEngineForCompany(tx, companyID)
	if err != nil {
		return fmt.Errorf("resolve costing engine: %w", err)
	}

	for _, orig := range origMovements {
		// Original purchase movement has positive qty_delta; we remove that qty.
		removeQty := orig.QuantityDelta.Abs()
		if removeQty.IsZero() {
			continue
		}

		// Apply outbound — will fail if insufficient stock.
		result, err := engine.ApplyOutbound(tx, OutboundRequest{
			CompanyID:    companyID,
			ItemID:       orig.ItemID,
			Quantity:     removeQty,
			MovementType: models.MovementTypePurchase,
			LocationType: models.LocationTypeInternal,
		})
		if err != nil {
			return fmt.Errorf("reverse purchase stock for item %d: %w", orig.ItemID, err)
		}

		unitCost := result.UnitCostUsed
		sourceID := bill.ID
		mov := models.InventoryMovement{
			CompanyID:      companyID,
			ItemID:         orig.ItemID,
			MovementType:   models.MovementTypePurchase, // keeps the business context
			QuantityDelta:  removeQty.Neg(),             // negative = stock removed
			UnitCost:       &unitCost,
			TotalCost:      &result.TotalCost,
			SourceType:     "bill_reversal",
			SourceID:       &sourceID,
			JournalEntryID: &reversalJEID,
			ReferenceNote:  "Void: " + bill.BillNumber,
			MovementDate:   bill.BillDate,
		}
		if err := tx.Create(&mov).Error; err != nil {
			return fmt.Errorf("create reversal movement for item %d: %w", orig.ItemID, err)
		}
	}

	return nil
}
