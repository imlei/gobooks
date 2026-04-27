// 遵循project_guide.md
package web

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

// handleSalesTransactions serves the unified Sales Transactions page.
// Query params:
//   - type:     document-type filter (all|invoices|quotes|sales_orders|...).
//     Matches the sales-tx dropdown; empty = all.
//   - date:     preset token ("today", "this_month", ...) or "custom".
//   - from/to:  YYYY-MM-DD bounds when date=custom.
//   - status:   per-type native status string (empty = all).
//   - customer_id: filter to one customer.
//   - q:        LIKE match on number/memo.
//   - page, size: pagination (defaults 1/50).
//
// Delegates heavy lifting to services.ListSalesTransactions +
// services.ComputeSalesTxKPI. The handler's job is only to translate
// query strings into the filter + pagination inputs.
func (s *Server) handleSalesTransactions(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	typeFilter := strings.TrimSpace(c.Query("type"))
	dateToken := strings.TrimSpace(c.Query("date"))
	customFrom := strings.TrimSpace(c.Query("from"))
	customTo := strings.TrimSpace(c.Query("to"))
	statusFilter := strings.TrimSpace(c.Query("status"))
	search := strings.TrimSpace(c.Query("q"))
	sortBy, sortDir := services.NormalizeSalesTxSort(c.Query("sort"), c.Query("dir"))

	var customerID uint
	if cid := strings.TrimSpace(c.Query("customer_id")); cid != "" {
		if v, err := strconv.ParseUint(cid, 10, 64); err == nil {
			customerID = uint(v)
		}
	}

	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.Query("size"))
	if size < 1 {
		size = 50
	}
	if size > 200 {
		size = 200
	}

	dateFrom, dateTo := resolveSalesTxDateRange(dateToken, customFrom, customTo)

	filter := services.SalesTxFilter{
		Type:       typeFilter,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		CustomerID: customerID,
		Status:     statusFilter,
		Search:     search,
		SortBy:     sortBy,
		SortDir:    sortDir,
	}

	rows, total, err := services.ListSalesTransactions(s.DB, companyID, filter, page, size)
	if err != nil {
		rows = nil
		total = 0
	}

	kpi, _ := services.ComputeSalesTxKPI(s.DB, companyID)

	customers, _ := s.customersForCompany(companyID)

	// Customer label for echo — look up from loaded list so the filter
	// dropdown can render a readable value after submit.
	customerLabel := ""
	if customerID != 0 {
		for _, cu := range customers {
			if cu.ID == customerID {
				customerLabel = cu.Name
				break
			}
		}
	}

	totalPages := (total + size - 1) / size
	if totalPages < 1 {
		totalPages = 1
	}

	vm := pages.SalesTxVM{
		HasCompany:    true,
		KPI:           kpi,
		TypeFilter:    typeFilter,
		DateFilter:    dateToken,
		DateFrom:      customFrom,
		DateTo:        customTo,
		StatusFilter:  statusFilter,
		CustomerID:    customerID,
		CustomerLabel: customerLabel,
		Search:        search,
		SortBy:        sortBy,
		SortDir:       sortDir,
		Customers:     customers,
		Rows:          rows,
		Page:          page,
		PageSize:      size,
		Total:         total,
		TotalPages:    totalPages,
	}
	return pages.SalesTransactions(vm).Render(c.Context(), c)
}

// resolveSalesTxDateRange maps a preset token (or "custom" + raw strings)
// to a concrete (from, to) pair. Anything unrecognised returns (nil, nil)
// → unbounded.
func resolveSalesTxDateRange(token, fromStr, toStr string) (*time.Time, *time.Time) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := func(d time.Time) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, d.Location())
	}

	switch strings.ToLower(token) {
	case "", "all":
		return nil, nil
	case "today":
		to := endOfDay(today)
		return &today, &to
	case "yesterday":
		y := today.AddDate(0, 0, -1)
		to := endOfDay(y)
		return &y, &to
	case "this_week":
		// Week starts Sunday; offset to Monday for accounting-friendly week.
		wd := int(today.Weekday())
		if wd == 0 {
			wd = 7
		}
		start := today.AddDate(0, 0, -(wd - 1))
		to := endOfDay(today)
		return &start, &to
	case "last_week":
		wd := int(today.Weekday())
		if wd == 0 {
			wd = 7
		}
		thisMon := today.AddDate(0, 0, -(wd - 1))
		start := thisMon.AddDate(0, 0, -7)
		end := endOfDay(thisMon.AddDate(0, 0, -1))
		return &start, &end
	case "this_month":
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		to := endOfDay(today)
		return &start, &to
	case "last_month":
		start := time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, today.Location())
		end := endOfDay(time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location()).AddDate(0, 0, -1))
		return &start, &end
	case "last_30_days":
		start := today.AddDate(0, 0, -30)
		to := endOfDay(today)
		return &start, &to
	case "this_quarter":
		q := ((int(today.Month()) - 1) / 3) * 3
		start := time.Date(today.Year(), time.Month(q+1), 1, 0, 0, 0, 0, today.Location())
		to := endOfDay(today)
		return &start, &to
	case "last_quarter":
		q := ((int(today.Month()) - 1) / 3) * 3
		start := time.Date(today.Year(), time.Month(q+1), 1, 0, 0, 0, 0, today.Location()).AddDate(0, 0, -1)
		startLastQ := time.Date(start.Year(), time.Month(((int(start.Month())-1)/3)*3+1), 1, 0, 0, 0, 0, start.Location())
		end := endOfDay(time.Date(today.Year(), time.Month(q+1), 1, 0, 0, 0, 0, today.Location()).AddDate(0, 0, -1))
		return &startLastQ, &end
	case "this_year":
		start := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, today.Location())
		to := endOfDay(today)
		return &start, &to
	case "last_year":
		start := time.Date(today.Year()-1, 1, 1, 0, 0, 0, 0, today.Location())
		end := endOfDay(time.Date(today.Year()-1, 12, 31, 0, 0, 0, 0, today.Location()))
		return &start, &end
	case "custom":
		var fromPtr, toPtr *time.Time
		if fromStr != "" {
			if d, err := time.Parse("2006-01-02", fromStr); err == nil {
				fromPtr = &d
			}
		}
		if toStr != "" {
			if d, err := time.Parse("2006-01-02", toStr); err == nil {
				end := endOfDay(d)
				toPtr = &end
			}
		}
		return fromPtr, toPtr
	}
	return nil, nil
}
