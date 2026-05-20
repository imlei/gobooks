package ui_test

import (
	"context"
	"strings"
	"testing"

	"balanciz/internal/web/templates/ui"
)

func TestSidebarSettingsIncludesTemplatesEntry(t *testing.T) {
	ctx := ui.WithSidebarData(context.Background(), ui.SidebarData{ShowSettings: true})
	var sb strings.Builder
	if err := ui.Sidebar(ui.SidebarVM{Active: "Templates", HasCompany: true}).Render(ctx, &sb); err != nil {
		t.Fatalf("render sidebar: %v", err)
	}
	html := sb.String()

	for _, want := range []string{
		`href="/settings/templates"`,
		"Templates",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected sidebar HTML to contain %q", want)
		}
	}
}

func TestSidebarInventoryHidesWorkflowEntries(t *testing.T) {
	ctx := ui.WithSidebarData(context.Background(), ui.SidebarData{ShowInventory: true})
	var sb strings.Builder
	if err := ui.Sidebar(ui.SidebarVM{Active: "Warehouses", HasCompany: true}).Render(ctx, &sb); err != nil {
		t.Fatalf("render sidebar: %v", err)
	}
	html := sb.String()

	for _, want := range []string{
		`href="/products-services"`,
		`href="/warehouses"`,
		"Products &amp; Services",
		"Warehouses",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected sidebar HTML to contain %q", want)
		}
	}
	for _, notWant := range []string{
		`href="/inventory/transfers"`,
		`href="/inventory/stock"`,
		`href="/ar-return-receipts"`,
		`href="/vendor-return-shipments"`,
		"Warehouse Transfers",
		"Stock Report",
		"Return Receipts",
		"Returns to Vendor",
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("expected sidebar HTML not to contain %q", notWant)
		}
	}
}

func TestSidebarHidesSensitiveModuleEntriesByDefault(t *testing.T) {
	var sb strings.Builder
	if err := ui.Sidebar(ui.SidebarVM{Active: "Dashboard", HasCompany: true}).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render sidebar: %v", err)
	}
	html := sb.String()

	for _, notWant := range []string{
		`href="/sales-overview"`,
		`href="/expenses-overview"`,
		`href="/reports"`,
		`href="/accounts"`,
		`href="/employees"`,
		`href="/payroll/runs"`,
		`href="/payroll/remittances"`,
		`href="/payroll/reports/summary"`,
		`href="/cheques"`,
		`href="/tasks"`,
		`href="/tasks/new"`,
		"People & Payroll",
		"Cheques",
		"Tasks",
		"Work",
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("expected sidebar HTML not to contain %q", notWant)
		}
	}
}

func TestSidebarShowsModuleEntriesFromPermissionFilteredSidebarData(t *testing.T) {
	ctx := ui.WithSidebarData(context.Background(), ui.SidebarData{
		ShowCreateNew:      true,
		ShowSales:          true,
		ShowAP:             true,
		ShowInventory:      true,
		ShowJournal:        true,
		ShowReconciliation: true,
		ShowReports:        true,
		ShowAccounts:       true,
		ShowSettings:       true,
		ShowEmployees:      true,
		ShowTasks:          true,
		ShowPayroll:        true,
		ShowPayrollDetails: true,
		ShowPayrollReports: true,
		ShowCheques:        true,
		CanCreateSales:     true,
		CanCreateAP:        true,
		CanCreateJournal:   true,
		CanCreateWarehouse: true,
		CanManageCatalog:   true,
		CanCreateEmployee:  true,
		CanCreateTask:      true,
		CanCreatePayroll:   true,
		CanCreateCheque:    true,
	})
	var sb strings.Builder
	if err := ui.Sidebar(ui.SidebarVM{Active: "Payroll Reports", HasCompany: true}).Render(ctx, &sb); err != nil {
		t.Fatalf("render sidebar: %v", err)
	}
	html := sb.String()

	for _, want := range []string{
		"People & Payroll",
		"Sales & Get Paid",
		`href="/sales-overview"`,
		"Expense & Bills",
		`href="/expenses-overview"`,
		"Inventory",
		`href="/products-services"`,
		"Accounting",
		`href="/journal-entry/list"`,
		`href="/banking/reconcile"`,
		`href="/reports"`,
		`href="/accounts"`,
		"Settings",
		`href="/employees"`,
		"Employees",
		"Work",
		`href="/tasks"`,
		"Tasks",
		`href="/payroll/runs"`,
		"Payroll Runs",
		`href="/payroll/remittances"`,
		"Remittances",
		`href="/payroll/reports/summary"`,
		"Payroll Reports",
		`href="/cheques"`,
		"Cheques",
		"Add Employee",
		"Invoice",
		"Bill",
		"Journal Entry",
		"Add Warehouse",
		"Add Product/Service",
		"Task",
		"Payroll Run",
		"Cheque",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected sidebar HTML to contain %q", want)
		}
	}
}
