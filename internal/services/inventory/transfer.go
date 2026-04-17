// 遵循project_guide.md
package inventory

// transfer.go — TransferStock (warehouse-to-warehouse) — fourth IN verb,
// Phase D.0 slice 6.
//
// Transfers are cost-neutral: the unit cost snapshotted off the source
// warehouse's weighted average travels with the units to the destination,
// unchanged. The same UnitCostBase appears on the IssueStock leg and the
// ReceiveStock leg so the company-wide valuation is preserved.
//
// Two-phase support: when ReceivedDate is nil the transfer is in transit —
// only the issue leg runs. A later call with both dates completes the
// receive half. The idempotency key scheme lets callers run each leg
// exactly once even across retries.

import (
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TransferStock books a warehouse transfer as a coordinated Issue+Receive
// pair. See INVENTORY_MODULE_API.md §3.4.
func TransferStock(db *gorm.DB, in TransferStockInput) (*TransferStockResult, error) {
	if err := validateTransferInput(in); err != nil {
		return nil, err
	}
	if in.FromWarehouseID == in.ToWarehouseID {
		return nil, fmt.Errorf("inventory.TransferStock: FromWarehouseID and ToWarehouseID must differ")
	}

	// Leg-specific idempotency keys derived from the caller's transfer-level
	// key. Each leg's sub-call does its own cached-result lookup, so retry-
	// after-partial-completion is safe: if the issue leg already ran the
	// cached result returns instantly and we proceed to the receive leg.
	issueKey := ""
	receiveKey := ""
	if in.IdempotencyKey != "" {
		issueKey = in.IdempotencyKey + ":out"
		receiveKey = in.IdempotencyKey + ":in"
	}

	// Common memo so both legs share a story in audit reports.
	memo := in.Memo
	if memo == "" {
		memo = fmt.Sprintf("Warehouse transfer #%d: WH%d → WH%d",
			in.TransferID, in.FromWarehouseID, in.ToWarehouseID)
	}

	// Leg 1 — Issue from source. The returned UnitCostBase is the cost that
	// travels with the units; never recalculated on the destination side.
	issue, err := IssueStock(db, IssueStockInput{
		CompanyID:      in.CompanyID,
		ItemID:         in.ItemID,
		WarehouseID:    in.FromWarehouseID,
		Quantity:       in.Quantity,
		MovementDate:   in.ShippedDate,
		SourceType:     "transfer_out",
		SourceID:       in.TransferID,
		IdempotencyKey: issueKey,
		ActorUserID:    in.ActorUserID,
		Memo:           memo,
	})
	if err != nil {
		return nil, fmt.Errorf("inventory.TransferStock: issue leg: %w", err)
	}

	result := &TransferStockResult{
		IssueMovementID:  issue.MovementID,
		UnitCostBase:     issue.UnitCostBase,
		TransitValueBase: issue.CostOfIssueBase,
	}

	// Leg 2 — Receive at destination using the SAME unit cost. Skipped when
	// the transfer is still in transit (ReceivedDate == nil).
	if in.ReceivedDate == nil {
		return result, nil
	}

	receive, err := ReceiveStock(db, ReceiveStockInput{
		CompanyID:    in.CompanyID,
		ItemID:       in.ItemID,
		WarehouseID:  in.ToWarehouseID,
		Quantity:     in.Quantity,
		MovementDate: *in.ReceivedDate,
		UnitCost:     issue.UnitCostBase, // snapshot from source; no currency conversion
		ExchangeRate: decimal.NewFromInt(1),
		SourceType:   "transfer_in",
		SourceID:     in.TransferID,
		IdempotencyKey: receiveKey,
		ActorUserID:    in.ActorUserID,
		Memo:           memo,
	})
	if err != nil {
		return nil, fmt.Errorf("inventory.TransferStock: receive leg: %w", err)
	}
	result.ReceiveMovementID = &receive.MovementID
	return result, nil
}

func validateTransferInput(in TransferStockInput) error {
	if in.CompanyID == 0 {
		return fmt.Errorf("inventory.TransferStock: CompanyID required")
	}
	if in.ItemID == 0 {
		return fmt.Errorf("inventory.TransferStock: ItemID required")
	}
	if in.FromWarehouseID == 0 || in.ToWarehouseID == 0 {
		return fmt.Errorf("inventory.TransferStock: both warehouse IDs required")
	}
	if !in.Quantity.IsPositive() {
		return ErrNegativeQuantity
	}
	if in.ShippedDate.IsZero() {
		return fmt.Errorf("inventory.TransferStock: ShippedDate required")
	}
	if in.ReceivedDate != nil && in.ReceivedDate.Before(in.ShippedDate) {
		return fmt.Errorf("inventory.TransferStock: ReceivedDate cannot precede ShippedDate")
	}
	return nil
}
