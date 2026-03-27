// 遵循产品需求 v1.0
package web

import "github.com/gofiber/fiber/v2"

func (s *Server) registerRoutes(app *fiber.App) {
	// Static assets (Tailwind output).
	app.Static("/static", "internal/web/static")

	// Basic routes.
	app.Get("/", s.handleDashboard)

	// Setup wizard (first-run).
	app.Get("/setup", s.handleSetupForm)
	app.Post("/setup", s.handleSetupSubmit)

	// Settings (after setup).
	app.Get("/settings/company", s.handleCompanySettingsForm)
	app.Post("/settings/company", s.handleCompanySettingsSubmit)
	app.Get("/settings/numbering", s.handleNumberingSettingsGet)
	app.Post("/settings/numbering", s.handleNumberingSettingsPost)
	app.Get("/settings/ai-connect", s.handleAIConnectGet)
	app.Post("/settings/ai-connect", s.handleAIConnectPost)
	app.Post("/settings/ai-connect/test", s.handleAIConnectTestPost)
	app.Get("/settings/audit-log", s.handleAuditLog)

	// Chart of Accounts (after setup).
	app.Get("/accounts", s.handleAccounts)
	app.Post("/accounts", s.handleAccountCreate)

	// Journal Entry (core).
	app.Get("/journal-entry", s.handleJournalEntryForm)
	app.Post("/journal-entry", s.handleJournalEntryPost)
	app.Get("/journal-entry/list", s.handleJournalEntryList)
	app.Post("/journal-entry/:id/reverse", s.handleJournalEntryReverse)

	// Invoices & Bills.
	app.Get("/invoices", s.handleInvoices)
	app.Post("/invoices", s.handleInvoiceCreate)
	app.Get("/bills", s.handleBills)
	app.Post("/bills", s.handleBillCreate)

	// Reports.
	app.Get("/reports", func(c *fiber.Ctx) error { return c.Redirect("/reports/trial-balance", fiber.StatusSeeOther) })
	app.Get("/reports/trial-balance", s.handleTrialBalance)
	app.Get("/reports/income-statement", s.handleIncomeStatement)
	app.Get("/reports/balance-sheet", s.handleBalanceSheet)

	// Contacts (Customers / Vendors).
	app.Get("/customers", s.handleCustomers)
	app.Post("/customers", s.handleCustomerCreate)
	app.Get("/vendors", s.handleVendors)
	app.Post("/vendors", s.handleVendorCreate)

	// Banking.
	app.Get("/banking/reconcile", s.handleBankReconcileForm)
	app.Post("/banking/reconcile", s.handleBankReconcileSubmit)
	app.Get("/banking/receive-payment", s.handleReceivePaymentForm)
	app.Post("/banking/receive-payment", s.handleReceivePaymentSubmit)
	app.Get("/banking/pay-bills", s.handlePayBillsForm)
	app.Post("/banking/pay-bills", s.handlePayBillsSubmit)
}

