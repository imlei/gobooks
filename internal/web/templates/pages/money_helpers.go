// 遵循project_guide.md
package pages

import "github.com/shopspring/decimal"

// MoneyCell is the unified shape for a money value that may or may not
// drill into the underlying ledger. Summary report templates (BS / IS /
// TB / Aging / Sales by X) render this struct so a click on any per-
// account number lands on the Account Transactions page filtered to
// that account + period — the QB-style "follow the money" UX.
//
// DrillURL == "" means "render as plain text" — used for computed
// totals (Net Income, Total Assets) that have no single underlying
// account to drill into.
type MoneyCell struct {
	Amount decimal.Decimal
	// DrillURL is built by services.AccountDrillURL. Empty for cells
	// whose value isn't sourced from one ledger account.
	DrillURL string
}

// NewMoneyCell is the convenience constructor used by report builders
// that pre-resolve the drill URL via services.AccountDrillURL.
func NewMoneyCell(amount decimal.Decimal, drillURL string) MoneyCell {
	return MoneyCell{Amount: amount, DrillURL: drillURL}
}

// Money formats a decimal with 2 decimal places.
// This avoids float64 formatting in templates.
func Money(d decimal.Decimal) string {
	return d.StringFixed(2)
}

// MoneyBlankZero returns Money(d) unless d is zero, in which case it returns
// an empty string. Used in aging tables where empty cells are cleaner than "0.00".
func MoneyBlankZero(d decimal.Decimal) string {
	if d.IsZero() {
		return ""
	}
	return d.StringFixed(2)
}

