package services

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

func testSecurityHooksDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:security_hooks_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Company{},
		&models.CompanySecuritySettings{},
		&models.SystemSecuritySettings{},
		&models.SecurityEvent{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEvaluateLoginSecurityCreatesUnusualIPAlertAfterPriorSuccess(t *testing.T) {
	db := testSecurityHooksDB(t)

	company := models.Company{
		Name:                    "Acme",
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
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.SystemSecuritySettings{
		UnusualIPLoginAlertDefaultEnabled:    true,
		UnusualIPLoginCompanyOverrideAllowed: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CompanySecuritySettings{
		CompanyID:                  company.ID,
		UnusualIPLoginAlertEnabled: true,
		UnusualIPLoginAlertChannel: models.AlertChannelBoth,
	}).Error; err != nil {
		t.Fatal(err)
	}

	userID := "user-1"
	if err := LogSecurityEvent(db, &company.ID, &userID, loginEventType(true), "203.0.113.10", "Mozilla/5.0", nil); err != nil {
		t.Fatal(err)
	}

	EvaluateLoginSecurity(db, LoginSecurityContext{
		CompanyID: &company.ID,
		UserID:    userID,
		UserEmail: "user@example.com",
		IPAddress: "198.51.100.20",
		UserAgent: "Mozilla/5.0",
		Success:   true,
	})

	var events []models.SecurityEvent
	if err := db.Order("id asc").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[1].EventType != loginEventType(true) {
		t.Fatalf("expected second event to be raw login.success, got %s", events[1].EventType)
	}
	if events[2].EventType != unusualIPAlertEventType {
		t.Fatalf("expected unusual IP alert event, got %s", events[2].EventType)
	}
	if events[2].MetadataJSON == nil || !strings.Contains(*events[2].MetadataJSON, `"channel":"both"`) {
		t.Fatalf("expected alert metadata to include configured channel, got %+v", events[2].MetadataJSON)
	}
}
