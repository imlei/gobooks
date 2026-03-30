// 遵循project_guide.md
package pages

import "github.com/shopspring/decimal"

// Money formats a decimal with 2 decimal places.
// This avoids float64 formatting in templates.
func Money(d decimal.Decimal) string {
	return d.StringFixed(2)
}

