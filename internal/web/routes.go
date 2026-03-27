// 遵循产品需求 v1.0
package web

import (
	"github.com/gofiber/fiber/v2"

	"gobooks/internal/models"
)

func (s *Server) registerRoutes(app *fiber.App) {
	// Static assets (Tailwind output).
	app.Static("/static", "internal/web/static")

	// Basic routes.
	app.Get("/", s.handleDashboard)

	// Setup wizard (first-run).
	app.Get("/setup/bootstrap", s.handleBootstrapForm)
	app.Post("/setup/bootstrap", s.handleBootstrapSubmit)
	app.Get("/setup", s.handleSetupForm)
	app.Post("/setup", s.LoadSession(), s.handleSetupSubmit)

	// Auth (email + password).
	app.Get("/login", s.handleLoginForm)
	app.Post("/login", s.handleLoginPost)
	app.Post("/logout", s.handleLogoutPost)

	// Company selection (users with multiple active memberships).
	app.Get("/select-company", s.LoadSession(), s.RequireAuth(), s.handleSelectCompanyGet)
	app.Post("/select-company", s.LoadSession(), s.RequireAuth(), s.handleSelectCompanyPost)

	// Settings (after setup).
	app.Get("/settings/company/profile", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCompanyProfileForm)
	app.Post("/settings/company/profile", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCompanyProfileSubmit)
	app.Get("/settings/company/templates", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCompanyTemplatesGet)
	app.Get("/settings/company/sales-tax", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCompanySalesTaxGet)
	app.Get("/settings/company/numbering", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleNumberingSettingsGet)
	app.Post("/settings/company/numbering", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleNumberingSettingsPost)
	app.Get("/settings/company", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCompanyHub)
	app.Post("/settings/company", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCompanyProfileSubmit)
	// Backward compatibility: old numbering URL (POST only; GET redirects to canonical path).
	app.Post("/settings/numbering", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleNumberingSettingsPost)
	app.Get("/settings/numbering", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), func(c *fiber.Ctx) error {
		return c.Redirect("/settings/company/numbering", fiber.StatusSeeOther)
	})
	app.Get("/settings/ai-connect", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAIConnectGet)
	app.Post("/settings/ai-connect", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.RequireRole(models.CompanyRoleOwner, models.CompanyRoleAdmin), s.handleAIConnectPost)
	app.Post("/settings/ai-connect/test", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.RequireRole(models.CompanyRoleOwner, models.CompanyRoleAdmin), s.handleAIConnectTestPost)
	app.Get("/settings/members", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleMembersGet)
	app.Post("/settings/members/invite", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.RequireRole(models.CompanyRoleOwner, models.CompanyRoleAdmin), s.handleMembersInvitePost)
	app.Get("/settings/audit-log", s.handleAuditLog)

	// Chart of Accounts (company-scoped; multi-tenant middleware).
	app.Get("/accounts", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAccounts)
	app.Post("/accounts", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAccountCreate)
	app.Post("/accounts/update", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAccountUpdate)
	app.Post("/accounts/inactive", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAccountInactive)
	app.Post("/accounts/suggestions", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAccountSuggestions)
	app.Post("/api/ai/recommend/account", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAIRecommendAccount)
	app.Post("/api/accounts/recommendations", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleAccountRecommendations)

	// Journal Entry (company-scoped; multi-tenant middleware).
	app.Get("/journal-entry", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleJournalEntryForm)
	app.Post("/journal-entry", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleJournalEntryPost)
	app.Get("/journal-entry/list", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleJournalEntryList)
	app.Post("/journal-entry/:id/reverse", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleJournalEntryReverse)

	// Invoices & Bills.
	app.Get("/invoices", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleInvoices)
	app.Post("/invoices", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleInvoiceCreate)
	app.Get("/bills", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleBills)
	app.Post("/bills", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleBillCreate)

	// Reports.
	app.Get("/reports", func(c *fiber.Ctx) error { return c.Redirect("/reports/trial-balance", fiber.StatusSeeOther) })
	app.Get("/reports/trial-balance", s.handleTrialBalance)
	app.Get("/reports/income-statement", s.handleIncomeStatement)
	app.Get("/reports/balance-sheet", s.handleBalanceSheet)

	// Contacts (Customers / Vendors).
	app.Get("/customers", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCustomers)
	app.Post("/customers", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleCustomerCreate)
	app.Get("/vendors", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleVendors)
	app.Post("/vendors", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleVendorCreate)

	// Banking (company-scoped; multi-tenant middleware).
	app.Get("/banking/reconcile", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleBankReconcileForm)
	app.Post("/banking/reconcile", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleBankReconcileSubmit)
	app.Get("/banking/receive-payment", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleReceivePaymentForm)
	app.Post("/banking/receive-payment", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handleReceivePaymentSubmit)
	app.Get("/banking/pay-bills", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handlePayBillsForm)
	app.Post("/banking/pay-bills", s.LoadSession(), s.RequireAuth(), s.ResolveActiveCompany(), s.RequireMembership(), s.handlePayBillsSubmit)
}

