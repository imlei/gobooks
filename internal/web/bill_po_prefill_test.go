package web

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
	"balanciz/internal/web/templates/pages"
)

func TestPrefillBillFromPOCarriesPONumberIntoMemo(t *testing.T) {
	db := testEditorFlowDB(t)
	if err := db.AutoMigrate(&models.PurchaseOrder{}, &models.PurchaseOrderLine{}); err != nil {
		t.Fatalf("migrate PO tables: %v", err)
	}
	server := &Server{DB: db}
	companyID := seedValidationCompany(t, db, "PO To Bill Memo Co")
	vendorID := seedEditorFlowVendor(t, db, companyID, "PO Vendor")

	po := models.PurchaseOrder{
		CompanyID:    companyID,
		PONumber:     "PO-0007",
		VendorID:     vendorID,
		Status:       models.POStatusConfirmed,
		PODate:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "USD",
		ExchangeRate: decimal.RequireFromString("1.3617"),
		Notes:        "Handle with care",
		Amount:       decimal.NewFromInt(100),
		Subtotal:     decimal.NewFromInt(100),
	}
	if err := db.Create(&po).Error; err != nil {
		t.Fatalf("create PO: %v", err)
	}
	line := models.PurchaseOrderLine{
		CompanyID:       companyID,
		PurchaseOrderID: po.ID,
		Description:     "Computer 1",
		Qty:             decimal.NewFromInt(1),
		UnitPrice:       decimal.NewFromInt(100),
		LineNet:         decimal.NewFromInt(100),
	}
	if err := db.Create(&line).Error; err != nil {
		t.Fatalf("create PO line: %v", err)
	}
	blankLine := models.PurchaseOrderLine{
		CompanyID:       companyID,
		PurchaseOrderID: po.ID,
		Qty:             decimal.NewFromInt(1),
		UnitPrice:       decimal.Zero,
		LineNet:         decimal.Zero,
	}
	if err := db.Create(&blankLine).Error; err != nil {
		t.Fatalf("create blank PO line: %v", err)
	}

	vm := pages.BillEditorVM{
		BillNumber:       "BILL019",
		BaseCurrencyCode: "CAD",
	}
	if ok := server.prefillBillFromPO(companyID, po.ID, &vm); !ok {
		t.Fatal("expected PO prefill to succeed")
	}

	if vm.BillNumber != "BILL019" {
		t.Fatalf("BillNumber should remain the bill's own number, got %q", vm.BillNumber)
	}
	if vm.Memo != "PO #: PO-0007 - Handle with care" {
		t.Fatalf("Memo = %q, want PO number and notes", vm.Memo)
	}
	if vm.VendorID == "" || vm.CurrencyCode != "USD" || vm.ExchangeRate == "" {
		t.Fatalf("expected existing PO header prefill to remain intact: %#v", vm)
	}
	if len(vm.Lines) != 1 {
		t.Fatalf("expected PO prefill to skip default blank lines, got %d rows", len(vm.Lines))
	}
	if vm.Lines[0].Description != "Computer 1" || vm.Lines[0].Amount != "100.00" {
		t.Fatalf("unexpected PO prefill line: %#v", vm.Lines[0])
	}
}

func TestBillMemoFromPOUsesPONumberWithoutNotes(t *testing.T) {
	got := billMemoFromPO(&models.PurchaseOrder{PONumber: "PO-0042"})
	if got != "PO #: PO-0042" {
		t.Fatalf("billMemoFromPO = %q", got)
	}
}
