// 遵循project_guide.md
package services

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"balanciz/internal/models"
)

func TestGetWarehouseQueueSummary(t *testing.T) {
	db := testWarehouseDB(t)
	if err := db.AutoMigrate(
		&models.Vendor{},
		&models.Customer{},
		&models.PurchaseOrder{},
		&models.SalesOrder{},
	); err != nil {
		t.Fatalf("migrate queue tables: %v", err)
	}
	companyID := seedWarehouseCompany(t, db)
	dueSoon := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	dueLater := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)

	vendor := models.Vendor{CompanyID: companyID, Name: "Northwind Supply", IsActive: true}
	if err := db.Create(&vendor).Error; err != nil {
		t.Fatalf("create vendor: %v", err)
	}
	customer := models.Customer{CompanyID: companyID, Name: "Acme Customer", IsActive: true}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}

	pos := []models.PurchaseOrder{
		{
			CompanyID:    companyID,
			PONumber:     "PO-OPEN",
			VendorID:     vendor.ID,
			Status:       models.POStatusConfirmed,
			PODate:       dueSoon,
			ExpectedDate: &dueSoon,
			Amount:       decimal.NewFromInt(1200),
			CurrencyCode: "USD",
		},
		{
			CompanyID:    companyID,
			PONumber:     "PO-PARTIAL",
			VendorID:     vendor.ID,
			Status:       models.POStatusPartiallyReceived,
			PODate:       dueLater,
			ExpectedDate: &dueLater,
			Amount:       decimal.NewFromInt(500),
			CurrencyCode: "CAD",
		},
		{
			CompanyID: companyID,
			PONumber:  "PO-DONE",
			VendorID:  vendor.ID,
			Status:    models.POStatusReceived,
			PODate:    dueSoon,
			Amount:    decimal.NewFromInt(90),
		},
	}
	if err := db.Create(&pos).Error; err != nil {
		t.Fatalf("create purchase orders: %v", err)
	}

	sos := []models.SalesOrder{
		{
			CompanyID:    companyID,
			CustomerID:   customer.ID,
			OrderNumber:  "SO-OPEN",
			Status:       models.SalesOrderStatusConfirmed,
			OrderDate:    dueSoon,
			RequiredBy:   &dueSoon,
			Total:        decimal.NewFromInt(700),
			CurrencyCode: "USD",
		},
		{
			CompanyID:   companyID,
			CustomerID:  customer.ID,
			OrderNumber: "SO-DONE",
			Status:      models.SalesOrderStatusFullyInvoiced,
			OrderDate:   dueLater,
			Total:       decimal.NewFromInt(80),
		},
	}
	if err := db.Create(&sos).Error; err != nil {
		t.Fatalf("create sales orders: %v", err)
	}

	summary, err := GetWarehouseQueueSummary(db, companyID, 5)
	if err != nil {
		t.Fatalf("GetWarehouseQueueSummary: %v", err)
	}
	if len(summary.WaitingToReceive) != 2 {
		t.Fatalf("WaitingToReceive len = %d, want 2", len(summary.WaitingToReceive))
	}
	if summary.WaitingToReceive[0].Number != "PO-OPEN" || summary.WaitingToReceive[0].Counterparty != "Northwind Supply" {
		t.Fatalf("first receive item = %#v", summary.WaitingToReceive[0])
	}
	if len(summary.WaitingToShip) != 1 {
		t.Fatalf("WaitingToShip len = %d, want 1", len(summary.WaitingToShip))
	}
	if summary.WaitingToShip[0].Number != "SO-OPEN" || summary.WaitingToShip[0].Href != "/sales-orders/1" {
		t.Fatalf("ship item = %#v", summary.WaitingToShip[0])
	}
}
