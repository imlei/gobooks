// 遵循project_guide.md
package web

// vendor_detail_handlers.go — GET /vendors/:id — vendor profile page.
// AP mirror of customer detail. Display-only today — vendor editing still
// routes through the create form on /vendors; adding edit is a separate task.

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

const vendorDetailBillCap = 25 // cap table rows to avoid unbounded rendering on noisy vendors

func (s *Server) handleVendorDetail(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	vendorID, err := parseVendorIDParam(c)
	if err != nil {
		return redirectErr(c, "/vendors", "invalid vendor ID")
	}

	var vendor models.Vendor
	if err := s.DB.Where("id = ? AND company_id = ?", vendorID, companyID).First(&vendor).Error; err != nil {
		return redirectErr(c, "/vendors", "vendor not found")
	}

	// Outstanding bills — posted / partially_paid with positive balance, soonest due first.
	var outstandingBills []models.Bill
	s.DB.Preload("Vendor").
		Where("company_id = ? AND vendor_id = ? AND status IN ? AND balance_due > 0",
			companyID, vendorID,
			[]models.BillStatus{models.BillStatusPosted, models.BillStatusPartiallyPaid}).
		Order("due_date asc NULLS LAST, bill_date asc").
		Limit(vendorDetailBillCap).
		Find(&outstandingBills)

	// Recent bills — any status, newest first. Separate query (not the same
	// set as outstanding) so the user sees what's been drafted / paid / voided
	// too when scrolling this page.
	var recentBills []models.Bill
	s.DB.Preload("Vendor").
		Where("company_id = ? AND vendor_id = ?", companyID, vendorID).
		Order("bill_date desc, id desc").
		Limit(vendorDetailBillCap).
		Find(&recentBills)

	// Aggregates — fresh queries so counts aren't capped by vendorDetailBillCap.
	openStatuses := []models.BillStatus{models.BillStatusPosted, models.BillStatusPartiallyPaid}

	var outstandingCount int64
	s.DB.Model(&models.Bill{}).
		Where("company_id = ? AND vendor_id = ? AND status IN ? AND balance_due > 0",
			companyID, vendorID, openStatuses).
		Count(&outstandingCount)

	var outstandingTotal decimal.Decimal
	var totalResult struct{ Total decimal.Decimal }
	s.DB.Model(&models.Bill{}).
		Select("COALESCE(SUM(balance_due), 0) AS total").
		Where("company_id = ? AND vendor_id = ? AND status IN ? AND balance_due > 0",
			companyID, vendorID, openStatuses).
		Scan(&totalResult)
	outstandingTotal = totalResult.Total

	var overdueCount int64
	today := time.Now().Format("2006-01-02")
	s.DB.Model(&models.Bill{}).
		Where("company_id = ? AND vendor_id = ? AND status IN ? AND balance_due > 0 AND due_date IS NOT NULL AND due_date < ?",
			companyID, vendorID, openStatuses, today).
		Count(&overdueCount)

	// Vendor credit remaining — identical logic to /vendors/:id/credits page.
	creditNotes, _ := services.ListVendorCreditNotes(s.DB, companyID, "", vendorID)
	creditRemaining := decimal.Zero
	creditCount := 0
	for _, cn := range creditNotes {
		if cn.Status == models.VendorCreditNoteStatusPosted ||
			cn.Status == models.VendorCreditNoteStatusPartiallyApplied {
			creditRemaining = creditRemaining.Add(cn.RemainingAmount)
			if cn.RemainingAmount.IsPositive() {
				creditCount++
			}
		}
	}

	// Payment term label — look up the code in company-scoped payment_terms.
	var termLabel string
	if code := vendor.DefaultPaymentTermCode; code != "" {
		var term models.PaymentTerm
		if err := s.DB.Where("company_id = ? AND code = ?", companyID, code).First(&term).Error; err == nil {
			termLabel = term.Description
		}
	}

	vm := pages.VendorDetailVM{
		HasCompany:              true,
		Vendor:                  vendor,
		DefaultPaymentTermLabel: termLabel,
		OutstandingBills:        outstandingBills,
		RecentBills:             recentBills,
		OutstandingBillCount:    int(outstandingCount),
		OutstandingTotal:        outstandingTotal,
		OverdueBillCount:        int(overdueCount),
		CreditCount:             creditCount,
		CreditRemaining:         creditRemaining,
	}
	return pages.VendorDetail(vm).Render(c.Context(), c)
}
