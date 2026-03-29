package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"gobooks/internal/config"
	"gobooks/internal/models"
)

func testRouteDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:web_routes_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Company{},
		&models.Session{},
		&models.CompanyMembership{},
		&models.SystemSetting{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func testRouteApp(t *testing.T, db *gorm.DB) *fiber.App {
	t.Helper()
	return NewServer(config.Config{
		Env:  "test",
		Addr: ":0",
	}, db)
}

func seedCompany(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()
	company := models.Company{
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
	}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	return company.ID
}

func seedUserSession(t *testing.T, db *gorm.DB, activeCompanyID *uint) (models.User, string) {
	t.Helper()

	user := models.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("%s@example.com", t.Name()),
		PasswordHash: "not-used-in-route-tests",
		DisplayName:  "Route Test",
		IsActive:     true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	rawToken, tokenHash, err := NewOpaqueSessionToken()
	if err != nil {
		t.Fatal(err)
	}

	session := models.Session{
		ID:              uuid.New(),
		TokenHash:       tokenHash,
		UserID:          user.ID,
		ActiveCompanyID: activeCompanyID,
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
		CreatedAt:       time.Now().UTC(),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}

	return user, rawToken
}

func seedMembership(t *testing.T, db *gorm.DB, userID uuid.UUID, companyID uint) {
	t.Helper()

	membership := models.CompanyMembership{
		ID:        uuid.New(),
		UserID:    userID,
		CompanyID: companyID,
		Role:      models.CompanyRoleAdmin,
		IsActive:  true,
	}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatal(err)
	}
}

func performRequest(t *testing.T, app *fiber.App, path string, rawToken string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if rawToken != "" {
		req.AddCookie(&http.Cookie{
			Name:  SessionCookieName,
			Value: rawToken,
			Path:  "/",
		})
	}

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestProtectedRoutesRedirectToLoginWhenUnauthenticated(t *testing.T) {
	db := testRouteDB(t)
	seedCompany(t, db, "Acme")
	app := testRouteApp(t, db)

	tests := []struct {
		name string
		path string
	}{
		{name: "dashboard", path: "/"},
		{name: "report", path: "/reports/trial-balance"},
		{name: "audit log", path: "/settings/audit-log"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := performRequest(t, app, tc.path, "")
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("expected %d, got %d", http.StatusSeeOther, resp.StatusCode)
			}
			if got := resp.Header.Get("Location"); got != "/login" {
				t.Fatalf("expected redirect to /login, got %q", got)
			}
		})
	}
}

func TestProtectedRoutesRedirectToSelectCompanyWhenSessionHasNoResolvableActiveCompany(t *testing.T) {
	db := testRouteDB(t)
	companyA := seedCompany(t, db, "Acme")
	companyB := seedCompany(t, db, "Beta")
	user, rawToken := seedUserSession(t, db, nil)
	seedMembership(t, db, user.ID, companyA)
	seedMembership(t, db, user.ID, companyB)

	app := testRouteApp(t, db)
	resp := performRequest(t, app, "/reports/trial-balance", rawToken)

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/select-company" {
		t.Fatalf("expected redirect to /select-company, got %q", got)
	}
}

func TestProtectedRoutesRedirectToSelectCompanyWhenSessionActiveCompanyIsStale(t *testing.T) {
	db := testRouteDB(t)
	companyA := seedCompany(t, db, "Acme")
	companyB := seedCompany(t, db, "Beta")
	staleCompanyID := uint(9999)
	user, rawToken := seedUserSession(t, db, &staleCompanyID)
	seedMembership(t, db, user.ID, companyA)
	seedMembership(t, db, user.ID, companyB)

	app := testRouteApp(t, db)
	resp := performRequest(t, app, "/settings/audit-log", rawToken)

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/select-company" {
		t.Fatalf("expected redirect to /select-company, got %q", got)
	}
}
