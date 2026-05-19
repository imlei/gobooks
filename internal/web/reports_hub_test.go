package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"balanciz/internal/models"
	"balanciz/internal/services"
)

func TestReportsHubHighlightsCorePackage(t *testing.T) {
	db := testJournalRouteDB(t)
	app := testRouteApp(t, db)
	companyID := seedCompany(t, db, "Reports Hub Co")
	user, token := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)

	resp := performRequest(t, app, "/reports", token)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected Reports hub, got %d", resp.StatusCode)
	}
	body := readResponseBody(t, resp)
	for _, want := range []string{
		"Core Report Package",
		"Profit &amp; Loss (Income Statement)",
		"Balance Sheet",
		"Cash Flow Summary",
		"Trial Balance",
		"General Ledger",
		"Journal Entries",
		"Interactive",
		"Drill-through",
		"/reports/income-statement/export.csv",
		"/reports/balance-sheet/export.csv",
		"/reports/trial-balance/export.csv",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected Reports hub to contain %q, got %q", want, body)
		}
	}
}

func TestReportsHubHidesPayrollReportsUntilFeatureAndDetailPermission(t *testing.T) {
	db := testJournalRouteDB(t)
	app := testRouteApp(t, db)
	companyID := seedCompany(t, db, "Reports Hub Payroll Co")
	user, token := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)
	if err := db.AutoMigrate(&models.CompanyFeature{}); err != nil {
		t.Fatal(err)
	}

	resp := performRequest(t, app, "/reports", token)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected Reports hub, got %d", resp.StatusCode)
	}
	body := readResponseBody(t, resp)
	if strings.Contains(body, "Payroll Summary") || strings.Contains(body, "/payroll/reports/summary") {
		t.Fatalf("payroll reports should be hidden while payroll feature is off:\n%s", body)
	}

	if err := db.Create(&models.CompanyFeature{
		CompanyID:  companyID,
		FeatureKey: models.FeatureKeyPayroll,
		Status:     models.FeatureStatusEnabled,
		Maturity:   models.FeatureMaturityAlpha,
	}).Error; err != nil {
		t.Fatal(err)
	}

	resp = performRequest(t, app, "/reports", token)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected Reports hub, got %d", resp.StatusCode)
	}
	body = readResponseBody(t, resp)
	for _, want := range []string{
		"Payroll",
		"Payroll Summary",
		"Employee Payroll History",
		"/payroll/reports/summary",
		"/payroll/reports/employee-history",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected Reports hub to contain %q, got %q", want, body)
		}
	}
}

func TestReportsHubHidesTaskReportsUntilFeatureAndTaskPermission(t *testing.T) {
	db := testJournalRouteDB(t)
	app := testRouteApp(t, db)
	companyID := seedCompany(t, db, "Reports Hub Task Co")
	user, token := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)
	if err := db.AutoMigrate(&models.CompanyFeature{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.CompanyFeature{}).
		Where("company_id = ? AND feature_key = ?", companyID, models.FeatureKeyTask).
		Update("status", models.FeatureStatusOff).Error; err != nil {
		t.Fatal(err)
	}

	resp := performRequest(t, app, "/reports", token)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected Reports hub, got %d", resp.StatusCode)
	}
	body := readResponseBody(t, resp)
	if strings.Contains(body, "Task Monthly Summary") || strings.Contains(body, "/tasks/monthly-report") {
		t.Fatalf("task reports should be hidden while task feature is off:\n%s", body)
	}

	if err := db.Model(&models.CompanyFeature{}).
		Where("company_id = ? AND feature_key = ?", companyID, models.FeatureKeyTask).
		Update("status", models.FeatureStatusEnabled).Error; err != nil {
		t.Fatal(err)
	}

	resp = performRequest(t, app, "/reports", token)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected Reports hub, got %d", resp.StatusCode)
	}
	body = readResponseBody(t, resp)
	for _, want := range []string{
		"Tasks",
		"Task Monthly Summary",
		"Billable Work Report",
		"/tasks/monthly-report",
		"/tasks/billable-work/report",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected Reports hub to contain %q, got %q", want, body)
		}
	}
}

func TestReportsHubPayrollCSVCapabilityRequiresPayrollExport(t *testing.T) {
	server := &Server{}
	entries := server.filterReportsForHub([]services.ReportEntry{{
		Key:             "payroll-summary",
		Title:           "Payroll Summary",
		Href:            "/payroll/reports/summary",
		Category:        services.ReportCategoryPayroll,
		Mode:            "Period",
		CSVHref:         "/payroll/reports/summary/export.csv",
		RequiredFeature: models.FeatureKeyPayroll,
		RequiredAction:  ActionPayrollViewDetails,
		CSVAction:       ActionPayrollExport,
	}}, reportHubVisibility{
		EnabledFeatures: map[models.FeatureKey]bool{
			models.FeatureKeyPayroll: true,
		},
		AllowedActions: map[string]bool{
			ActionPayrollViewDetails: true,
			ActionPayrollExport:      false,
		},
	})
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	item := buildReportsHubItems(entries)[0]
	if item.CSVHref != "" {
		t.Fatalf("payroll CSV href should be hidden without export permission: %+v", item)
	}
	for _, capability := range item.Capabilities {
		if capability == "CSV" {
			t.Fatalf("payroll CSV capability should be hidden without export permission: %+v", item.Capabilities)
		}
	}
}

func TestReportsHubModuleReportRequiresAction(t *testing.T) {
	server := &Server{}
	entries := server.filterReportsForHub([]services.ReportEntry{{
		Key:             "task-monthly-summary",
		Title:           "Task Monthly Summary",
		Href:            "/tasks/monthly-report",
		Category:        services.ReportCategoryTasks,
		RequiredFeature: models.FeatureKeyTask,
		RequiredAction:  ActionTaskView,
	}}, reportHubVisibility{
		EnabledFeatures: map[models.FeatureKey]bool{
			models.FeatureKeyTask: true,
		},
		AllowedActions: map[string]bool{
			ActionTaskView: false,
		},
	})
	if len(entries) != 0 {
		t.Fatalf("task report should be hidden without task view permission: %+v", entries)
	}
}

func TestReportsHubFavouriteToggleRejectsHiddenModuleReports(t *testing.T) {
	db := testJournalRouteDB(t)
	app := testRouteApp(t, db)
	companyID := seedCompany(t, db, "Reports Hub Hidden Favourite Co")
	user, token := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)
	if err := db.AutoMigrate(&models.CompanyFeature{}, &models.ReportFavourite{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.CompanyFeature{}).
		Where("company_id = ? AND feature_key = ?", companyID, models.FeatureKeyTask).
		Update("status", models.FeatureStatusOff).Error; err != nil {
		t.Fatal(err)
	}

	resp := performReportFavouriteToggle(t, app, token, url.Values{
		"report_key": {"task-monthly-summary"},
	})
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	favs, err := services.ListUserReportFavourites(db, user.ID, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if favs["task-monthly-summary"] {
		t.Fatal("hidden task report should not be stored as a favourite")
	}

	resp = performReportFavouriteToggle(t, app, token, url.Values{
		"report_key": {"balance-sheet"},
	})
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	favs, err = services.ListUserReportFavourites(db, user.ID, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if !favs["balance-sheet"] {
		t.Fatal("visible report should still be stored as a favourite")
	}
}

func performReportFavouriteToggle(t *testing.T, app *fiber.App, rawToken string, form url.Values) *http.Response {
	t.Helper()

	csrf := newCSRFToken(t)
	form.Set(CSRFFormField, csrf)
	req, err := http.NewRequest(http.MethodPost, "/reports/favourites/toggle", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: rawToken, Path: "/"})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf, Path: "/"})
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestReportsHubShowsFavouriteWithCapabilities(t *testing.T) {
	db := testJournalRouteDB(t)
	app := testRouteApp(t, db)
	companyID := seedCompany(t, db, "Reports Hub Fav Co")
	user, token := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)
	if err := db.AutoMigrate(&models.ReportFavourite{}); err != nil {
		t.Fatal(err)
	}

	starred, err := services.ToggleReportFavourite(db, user.ID, companyID, "balance-sheet")
	if err != nil {
		t.Fatal(err)
	}
	if !starred {
		t.Fatal("expected balance-sheet to be starred")
	}

	resp := performRequest(t, app, "/reports", token)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected Reports hub, got %d", resp.StatusCode)
	}
	body := readResponseBody(t, resp)
	for _, want := range []string{
		"Favourites",
		"Balance Sheet",
		"As-of",
		"CSV",
		"Remove from favourites",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected Reports hub favourite view to contain %q, got %q", want, body)
		}
	}
}
