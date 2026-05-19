package pages

import "testing"

func TestTaskExportURLCarriesListFilters(t *testing.T) {
	vm := TasksVM{
		FilterCustomerID: "42",
		FilterStatus:     "completed",
		FilterFrom:       "2026-04-01",
		FilterTo:         "2026-04-30",
	}

	got := taskExportURL(vm)
	want := "/api/tasks/export?customer_id=42&from=2026-04-01&status=completed&to=2026-04-30"
	if got != want {
		t.Fatalf("taskExportURL = %q, want %q", got, want)
	}
}

func TestTaskExportURLWithoutFilters(t *testing.T) {
	if got := taskExportURL(TasksVM{}); got != "/api/tasks/export" {
		t.Fatalf("taskExportURL without filters = %q", got)
	}
}
