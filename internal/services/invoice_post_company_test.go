package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

func testInvoicePostDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:invoice_post_company_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Company{},
		&models.Customer{},
		&models.Account{},
		&models.TaxCode{},
		&models.ProductService{},
		&models.Invoice{},
		&models.InvoiceLine{},
		&models.JournalEntry{},
		&models.JournalLine{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedInvoicePostCompany(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()

	row := models.Company{
		Name:                    name,
		EntityType:              models.EntityTypeIncorporated,
		BusinessType:            models.BusinessTypeRetail,
		Industry:                models.IndustryRetail,
		IncorporatedDate:        "2024-01-01",
		FiscalYearEnd:           "12-31",
		BusinessNumber:          "123456789",
		AddressLine:             "123 Main",
		City:                    "Vancouver",
		Province:                "BC",
		PostalCode:              "V6B1A1",
		Country:                 "CA",
		AccountCodeLength:       4,
		AccountCodeLengthLocked: true,
		IsActive:                true,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedInvoicePostAccount(t *testing.T, db *gorm.DB, companyID uint, code string, root models.RootAccountType, detail models.DetailAccountType) uint {
	t.Helper()

	row := models.Account{
		CompanyID:         companyID,
		Code:              code,
		Name:              code,
		RootAccountType:   root,
		DetailAccountType: detail,
		IsActive:          true,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func TestPostInvoiceRejectsCrossCompanyProductService(t *testing.T) {
	db := testInvoicePostDB(t)

	companyA := seedInvoicePostCompany(t, db, "Acme")
	companyB := seedInvoicePostCompany(t, db, "Beta")
	customer := models.Customer{CompanyID: companyA, Name: "Customer A"}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatal(err)
	}

	_ = seedInvoicePostAccount(t, db, companyA, "1100", models.RootAsset, models.DetailAccountsReceivable)
	revenueB := seedInvoicePostAccount(t, db, companyB, "4000", models.RootRevenue, models.DetailServiceRevenue)
	productB := models.ProductService{
		CompanyID:        companyB,
		Name:             "Beta Service",
		Type:             models.ProductServiceTypeService,
		RevenueAccountID: revenueB,
		IsActive:         true,
	}
	if err := db.Create(&productB).Error; err != nil {
		t.Fatal(err)
	}

	invoice := models.Invoice{
		CompanyID:     companyA,
		InvoiceNumber: "IN001",
		CustomerID:    customer.ID,
		InvoiceDate:   time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
		Status:        models.InvoiceStatusDraft,
		Amount:        decimal.RequireFromString("100.00"),
		Subtotal:      decimal.RequireFromString("100.00"),
		TaxTotal:      decimal.Zero,
	}
	if err := db.Create(&invoice).Error; err != nil {
		t.Fatal(err)
	}

	line := models.InvoiceLine{
		CompanyID:        companyA,
		InvoiceID:        invoice.ID,
		SortOrder:        1,
		ProductServiceID: &productB.ID,
		Description:      "Cross-company line",
		Qty:              decimal.RequireFromString("1"),
		UnitPrice:        decimal.RequireFromString("100"),
		LineNet:          decimal.RequireFromString("100.00"),
		LineTax:          decimal.Zero,
		LineTotal:        decimal.RequireFromString("100.00"),
	}
	if err := db.Create(&line).Error; err != nil {
		t.Fatal(err)
	}

	err := PostInvoice(db, companyA, invoice.ID, "tester", nil)
	if err == nil || !strings.Contains(err.Error(), "product/service is not valid for this company") {
		t.Fatalf("expected cross-company product/service error, got %v", err)
	}
}
