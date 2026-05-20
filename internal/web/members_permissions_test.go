package web

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

func TestMemberPermissionOverridesGrantModuleAccess(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Member Permissions Co")

	admin, adminToken := seedUserSession(t, db, &companyID)
	seedMembership(t, db, admin.ID, companyID)

	target, targetToken := seedUserSessionForEmail(t, db, &companyID, "payroll-viewer@example.com")
	targetMembership := models.CompanyMembership{
		ID:        uuid.New(),
		UserID:    target.ID,
		CompanyID: companyID,
		Role:      models.CompanyRoleViewer,
		IsActive:  true,
	}
	if err := db.Create(&targetMembership).Error; err != nil {
		t.Fatal(err)
	}

	probe := testPermissionProbeApp(db, ActionPayrollView)
	before := performRequest(t, probe, "/probe", targetToken)
	if before.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer should not have payroll access before grant, got %d", before.StatusCode)
	}
	sensitiveProbe := testPermissionProbeApp(db, ActionSettingsSensitiveView)
	sensitiveBefore := performRequest(t, sensitiveProbe, "/probe", targetToken)
	if sensitiveBefore.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer should not have sensitive settings access before grant, got %d", sensitiveBefore.StatusCode)
	}

	app := testMemberPermissionsApp(db)
	resp := performFormRequest(t, app, http.MethodPost, "/settings/members/permissions", url.Values{
		"user_id": {target.ID.String()},
		"permission_" + PermViewSensitiveSettings: {"grant"},
		"permission_" + PermPayrollView:           {"grant"},
		"permission_" + PermPayrollExport:         {"deny"},
	}, adminToken)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("save permissions status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if got := resp.Header.Get("Location"); got != "/settings/members?saved=1" {
		t.Fatalf("redirect = %q", got)
	}

	var rows []models.UserCompanyPermission
	if err := db.Where("company_id = ? AND user_id = ?", companyID, target.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("permission rows = %d, want 3: %+v", len(rows), rows)
	}

	after := performRequest(t, probe, "/probe", targetToken)
	if after.StatusCode != http.StatusOK {
		t.Fatalf("viewer should have payroll access after grant, got %d", after.StatusCode)
	}
	sensitiveAfter := performRequest(t, sensitiveProbe, "/probe", targetToken)
	if sensitiveAfter.StatusCode != http.StatusOK {
		t.Fatalf("viewer should have sensitive settings access after grant, got %d", sensitiveAfter.StatusCode)
	}
}

func TestMembersPageShowsModuleFeatureAvailability(t *testing.T) {
	db := testRouteDB(t)
	companyID := seedCompany(t, db, "Member Module Status Co")
	admin, adminToken := seedUserSession(t, db, &companyID)
	seedMembership(t, db, admin.ID, companyID)

	app := testRouteApp(t, db)
	resp := performRequest(t, app, "/settings/members", adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("members page status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	for _, want := range []string{
		"Module permissions combine with Company Features",
		"routes, navigation, and search stay hidden",
		"Settings",
		"View sensitive settings",
		"Task",
		"Enabled",
		"Payroll",
		"Feature off",
		"You can prepare permissions now",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected members page to contain %q", want)
		}
	}
}

func seedUserSessionForEmail(t *testing.T, db *gorm.DB, activeCompanyID *uint, email string) (models.User, string) {
	t.Helper()
	user := models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "not-used-in-route-tests",
		DisplayName:  "Permission Target",
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

func testPermissionProbeApp(db *gorm.DB, action string) *fiber.App {
	s := &Server{DB: db}
	app := fiber.New()
	app.Get("/probe", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.RequirePermission(action), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})
	return app
}

func testMemberPermissionsApp(db *gorm.DB) *fiber.App {
	s := &Server{DB: db}
	app := fiber.New()
	app.Post("/settings/members/permissions", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.RequirePermission(ActionMemberManage), s.handleMemberPermissionsPost)
	return app
}
