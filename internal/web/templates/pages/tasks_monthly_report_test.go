package pages

import (
	"testing"

	"github.com/shopspring/decimal"

	"balanciz/internal/services"
)

func TestGetSortedCustomersOrdersByAmountNameAndID(t *testing.T) {
	summary := &services.MonthlyTaskSummary{
		ByCustomer: map[string]*services.CustomerTaskSummary{
			"beta":  {CustomerID: 20, CustomerName: "Beta", Amount: decimal.RequireFromString("200.00")},
			"acme2": {CustomerID: 2, CustomerName: "Acme", Amount: decimal.RequireFromString("200.00")},
			"acme1": {CustomerID: 1, CustomerName: "Acme", Amount: decimal.RequireFromString("200.00")},
			"small": {CustomerID: 30, CustomerName: "Small", Amount: decimal.RequireFromString("50.00")},
		},
	}

	got := getSortedCustomers(summary)
	if len(got) != 4 {
		t.Fatalf("expected 4 customers, got %d", len(got))
	}

	wantIDs := []uint{1, 2, 20, 30}
	for i, want := range wantIDs {
		if got[i].CustomerID != want {
			t.Fatalf("sorted customer %d id = %d, want %d; full order = %+v", i, got[i].CustomerID, want, got)
		}
	}
}

func TestGetSortedCustomersHandlesNilSummary(t *testing.T) {
	if got := getSortedCustomers(nil); len(got) != 0 {
		t.Fatalf("expected empty list for nil summary, got %+v", got)
	}
}
