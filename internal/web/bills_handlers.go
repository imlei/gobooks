// 遵循产品需求 v1.0
package web

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

func (s *Server) handleBills(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	vendors, err := s.vendorsForCompany(companyID)
	if err != nil {
		return pages.Bills(pages.BillsVM{
			HasCompany: true,
			FormError:  "Could not load vendors.",
		}).Render(c.Context(), c)
	}

	filterQ := strings.TrimSpace(c.Query("q"))
	filterVendorID := strings.TrimSpace(c.Query("vendor_id"))
	filterFrom := strings.TrimSpace(c.Query("from"))
	filterTo := strings.TrimSpace(c.Query("to"))

	qry := s.DB.Preload("Vendor").Model(&models.Bill{}).Where("company_id = ?", companyID)
	if filterQ != "" {
		qry = qry.Where("LOWER(bill_number) LIKE LOWER(?)", "%"+filterQ+"%")
	}
	if filterVendorID != "" {
		if id, err := services.ParseUint(filterVendorID); err == nil && id > 0 {
			qry = qry.Where("vendor_id = ?", uint(id))
		}
	}
	if filterFrom != "" {
		if d, err := time.Parse("2006-01-02", filterFrom); err == nil {
			qry = qry.Where("bill_date >= ?", d)
		}
	}
	if filterTo != "" {
		if d, err := time.Parse("2006-01-02", filterTo); err == nil {
			qry = qry.Where("bill_date < ?", d.AddDate(0, 0, 1))
		}
	}

	var bills []models.Bill
	if err := qry.Order("bill_date desc, id desc").Find(&bills).Error; err != nil {
		return pages.Bills(pages.BillsVM{
			HasCompany: true,
			FormError:  "Could not load bills.",
		}).Render(c.Context(), c)
	}

	nextNo := "BILL001"
	var latest models.Bill
	if err := s.DB.Where("company_id = ?", companyID).Order("id desc").First(&latest).Error; err == nil {
		nextNo = services.NextDocumentNumber(latest.BillNumber, "BILL001")
	}

	return pages.Bills(pages.BillsVM{
		HasCompany:     true,
		Vendors:        vendors,
		Bills:          bills,
		BillDate:       time.Now().Format("2006-01-02"),
		BillNumber:     nextNo,
		Created:        c.Query("created") == "1",
		FilterQ:        filterQ,
		FilterVendorID: filterVendorID,
		FilterFrom:     filterFrom,
		FilterTo:       filterTo,
	}).Render(c.Context(), c)
}

func (s *Server) handleBillCreate(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	vendors, err := s.vendorsForCompany(companyID)
	if err != nil {
		return pages.Bills(pages.BillsVM{
			HasCompany: true,
			FormError:  "Could not load vendors.",
		}).Render(c.Context(), c)
	}

	bills, err := s.billsForCompany(companyID)
	if err != nil {
		return pages.Bills(pages.BillsVM{
			HasCompany: true,
			FormError:  "Could not load bills.",
		}).Render(c.Context(), c)
	}

	billNo := strings.TrimSpace(c.FormValue("bill_number"))
	vendorRaw := strings.TrimSpace(c.FormValue("vendor_id"))
	dateRaw := strings.TrimSpace(c.FormValue("bill_date"))
	amountRaw := strings.TrimSpace(c.FormValue("amount"))
	memo := strings.TrimSpace(c.FormValue("memo"))
	forceDuplicate := strings.TrimSpace(c.FormValue("force_duplicate")) == "1"

	vm := pages.BillsVM{
		HasCompany: true,
		Vendors:    vendors,
		Bills:      bills,
		BillNumber: billNo,
		VendorID:   vendorRaw,
		BillDate:   dateRaw,
		Amount:     amountRaw,
		Memo:       memo,
	}

	if billNo == "" {
		vm.BillNumberError = "Bill Number is required."
	} else if err := services.ValidateDocumentNumber(billNo); err != nil {
		vm.BillNumberError = err.Error()
	}
	vendorID, err := services.ParseUint(vendorRaw)
	if err != nil || vendorID == 0 {
		vm.VendorError = "Vendor is required."
	}
	billDate, err := time.Parse("2006-01-02", dateRaw)
	if err != nil {
		vm.DateError = "Bill Date is required."
	}
	amount, err := services.ParseDecimalMoney(amountRaw)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		vm.AmountError = "Amount must be greater than 0."
	}

	if vm.BillNumberError != "" || vm.VendorError != "" || vm.DateError != "" || vm.AmountError != "" {
		return pages.Bills(vm).Render(c.Context(), c)
	}

	var venCount int64
	if err := s.DB.Model(&models.Vendor{}).
		Where("id = ? AND company_id = ?", uint(vendorID), companyID).
		Count(&venCount).Error; err != nil {
		vm.FormError = "Could not validate vendor."
		return pages.Bills(vm).Render(c.Context(), c)
	}
	if venCount == 0 {
		vm.VendorError = "Vendor is not valid for this company."
		return pages.Bills(vm).Render(c.Context(), c)
	}

	var dupCount int64
	if err := s.DB.Model(&models.Bill{}).
		Where("company_id = ? AND vendor_id = ? AND LOWER(bill_number) = LOWER(?)", companyID, uint(vendorID), billNo).
		Count(&dupCount).Error; err != nil {
		vm.FormError = "Could not validate Bill Number."
		return pages.Bills(vm).Render(c.Context(), c)
	}
	if dupCount > 0 && !forceDuplicate {
		vm.DuplicateWarning = true
		vm.DuplicateMessage = "Duplicate detected for this Vendor + Bill Number (case-insensitive)."
		return pages.Bills(vm).Render(c.Context(), c)
	}

	bill := models.Bill{
		CompanyID:  companyID,
		BillNumber: billNo,
		VendorID:   uint(vendorID),
		BillDate:   billDate,
		Amount:     amount,
		Memo:       memo,
	}
	if err := s.DB.Create(&bill).Error; err != nil {
		vm.FormError = "Could not create bill. Please try again."
		return pages.Bills(vm).Render(c.Context(), c)
	}

	cid := companyID
	uid := user.ID
	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	_ = services.WriteAuditLogWithContext(s.DB, "bill.created", "bill", bill.ID, actor, map[string]any{
		"bill_number": bill.BillNumber,
		"vendor_id":   bill.VendorID,
		"amount":      bill.Amount.StringFixed(2),
		"company_id":  companyID,
	}, &cid, &uid)

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/bills?created=1")
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.Redirect("/bills?created=1", fiber.StatusSeeOther)
}

func (s *Server) billsForCompany(companyID uint) ([]models.Bill, error) {
	var bills []models.Bill
	err := s.DB.Preload("Vendor").Where("company_id = ?", companyID).Order("bill_date desc, id desc").Find(&bills).Error
	return bills, err
}
