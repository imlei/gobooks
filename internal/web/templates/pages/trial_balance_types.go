// 遵循project_guide.md
package pages

import (
	"time"

	"gobooks/internal/services"
)

type TrialBalanceVM struct {
	HasCompany bool

	From string
	To   string

	ActiveTab string

	Rows []services.TrialBalanceRow

	TotalDebits  string
	TotalCredits string

	FormError string
}

type IncomeStatementVM struct {
	HasCompany bool

	From string
	To   string

	ActiveTab string

	Report services.IncomeStatement

	FormError string
}

type BalanceSheetVM struct {
	HasCompany bool

	AsOf string

	ActiveTab string

	Report services.BalanceSheet

	FormError string

	// Used for display without extra parsing in templates.
	AsOfTime time.Time
}

