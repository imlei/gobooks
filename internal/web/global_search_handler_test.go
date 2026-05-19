// 遵循project_guide.md
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"balanciz/internal/models"
	"balanciz/internal/services/search_engine"
)

// stubEngineForHandler is the minimum stub the global-search handler
// needs: returns a fixed response so we can verify the JSON shape +
// status code without a Postgres / ent fixture.
type stubEngineForHandler struct {
	mode   search_engine.Mode
	gotReq search_engine.SearchRequest
	resp   *search_engine.SearchResponse
	err    error
}

func (s *stubEngineForHandler) Mode() search_engine.Mode { return s.mode }
func (s *stubEngineForHandler) Search(_ context.Context, req search_engine.SearchRequest) (*search_engine.SearchResponse, error) {
	s.gotReq = req
	return s.resp, s.err
}
func (s *stubEngineForHandler) SearchAdvanced(_ context.Context, _ search_engine.AdvancedRequest) (*search_engine.AdvancedResponse, error) {
	return &search_engine.AdvancedResponse{}, s.err
}

// newGlobalSearchTestApp wires a Fiber app with handleGlobalSearch and
// a synthetic ResolveActiveCompany middleware that injects companyID=42
// without touching auth tables. Lets us drive the handler in isolation.
func newGlobalSearchTestApp(t *testing.T, sel *search_engine.Selector) *fiber.App {
	t.Helper()
	app := fiber.New()
	s := &Server{SearchSelector: sel}
	app.Get("/api/global-search", func(c *fiber.Ctx) error {
		c.Locals(LocalsActiveCompanyID, uint(42))
		return s.handleGlobalSearch(c)
	})
	return app
}

func runGet(t *testing.T, app *fiber.App, url string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// Phase 5.1 smoke contract: ent-mode search returns 200 + valid JSON
// shape with grouped candidates.
func TestHandleGlobalSearch_EntModeReturnsJSON(t *testing.T) {
	stub := &stubEngineForHandler{
		mode: search_engine.ModeEnt,
		resp: &search_engine.SearchResponse{
			Source: "ranked",
			Candidates: []search_engine.Candidate{
				{
					ID:         "1",
					Primary:    "POSX US INC.",
					Secondary:  "Invoice INV-1 · 2026-04-22 · CAD 100.00",
					GroupKey:   search_engine.GroupTransactions,
					GroupLabel: "Transactions",
					ActionKind: search_engine.ActionNavigate,
					URL:        "/invoices/1",
					EntityType: "invoice",
				},
			},
		},
	}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newGlobalSearchTestApp(t, sel)

	status, body := runGet(t, app, "/api/global-search?q=posx&limit=5")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}
	var got struct {
		Candidates []map[string]any `json:"candidates"`
		Source     string           `json:"source"`
		Mode       string           `json:"mode"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json: %v; body=%s", err, body)
	}
	if got.Mode != "ent" {
		t.Errorf("mode = %q, want ent", got.Mode)
	}
	if got.Source != "ranked" {
		t.Errorf("source = %q, want ranked", got.Source)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1", len(got.Candidates))
	}
	c := got.Candidates[0]
	for _, key := range []string{"id", "primary", "group_key", "url", "entity_type"} {
		if _, ok := c[key]; !ok {
			t.Errorf("missing JSON key %q in candidate: %+v", key, c)
		}
	}
}

// Phase 5.0 contract repeated end-to-end through the handler: legacy
// mode returns 200 + empty candidates, never 500. This guards the
// dropdown UI from breaking when an operator pins SEARCH_ENGINE=legacy.
func TestHandleGlobalSearch_LegacyModeReturnsEmpty200(t *testing.T) {
	sel := search_engine.NewSelector(
		search_engine.ModeLegacy,
		search_engine.NewLegacyEngine(),
		nil, nil,
	)
	app := newGlobalSearchTestApp(t, sel)

	status, body := runGet(t, app, "/api/global-search?q=anything")
	if status != http.StatusOK {
		t.Fatalf("legacy mode must return 200, got %d; body=%s", status, body)
	}
	var got struct {
		Candidates []any  `json:"candidates"`
		Source     string `json:"source"`
		Mode       string `json:"mode"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got.Mode != "legacy" {
		t.Errorf("mode = %q, want legacy", got.Mode)
	}
	if len(got.Candidates) != 0 {
		t.Errorf("candidates len = %d, want 0", len(got.Candidates))
	}
}

func TestHandleGlobalSearch_NoSelectorReturns503(t *testing.T) {
	app := fiber.New()
	s := &Server{} // SearchSelector intentionally nil
	app.Get("/api/global-search", func(c *fiber.Ctx) error {
		c.Locals(LocalsActiveCompanyID, uint(42))
		return s.handleGlobalSearch(c)
	})
	status, _ := runGet(t, app, "/api/global-search?q=x")
	if status != http.StatusServiceUnavailable {
		t.Errorf("missing selector should return 503, got %d", status)
	}
}

func TestHandleGlobalSearch_NoCompanyReturns400(t *testing.T) {
	app := fiber.New()
	s := &Server{SearchSelector: search_engine.NewSelector(search_engine.ModeLegacy, search_engine.NewLegacyEngine(), nil, nil)}
	// No company injection — the handler should reject.
	app.Get("/api/global-search", s.handleGlobalSearch)
	status, _ := runGet(t, app, "/api/global-search?q=x")
	if status != http.StatusBadRequest {
		t.Errorf("missing active company should return 400, got %d", status)
	}
}

func TestHandleGlobalSearchAllowedEntitiesRespectFeatureFlagsAndOverrides(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Search Permissions Co")
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
	for _, feature := range []models.FeatureKey{models.FeatureKeyEmployee, models.FeatureKeyPayroll} {
		if err := db.Create(&models.CompanyFeature{
			CompanyID:  companyID,
			FeatureKey: feature,
			Status:     models.FeatureStatusEnabled,
			Maturity:   models.FeatureMaturityBeta,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []models.UserCompanyPermission{
		{UserID: user.ID, CompanyID: companyID, Permission: PermEmployeeView, Granted: true, GrantedBy: user.ID},
		{UserID: user.ID, CompanyID: companyID, Permission: PermPayrollView, Granted: true, GrantedBy: user.ID},
		{UserID: user.ID, CompanyID: companyID, Permission: PermPayrollDetails, Granted: true, GrantedBy: user.ID},
		{UserID: user.ID, CompanyID: companyID, Permission: PermChequeView, Granted: true, GrantedBy: user.ID},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}

	stub := &stubEngineForHandler{
		mode: search_engine.ModeEnt,
		resp: &search_engine.SearchResponse{},
	}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newAuthenticatedGlobalSearchTestApp(db, sel)

	status, body := runGetWithToken(t, app, "/api/global-search?q=pay", token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}
	allowed := stub.gotReq.AllowedEntityTypes
	for _, want := range []string{"employee", "payroll_run", "payroll_entry", "payroll_remittance"} {
		if !containsString(allowed, want) {
			t.Fatalf("allowed entity types missing %q: %+v", want, allowed)
		}
	}
	for _, notWant := range []string{"task", "cheque"} {
		if containsString(allowed, notWant) {
			t.Fatalf("allowed entity types should not contain %q while denied or feature-off: %+v", notWant, allowed)
		}
	}

	if err := db.Create(&models.CompanyFeature{
		CompanyID:  companyID,
		FeatureKey: models.FeatureKeyCheque,
		Status:     models.FeatureStatusEnabled,
		Maturity:   models.FeatureMaturityBeta,
	}).Error; err != nil {
		t.Fatal(err)
	}
	status, body = runGetWithToken(t, app, "/api/global-search?q=cheque", token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}
	if !containsString(stub.gotReq.AllowedEntityTypes, "cheque") {
		t.Fatalf("cheque should be searchable after permission grant and feature enable: %+v", stub.gotReq.AllowedEntityTypes)
	}

	if err := db.Create(&models.CompanyFeature{
		CompanyID:  companyID,
		FeatureKey: models.FeatureKeyTask,
		Status:     models.FeatureStatusEnabled,
		Maturity:   models.FeatureMaturityBeta,
	}).Error; err != nil {
		t.Fatal(err)
	}
	status, body = runGetWithToken(t, app, "/api/global-search?q=task", token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}
	if !containsString(stub.gotReq.AllowedEntityTypes, "task") {
		t.Fatalf("task should be searchable after feature enable and role permission: %+v", stub.gotReq.AllowedEntityTypes)
	}
}

func TestHandleGlobalSearchSanitizesSensitiveReturnedCandidates(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Search Sanitizer Co")
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

	stub := &stubEngineForHandler{
		mode: search_engine.ModeEnt,
		resp: &search_engine.SearchResponse{
			Source: "ranked",
			Candidates: []search_engine.Candidate{
				{
					ID:         "10",
					Primary:    "Payroll Run PAY-10",
					GroupKey:   search_engine.GroupPayroll,
					GroupLabel: "Payroll",
					ActionKind: search_engine.ActionNavigate,
					URL:        "/payroll/runs/10",
					EntityType: "payroll_run",
					Payload:    map[string]string{"amount": "1234.56", "currency": "CAD", "status": "posted", "doc_num": "PAY-10"},
				},
				{
					ID:         "11",
					Primary:    "Jane Private",
					GroupKey:   search_engine.GroupPayroll,
					GroupLabel: "Payroll",
					ActionKind: search_engine.ActionNavigate,
					URL:        "/payroll/runs/10#entry-11",
					EntityType: "payroll_entry",
					Payload:    map[string]string{"amount": "777.77", "currency": "CAD"},
				},
			},
		},
	}
	sel := search_engine.NewSelector(search_engine.ModeEnt, search_engine.NewLegacyEngine(), nil, stub)
	app := newAuthenticatedGlobalSearchTestApp(db, sel)

	status, body := runGetWithToken(t, app, "/api/global-search?q=pay", token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}
	var got struct {
		Candidates []globalSearchCandidate `json:"candidates"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("expected only allowed payroll_run candidate, got %+v", got.Candidates)
	}
	if got.Candidates[0].EntityType != "payroll_run" {
		t.Fatalf("expected payroll_run, got %+v", got.Candidates[0])
	}
	if _, ok := got.Candidates[0].Payload["amount"]; ok {
		t.Fatalf("payroll amount should be redacted without details permission: %+v", got.Candidates[0].Payload)
	}
	if _, ok := got.Candidates[0].Payload["currency"]; ok {
		t.Fatalf("payroll currency should be redacted with amount: %+v", got.Candidates[0].Payload)
	}
}

func newAuthenticatedGlobalSearchTestApp(db *gorm.DB, sel *search_engine.Selector) *fiber.App {
	app := fiber.New()
	s := &Server{DB: db, SearchSelector: sel}
	app.Get("/api/global-search", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleGlobalSearch)
	return app
}

func runGetWithToken(t *testing.T, app *fiber.App, url string, rawToken string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if rawToken != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: rawToken, Path: "/"})
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}
