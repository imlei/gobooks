package web

import (
	"github.com/gofiber/fiber/v2"

	"balanciz/internal/models"
	"balanciz/internal/services"
	"balanciz/internal/services/search_engine"
	"balanciz/internal/web/templates/pages"
)

var baseGlobalSearchEntityTypes = []string{
	"invoice",
	"bill",
	"quote",
	"sales_order",
	"purchase_order",
	"customer_receipt",
	"expense",
	"journal_entry",
	"credit_note",
	"vendor_credit_note",
	"ar_return",
	"vendor_return",
	"ar_refund",
	"vendor_refund",
	"customer_deposit",
	"vendor_prepayment",
	"customer",
	"vendor",
	"product_service",
}

func (s *Server) allowedGlobalSearchEntityTypes(c *fiber.Ctx) []string {
	if MembershipFromCtx(c) == nil {
		return nil
	}

	allowed := append([]string{}, baseGlobalSearchEntityTypes...)
	if s.searchFeatureEnabled(c, models.FeatureKeyTask) && CanFromCtx(c, ActionTaskView) {
		allowed = append(allowed, "task")
	}
	if s.searchFeatureEnabled(c, models.FeatureKeyEmployee) && CanFromCtx(c, ActionEmployeeView) {
		allowed = append(allowed, "employee")
	}
	if s.searchFeatureEnabled(c, models.FeatureKeyPayroll) {
		if CanFromCtx(c, ActionPayrollView) {
			allowed = append(allowed, "payroll_run")
		}
		if CanFromCtx(c, ActionPayrollViewDetails) {
			allowed = append(allowed, "payroll_entry", "payroll_remittance")
		}
	}
	if s.searchFeatureEnabled(c, models.FeatureKeyCheque) && CanFromCtx(c, ActionChequeView) {
		allowed = append(allowed, "cheque")
	}
	return allowed
}

func (s *Server) searchFeatureEnabled(c *fiber.Ctx, key models.FeatureKey) bool {
	if s == nil || s.DB == nil {
		return false
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return false
	}
	enabled, err := services.IsCompanyFeatureEnabled(s.DB, companyID, key)
	return err == nil && enabled
}

func entityTypeAllowed(entityType string, allowed []string) bool {
	if entityType == "" || allowed == nil {
		return true
	}
	for _, t := range allowed {
		if t == entityType {
			return true
		}
	}
	return false
}

func filterAdvancedSearchEntityOptions(options []pages.EntityTypeOption, allowed []string) []pages.EntityTypeOption {
	if allowed == nil {
		return options
	}
	out := make([]pages.EntityTypeOption, 0, len(options))
	for _, opt := range options {
		if entityTypeAllowed(opt.Value, allowed) {
			out = append(out, opt)
		}
	}
	return out
}

func (s *Server) sanitizeSearchCandidatesForContext(c *fiber.Ctx, allowed []string, rows []search_engine.Candidate) []search_engine.Candidate {
	if len(rows) == 0 {
		return rows
	}
	out := make([]search_engine.Candidate, 0, len(rows))
	for _, row := range rows {
		if !entityTypeAllowed(row.EntityType, allowed) {
			continue
		}
		out = append(out, sanitizeSearchCandidatePayload(c, row))
	}
	return out
}

func sanitizeSearchCandidatePayload(c *fiber.Ctx, row search_engine.Candidate) search_engine.Candidate {
	if !searchCandidateRequiresPayrollDetails(row.EntityType) || CanFromCtx(c, ActionPayrollViewDetails) {
		return row
	}
	if len(row.Payload) == 0 {
		return row
	}
	payload := make(map[string]string, len(row.Payload))
	for k, v := range row.Payload {
		switch k {
		case "amount", "currency":
			continue
		default:
			payload[k] = v
		}
	}
	row.Payload = payload
	return row
}

func searchCandidateRequiresPayrollDetails(entityType string) bool {
	switch entityType {
	case "payroll_run", "payroll_entry", "payroll_remittance":
		return true
	default:
		return false
	}
}
