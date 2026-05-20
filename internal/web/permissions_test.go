package web

import "testing"

func TestCanPerformActionPermissionMatrix(t *testing.T) {
	tests := []struct {
		name   string
		role   string
		action string
		want   bool
	}{
		{name: "owner can update settings", role: "owner", action: ActionSettingsUpdate, want: true},
		{name: "owner can view sensitive settings", role: "owner", action: ActionSettingsSensitiveView, want: true},
		{name: "admin can view sensitive settings", role: "admin", action: ActionSettingsSensitiveView, want: true},
		{name: "ap can view settings read pages", role: "ap", action: ActionSettingsView, want: true},
		{name: "ap cannot view sensitive settings", role: "ap", action: ActionSettingsSensitiveView, want: false},
		{name: "viewer can view settings read pages", role: "viewer", action: ActionSettingsView, want: true},
		{name: "viewer cannot view sensitive settings", role: "viewer", action: ActionSettingsSensitiveView, want: false},
		{name: "admin can manage members", role: "admin", action: ActionMemberManage, want: true},
		{name: "accountant can approve invoices", role: "accountant", action: ActionInvoiceApprove, want: true},
		{name: "accountant cannot update settings", role: "accountant", action: ActionSettingsUpdate, want: false},
		{name: "bookkeeper can create invoices", role: "bookkeeper", action: ActionInvoiceCreate, want: true},
		{name: "bookkeeper can create tasks", role: "bookkeeper", action: ActionTaskCreate, want: true},
		{name: "bookkeeper can export tasks", role: "bookkeeper", action: ActionTaskExport, want: true},
		{name: "bookkeeper can view journal entries", role: "bookkeeper", action: ActionJournalView, want: true},
		{name: "bookkeeper cannot post manual journal entries", role: "bookkeeper", action: ActionJournalCreate, want: false},
		{name: "bookkeeper cannot approve invoices", role: "bookkeeper", action: ActionInvoiceApprove, want: false},
		{name: "ap can pay bills", role: "ap", action: ActionBillPay, want: true},
		{name: "ap can view tasks", role: "ap", action: ActionTaskView, want: true},
		{name: "ap cannot export tasks", role: "ap", action: ActionTaskExport, want: false},
		{name: "ap cannot create tasks", role: "ap", action: ActionTaskCreate, want: false},
		{name: "ap cannot access ar write flows", role: "ap", action: ActionInvoiceCreate, want: false},
		{name: "ap cannot access reconciliation writes", role: "ap", action: ActionJournalCreate, want: false},
		{name: "ap cannot view ar search domain", role: "ap", action: ActionInvoiceView, want: false},
		{name: "ap can view ap search domain", role: "ap", action: ActionBillView, want: true},
		{name: "ap cannot view reports", role: "ap", action: ActionReportView, want: false},
		{name: "ap can view inventory", role: "ap", action: ActionInventoryView, want: true},
		{name: "ap can view warehouses", role: "ap", action: ActionWarehouseView, want: true},
		{name: "viewer can view reports", role: "viewer", action: ActionReportView, want: true},
		{name: "viewer cannot create accounts", role: "viewer", action: ActionAccountCreate, want: false},
		{name: "unknown action fails closed", role: "owner", action: "missing:action", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CanPerformAction(tc.role, tc.action)
			if got != tc.want {
				t.Fatalf("CanPerformAction(%q, %q) = %v, want %v", tc.role, tc.action, got, tc.want)
			}
		})
	}
}

func TestPermissionOverrides(t *testing.T) {
	denyAR := newPermissionOverrides([]permissionOverride{{Permission: PermARAccess, Granted: false}})
	if CanPerformActionWithOverrides("bookkeeper", ActionInvoiceCreate, denyAR) {
		t.Fatalf("deny override should block role-granted AR access")
	}

	grantPayroll := newPermissionOverrides([]permissionOverride{{Permission: PermPayrollView, Granted: true}})
	if !CanPerformActionWithOverrides("ap", ActionPayrollView, grantPayroll) {
		t.Fatalf("grant override should allow payroll view")
	}

	denyWins := newPermissionOverrides([]permissionOverride{
		{Permission: PermPayrollView, Granted: true},
		{Permission: PermPayrollView, Granted: false},
	})
	if CanPerformActionWithOverrides("owner", ActionPayrollView, denyWins) {
		t.Fatalf("deny override should win over grant and base role")
	}
}
