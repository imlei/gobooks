// 遵循project_guide.md
package services

import (
	"net/url"
	"strconv"
	"time"
)

// AccountDrillURL builds the canonical "click a number on a report → see
// the underlying ledger entries" URL. Single source of truth so every
// summary report (Balance Sheet, Income Statement, Trial Balance,
// Aging, Sales-by-X, …) lands on the same Account Transactions page
// with the right account + date scope.
//
// accountID == 0 returns "" — useful for VM cells whose value is a
// computed total (Net Income, Total Assets) without an underlying
// ledger account. Callers should treat the empty string as "render as
// plain text, not a link".
//
// from / to are formatted as YYYY-MM-DD. Zero values are omitted; the
// account-transactions handler then falls back to its default
// resolvePeriodDates flow (period=last_month etc).
func AccountDrillURL(accountID uint, from, to time.Time) string {
	if accountID == 0 {
		return ""
	}
	q := url.Values{}
	q.Set("account_id", strconv.FormatUint(uint64(accountID), 10))
	if !from.IsZero() {
		q.Set("from", from.Format("2006-01-02"))
	}
	if !to.IsZero() {
		q.Set("to", to.Format("2006-01-02"))
	}
	return "/reports/account-transactions?" + q.Encode()
}
