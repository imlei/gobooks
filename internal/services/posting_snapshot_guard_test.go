package services

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

func TestPostingSnapshotGuardDetectsLineChanges(t *testing.T) {
	now := time.Now().UTC()
	original := []models.InvoiceLine{
		{ID: 1, UpdatedAt: now},
		{ID: 2, UpdatedAt: now},
	}
	current := []models.InvoiceLine{
		{ID: 1, UpdatedAt: now},
		{ID: 2, UpdatedAt: now.Add(time.Second)},
	}
	if !lineSnapshotChanged(original, current) {
		t.Fatal("expected changed line timestamp to invalidate posting snapshot")
	}
	if !lineSnapshotChanged(original, current[:1]) {
		t.Fatal("expected changed line count to invalidate posting snapshot")
	}
}

func TestEnsureInvoicePostingSnapshotFreshReturnsConflict(t *testing.T) {
	db := newPostingSnapshotTestDB(t)
	inv := models.Invoice{
		CompanyID:     1,
		CustomerID:    1,
		InvoiceNumber: "INV-1",
		Status:        models.InvoiceStatusDraft,
		InvoiceDate:   time.Now(),
		Amount:        decimal.NewFromInt(100),
	}
	if err := db.Create(&inv).Error; err != nil {
		t.Fatal(err)
	}
	line := models.InvoiceLine{
		CompanyID:   1,
		InvoiceID:   inv.ID,
		Description: "Line",
		Qty:         decimal.NewFromInt(1),
		UnitPrice:   decimal.NewFromInt(100),
		LineNet:     decimal.NewFromInt(100),
		LineTotal:   decimal.NewFromInt(100),
	}
	if err := db.Create(&line).Error; err != nil {
		t.Fatal(err)
	}
	original := inv
	original.Lines = []models.InvoiceLine{line}

	if err := db.Model(&line).Updates(map[string]any{
		"description": "Changed",
		"updated_at":  line.UpdatedAt.Add(time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return ensureInvoicePostingSnapshotFresh(tx, 1, inv.ID, original)
	})
	if !errors.Is(err, ErrPostingSourceChanged) {
		t.Fatalf("expected ErrPostingSourceChanged, got %v", err)
	}
}

func TestPostingSnapshotGuardDetectsInvoiceLineAmountChangeWithoutTimestamp(t *testing.T) {
	db := newPostingSnapshotTestDB(t)
	inv := models.Invoice{
		CompanyID:     1,
		CustomerID:    1,
		InvoiceNumber: "INV-AMT",
		Status:        models.InvoiceStatusDraft,
		InvoiceDate:   time.Now(),
		Amount:        decimal.NewFromInt(100),
		Subtotal:      decimal.NewFromInt(100),
	}
	if err := db.Create(&inv).Error; err != nil {
		t.Fatal(err)
	}
	line := models.InvoiceLine{
		CompanyID:   1,
		InvoiceID:   inv.ID,
		Description: "Line",
		Qty:         decimal.NewFromInt(1),
		UnitPrice:   decimal.NewFromInt(100),
		LineNet:     decimal.NewFromInt(100),
		LineTotal:   decimal.NewFromInt(100),
	}
	if err := db.Create(&line).Error; err != nil {
		t.Fatal(err)
	}
	original := inv
	original.UpdatedAt = time.Time{}
	line.UpdatedAt = time.Time{}
	original.Lines = []models.InvoiceLine{line}

	if err := db.Model(&models.InvoiceLine{}).
		Where("id = ?", line.ID).
		Update("line_net", decimal.NewFromInt(125)).Error; err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return ensureInvoicePostingSnapshotFresh(tx, 1, inv.ID, original)
	})
	if !errors.Is(err, ErrPostingSourceChanged) {
		t.Fatalf("expected ErrPostingSourceChanged, got %v", err)
	}
}

func TestPostingSnapshotGuardDetectsBillLineAccountChangeWithoutTimestamp(t *testing.T) {
	db := newPostingSnapshotTestDB(t)
	expenseA := uint(10)
	expenseB := uint(11)
	bill := models.Bill{
		CompanyID:  1,
		VendorID:   1,
		BillNumber: "BILL-ACCT",
		Status:     models.BillStatusDraft,
		BillDate:   time.Now(),
		Amount:     decimal.NewFromInt(100),
		Subtotal:   decimal.NewFromInt(100),
	}
	if err := db.Create(&bill).Error; err != nil {
		t.Fatal(err)
	}
	line := models.BillLine{
		CompanyID:        1,
		BillID:           bill.ID,
		Description:      "Line",
		ExpenseAccountID: &expenseA,
		Qty:              decimal.NewFromInt(1),
		UnitPrice:        decimal.NewFromInt(100),
		LineNet:          decimal.NewFromInt(100),
		LineTotal:        decimal.NewFromInt(100),
	}
	if err := db.Create(&line).Error; err != nil {
		t.Fatal(err)
	}
	original := bill
	original.UpdatedAt = time.Time{}
	line.UpdatedAt = time.Time{}
	original.Lines = []models.BillLine{line}

	if err := db.Model(&models.BillLine{}).
		Where("id = ?", line.ID).
		Update("expense_account_id", expenseB).Error; err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return ensureBillPostingSnapshotFresh(tx, 1, bill.ID, original)
	})
	if !errors.Is(err, ErrPostingSourceChanged) {
		t.Fatalf("expected ErrPostingSourceChanged, got %v", err)
	}
}

func newPostingSnapshotTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:posting_snapshot_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Invoice{}, &models.InvoiceLine{}, &models.Bill{}, &models.BillLine{}); err != nil {
		t.Fatal(err)
	}
	return db
}
