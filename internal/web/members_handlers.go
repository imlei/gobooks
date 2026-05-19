// 遵循project_guide.md
package web

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"balanciz/internal/models"
	"balanciz/internal/services"
	"balanciz/internal/web/templates/pages"
)

func (s *Server) handleMembersGet(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	memberships, err := s.loadActiveCompanyMemberships(companyID)
	if err != nil {
		return pages.Members(pages.MembersVM{
			HasCompany: true,
			Active:     "Members Settings",
			FormError:  "Could not load members.",
		}).Render(c.Context(), c)
	}
	moduleStates := s.memberPermissionModuleStates(companyID)
	memberRows := s.memberRows(companyID, memberships, !OwnerOrAdminFromCtx(c), moduleStates)

	invs, err := services.ListPendingInvitationsForCompany(s.DB, companyID)
	if err != nil {
		return pages.Members(pages.MembersVM{
			HasCompany: true,
			Active:     "Members Settings",
			FormError:  "Could not load invitations.",
		}).Render(c.Context(), c)
	}

	now := time.Now()
	invRows := make([]pages.InvitationRow, 0, len(invs))
	for _, inv := range invs {
		by := inv.InvitedBy.Email
		if strings.TrimSpace(by) == "" {
			by = "—"
		}
		invRows = append(invRows, pages.InvitationRow{
			Email:     inv.Email,
			Role:      string(inv.Role),
			Expires:   inv.ExpiresAt.Format("2006-01-02 15:04"),
			InvitedBy: by,
			Created:   inv.CreatedAt.Format("2006-01-02"),
			IsExpired: now.After(inv.ExpiresAt),
		})
	}

	return pages.Members(pages.MembersVM{
		HasCompany:  true,
		Active:      "Members Settings",
		ReadOnly:    !OwnerOrAdminFromCtx(c),
		Members:     memberRows,
		Invitations: invRows,
		Created:     c.Query("created") == "1",
		Saved:       c.Query("saved") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleMembersInvitePost(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	if !OwnerOrAdminFromCtx(c) {
		return fiber.NewError(fiber.StatusForbidden, "Forbidden")
	}

	email := strings.TrimSpace(c.FormValue("email"))
	roleRaw := strings.TrimSpace(c.FormValue("role"))

	vm := pages.MembersVM{
		HasCompany: true,
		Active:     "Members Settings",
		ReadOnly:   false,
		Email:      email,
		Role:       roleRaw,
	}

	if email == "" {
		vm.EmailError = "Email is required."
	} else if !strings.Contains(email, "@") {
		vm.EmailError = "Enter a valid email address."
	}

	var role models.CompanyRole
	if roleRaw == "" {
		vm.RoleError = "Role is required."
	} else {
		var err error
		role, err = models.ParseCompanyRole(roleRaw)
		if err != nil {
			vm.RoleError = "Invalid role."
		}
	}

	if vm.EmailError != "" || vm.RoleError != "" {
		return s.renderMembersPageWithErrors(c, companyID, vm)
	}

	inv, _, err := services.CreateCompanyInvitation(s.DB, companyID, user.ID, email, role)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvitationDuplicate):
			vm.FormError = "A pending invitation already exists for this email."
		case errors.Is(err, services.ErrInvitationAlreadyMember):
			vm.FormError = "This user is already a member of the company."
		case errors.Is(err, services.ErrInvitationInvalidRole):
			vm.FormError = "Invitations cannot assign the owner role."
		default:
			vm.FormError = "Could not create invitation. Please try again."
		}
		return s.renderMembersPageWithErrors(c, companyID, vm)
	}

	cid := companyID
	uid := user.ID
	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	services.TryWriteAuditLogWithContext(s.DB, "invitation.created", "company_invitation", 0, actor, map[string]any{
		"invitation_id": inv.ID.String(),
		"email":         inv.Email,
		"role":          string(inv.Role),
		"expires_at":    inv.ExpiresAt.Format(time.RFC3339),
		"company_id":    companyID,
	}, &cid, &uid)

	return c.Redirect("/settings/members?created=1", fiber.StatusSeeOther)
}

func (s *Server) handleMemberPermissionsPost(c *fiber.Ctx) error {
	actor := UserFromCtx(c)
	if actor == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	if !OwnerOrAdminFromCtx(c) {
		return fiber.NewError(fiber.StatusForbidden, "Forbidden")
	}

	targetUserID, err := uuid.Parse(strings.TrimSpace(c.FormValue("user_id")))
	if err != nil {
		return s.renderMembersPageWithErrors(c, companyID, pages.MembersVM{FormError: "Invalid member."})
	}

	var membership models.CompanyMembership
	if err := s.DB.Where("company_id = ? AND user_id = ? AND is_active = ?", companyID, targetUserID, true).
		First(&membership).Error; err != nil {
		return s.renderMembersPageWithErrors(c, companyID, pages.MembersVM{FormError: "Member not found."})
	}
	if membership.Role == models.CompanyRoleOwner {
		return s.renderMembersPageWithErrors(c, companyID, pages.MembersVM{FormError: "Owner permissions cannot be changed."})
	}

	permissions := memberModulePermissions()
	validPermissions := make([]string, 0, len(permissions))
	for _, def := range permissions {
		validPermissions = append(validPermissions, def.Permission)
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ? AND user_id = ? AND permission IN ?", companyID, targetUserID, validPermissions).
			Delete(&models.UserCompanyPermission{}).Error; err != nil {
			return err
		}
		for _, def := range permissions {
			value := strings.TrimSpace(c.FormValue("permission_" + def.Permission))
			if value == "" || value == "default" {
				continue
			}
			if value != "grant" && value != "deny" {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid permission setting")
			}
			row := models.UserCompanyPermission{
				UserID:     targetUserID,
				CompanyID:  companyID,
				Permission: def.Permission,
				Granted:    value == "grant",
				GrantedBy:  actor.ID,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return s.renderMembersPageWithErrors(c, companyID, pages.MembersVM{FormError: "Could not save member permissions."})
	}

	cid := companyID
	uid := actor.ID
	actorEmail := actor.Email
	if actorEmail == "" {
		actorEmail = "user"
	}
	services.TryWriteAuditLogWithContext(s.DB, "member.permissions_updated", "user_company_permission", 0, actorEmail, map[string]any{
		"target_user_id": targetUserID.String(),
		"company_id":     companyID,
	}, &cid, &uid)

	return c.Redirect("/settings/members?saved=1", fiber.StatusSeeOther)
}

func (s *Server) renderMembersPageWithErrors(c *fiber.Ctx, companyID uint, vm pages.MembersVM) error {
	memberships, _ := s.loadActiveCompanyMemberships(companyID)
	moduleStates := s.memberPermissionModuleStates(companyID)
	memberRows := s.memberRows(companyID, memberships, !OwnerOrAdminFromCtx(c), moduleStates)

	invs, _ := services.ListPendingInvitationsForCompany(s.DB, companyID)
	now := time.Now()
	invRows := make([]pages.InvitationRow, 0, len(invs))
	for _, inv := range invs {
		by := inv.InvitedBy.Email
		if strings.TrimSpace(by) == "" {
			by = "—"
		}
		invRows = append(invRows, pages.InvitationRow{
			Email:     inv.Email,
			Role:      string(inv.Role),
			Expires:   inv.ExpiresAt.Format("2006-01-02 15:04"),
			InvitedBy: by,
			Created:   inv.CreatedAt.Format("2006-01-02"),
			IsExpired: now.After(inv.ExpiresAt),
		})
	}

	vm.HasCompany = true
	vm.Active = "Members Settings"
	vm.Members = memberRows
	vm.Invitations = invRows
	vm.ReadOnly = !OwnerOrAdminFromCtx(c)

	return pages.Members(vm).Render(c.Context(), c)
}

func (s *Server) loadActiveCompanyMemberships(companyID uint) ([]models.CompanyMembership, error) {
	var memberships []models.CompanyMembership
	if err := s.DB.Preload("User").Where("company_id = ? AND is_active = ?", companyID, true).Find(&memberships).Error; err != nil {
		return nil, err
	}
	sort.Slice(memberships, func(i, j int) bool {
		return strings.ToLower(memberships[i].User.Email) < strings.ToLower(memberships[j].User.Email)
	})
	return memberships, nil
}

type memberModulePermissionDef struct {
	Group      string
	Permission string
	Label      string
}

func memberModulePermissions() []memberModulePermissionDef {
	return []memberModulePermissionDef{
		{Group: "Task", Permission: PermTaskAccess, Label: "View tasks"},
		{Group: "Task", Permission: PermTaskCreate, Label: "Create tasks"},
		{Group: "Task", Permission: PermTaskUpdate, Label: "Update, complete, and cancel tasks"},
		{Group: "Task", Permission: PermTaskBill, Label: "Generate invoices from tasks"},
		{Group: "Employee", Permission: PermEmployeeView, Label: "View employees"},
		{Group: "Employee", Permission: PermEmployeeManage, Label: "Manage employees"},
		{Group: "Employee", Permission: PermEmployeeSensitive, Label: "View sensitive employee data"},
		{Group: "Payroll", Permission: PermPayrollView, Label: "View payroll runs"},
		{Group: "Payroll", Permission: PermPayrollDetails, Label: "View payroll details"},
		{Group: "Payroll", Permission: PermPayrollRun, Label: "Create and calculate payroll"},
		{Group: "Payroll", Permission: PermPayrollFinalize, Label: "Finalize, post, remit, and void payroll"},
		{Group: "Payroll", Permission: PermPayrollExport, Label: "Export payroll CSV"},
		{Group: "Payroll", Permission: PermPayrollSettings, Label: "Manage payroll settings"},
		{Group: "Cheque", Permission: PermChequeView, Label: "View cheques"},
		{Group: "Cheque", Permission: PermChequePrint, Label: "Create, print, and void cheques"},
		{Group: "Cheque", Permission: PermChequeManageBank, Label: "Manage cheque bank accounts"},
	}
}

func (s *Server) memberRows(companyID uint, memberships []models.CompanyMembership, readOnly bool, moduleStates map[string]bool) []pages.MemberRow {
	overrideByUser := s.memberPermissionOverrideMap(companyID)
	rows := make([]pages.MemberRow, 0, len(memberships))
	for _, m := range memberships {
		rows = append(rows, pages.MemberRow{
			UserID:             m.UserID.String(),
			Email:              m.User.Email,
			Role:               string(m.Role),
			Since:              m.CreatedAt.Format("2006-01-02"),
			IsOwner:            m.Role == models.CompanyRoleOwner,
			CanEditPermissions: !readOnly && m.Role != models.CompanyRoleOwner,
			PermissionGroups:   memberPermissionGroups(m.Role, overrideByUser[m.UserID], moduleStates),
		})
	}
	return rows
}

func (s *Server) memberPermissionModuleStates(companyID uint) map[string]bool {
	return map[string]bool{
		"Task":     s.memberFeatureEnabled(companyID, models.FeatureKeyTask),
		"Employee": s.memberFeatureEnabled(companyID, models.FeatureKeyEmployee),
		"Payroll":  s.memberFeatureEnabled(companyID, models.FeatureKeyPayroll),
		"Cheque":   s.memberFeatureEnabled(companyID, models.FeatureKeyCheque),
	}
}

func (s *Server) memberFeatureEnabled(companyID uint, key models.FeatureKey) bool {
	enabled, err := services.IsCompanyFeatureEnabled(s.DB, companyID, key)
	return err == nil && enabled
}

func (s *Server) memberPermissionOverrideMap(companyID uint) map[uuid.UUID]map[string]string {
	out := map[uuid.UUID]map[string]string{}
	if !s.DB.Migrator().HasTable(&models.UserCompanyPermission{}) {
		return out
	}
	var rows []models.UserCompanyPermission
	if err := s.DB.Where("company_id = ?", companyID).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		if _, ok := out[row.UserID]; !ok {
			out[row.UserID] = map[string]string{}
		}
		if row.Granted {
			out[row.UserID][row.Permission] = "grant"
		} else {
			out[row.UserID][row.Permission] = "deny"
		}
	}
	return out
}

func memberPermissionGroups(role models.CompanyRole, overrides map[string]string, moduleStates map[string]bool) []pages.MemberPermissionGroup {
	groups := []pages.MemberPermissionGroup{
		memberPermissionGroup("Task", moduleStates),
		memberPermissionGroup("Employee", moduleStates),
		memberPermissionGroup("Payroll", moduleStates),
		memberPermissionGroup("Cheque", moduleStates),
	}
	groupIndex := map[string]int{
		"Task":     0,
		"Employee": 1,
		"Payroll":  2,
		"Cheque":   3,
	}
	for _, def := range memberModulePermissions() {
		value := "default"
		if overrides != nil {
			if override, ok := overrides[def.Permission]; ok {
				value = override
			}
		}
		idx := groupIndex[def.Group]
		groups[idx].Permissions = append(groups[idx].Permissions, pages.MemberPermissionOption{
			Permission:  def.Permission,
			Label:       def.Label,
			Value:       value,
			RoleDefault: HasPermission(string(role), def.Permission),
		})
	}
	return groups
}

func memberPermissionGroup(label string, moduleStates map[string]bool) pages.MemberPermissionGroup {
	enabled := false
	if moduleStates != nil {
		enabled = moduleStates[label]
	}
	group := pages.MemberPermissionGroup{
		Label:          label,
		FeatureEnabled: enabled,
	}
	if enabled {
		group.StatusText = "Enabled"
		group.Note = "These permissions affect visible module routes and search results."
	} else {
		group.StatusText = "Feature off"
		group.Note = "You can prepare permissions now, but the module stays hidden until enabled in Company Features."
	}
	return group
}
