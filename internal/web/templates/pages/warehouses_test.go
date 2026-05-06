package pages

import (
	"context"
	"strings"
	"testing"
	"time"

	"balanciz/internal/models"
	"balanciz/internal/services"

	"github.com/shopspring/decimal"
)

func TestWarehousesPageShowsOperationalActionsAndQueues(t *testing.T) {
	due := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	vm := WarehousesVM{
		HasCompany: true,
		Warehouses: []models.Warehouse{
			{ID: 1, Code: "MAIN", Name: "Main Warehouse", IsActive: true, IsDefault: true},
		},
		WarehouseQueue: &services.WarehouseQueueSummary{
			WaitingToReceive: []services.WarehouseQueueItem{
				{
					ID:           9,
					Number:       "PO-0009",
					Counterparty: "Northwind Supply",
					DueDate:      &due,
					Status:       "Confirmed",
					Amount:       decimal.NewFromInt(1250),
					CurrencyCode: "USD",
					Href:         "/purchase-orders/9",
				},
			},
			WaitingToShip: []services.WarehouseQueueItem{
				{
					ID:           12,
					Number:       "SO-0012",
					Counterparty: "Acme Customer",
					DueDate:      &due,
					Status:       "Partially Invoiced",
					Amount:       decimal.NewFromInt(890),
					CurrencyCode: "CAD",
					Href:         "/sales-orders/12",
				},
			},
		},
	}

	var sb strings.Builder
	if err := Warehouses(vm).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render warehouses page: %v", err)
	}
	html := sb.String()
	for _, want := range []string{
		`href="/inventory/transfers"`,
		"Warehouse Transfer",
		`border border-border-input`,
		`href="/warehouses/new"`,
		"New Warehouse",
		`bg-primary px-4 py-2`,
		`href="/purchase-orders"`,
		"Receiving",
		`href="/vendor-return-shipments/new"`,
		"Return",
		`href="/sales-orders"`,
		"Shipping",
		`href="/ar-return-receipts"`,
		"Return Receipts",
		"All Warehouses",
		`bg-surface-tableHeader`,
		`hover:bg-surface-rowHover`,
		`href="/warehouses/1/stock"`,
		`href="/warehouses/1"`,
		"Edit",
		"Waiting to Receive",
		"Waiting to Ship",
		`href="/purchase-orders/9"`,
		"PO-0009",
		"Northwind Supply",
		"USD 1250.00",
		"May 12, 2026",
		`href="/sales-orders/12"`,
		"SO-0012",
		"Acme Customer",
		"CAD 890.00",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected warehouses page HTML to contain %q", want)
		}
	}
	for _, notWant := range []string{
		`btn btn-primary`,
		`table table-zebra`,
		`badge badge-`,
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("expected warehouses page HTML not to contain legacy class %q", notWant)
		}
	}
}
