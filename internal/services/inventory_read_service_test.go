// 遵循project_guide.md
package services

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestGetInventorySnapshot_WithBalance(t *testing.T) {
	db := testInventoryDB(t)
	companyID := seedInventoryCompany(t, db)
	itemID := seedInventoryItem(t, db, companyID)

	CreateOpeningBalance(db, OpeningBalanceInput{
		CompanyID: companyID, ItemID: itemID,
		Quantity: decimal.NewFromInt(100), UnitCost: decimal.NewFromFloat(12.50),
		AsOfDate: time.Now(),
	})

	snap, err := GetInventorySnapshot(db, companyID, itemID)
	if err != nil {
		t.Fatal(err)
	}

	if !snap.QuantityOnHand.Equal(decimal.NewFromInt(100)) {
		t.Errorf("qty expected 100, got %s", snap.QuantityOnHand)
	}
	if !snap.AverageCost.Equal(decimal.NewFromFloat(12.50)) {
		t.Errorf("avg cost expected 12.50, got %s", snap.AverageCost)
	}
	expectedValue := decimal.NewFromFloat(1250.00)
	if !snap.InventoryValue.Equal(expectedValue) {
		t.Errorf("value expected %s, got %s", expectedValue, snap.InventoryValue)
	}
	if !snap.HasOpening {
		t.Error("HasOpening should be true")
	}
}

func TestGetInventorySnapshot_NoBalance(t *testing.T) {
	db := testInventoryDB(t)
	companyID := seedInventoryCompany(t, db)
	itemID := seedInventoryItem(t, db, companyID)

	snap, err := GetInventorySnapshot(db, companyID, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.QuantityOnHand.IsZero() {
		t.Errorf("expected zero qty, got %s", snap.QuantityOnHand)
	}
	if !snap.InventoryValue.IsZero() {
		t.Errorf("expected zero value, got %s", snap.InventoryValue)
	}
}

func TestListItemValuations_ReturnsInventoryItemsOnly(t *testing.T) {
	db := testInventoryDB(t)
	companyID := seedInventoryCompany(t, db)
	invItemID := seedInventoryItem(t, db, companyID)
	_ = seedServiceItem(t, db, companyID)

	CreateOpeningBalance(db, OpeningBalanceInput{
		CompanyID: companyID, ItemID: invItemID,
		Quantity: decimal.NewFromInt(50), UnitCost: decimal.NewFromInt(10),
		AsOfDate: time.Now(),
	})

	vals := ListItemValuations(db, companyID)
	if _, ok := vals[invItemID]; !ok {
		t.Fatal("inventory item should have valuation entry")
	}
	// Service item should NOT appear (no balance row)
	if len(vals) != 1 {
		t.Errorf("expected 1 valuation entry, got %d", len(vals))
	}
}

func TestListMovements_OrderAndCompanyScope(t *testing.T) {
	db := testInventoryDB(t)
	companyID := seedInventoryCompany(t, db)
	itemID := seedInventoryItem(t, db, companyID)

	// Create opening + adjustment
	CreateOpeningBalance(db, OpeningBalanceInput{
		CompanyID: companyID, ItemID: itemID,
		Quantity: decimal.NewFromInt(100), UnitCost: decimal.NewFromInt(5),
		AsOfDate: time.Now(),
	})
	CreateAdjustment(db, AdjustmentInput{
		CompanyID: companyID, ItemID: itemID,
		QuantityDelta: decimal.NewFromInt(-10), MovementDate: time.Now(),
	})

	rows, total, err := ListMovements(db, companyID, itemID, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("expected 2 total movements, got %d", total)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// Most recent first (adjustment before opening in DESC order)
	if rows[0].MovementType != "adjustment" {
		t.Errorf("first row should be adjustment (most recent), got %s", rows[0].MovementType)
	}
}

func TestListMovements_CrossCompanyIsolation(t *testing.T) {
	db := testInventoryDB(t)
	company1 := seedInventoryCompany(t, db)
	company2 := seedInventoryCompany(t, db)
	itemID := seedInventoryItem(t, db, company1)

	CreateOpeningBalance(db, OpeningBalanceInput{
		CompanyID: company1, ItemID: itemID,
		Quantity: decimal.NewFromInt(50), UnitCost: decimal.NewFromInt(10),
		AsOfDate: time.Now(),
	})

	// Company 2 should see no movements
	rows, total, err := ListMovements(db, company2, itemID, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(rows) != 0 {
		t.Error("company 2 should see no movements for company 1's item")
	}
}

func TestMovementTypeLabel(t *testing.T) {
	cases := map[string]string{
		"opening":    "Opening",
		"adjustment": "Adjustment",
		"purchase":   "Purchase",
		"sale":       "Sale",
		"unknown":    "unknown",
	}
	for input, expected := range cases {
		if got := movementTypeLabel(input); got != expected {
			t.Errorf("movementTypeLabel(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestSourceTypeLabel(t *testing.T) {
	cases := map[string]string{
		"opening":           "Opening Balance",
		"adjustment":        "Manual Adjustment",
		"invoice":           "Invoice",
		"bill":              "Bill",
		"invoice_reversal":  "Invoice Reversal",
		"bill_reversal":     "Bill Reversal",
		"":                  "—",
	}
	for input, expected := range cases {
		if got := sourceTypeLabel(input); got != expected {
			t.Errorf("sourceTypeLabel(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestListMovements_NonInventoryItem_ReturnsEmpty(t *testing.T) {
	db := testInventoryDB(t)
	companyID := seedInventoryCompany(t, db)
	svcItemID := seedServiceItem(t, db, companyID)

	rows, total, err := ListMovements(db, companyID, svcItemID, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(rows) != 0 {
		t.Error("service item should have no movements")
	}
}

// Verify valuation is correct for non-trivial balance.
func TestListItemValuations_ValueCalculation(t *testing.T) {
	db := testInventoryDB(t)
	companyID := seedInventoryCompany(t, db)
	itemID := seedInventoryItem(t, db, companyID)

	// 25 units @ $40.00 = $1000.00
	CreateOpeningBalance(db, OpeningBalanceInput{
		CompanyID: companyID, ItemID: itemID,
		Quantity: decimal.NewFromInt(25), UnitCost: decimal.NewFromInt(40),
		AsOfDate: time.Now(),
	})

	vals := ListItemValuations(db, companyID)
	v, ok := vals[itemID]
	if !ok {
		t.Fatal("missing valuation")
	}
	if v.InventoryValue != "1000.00" {
		t.Errorf("expected value 1000.00, got %s", v.InventoryValue)
	}
	if v.QuantityOnHand != "25" {
		t.Errorf("expected qty 25, got %s", v.QuantityOnHand)
	}
}
