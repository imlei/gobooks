package web

import (
	"net/http"
	"strings"
	"testing"

	"balanciz/internal/models"
	"balanciz/internal/services"
)

func TestTasksMonthlyReportInvalidPeriodRendersSafeNavigation(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Task Monthly Invalid Period Co")
	user, rawToken := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)
	app := testRouteApp(t, db)

	resp := performRequest(t, app, "/tasks/monthly-report?year=2026&month=13", rawToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := readResponseBody(t, resp)
	if !strings.Contains(body, services.ErrTaskReportMonthInvalid.Error()) {
		t.Fatalf("expected invalid month message, got %q", body)
	}
	for _, forbidden := range []string{"%!Month", "year=0", "month=0", "month=13"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("expected safe month navigation without %q, got %q", forbidden, body)
		}
	}
}

func TestTasksMonthlyReportRequiresYearAndMonthTogether(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Task Monthly Partial Period Co")
	user, rawToken := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)
	app := testRouteApp(t, db)

	resp := performRequest(t, app, "/tasks/monthly-report?year=2026", rawToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := readResponseBody(t, resp)
	if !strings.Contains(body, "year and month are required together") {
		t.Fatalf("expected paired period error, got %q", body)
	}
}

func TestTasksMonthlyReportUsesRequestedPeriod(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Task Monthly Valid Period Co")
	user, rawToken := seedUserSession(t, db, &companyID)
	seedMembership(t, db, user.ID, companyID)
	customerID := seedValidationCustomer(t, db, companyID, "Monthly Customer")
	_ = seedTaskForWeb(t, db, companyID, customerID, models.TaskStatusCompleted, "Monthly report task")
	app := testRouteApp(t, db)

	resp := performRequest(t, app, "/tasks/monthly-report?year=2026&month=4", rawToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := readResponseBody(t, resp)
	if !strings.Contains(body, "April 2026") {
		t.Fatalf("expected requested month in report, got %q", body)
	}
}
