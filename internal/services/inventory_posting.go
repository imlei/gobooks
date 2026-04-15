// 遵循project_guide.md
package services

// inventory_posting.go — Inventory integration with the posting engine.
//
// All costing logic is delegated to the CostingEngine interface.
// Bundle lines are expanded into component-level stock operations.
//
// This file provides:
//   - Fragment builders for COGS (invoice) and inventory receipt (bill)
//   - Pre-flight stock validation (invoice) with bundle expansion
//   - Transactional movement creators that call CostingEngine

import (
	"fmt"

	"github.com/shopspring/decimal"
	"gobooks/internal/models"
	"gorm.io/gorm"
)

// ── COGS fragment builder (invoice sale) ─────────────────────────────────────

// BuildCOGSFragments generates Dr COGS / Cr Inventory Asset fragments for
// inventory items on an invoice. Handles both single stock items and bundle
// component items. outboundCosts maps component_item_id → OutboundResult.
//
// Bundle lines: COGS is generated for each component item, not the bundle itself.
// Single stock lines: COGS is generated for the line item directly.
func BuildCOGSFragments(lines []models.InvoiceLine, outboundCosts map[uint]*OutboundResult, bundleExpansions []ExpandedComponent) []PostingFragment {
	var frags []PostingFragment

	// 1. Single stock items (non-bundle).
	for _, l := range lines {
		if l.ProductService == nil || !l.ProductService.IsStockItem {
			continue
		}
		if l.ProductService.COGSAccountID == nil || l.ProductService.InventoryAccountID == nil {
			continue
		}
		result, ok := outboundCosts[l.ProductService.ID]
		if !ok || result == nil {
			continue
		}
		cogsAmount := l.Qty.Mul(result.UnitCostUsed).RoundBank(2)
		if cogsAmount.IsZero() {
			continue
		}
		frags = append(frags,
			PostingFragment{AccountID: *l.ProductService.COGSAccountID, Debit: cogsAmount, Credit: decimal.Zero, Memo: "COGS: " + l.Description},
			PostingFragment{AccountID: *l.ProductService.InventoryAccountID, Debit: decimal.Zero, Credit: cogsAmount, Memo: "Inventory out: " + l.Description},
		)
	}

	// 2. Bundle component items.
	for _, ec := range bundleExpansions {
		if ec.ComponentItem == nil || ec.ComponentItem.COGSAccountID == nil || ec.ComponentItem.InventoryAccountID == nil {
			continue
		}
		result, ok := outboundCosts[ec.ComponentItem.ID]
		if !ok || result == nil {
			continue
		}
		cogsAmount := ec.RequiredQty.Mul(result.UnitCostUsed).RoundBank(2)
		if cogsAmount.IsZero() {
			continue
		}
		frags = append(frags,
			PostingFragment{AccountID: *ec.ComponentItem.COGSAccountID, Debit: cogsAmount, Credit: decimal.Zero, Memo: "COGS (bundle): " + ec.ComponentItem.Name},
			PostingFragment{AccountID: *ec.ComponentItem.InventoryAccountID, Debit: decimal.Zero, Credit: cogsAmount, Memo: "Inventory out (bundle): " + ec.ComponentItem.Name},
		)
	}

	return frags
}

// ── Bill inventory fragment adjustment ───────────────────────────────────────

// AdjustBillFragmentsForInventory modifies bill posting fragments so that
// inventory items debit the Inventory Asset account instead of the Expense account.
// Non-inventory items are left unchanged. Bundle items on bills are not supported.
func AdjustBillFragmentsForInventory(frags []PostingFragment, bill models.Bill) []PostingFragment {
	invAcctMap := map[uint]uint{}
	for _, l := range bill.Lines {
		if l.ProductService == nil || !l.ProductService.IsStockItem {
			continue
		}
		if l.ExpenseAccountID == nil || l.ProductService.InventoryAccountID == nil {
			continue
		}
		invAcctMap[*l.ExpenseAccountID] = *l.ProductService.InventoryAccountID
	}
	if len(invAcctMap) == 0 {
		return frags
	}
	for i := range frags {
		if frags[i].Debit.IsPositive() {
			if invAcctID, ok := invAcctMap[frags[i].AccountID]; ok {
				frags[i].AccountID = invAcctID
			}
		}
	}
	return frags
}

// ── Pre-flight stock validation (invoice) ────────────────────────────────────

// ValidateStockForInvoice checks that sufficient inventory exists for all
// stock items on the invoice, including bundle component items.
// warehouseID routes the check to a specific warehouse (nil = legacy path).
// Returns per-item outbound cost results and the expanded bundle components.
func ValidateStockForInvoice(db *gorm.DB, companyID uint, lines []models.InvoiceLine, warehouseID *uint) (
	outboundCosts map[uint]*OutboundResult,
	bundleExpansions []ExpandedComponent,
	err error,
) {
	engine, err := ResolveCostingEngineForCompany(db, companyID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve costing engine: %w", err)
	}

	outboundCosts = make(map[uint]*OutboundResult)

	// Aggregate required quantities per item from single stock lines.
	required := map[uint]decimal.Decimal{}
	for _, l := range lines {
		if l.ProductService == nil || !l.ProductService.IsStockItem {
			continue
		}
		required[l.ProductService.ID] = required[l.ProductService.ID].Add(l.Qty)
	}

	// Expand bundle lines and add component requirements.
	bundleExpansions, err = ExpandBundleLinesForInvoice(db, companyID, lines)
	if err != nil {
		return nil, nil, fmt.Errorf("expand bundle lines: %w", err)
	}
	for _, ec := range bundleExpansions {
		required[ec.ComponentItem.ID] = required[ec.ComponentItem.ID].Add(ec.RequiredQty)
	}

	// Validate stock availability for all required items.
	for itemID, needQty := range required {
		req := OutboundRequest{
			CompanyID:    companyID,
			ItemID:       itemID,
			Quantity:     needQty,
			MovementType: models.MovementTypeSale,
			WarehouseID:  warehouseID,
		}
		if warehouseID == nil {
			req.LocationType = models.LocationTypeInternal
		}
		result, err := engine.PreviewOutbound(db, req)
		if err != nil {
			itemName := fmt.Sprintf("#%d", itemID)
			for _, l := range lines {
				if l.ProductService != nil && l.ProductService.ID == itemID {
					itemName = l.ProductService.Name
					break
				}
			}
			// Also check bundle components for name.
			for _, ec := range bundleExpansions {
				if ec.ComponentItem != nil && ec.ComponentItem.ID == itemID {
					itemName = ec.ComponentItem.Name + " (bundle component)"
					break
				}
			}
			return nil, nil, fmt.Errorf("insufficient inventory for %q: %w", itemName, err)
		}
		outboundCosts[itemID] = result
	}

	return outboundCosts, bundleExpansions, nil
}

// ── Transactional movement creators ──────────────────────────────────────────

// CreateSaleMovements records inventory outflows for stock items on a posted
// invoice. Handles both single stock items and bundle component items.
// warehouseID routes movements to a specific warehouse (nil = legacy path).
// Must be called inside the same transaction as the JE creation.
func CreateSaleMovements(tx *gorm.DB, companyID uint, inv models.Invoice, jeID uint,
	outboundCosts map[uint]*OutboundResult, bundleExpansions []ExpandedComponent, warehouseID *uint) error {

	engine, err := ResolveCostingEngineForCompany(tx, companyID)
	if err != nil {
		return fmt.Errorf("resolve costing engine: %w", err)
	}

	// 1. Single stock items.
	for _, l := range inv.Lines {
		if l.ProductService == nil || !l.ProductService.IsStockItem {
			continue
		}
		if err := createSaleMovement(tx, engine, companyID, inv, jeID, l.ProductService.ID, l.Qty, warehouseID); err != nil {
			return err
		}
	}

	// 2. Bundle component items.
	for _, ec := range bundleExpansions {
		if err := createSaleMovement(tx, engine, companyID, inv, jeID, ec.ComponentItem.ID, ec.RequiredQty, warehouseID); err != nil {
			return err
		}
	}

	return nil
}

func createSaleMovement(tx *gorm.DB, engine CostingEngine, companyID uint,
	inv models.Invoice, jeID, itemID uint, qty decimal.Decimal, warehouseID *uint) error {

	req := OutboundRequest{
		CompanyID:    companyID,
		ItemID:       itemID,
		Quantity:     qty,
		MovementType: models.MovementTypeSale,
		WarehouseID:  warehouseID,
		Date:         inv.InvoiceDate,
	}
	if warehouseID == nil {
		req.LocationType = models.LocationTypeInternal
	}
	result, err := engine.ApplyOutbound(tx, req)
	if err != nil {
		return fmt.Errorf("apply outbound for item %d: %w", itemID, err)
	}

	sourceID := inv.ID
	mov := models.InventoryMovement{
		CompanyID:      companyID,
		ItemID:         itemID,
		MovementType:   models.MovementTypeSale,
		QuantityDelta:  qty.Neg(),
		UnitCost:       &result.UnitCostUsed,
		TotalCost:      &result.TotalCost,
		SourceType:     "invoice",
		SourceID:       &sourceID,
		JournalEntryID: &jeID,
		ReferenceNote:  "Sale: " + inv.InvoiceNumber,
		MovementDate:   inv.InvoiceDate,
		WarehouseID:    warehouseID,
	}
	return tx.Create(&mov).Error
}

// CreatePurchaseMovements records inventory inflows for stock items on a posted
// bill. Bundle items on bills are not expanded (bundles are sales-only).
// warehouseID routes movements to a specific warehouse (nil = legacy path).
// Must be called inside the same transaction as the JE creation.
func CreatePurchaseMovements(tx *gorm.DB, companyID uint, bill models.Bill, jeID uint, warehouseID *uint) error {
	engine, err := ResolveCostingEngineForCompany(tx, companyID)
	if err != nil {
		return fmt.Errorf("resolve costing engine: %w", err)
	}

	for _, l := range bill.Lines {
		if l.ProductService == nil || !l.ProductService.IsStockItem {
			continue
		}

		req := InboundRequest{
			CompanyID:    companyID,
			ItemID:       l.ProductService.ID,
			Quantity:     l.Qty,
			UnitCost:     l.UnitPrice,
			MovementType: models.MovementTypePurchase,
			WarehouseID:  warehouseID,
			Date:         bill.BillDate,
		}
		if warehouseID == nil {
			req.LocationType = models.LocationTypeInternal
		}
		result, err := engine.ApplyInbound(tx, req)
		if err != nil {
			return fmt.Errorf("apply inbound for item %d: %w", l.ProductService.ID, err)
		}

		sourceID := bill.ID
		mov := models.InventoryMovement{
			CompanyID:      companyID,
			ItemID:         l.ProductService.ID,
			MovementType:   models.MovementTypePurchase,
			QuantityDelta:  l.Qty,
			UnitCost:       &result.UnitCostUsed,
			TotalCost:      &result.TotalCost,
			SourceType:     "bill",
			SourceID:       &sourceID,
			JournalEntryID: &jeID,
			ReferenceNote:  "Purchase: " + bill.BillNumber,
			MovementDate:   bill.BillDate,
			WarehouseID:    warehouseID,
		}
		if err := tx.Create(&mov).Error; err != nil {
			return fmt.Errorf("create purchase movement for item %d: %w", l.ProductService.ID, err)
		}
	}
	return nil
}
