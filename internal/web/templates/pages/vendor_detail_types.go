// 遵循project_guide.md
package pages

import (
	"gobooks/internal/models"

	"github.com/shopspring/decimal"
)

// VendorDetailVM is the view model for `/vendors/:id`. The AP-side mirror of
// CustomerDetailVM but deliberately leaner: there is no billable-work/tasks
// concept on the vendor side, and no VendorAPSummary service yet, so this
// VM focuses on what's immediately useful — vendor profile + open credit
// totals + recent bill activity — and leaves richer summary metrics for a
// future pass.
type VendorDetailVM struct {
	HasCompany bool

	Vendor                  models.Vendor
	DefaultPaymentTermLabel string // human-readable term name (empty when code unset)

	// Bills lists. Each is capped in the handler to keep the page snappy.
	OutstandingBills []models.Bill // status in {posted, partially_paid} ordered by due_date asc
	RecentBills      []models.Bill // newest-first, capped (any status)

	// Aggregate counts/totals for quick-scan header strip.
	OutstandingBillCount int
	OutstandingTotal     decimal.Decimal // sum of BalanceDue across OutstandingBills (doc currency — company base)
	OverdueBillCount     int             // outstanding bills whose due_date < today

	// Vendor-credit totals (sum of VCN RemainingAmount where status ∈
	// {posted, partially_applied}). Same number the /vendors/:id/credits
	// hub page shows at the top.
	CreditCount     int
	CreditRemaining decimal.Decimal
}
