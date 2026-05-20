// 遵循project_guide.md
package web

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"balanciz/internal/models"
	"balanciz/internal/services/search_engine"
)

// stubAdvancedEngine captures the AdvancedRequest the selector dispatches
// so tests can assert query-param parsing without spinning up a database.
type stubAdvancedEngine struct {
	mode   search_engine.Mode
	gotReq search_engine.AdvancedRequest
	resp   *search_engine.AdvancedResponse
	err    error
}

func (s *stubAdvancedEngine) Mode() search_engine.Mode { return s.mode }
func (*stubAdvancedEngine) Search(_ context.Context, _ search_engine.SearchRequest) (*search_engine.SearchResponse, error) {
	return &search_engine.SearchResponse{}, nil
}
func (s *stubAdvancedEngine) SearchAdvanced(_ context.Context, req search_engine.AdvancedRequest) (*search_engine.AdvancedResponse, error) {
	s.gotReq = req
	if s.resp != nil {
		return s.resp, s.err
	}
	return &search_engine.AdvancedResponse{Page: req.Page, PageSize: req.PageSize}, s.err
}

// newAdvancedSearchTestApp wires a Fiber app with handleAdvancedSearch
// and a synthetic ResolveActiveCompany middleware (companyID=42), same
// pattern as newGlobalSearchTestApp.
func newAdvancedSearchTestApp(t *testing.T, sel *search_engine.Selector) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	s := &Server{SearchSelector: sel}
	app.Get("/advanced-search", func(c *fiber.Ctx) error {
		c.Locals(LocalsActiveCompanyID, uint(42))
		return s.handleAdvancedSearch(c)
	})
	return app
}

// Smoke contract: handler renders 200 and forwards every query param into
// the AdvancedRequest (entity_type narrowing, date bounds, status, paging).
func TestHandleAdvancedSearch_ForwardsQueryParams(t *testing.T) {
	stub := &stubAdvancedEngine{mode: search_engine.ModeEnt}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newAdvancedSearchTestApp(t, sel)

	status, body := runGet(t, app, "/advanced-search?q=lighting&type=invoice&from=2026-04-01&to=2026-04-22&status=paid&page=2&size=25")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, string(body))
	}
	got := stub.gotReq
	if got.CompanyID != 42 {
		t.Errorf("CompanyID = %d, want 42", got.CompanyID)
	}
	if got.Query != "lighting" {
		t.Errorf("Query = %q, want lighting", got.Query)
	}
	if got.EntityType != "invoice" {
		t.Errorf("EntityType = %q, want invoice", got.EntityType)
	}
	if got.Status != "paid" {
		t.Errorf("Status = %q, want paid", got.Status)
	}
	if got.Page != 2 || got.PageSize != 25 {
		t.Errorf("Page/Size = %d/%d, want 2/25", got.Page, got.PageSize)
	}
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if !got.DateFrom.Equal(want) {
		t.Errorf("DateFrom = %v, want %v", got.DateFrom, want)
	}
	// DateTo is bumped to end-of-day so a row dated 2026-04-22 is included.
	if got.DateTo.Year() != 2026 || got.DateTo.Month() != 4 || got.DateTo.Day() != 22 || got.DateTo.Hour() != 23 {
		t.Errorf("DateTo = %v, want 2026-04-22T23:59:59", got.DateTo)
	}
	// Sanity: page rendered without HTTP error and contains the page heading.
	if !strings.Contains(string(body), "Advanced search") {
		t.Errorf("response body missing page title; body=%s", string(body))
	}
}

// Unknown entity_type values must be silently dropped — stops the URL bar
// from being a typo-poisoning vector.
func TestHandleAdvancedSearch_DropsUnknownEntityType(t *testing.T) {
	stub := &stubAdvancedEngine{mode: search_engine.ModeEnt}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newAdvancedSearchTestApp(t, sel)

	status, _ := runGet(t, app, "/advanced-search?type=not_a_real_entity")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if stub.gotReq.EntityType != "" {
		t.Errorf("EntityType = %q, want empty (unknown value should drop)", stub.gotReq.EntityType)
	}
}

// Engine errors render an empty page rather than a 500 — the operator can
// tweak filters and retry without losing the editor session.
func TestHandleAdvancedSearch_EngineErrorRendersEmptyPage(t *testing.T) {
	stub := &stubAdvancedEngine{mode: search_engine.ModeEnt, err: context.Canceled}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newAdvancedSearchTestApp(t, sel)

	status, body := runGet(t, app, "/advanced-search?q=x")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (engine errors should not 500); body=%s", status, string(body))
	}
	if !strings.Contains(string(body), "No results match your filters.") {
		t.Errorf("expected empty-state copy in body, got: %s", string(body))
	}
}

func TestHandleAdvancedSearchDropsDisallowedSensitiveEntityType(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Advanced Search Permissions Co")
	user, token := seedUserSession(t, db, &companyID)
	if err := db.Create(&models.CompanyMembership{
		ID:        uuid.New(),
		UserID:    user.ID,
		CompanyID: companyID,
		Role:      models.CompanyRoleViewer,
		IsActive:  true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CompanyFeature{
		CompanyID:  companyID,
		FeatureKey: models.FeatureKeyPayroll,
		Status:     models.FeatureStatusEnabled,
		Maturity:   models.FeatureMaturityBeta,
	}).Error; err != nil {
		t.Fatal(err)
	}

	stub := &stubAdvancedEngine{mode: search_engine.ModeEnt}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newAuthenticatedAdvancedSearchTestApp(db, sel)

	status, body := runGetWithToken(t, app, "/advanced-search?type=payroll_entry", token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, string(body))
	}
	if stub.gotReq.EntityType != "" {
		t.Fatalf("disallowed payroll_entry type should be dropped, got %q", stub.gotReq.EntityType)
	}
	if containsString(stub.gotReq.AllowedEntityTypes, "payroll_entry") {
		t.Fatalf("payroll_entry should not be in allowed types without payroll detail permission: %+v", stub.gotReq.AllowedEntityTypes)
	}

	for _, row := range []models.UserCompanyPermission{
		{UserID: user.ID, CompanyID: companyID, Permission: PermPayrollView, Granted: true, GrantedBy: user.ID},
		{UserID: user.ID, CompanyID: companyID, Permission: PermPayrollDetails, Granted: true, GrantedBy: user.ID},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	status, body = runGetWithToken(t, app, "/advanced-search?type=payroll_entry", token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, string(body))
	}
	if stub.gotReq.EntityType != "payroll_entry" {
		t.Fatalf("granted payroll_entry type should be forwarded, got %q", stub.gotReq.EntityType)
	}
	if !containsString(stub.gotReq.AllowedEntityTypes, "payroll_entry") {
		t.Fatalf("payroll_entry should be allowed after grant: %+v", stub.gotReq.AllowedEntityTypes)
	}
}

func TestHandleAdvancedSearchSanitizesSensitiveReturnedRows(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Advanced Search Sanitizer Co")
	user, token := seedUserSession(t, db, &companyID)
	if err := db.Create(&models.CompanyMembership{
		ID:        uuid.New(),
		UserID:    user.ID,
		CompanyID: companyID,
		Role:      models.CompanyRoleViewer,
		IsActive:  true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CompanyFeature{
		CompanyID:  companyID,
		FeatureKey: models.FeatureKeyPayroll,
		Status:     models.FeatureStatusEnabled,
		Maturity:   models.FeatureMaturityBeta,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.UserCompanyPermission{
		UserID: user.ID, CompanyID: companyID, Permission: PermPayrollView, Granted: true, GrantedBy: user.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	stub := &stubAdvancedEngine{
		mode: search_engine.ModeEnt,
		resp: &search_engine.AdvancedResponse{
			Page:     1,
			PageSize: 50,
			Total:    2,
			Rows: []search_engine.Candidate{
				{
					ID:         "10",
					Primary:    "Payroll Run PAY-10",
					URL:        "/payroll/runs/10",
					EntityType: "payroll_run",
					Payload:    map[string]string{"amount": "1234.56", "currency": "CAD", "status": "posted", "doc_num": "PAY-10"},
				},
				{
					ID:         "11",
					Primary:    "Jane Private",
					URL:        "/payroll/runs/10#entry-11",
					EntityType: "payroll_entry",
					Payload:    map[string]string{"amount": "777.77", "currency": "CAD"},
				},
			},
		},
	}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newAuthenticatedAdvancedSearchTestApp(db, sel)

	status, body := runGetWithToken(t, app, "/advanced-search?q=pay", token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, string(body))
	}
	bodyText := string(body)
	for _, forbidden := range []string{"1234.56", "777.77", "Jane Private", "payroll_entry"} {
		if strings.Contains(bodyText, forbidden) {
			t.Fatalf("advanced search leaked %q in body: %s", forbidden, bodyText)
		}
	}
	if !strings.Contains(bodyText, "Payroll Run PAY-10") {
		t.Fatalf("expected allowed payroll run title in body: %s", bodyText)
	}
}

func newAuthenticatedAdvancedSearchTestApp(db *gorm.DB, sel *search_engine.Selector) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	s := &Server{DB: db, SearchSelector: sel}
	app.Get("/advanced-search", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAdvancedSearch)
	return app
}
