// 遵循project_guide.md
package services

import (
	"strings"
	"testing"
	"time"
)

// TestAccountDrillURL_HappyPath locks the canonical URL shape that
// every summary report (BS / IS / TB / Aging) emits when an operator
// clicks a money cell. Single source of truth — change here propagates
// to every report at once.
func TestAccountDrillURL_HappyPath(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 23, 23, 59, 59, 0, time.UTC)

	got := AccountDrillURL(42, from, to)
	want := "/reports/account-transactions?account_id=42&from=2026-01-01&to=2026-04-23"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestAccountDrillURL_EmptyAccountReturnsBlank locks the empty-string
// contract for cells that have no underlying ledger account (Net
// Income, Total Assets, etc.). Templates treat "" as "render plain
// text instead of a link" — see RenderMoneyCell.
func TestAccountDrillURL_EmptyAccountReturnsBlank(t *testing.T) {
	if got := AccountDrillURL(0, time.Now(), time.Now()); got != "" {
		t.Errorf("expected empty URL for accountID=0, got %q", got)
	}
}

// TestAccountDrillURL_OmitsZeroDates verifies the BS use case (as-of
// reports) where only `to` is meaningful — `from` is zero and must be
// omitted from the query string so the receiving handler can apply its
// own default period.
func TestAccountDrillURL_OmitsZeroDates(t *testing.T) {
	to := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)

	got := AccountDrillURL(7, time.Time{}, to)
	if got != "/reports/account-transactions?account_id=7&to=2026-04-23" {
		t.Errorf("got %q, want account_id + to only", got)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got2 := AccountDrillURL(7, from, time.Time{})
	if got2 != "/reports/account-transactions?account_id=7&from=2026-01-01" {
		t.Errorf("got %q, want account_id + from only", got2)
	}

	// Both zero — just the account_id.
	got3 := AccountDrillURL(7, time.Time{}, time.Time{})
	if !strings.Contains(got3, "account_id=7") {
		t.Errorf("got %q, want at least account_id", got3)
	}
	if strings.Contains(got3, "from=") || strings.Contains(got3, "to=") {
		t.Errorf("got %q, must not contain from/to when both zero", got3)
	}
}
