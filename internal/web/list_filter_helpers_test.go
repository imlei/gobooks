// 遵循project_guide.md
package web

import "testing"

// TestNormaliseListStatus locks the contract list pages rely on:
// empty / garbage input collapses to "active" (the default), known values
// pass through, case + whitespace tolerant.
func TestNormaliseListStatus(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "active"},
		{"   ", "active"},
		{"active", "active"},
		{"INACTIVE", "inactive"},
		{"  All  ", "all"},
		{"bogus", "active"},
	}
	for _, tc := range cases {
		if got := normaliseListStatus(tc.in); got != tc.want {
			t.Errorf("normaliseListStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNormaliseStockLevel locks the contract for the Products & Services
// stock filter: only "in_stock" / "out_of_stock" pass through; everything
// else (empty, "any", garbage) collapses to "" so a URL-bar typo can't
// silently produce an empty page.
func TestNormaliseStockLevel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"any", ""},
		{"low_stock", ""}, // "low stock" intentionally unsupported (no reorder_point in schema)
		{"in_stock", "in_stock"},
		{"  IN_STOCK  ", "in_stock"},
		{"out_of_stock", "out_of_stock"},
	}
	for _, tc := range cases {
		if got := normaliseStockLevel(tc.in); got != tc.want {
			t.Errorf("normaliseStockLevel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNormaliseProductType locks the validation guard for the Type select:
// known type strings round-trip; unknown values collapse to "" so the
// query doesn't run against a typo'd type column.
func TestNormaliseProductType(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"bogus", ""},
		{"service", "service"},
		{"INVENTORY", "inventory"},
		{"  non_inventory  ", "non_inventory"},
		{"product", "non_inventory"}, // alias accepted
		{"other_charge", "other_charge"},
	}
	for _, tc := range cases {
		if got := normaliseProductType(tc.in); got != tc.want {
			t.Errorf("normaliseProductType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
