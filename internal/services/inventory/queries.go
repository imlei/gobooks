// 遵循project_guide.md
package inventory

// queries.go — OUT contract (read-only queries). Phase D.0 slice 7.
//
// These functions are the public read side of the inventory bounded
// context. They never write; every mutation goes through the IN verbs
// in receive.go / issue.go / adjust.go / transfer.go / reverse.go.
//
// Phase D.0 implements the five queries that operate on data already
// captured by slices 1-6 (OnHand, Movements, ItemLedger, Valuation,
// CostingPreview). ExplodeBOM and GetAvailableForBuild are declared
// here for API stability but return an explicit "not yet implemented"
// error until Phase D.1 adds the product_components table.

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

// ErrNotImplemented surfaces the BOM-dependent OUT queries before
// Phase D.1 wires the product_components table in.
var ErrNotImplemented = errors.New("inventory: query not implemented until Phase D.1 BOM work")

// ── GetOnHand ────────────────────────────────────────────────────────────────

// GetOnHand returns one row per (item, warehouse) matching the query. When
// AsOfDate is nil the cached InventoryBalance rows are returned verbatim
// — the common case and the fastest. When AsOfDate is set the historical
// quantity is reconstructed from the movement ledger (expensive; avoid in
// hot paths) with the caveat that AverageCost is not re-derived and stays
// blank on historical rows. Full historical valuation is a Phase E task.
func GetOnHand(db *gorm.DB, q OnHandQuery) ([]OnHandRow, error) {
	if q.CompanyID == 0 {
		return nil, fmt.Errorf("inventory.GetOnHand: CompanyID required")
	}

	if q.AsOfDate != nil {
		return getOnHandHistorical(db, q)
	}

	dbq := db.Model(&models.InventoryBalance{}).Where("company_id = ?", q.CompanyID)
	if q.ItemID != 0 {
		dbq = dbq.Where("item_id = ?", q.ItemID)
	}
	if q.WarehouseID != 0 {
		dbq = dbq.Where("warehouse_id = ?", q.WarehouseID)
	}
	var balances []models.InventoryBalance
	if err := dbq.Find(&balances).Error; err != nil {
		return nil, fmt.Errorf("inventory.GetOnHand: %w", err)
	}

	rows := make([]OnHandRow, 0, len(balances))
	for _, bal := range balances {
		if !q.IncludeZero && bal.QuantityOnHand.IsZero() {
			continue
		}
		whID := uint(0)
		if bal.WarehouseID != nil {
			whID = *bal.WarehouseID
		}
		reserved := decimal.Zero // Phase E surfaces reserved_quantity
		rows = append(rows, OnHandRow{
			ItemID:            bal.ItemID,
			WarehouseID:       whID,
			QuantityOnHand:    bal.QuantityOnHand,
			QuantityReserved:  reserved,
			QuantityAvailable: bal.QuantityOnHand.Sub(reserved),
			AverageCostBase:   bal.AverageCost,
			TotalValueBase:    bal.QuantityOnHand.Mul(bal.AverageCost).RoundBank(2),
		})
	}
	return rows, nil
}

// getOnHandHistorical reconstructs on-hand quantity at AsOfDate by summing
// quantity_delta for movements up to and including that date. Average cost
// is not replayed — the cheapest correct thing to do is leave it at zero
// and flag the limitation in the doc comment of GetOnHand above.
func getOnHandHistorical(db *gorm.DB, q OnHandQuery) ([]OnHandRow, error) {
	type historicalRow struct {
		ItemID      uint
		WarehouseID *uint
		QtyDelta    decimal.Decimal
	}
	var rows []historicalRow

	dbq := db.Model(&models.InventoryMovement{}).
		Select("item_id, warehouse_id, COALESCE(SUM(quantity_delta), 0) AS qty_delta").
		Where("company_id = ? AND movement_date <= ?", q.CompanyID, *q.AsOfDate).
		Group("item_id, warehouse_id")
	if q.ItemID != 0 {
		dbq = dbq.Where("item_id = ?", q.ItemID)
	}
	if q.WarehouseID != 0 {
		dbq = dbq.Where("warehouse_id = ?", q.WarehouseID)
	}
	if err := dbq.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("inventory.GetOnHand historical: %w", err)
	}

	result := make([]OnHandRow, 0, len(rows))
	for _, r := range rows {
		if !q.IncludeZero && r.QtyDelta.IsZero() {
			continue
		}
		whID := uint(0)
		if r.WarehouseID != nil {
			whID = *r.WarehouseID
		}
		result = append(result, OnHandRow{
			ItemID:         r.ItemID,
			WarehouseID:    whID,
			QuantityOnHand: r.QtyDelta,
			// Historical cost reconstruction is Phase E work; leave blank.
		})
	}
	return result, nil
}

// ── GetMovements ─────────────────────────────────────────────────────────────

// GetMovements returns the movement ledger with a full filter set and
// optional running balance. Total count is returned alongside the page
// so callers can paginate without a separate COUNT query.
func GetMovements(db *gorm.DB, q MovementQuery) ([]MovementRow, int64, error) {
	if q.CompanyID == 0 {
		return nil, 0, fmt.Errorf("inventory.GetMovements: CompanyID required")
	}

	dbq := db.Model(&models.InventoryMovement{}).Where("company_id = ?", q.CompanyID)
	if q.ItemID != nil {
		dbq = dbq.Where("item_id = ?", *q.ItemID)
	}
	if q.WarehouseID != nil {
		dbq = dbq.Where("warehouse_id = ?", *q.WarehouseID)
	}
	if q.FromDate != nil {
		dbq = dbq.Where("movement_date >= ?", *q.FromDate)
	}
	if q.ToDate != nil {
		dbq = dbq.Where("movement_date <= ?", *q.ToDate)
	}
	if q.SourceType != "" {
		dbq = dbq.Where("source_type = ?", q.SourceType)
	}
	if q.SourceID != nil {
		dbq = dbq.Where("source_id = ?", *q.SourceID)
	}
	if q.Direction != nil {
		switch *q.Direction {
		case MovementDirectionIn:
			dbq = dbq.Where("quantity_delta > 0")
		case MovementDirectionOut:
			dbq = dbq.Where("quantity_delta < 0")
		}
	}

	var total int64
	if err := dbq.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("inventory.GetMovements count: %w", err)
	}

	pageQ := dbq.Order("movement_date asc, id asc")
	if q.Limit > 0 {
		pageQ = pageQ.Limit(q.Limit)
	}
	if q.Offset > 0 {
		pageQ = pageQ.Offset(q.Offset)
	}
	var movs []models.InventoryMovement
	if err := pageQ.Find(&movs).Error; err != nil {
		return nil, 0, fmt.Errorf("inventory.GetMovements: %w", err)
	}

	rows := make([]MovementRow, 0, len(movs))
	var runQty, runValue decimal.Decimal
	for _, m := range movs {
		unitCost := decimal.Zero
		if m.UnitCostBase != nil {
			unitCost = *m.UnitCostBase
		} else if m.UnitCost != nil {
			unitCost = *m.UnitCost
		}
		totalCost := m.QuantityDelta.Mul(unitCost).RoundBank(2)

		row := MovementRow{
			ID:            m.ID,
			MovementDate:  m.MovementDate,
			ItemID:        m.ItemID,
			WarehouseID:   m.WarehouseID,
			MovementType:  string(m.MovementType),
			QuantityDelta: m.QuantityDelta,
			UnitCostBase:  unitCost,
			TotalCostBase: totalCost,
			SourceType:    m.SourceType,
			SourceID:      m.SourceID,
			SourceLineID:  m.SourceLineID,
			Memo:          m.Memo,
			ActorUserID:   m.ActorUserID,
			CreatedAt:     m.CreatedAt,
		}
		if q.IncludeRunningBalance {
			runQty = runQty.Add(m.QuantityDelta)
			runValue = runValue.Add(totalCost)
			row.RunningQuantity = runQty
			row.RunningValueBase = runValue
		}
		rows = append(rows, row)
	}
	return rows, total, nil
}

// ── GetItemLedger ────────────────────────────────────────────────────────────

// GetItemLedger returns the per-item ledger for a date range: opening
// balance before FromDate, every movement within the window, closing
// balance as of ToDate, and the period totals useful for reconciliation.
func GetItemLedger(db *gorm.DB, q ItemLedgerQuery) (*ItemLedgerReport, error) {
	if q.CompanyID == 0 || q.ItemID == 0 {
		return nil, fmt.Errorf("inventory.GetItemLedger: CompanyID and ItemID required")
	}

	// Opening: sum of movements strictly before FromDate.
	openingQty, err := sumDeltas(db, q.CompanyID, q.ItemID, q.WarehouseID, nil, dayBefore(q.FromDate))
	if err != nil {
		return nil, err
	}
	openingValue := decimal.Zero // full historical valuation = Phase E

	// Closing: sum of movements up to and including ToDate.
	closingQty, err := sumDeltas(db, q.CompanyID, q.ItemID, q.WarehouseID, nil, ptrTime(q.ToDate))
	if err != nil {
		return nil, err
	}

	// Current balance to pick up the authoritative AverageCost when ToDate
	// is today (or later). For historical closing we leave it zero.
	closingUnitCost := decimal.Zero
	closingValue := decimal.Zero
	if isRecent(q.ToDate) {
		balQ := db.Model(&models.InventoryBalance{}).Where("company_id = ? AND item_id = ?", q.CompanyID, q.ItemID)
		if q.WarehouseID != nil {
			balQ = balQ.Where("warehouse_id = ?", *q.WarehouseID)
		}
		var balances []models.InventoryBalance
		balQ.Find(&balances)
		var totalQty, totalValue decimal.Decimal
		for _, b := range balances {
			totalQty = totalQty.Add(b.QuantityOnHand)
			totalValue = totalValue.Add(b.QuantityOnHand.Mul(b.AverageCost))
		}
		if totalQty.IsPositive() {
			closingUnitCost = totalValue.Div(totalQty).Round(4)
			closingValue = totalValue.RoundBank(2)
		}
	}

	// Period movements.
	movementRows, _, err := GetMovements(db, MovementQuery{
		CompanyID:             q.CompanyID,
		ItemID:                &q.ItemID,
		WarehouseID:           q.WarehouseID,
		FromDate:              &q.FromDate,
		ToDate:                &q.ToDate,
		IncludeRunningBalance: true,
	})
	if err != nil {
		return nil, err
	}

	// Totals over the period.
	var inQty, inValue, outQty, outCost decimal.Decimal
	for _, m := range movementRows {
		switch {
		case m.QuantityDelta.IsPositive():
			inQty = inQty.Add(m.QuantityDelta)
			inValue = inValue.Add(m.TotalCostBase)
		case m.QuantityDelta.IsNegative():
			outQty = outQty.Add(m.QuantityDelta.Abs())
			outCost = outCost.Add(m.TotalCostBase.Abs())
		}
	}

	return &ItemLedgerReport{
		ItemID:           q.ItemID,
		WarehouseID:      q.WarehouseID,
		OpeningQuantity:  openingQty,
		OpeningValueBase: openingValue,
		OpeningUnitCost:  decimal.Zero, // Phase E: historical avg reconstruction
		Movements:        movementRows,
		ClosingQuantity:  closingQty,
		ClosingValueBase: closingValue,
		ClosingUnitCost:  closingUnitCost,
		TotalInQty:       inQty,
		TotalInValue:     inValue,
		TotalOutQty:      outQty,
		TotalOutCostBase: outCost,
	}, nil
}

// ── GetValuationSnapshot ─────────────────────────────────────────────────────

// GetValuationSnapshot totals inventory value across items / warehouses.
// Phase D.0 supports current valuation only (ignores AsOfDate); point-in-
// time historical valuation is a Phase E task.
func GetValuationSnapshot(db *gorm.DB, q ValuationQuery) ([]ValuationRow, decimal.Decimal, error) {
	if q.CompanyID == 0 {
		return nil, decimal.Zero, fmt.Errorf("inventory.GetValuationSnapshot: CompanyID required")
	}

	dbq := db.Model(&models.InventoryBalance{}).
		Where("company_id = ? AND quantity_on_hand <> 0", q.CompanyID)
	if q.WarehouseID != nil {
		dbq = dbq.Where("warehouse_id = ?", *q.WarehouseID)
	}
	var balances []models.InventoryBalance
	if err := dbq.Find(&balances).Error; err != nil {
		return nil, decimal.Zero, fmt.Errorf("inventory.GetValuationSnapshot: %w", err)
	}

	// Aggregate. Each balance contributes qty × avg; group key per GroupBy.
	groups := map[string]*ValuationRow{}
	var grandTotal decimal.Decimal
	for _, b := range balances {
		value := b.QuantityOnHand.Mul(b.AverageCost).RoundBank(2)
		grandTotal = grandTotal.Add(value)

		key := ""
		switch q.GroupBy {
		case ValuationGroupByItem:
			key = fmt.Sprintf("item:%d", b.ItemID)
		case ValuationGroupByWarehouse:
			if b.WarehouseID != nil {
				key = fmt.Sprintf("warehouse:%d", *b.WarehouseID)
			} else {
				key = "warehouse:unassigned"
			}
		case ValuationGroupByCategory:
			// Category grouping needs a join to product_services.category
			// which isn't modeled cleanly today. Collapse to a single
			// "uncategorised" bucket until that's first-class.
			key = "category:uncategorised"
		default:
			key = "total"
		}

		if g, ok := groups[key]; ok {
			g.Quantity = g.Quantity.Add(b.QuantityOnHand)
			g.ValueBase = g.ValueBase.Add(value)
		} else {
			groups[key] = &ValuationRow{
				GroupKey:   key,
				GroupLabel: key,
				Quantity:   b.QuantityOnHand,
				ValueBase:  value,
			}
		}
	}

	rows := make([]ValuationRow, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, *g)
	}
	return rows, grandTotal.RoundBank(2), nil
}

// ── GetCostingPreview ────────────────────────────────────────────────────────

// GetCostingPreview reports the cost a hypothetical IssueStock would book,
// without touching state. Used for quotations and margin previews.
func GetCostingPreview(db *gorm.DB, q CostingPreviewQuery) (*CostingPreviewResult, error) {
	if q.CompanyID == 0 || q.ItemID == 0 || q.WarehouseID == 0 {
		return nil, fmt.Errorf("inventory.GetCostingPreview: CompanyID, ItemID, WarehouseID required")
	}
	if !q.Quantity.IsPositive() {
		return nil, ErrNegativeQuantity
	}

	var bal models.InventoryBalance
	err := db.Where("company_id = ? AND item_id = ? AND warehouse_id = ?",
		q.CompanyID, q.ItemID, q.WarehouseID).First(&bal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No balance row yet — the item is effectively zero at this warehouse.
		return &CostingPreviewResult{
			UnitCostBase:  decimal.Zero,
			TotalCostBase: decimal.Zero,
			Feasible:      false,
			ShortBy:       q.Quantity,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inventory.GetCostingPreview: %w", err)
	}

	feasible := bal.QuantityOnHand.GreaterThanOrEqual(q.Quantity)
	short := decimal.Zero
	if !feasible {
		short = q.Quantity.Sub(bal.QuantityOnHand)
	}
	return &CostingPreviewResult{
		UnitCostBase:  bal.AverageCost,
		TotalCostBase: q.Quantity.Mul(bal.AverageCost).RoundBank(2),
		Feasible:      feasible,
		ShortBy:       short,
	}, nil
}

// ── ExplodeBOM ───────────────────────────────────────────────────────────────

// bomExplodeMaxDepth caps recursion to guard against pathological BOMs and
// any undetected cycles (belt + braces alongside the visited-set check).
// Five levels comfortably covers "sub-assembly → component" chains; anything
// deeper is a design smell worth surfacing as an error.
const bomExplodeMaxDepth = 5

// ExplodeBOM recursively expands a parent product into its components using
// the item_components table. MultiLevel=false returns only direct children;
// MultiLevel=true recurses until every row is a leaf (no further
// item_components rows). Cycles are blocked with a visited-path set;
// exceeding bomExplodeMaxDepth returns ErrBOMTooDeep.
//
// Optional enrichments:
//   - IncludeCostEstimate populates EstimatedUnitCostBase /
//     EstimatedTotalCostBase from the component's current weighted-average
//     cost at whichever warehouse is keyed (or zero when WarehouseID nil).
//   - IncludeAvailability populates AvailableQuantity and ShortBy against
//     the target warehouse. Requires WarehouseID non-nil.
func ExplodeBOM(db *gorm.DB, q BOMExplodeQuery) ([]BOMExplodeRow, error) {
	if q.CompanyID == 0 || q.ParentItemID == 0 {
		return nil, fmt.Errorf("inventory.ExplodeBOM: CompanyID and ParentItemID required")
	}
	if !q.Quantity.IsPositive() {
		return nil, ErrNegativeQuantity
	}
	if q.IncludeAvailability && q.WarehouseID == nil {
		return nil, fmt.Errorf("inventory.ExplodeBOM: WarehouseID required when IncludeAvailability=true")
	}

	rows := make([]BOMExplodeRow, 0, 4)
	visited := map[uint]bool{q.ParentItemID: true}
	path := []uint{q.ParentItemID}

	if err := explodeWalk(db, q, q.ParentItemID, q.Quantity, 0, visited, path, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// explodeWalk is the recursive step. It reads item_components rows for the
// current parent and (depending on MultiLevel) either appends each component
// as a leaf or recurses into it. visited/path are carried by value-ish:
// visited is mutated on entry/exit to allow sibling branches to re-visit
// products that appear in multiple sub-trees (only pure cycles are blocked).
func explodeWalk(db *gorm.DB, q BOMExplodeQuery,
	parentID uint, parentQty decimal.Decimal, depth int,
	visited map[uint]bool, path []uint,
	out *[]BOMExplodeRow,
) error {
	if depth >= bomExplodeMaxDepth {
		return ErrBOMTooDeep
	}

	var comps []models.ItemComponent
	err := db.Preload("ComponentItem").
		Where("company_id = ? AND parent_item_id = ?", q.CompanyID, parentID).
		Order("sort_order asc, id asc").
		Find(&comps).Error
	if err != nil {
		return fmt.Errorf("inventory.ExplodeBOM: load components: %w", err)
	}

	for _, c := range comps {
		if visited[c.ComponentItemID] {
			return fmt.Errorf("%w: %d -> %d", ErrBOMCycle, parentID, c.ComponentItemID)
		}

		// Scrap is not yet on ItemComponent (it lives on the future
		// product_components schema we no longer need — item_components is
		// already the canonical table). Treat scrap as zero; when a scrap
		// column is added to item_components this is the one place to read
		// it.
		scrap := decimal.Zero
		effectiveQty := parentQty.Mul(c.Quantity)

		childPath := append(append([]uint(nil), path...), c.ComponentItemID)
		row := BOMExplodeRow{
			ComponentItemID: c.ComponentItemID,
			Depth:           depth,
			Path:            childPath,
			QuantityPerUnit: c.Quantity,
			TotalQuantity:   effectiveQty,
			ScrapPct:        scrap,
		}

		// Only recurse into this component when it's itself a parent with
		// rows in item_components AND MultiLevel is enabled AND we haven't
		// exhausted depth. Otherwise treat as a leaf and emit the row.
		recurse := false
		if q.MultiLevel {
			var childCount int64
			db.Model(&models.ItemComponent{}).
				Where("company_id = ? AND parent_item_id = ?", q.CompanyID, c.ComponentItemID).
				Count(&childCount)
			recurse = childCount > 0
		}

		if recurse {
			// Guard cycle detection: visit on descent, un-visit on return,
			// so a component appearing in two sibling sub-trees doesn't
			// spuriously trip as a cycle.
			visited[c.ComponentItemID] = true
			if err := explodeWalk(db, q, c.ComponentItemID, effectiveQty, depth+1, visited, childPath, out); err != nil {
				return err
			}
			delete(visited, c.ComponentItemID)
			continue
		}

		// Enrichments for leaf rows.
		if q.IncludeCostEstimate || q.IncludeAvailability {
			bal, err := lookupComponentBalance(db, q.CompanyID, c.ComponentItemID, q.WarehouseID)
			if err != nil {
				return err
			}
			if q.IncludeCostEstimate {
				unit := bal.AverageCost
				total := effectiveQty.Mul(unit).RoundBank(2)
				row.EstimatedUnitCostBase = &unit
				row.EstimatedTotalCostBase = &total
			}
			if q.IncludeAvailability {
				avail := bal.QuantityOnHand
				row.AvailableQuantity = &avail
				if avail.LessThan(effectiveQty) {
					short := effectiveQty.Sub(avail)
					row.ShortBy = &short
				}
			}
		}

		*out = append(*out, row)
	}
	return nil
}

// lookupComponentBalance reads the balance for (company, item, warehouse).
// When warehouseID is nil the balance is summed across warehouses so cost
// estimates reflect blended company-level avg.
func lookupComponentBalance(db *gorm.DB, companyID, itemID uint, warehouseID *uint) (models.InventoryBalance, error) {
	if warehouseID != nil {
		var bal models.InventoryBalance
		err := db.Where("company_id = ? AND item_id = ? AND warehouse_id = ?",
			companyID, itemID, *warehouseID).First(&bal).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.InventoryBalance{}, nil
		}
		return bal, err
	}

	// Aggregate across warehouses.
	var balances []models.InventoryBalance
	if err := db.Where("company_id = ? AND item_id = ?", companyID, itemID).
		Find(&balances).Error; err != nil {
		return models.InventoryBalance{}, err
	}
	var totalQty, totalValue decimal.Decimal
	for _, b := range balances {
		totalQty = totalQty.Add(b.QuantityOnHand)
		totalValue = totalValue.Add(b.QuantityOnHand.Mul(b.AverageCost))
	}
	agg := models.InventoryBalance{CompanyID: companyID, ItemID: itemID, QuantityOnHand: totalQty}
	if totalQty.IsPositive() {
		agg.AverageCost = totalValue.Div(totalQty).Round(4)
	}
	return agg, nil
}

// ── GetAvailableForBuild ─────────────────────────────────────────────────────

// GetAvailableForBuild reports the maximum quantity of parent_item that can
// be built at warehouseID given current component stock. Returns the
// bottleneck component — the one that caps the answer. Suitable for the
// "build" UI to show "you can assemble N units; you'd need M more of
// component X to reach the quantity you want".
func GetAvailableForBuild(db *gorm.DB, companyID, parentItemID, warehouseID uint) (decimal.Decimal, uint, error) {
	if companyID == 0 || parentItemID == 0 {
		return decimal.Zero, 0, fmt.Errorf("inventory.GetAvailableForBuild: CompanyID and ParentItemID required")
	}

	var comps []models.ItemComponent
	if err := db.
		Where("company_id = ? AND parent_item_id = ?", companyID, parentItemID).
		Find(&comps).Error; err != nil {
		return decimal.Zero, 0, fmt.Errorf("inventory.GetAvailableForBuild: load components: %w", err)
	}
	if len(comps) == 0 {
		return decimal.Zero, 0, fmt.Errorf("inventory.GetAvailableForBuild: parent %d has no components", parentItemID)
	}

	var whPtr *uint
	if warehouseID != 0 {
		id := warehouseID
		whPtr = &id
	}

	var (
		maxBuildable     decimal.Decimal
		bottleneckItemID uint
		first            = true
	)
	for _, c := range comps {
		bal, err := lookupComponentBalance(db, companyID, c.ComponentItemID, whPtr)
		if err != nil {
			return decimal.Zero, 0, err
		}
		// Maximum parent units this component can support = on_hand / per_unit.
		// Guard divide-by-zero on a misconfigured component row.
		if !c.Quantity.IsPositive() {
			continue
		}
		possible := bal.QuantityOnHand.Div(c.Quantity).Floor()
		if first || possible.LessThan(maxBuildable) {
			maxBuildable = possible
			bottleneckItemID = c.ComponentItemID
			first = false
		}
	}
	if first {
		return decimal.Zero, 0, fmt.Errorf("inventory.GetAvailableForBuild: no valid components with positive per-unit quantity")
	}
	if maxBuildable.IsNegative() {
		maxBuildable = decimal.Zero
	}
	return maxBuildable, bottleneckItemID, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// sumDeltas returns the signed sum of QuantityDelta for (company, item) over
// the given warehouse and date range. FromDate inclusive; ToDate inclusive.
// Either bound can be nil to leave that side unbounded.
func sumDeltas(db *gorm.DB, companyID, itemID uint, warehouseID *uint, fromDate, toDate *time.Time) (decimal.Decimal, error) {
	q := db.Model(&models.InventoryMovement{}).
		Where("company_id = ? AND item_id = ?", companyID, itemID)
	if warehouseID != nil {
		q = q.Where("warehouse_id = ?", *warehouseID)
	}
	if fromDate != nil {
		q = q.Where("movement_date >= ?", *fromDate)
	}
	if toDate != nil {
		q = q.Where("movement_date <= ?", *toDate)
	}
	var sum decimal.Decimal
	type row struct {
		Total decimal.Decimal
	}
	var r row
	if err := q.Select("COALESCE(SUM(quantity_delta), 0) AS total").Scan(&r).Error; err != nil {
		return decimal.Zero, fmt.Errorf("inventory: sum deltas: %w", err)
	}
	sum = r.Total
	return sum, nil
}

func ptrTime(t time.Time) *time.Time { return &t }
func dayBefore(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	d := t.Add(-1 * 24 * time.Hour)
	return &d
}
func isRecent(t time.Time) bool {
	return t.IsZero() || !t.Before(time.Now().Add(-24*time.Hour))
}
