// 遵循project_guide.md
package services

// shipment_service_test.go — Phase I slice I.2 contract tests.
//
// Locks three things:
//  1. Document-layer CRUD works: create with lines, read back, update
//     draft, list by company, delete draft, refuse post-state edits.
//  2. Status machine: draft → posted → voided. Non-path transitions
//     (e.g. posting a voided shipment, voiding a draft) are refused.
//  3. **Scope boundary for I.2**: Post and Void have zero side effects
//     on inventory (no movements, no cost layers, no balances, no
//     lots, no serial units) and no side effects on GL (no journal
//     entries, no journal lines). This prevents accidental I.3 slip
//     — the moment anyone adds an IssueStockFromShipment call inside
//     PostShipment, these assertions break in CI.
//  4. **Rail dormancy for I.2**: flipping companies.shipment_required
//     to TRUE does not change PostShipment / VoidShipment behavior.
//     Both paths remain byte-identical in I.2. The consumer that
//     makes shipment_required=true meaningful lands in I.3.

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gobooks/internal/models"
)

// testShipmentDocDB spins an in-memory DB with the full Phase I.2
// footprint plus the inventory / GL tables used by the boundary
// checks. The inventory tables are NOT expected to be written by
// I.2 code paths — they are present so that "assert zero rows"
// checks can actually run.
func testShipmentDocDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ship_doc_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Company{},
		&models.Warehouse{},
		&models.Customer{},
		&models.ProductService{},
		&models.Account{},
		&models.SalesOrder{},
		&models.SalesOrderLine{},
		&models.Shipment{},
		&models.ShipmentLine{},
		&models.InventoryMovement{},
		&models.InventoryBalance{},
		&models.InventoryCostLayer{},
		&models.InventoryLot{},
		&models.InventorySerialUnit{},
		&models.InventoryLayerConsumption{},
		&models.InventoryTrackingConsumption{},
		&models.JournalEntry{},
		&models.JournalLine{},
		&models.LedgerEntry{},
		&models.AuditLog{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

type shipmentFixture struct {
	CompanyID   uint
	WarehouseID uint
	CustomerID  uint
	ItemID      uint
}

func seedShipmentFixture(t *testing.T, db *gorm.DB) shipmentFixture {
	t.Helper()
	co := models.Company{Name: "ship-doc-co", IsActive: true}
	if err := db.Create(&co).Error; err != nil {
		t.Fatalf("seed company: %v", err)
	}
	wh := models.Warehouse{CompanyID: co.ID, Name: "Main", Code: "MAIN", IsActive: true}
	if err := db.Create(&wh).Error; err != nil {
		t.Fatalf("seed warehouse: %v", err)
	}
	c := models.Customer{CompanyID: co.ID, Name: "Acme Buyer", IsActive: true}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	rev := models.Account{
		CompanyID:         co.ID,
		Code:              "4000",
		Name:              "Revenue",
		RootAccountType:   models.RootRevenue,
		DetailAccountType: "sales_revenue",
		IsActive:          true,
	}
	if err := db.Create(&rev).Error; err != nil {
		t.Fatalf("seed revenue account: %v", err)
	}
	item := models.ProductService{
		CompanyID:        co.ID,
		Name:             "Widget",
		Type:             models.ProductServiceTypeInventory,
		RevenueAccountID: rev.ID,
		IsActive:         true,
	}
	item.ApplyTypeDefaults()
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("seed item: %v", err)
	}
	return shipmentFixture{
		CompanyID:   co.ID,
		WarehouseID: wh.ID,
		CustomerID:  c.ID,
		ItemID:      item.ID,
	}
}

// assertNoInventoryOrGLEffectForShipment verifies the I.2 boundary:
// no inventory or GL artefact has been written for the given company.
// Sell-side mirror of receipt_service_test.go's
// assertNoInventoryOrGLEffect. Duplicated (not shared) so that if the
// two slices ever diverge in their boundary contract (e.g. I.5
// matching writes a clearing entry that H.5 doesn't), the assertions
// can track independently.
func assertNoInventoryOrGLEffectForShipment(t *testing.T, db *gorm.DB, companyID uint) {
	t.Helper()
	checks := []struct {
		table string
		model any
	}{
		{"inventory_movements", &models.InventoryMovement{}},
		{"inventory_balances", &models.InventoryBalance{}},
		{"inventory_cost_layers", &models.InventoryCostLayer{}},
		{"inventory_lots", &models.InventoryLot{}},
		{"inventory_serial_units", &models.InventorySerialUnit{}},
		{"journal_entries", &models.JournalEntry{}},
		{"journal_lines", &models.JournalLine{}},
	}
	for _, c := range checks {
		var n int64
		if err := db.Model(c.model).
			Where("company_id = ?", companyID).
			Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", c.table, err)
		}
		if n != 0 {
			t.Fatalf("I.2 boundary violated: %s has %d row(s) for company %d; I.2 must not produce inventory or GL artefacts",
				c.table, n, companyID)
		}
	}
}

func buildSimpleShipmentCreateInput(fx shipmentFixture) CreateShipmentInput {
	return CreateShipmentInput{
		CompanyID:      fx.CompanyID,
		ShipmentNumber: "SHIP-001",
		CustomerID:     &fx.CustomerID,
		WarehouseID:    fx.WarehouseID,
		ShipDate:       time.Now().UTC(),
		Memo:           "smoke",
		Reference:      "BOL-9001",
		Lines: []CreateShipmentLineInput{
			{
				SortOrder:        1,
				ProductServiceID: fx.ItemID,
				Description:      "Widget carton",
				Qty:              decimal.NewFromInt(7),
				Unit:             "ea",
			},
		},
		Actor: "admin@test",
	}
}

// ── Create / Read ────────────────────────────────────────────────────────────

func TestCreateShipment_PersistsHeaderAndLines(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)

	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out.ID == 0 {
		t.Fatalf("expected non-zero ID")
	}
	if out.Status != models.ShipmentStatusDraft {
		t.Fatalf("status: got %q want draft", out.Status)
	}
	if len(out.Lines) != 1 {
		t.Fatalf("lines: got %d want 1", len(out.Lines))
	}
	if out.Lines[0].Qty.Cmp(decimal.NewFromInt(7)) != 0 {
		t.Fatalf("line qty: got %s want 7", out.Lines[0].Qty)
	}

	got, err := GetShipment(db, fx.CompanyID, out.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ShipmentNumber != "SHIP-001" || got.Reference != "BOL-9001" {
		t.Fatalf("round-trip fields: %+v", got)
	}
}

func TestCreateShipment_RejectsMissingWarehouse(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	in := buildSimpleShipmentCreateInput(fx)
	in.WarehouseID = 0
	if _, err := CreateShipment(db, in); err != ErrShipmentWarehouseRequired {
		t.Fatalf("got %v want ErrShipmentWarehouseRequired", err)
	}
}

func TestCreateShipment_RejectsMissingDate(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	in := buildSimpleShipmentCreateInput(fx)
	in.ShipDate = time.Time{}
	if _, err := CreateShipment(db, in); err != ErrShipmentDateRequired {
		t.Fatalf("got %v want ErrShipmentDateRequired", err)
	}
}

func TestCreateShipment_RejectsLineWithoutProduct(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	in := buildSimpleShipmentCreateInput(fx)
	in.Lines[0].ProductServiceID = 0
	_, err := CreateShipment(db, in)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !isErr(err, ErrShipmentLineProductRequired) {
		t.Fatalf("got %v want ErrShipmentLineProductRequired", err)
	}
}

func TestGetShipment_CompanyScopedNotFound(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := GetShipment(db, fx.CompanyID+999, out.ID); err != ErrShipmentNotFound {
		t.Fatalf("cross-company get: got %v want ErrShipmentNotFound", err)
	}
}

// ── Cross-company scope guards ───────────────────────────────────────────────

// Proves that referencing a warehouse owned by a DIFFERENT company is
// rejected before any Shipment write. Locks the boundary that I.3 /
// I.5 rely on: no Shipment can reach posted state carrying a cross-
// tenant reference that would later mis-attribute inventory or COGS.
func TestCreateShipment_RejectsCrossCompanyWarehouse(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)

	otherCo := models.Company{Name: "other", IsActive: true}
	if err := db.Create(&otherCo).Error; err != nil {
		t.Fatalf("seed other company: %v", err)
	}
	otherWh := models.Warehouse{CompanyID: otherCo.ID, Name: "other-wh", Code: "OTH", IsActive: true}
	if err := db.Create(&otherWh).Error; err != nil {
		t.Fatalf("seed other warehouse: %v", err)
	}

	in := buildSimpleShipmentCreateInput(fx)
	in.WarehouseID = otherWh.ID
	_, err := CreateShipment(db, in)
	if err == nil {
		t.Fatalf("expected cross-company rejection")
	}
	if !isErr(err, ErrShipmentCrossCompanyReference) {
		t.Fatalf("got %v want ErrShipmentCrossCompanyReference", err)
	}
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestUpdateShipment_DraftSucceeds(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newMemo := "updated memo"
	newRef := "BOL-9002"
	updated, err := UpdateShipment(db, fx.CompanyID, out.ID, UpdateShipmentInput{
		Memo:      &newMemo,
		Reference: &newRef,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Memo != newMemo || updated.Reference != newRef {
		t.Fatalf("update did not apply: %+v", updated)
	}
}

func TestUpdateShipment_RefusedOnPosted(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := PostShipment(db, fx.CompanyID, out.ID, "admin@test", nil); err != nil {
		t.Fatalf("post: %v", err)
	}

	newMemo := "too late"
	_, err = UpdateShipment(db, fx.CompanyID, out.ID, UpdateShipmentInput{Memo: &newMemo})
	if err == nil {
		t.Fatalf("expected error updating posted shipment")
	}
	if !isErr(err, ErrShipmentNotDraft) {
		t.Fatalf("got %v want ErrShipmentNotDraft", err)
	}
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestListShipments_CompanyScoped(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	for i := 0; i < 2; i++ {
		in := buildSimpleShipmentCreateInput(fx)
		in.ShipmentNumber = fmt.Sprintf("S%02d", i)
		if _, err := CreateShipment(db, in); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	otherCo := models.Company{Name: "other", IsActive: true}
	if err := db.Create(&otherCo).Error; err != nil {
		t.Fatalf("seed other company: %v", err)
	}
	otherWh := models.Warehouse{CompanyID: otherCo.ID, Name: "other-wh", Code: "OTH", IsActive: true}
	if err := db.Create(&otherWh).Error; err != nil {
		t.Fatalf("seed other warehouse: %v", err)
	}
	otherItem := models.ProductService{
		CompanyID: otherCo.ID, Name: "Other",
		Type: models.ProductServiceTypeInventory, IsActive: true,
	}
	otherItem.ApplyTypeDefaults()
	if err := db.Create(&otherItem).Error; err != nil {
		t.Fatalf("seed other item: %v", err)
	}
	if _, err := CreateShipment(db, CreateShipmentInput{
		CompanyID:      otherCo.ID,
		ShipmentNumber: "X-1",
		WarehouseID:    otherWh.ID,
		ShipDate:       time.Now().UTC(),
		Lines: []CreateShipmentLineInput{
			{ProductServiceID: otherItem.ID, Qty: decimal.NewFromInt(1)},
		},
	}); err != nil {
		t.Fatalf("create other: %v", err)
	}

	rows, err := ListShipments(db, fx.CompanyID, ListShipmentsFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("list: got %d rows want 2 (expected only fx.CompanyID's shipments)", len(rows))
	}
	for _, r := range rows {
		if r.CompanyID != fx.CompanyID {
			t.Fatalf("cross-tenant leak: row %d belongs to company %d", r.ID, r.CompanyID)
		}
	}
}

// ── Post / Void lifecycle ────────────────────────────────────────────────────

func TestPostShipment_FlipsStatusAndWritesAudit_NoInventoryOrGL(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	posted, err := PostShipment(db, fx.CompanyID, out.ID, "admin@test", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if posted.Status != models.ShipmentStatusPosted {
		t.Fatalf("status: got %q want posted", posted.Status)
	}
	if posted.PostedAt == nil {
		t.Fatalf("PostedAt not set")
	}
	if posted.JournalEntryID != nil {
		t.Fatalf("I.2 post must not link a JE; got %d", *posted.JournalEntryID)
	}

	var logs []models.AuditLog
	db.Where("entity_type = ? AND entity_id = ? AND action = ?",
		"shipment", out.ID, "shipment.posted").Find(&logs)
	if len(logs) != 1 {
		t.Fatalf("audit: got %d rows want 1 (%+v)", len(logs), logs)
	}
	if logs[0].Actor != "admin@test" {
		t.Fatalf("audit actor: got %q want admin@test", logs[0].Actor)
	}

	// Boundary lock — I.2 must not touch inventory or GL.
	assertNoInventoryOrGLEffectForShipment(t, db, fx.CompanyID)
}

func TestPostShipment_RefusedWhenAlreadyPosted(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := PostShipment(db, fx.CompanyID, out.ID, "admin@test", nil); err != nil {
		t.Fatalf("first post: %v", err)
	}
	_, err = PostShipment(db, fx.CompanyID, out.ID, "admin@test", nil)
	if err == nil {
		t.Fatalf("expected error posting twice")
	}
	if !isErr(err, ErrShipmentAlreadyPosted) {
		t.Fatalf("got %v want ErrShipmentAlreadyPosted", err)
	}
}

func TestVoidShipment_FlipsStatusAndWritesAudit_NoInventoryOrGL(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := PostShipment(db, fx.CompanyID, out.ID, "admin@test", nil); err != nil {
		t.Fatalf("post: %v", err)
	}
	voided, err := VoidShipment(db, fx.CompanyID, out.ID, "admin@test", nil)
	if err != nil {
		t.Fatalf("void: %v", err)
	}
	if voided.Status != models.ShipmentStatusVoided {
		t.Fatalf("status: got %q want voided", voided.Status)
	}
	if voided.VoidedAt == nil {
		t.Fatalf("VoidedAt not set")
	}

	var logs []models.AuditLog
	db.Where("entity_type = ? AND entity_id = ? AND action = ?",
		"shipment", out.ID, "shipment.voided").Find(&logs)
	if len(logs) != 1 {
		t.Fatalf("void audit: got %d rows want 1", len(logs))
	}

	assertNoInventoryOrGLEffectForShipment(t, db, fx.CompanyID)
}

func TestVoidShipment_RefusedOnDraft(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = VoidShipment(db, fx.CompanyID, out.ID, "admin@test", nil)
	if err == nil {
		t.Fatalf("expected void to fail on draft")
	}
	if !isErr(err, ErrShipmentNotPosted) {
		t.Fatalf("got %v want ErrShipmentNotPosted", err)
	}
}

// ── Delete ───────────────────────────────────────────────────────────────────

func TestDeleteShipment_DraftSucceeds(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := DeleteShipment(db, fx.CompanyID, out.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := GetShipment(db, fx.CompanyID, out.ID); err != ErrShipmentNotFound {
		t.Fatalf("after delete: got %v want ErrShipmentNotFound", err)
	}
	var lineCount int64
	db.Model(&models.ShipmentLine{}).Where("shipment_id = ?", out.ID).Count(&lineCount)
	if lineCount != 0 {
		t.Fatalf("lines: got %d want 0 after delete", lineCount)
	}
}

func TestDeleteShipment_RefusedOnPosted(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)
	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := PostShipment(db, fx.CompanyID, out.ID, "admin@test", nil); err != nil {
		t.Fatalf("post: %v", err)
	}
	err = DeleteShipment(db, fx.CompanyID, out.ID)
	if err == nil {
		t.Fatalf("expected delete to fail on posted shipment")
	}
	if !isErr(err, ErrShipmentNotDraft) {
		t.Fatalf("got %v want ErrShipmentNotDraft", err)
	}
}

// ── Rail dormancy (I.2 boundary) ────────────────────────────────────────────

// Flipping companies.shipment_required to TRUE must not change I.2
// behavior. PostShipment + VoidShipment stay status-flip-only, and
// no inventory / GL artefacts appear. This locks the "I.2 is rail-
// agnostic" contract. The consumer that makes the flag meaningful
// lands in I.3.
func TestPostShipment_RailOn_StillByteIdenticalInI2_NoInventoryOrGL(t *testing.T) {
	db := testShipmentDocDB(t)
	fx := seedShipmentFixture(t, db)

	if err := db.Model(&models.Company{}).
		Where("id = ?", fx.CompanyID).
		Update("shipment_required", true).Error; err != nil {
		t.Fatalf("flip shipment_required: %v", err)
	}

	out, err := CreateShipment(db, buildSimpleShipmentCreateInput(fx))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	posted, err := PostShipment(db, fx.CompanyID, out.ID, "admin@test", nil)
	if err != nil {
		t.Fatalf("post under rail=on: %v", err)
	}
	// Same rail-agnostic contract as rail=off.
	if posted.JournalEntryID != nil {
		t.Fatalf("I.2 post must not link a JE even with rail=on; got %d", *posted.JournalEntryID)
	}
	if _, err := VoidShipment(db, fx.CompanyID, out.ID, "admin@test", nil); err != nil {
		t.Fatalf("void under rail=on: %v", err)
	}
	assertNoInventoryOrGLEffectForShipment(t, db, fx.CompanyID)
}
