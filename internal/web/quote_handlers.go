// 遵循project_guide.md
package web

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

// parseIDParam parses the :id route parameter as a uint.
func parseIDParam(c *fiber.Ctx) (uint, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(id), nil
}

// ── List ─────────────────────────────────────────────────────────────────────

func (s *Server) handleQuotes(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	customers, _ := s.customersForCompany(companyID)
	filterStatus := strings.TrimSpace(c.Query("status"))
	filterCustomer := strings.TrimSpace(c.Query("customer_id"))

	var customerID uint
	if filterCustomer != "" {
		if id, err := strconv.ParseUint(filterCustomer, 10, 64); err == nil {
			customerID = uint(id)
		}
	}

	quotes, err := services.ListQuotes(s.DB, companyID, filterStatus, customerID)
	if err != nil {
		quotes = nil
	}

	return pages.Quotes(pages.QuotesVM{
		HasCompany:     true,
		Quotes:         quotes,
		Customers:      customers,
		FilterStatus:   filterStatus,
		FilterCustomer: filterCustomer,
		Created:        c.Query("created") == "1",
		Saved:          c.Query("saved") == "1",
	}).Render(c.Context(), c)
}

// ── New form ──────────────────────────────────────────────────────────────────

func (s *Server) handleQuoteNew(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	vm := pages.QuoteDetailVM{HasCompany: true}
	vm.Quote.QuoteDate = time.Now()
	s.loadQuoteFormData(companyID, &vm)
	return pages.QuoteDetail(vm).Render(c.Context(), c)
}

// ── Detail ────────────────────────────────────────────────────────────────────

func (s *Server) handleQuoteDetail(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	id, err := parseIDParam(c)
	if err != nil {
		return c.Redirect("/quotes", fiber.StatusSeeOther)
	}

	q, err := services.GetQuote(s.DB, companyID, id)
	if err != nil {
		return c.Redirect("/quotes", fiber.StatusSeeOther)
	}

	vm := pages.QuoteDetailVM{
		HasCompany: true,
		Quote:      *q,
		Saved:      c.Query("saved") == "1",
		Sent:       c.Query("sent") == "1",
		Accepted:   c.Query("accepted") == "1",
		Rejected:   c.Query("rejected") == "1",
		Converted:  c.Query("converted") == "1",
		Cancelled:  c.Query("cancelled") == "1",
	}
	s.loadQuoteFormData(companyID, &vm)
	return pages.QuoteDetail(vm).Render(c.Context(), c)
}

// ── Save (create / update) ────────────────────────────────────────────────────

func (s *Server) handleQuoteSave(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	quoteIDStr := strings.TrimSpace(c.FormValue("quote_id"))
	var quoteID uint
	if quoteIDStr != "" {
		if id, err := strconv.ParseUint(quoteIDStr, 10, 64); err == nil {
			quoteID = uint(id)
		}
	}

	in, err := parseQuoteInput(c)
	if err != nil {
		vm := pages.QuoteDetailVM{HasCompany: true, FormError: err.Error()}
		if quoteID > 0 {
			if q, e := services.GetQuote(s.DB, companyID, quoteID); e == nil {
				vm.Quote = *q
			}
		}
		s.loadQuoteFormData(companyID, &vm)
		return pages.QuoteDetail(vm).Render(c.Context(), c)
	}

	if quoteID == 0 {
		q, err := services.CreateQuote(s.DB, companyID, in)
		if err != nil {
			vm := pages.QuoteDetailVM{HasCompany: true, FormError: err.Error()}
			s.loadQuoteFormData(companyID, &vm)
			return pages.QuoteDetail(vm).Render(c.Context(), c)
		}
		return c.Redirect("/quotes/"+strconv.FormatUint(uint64(q.ID), 10)+"?created=1", fiber.StatusSeeOther)
	}

	_, err = services.UpdateQuote(s.DB, companyID, quoteID, in)
	if err != nil {
		vm := pages.QuoteDetailVM{HasCompany: true, FormError: err.Error()}
		if q, e := services.GetQuote(s.DB, companyID, quoteID); e == nil {
			vm.Quote = *q
		}
		s.loadQuoteFormData(companyID, &vm)
		return pages.QuoteDetail(vm).Render(c.Context(), c)
	}
	return c.Redirect("/quotes/"+strconv.FormatUint(uint64(quoteID), 10)+"?saved=1", fiber.StatusSeeOther)
}

// ── Status transitions ────────────────────────────────────────────────────────

func (s *Server) handleQuoteSend(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	id, err := parseIDParam(c)
	if err != nil {
		return c.Redirect("/quotes", fiber.StatusSeeOther)
	}
	_ = services.SendQuote(s.DB, companyID, id, "")
	return c.Redirect("/quotes/"+strconv.FormatUint(uint64(id), 10)+"?sent=1", fiber.StatusSeeOther)
}

func (s *Server) handleQuoteAccept(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	id, err := parseIDParam(c)
	if err != nil {
		return c.Redirect("/quotes", fiber.StatusSeeOther)
	}
	_ = services.AcceptQuote(s.DB, companyID, id)
	return c.Redirect("/quotes/"+strconv.FormatUint(uint64(id), 10)+"?accepted=1", fiber.StatusSeeOther)
}

func (s *Server) handleQuoteReject(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	id, err := parseIDParam(c)
	if err != nil {
		return c.Redirect("/quotes", fiber.StatusSeeOther)
	}
	_ = services.RejectQuote(s.DB, companyID, id)
	return c.Redirect("/quotes/"+strconv.FormatUint(uint64(id), 10)+"?rejected=1", fiber.StatusSeeOther)
}

func (s *Server) handleQuoteCancel(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	id, err := parseIDParam(c)
	if err != nil {
		return c.Redirect("/quotes", fiber.StatusSeeOther)
	}
	_ = services.CancelQuote(s.DB, companyID, id)
	return c.Redirect("/quotes/"+strconv.FormatUint(uint64(id), 10)+"?cancelled=1", fiber.StatusSeeOther)
}

func (s *Server) handleQuoteConvert(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	id, err := parseIDParam(c)
	if err != nil {
		return c.Redirect("/quotes", fiber.StatusSeeOther)
	}
	so, err := services.ConvertQuoteToSalesOrder(s.DB, companyID, id, "", nil)
	if err != nil {
		return c.Redirect("/quotes/"+strconv.FormatUint(uint64(id), 10)+"?converted=0", fiber.StatusSeeOther)
	}
	return c.Redirect("/sales-orders/"+strconv.FormatUint(uint64(so.ID), 10)+"?created=1", fiber.StatusSeeOther)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *Server) loadQuoteFormData(companyID uint, vm *pages.QuoteDetailVM) {
	vm.Customers, _ = s.customersForCompany(companyID)
	s.DB.Where("company_id = ? AND is_active = true AND scope != ?",
		companyID, models.TaxScopePurchase).Order("name asc").Find(&vm.TaxCodes)
	s.DB.Where("company_id = ? AND is_active = true", companyID).
		Order("name asc").Find(&vm.ProductServices)
}

func parseQuoteInput(c *fiber.Ctx) (services.QuoteInput, error) {
	customerIDStr := strings.TrimSpace(c.FormValue("customer_id"))
	if customerIDStr == "" {
		return services.QuoteInput{}, fiber.NewError(fiber.StatusBadRequest, "customer is required")
	}
	cid, err := strconv.ParseUint(customerIDStr, 10, 64)
	if err != nil || cid == 0 {
		return services.QuoteInput{}, fiber.NewError(fiber.StatusBadRequest, "invalid customer")
	}

	quoteDateStr := strings.TrimSpace(c.FormValue("quote_date"))
	quoteDate := time.Now()
	if quoteDateStr != "" {
		if d, e := time.Parse("2006-01-02", quoteDateStr); e == nil {
			quoteDate = d
		}
	}

	var expiryDate *time.Time
	if ed := strings.TrimSpace(c.FormValue("expiry_date")); ed != "" {
		if d, e := time.Parse("2006-01-02", ed); e == nil {
			expiryDate = &d
		}
	}

	lines := parseDocumentLines(c)
	if len(lines) == 0 {
		return services.QuoteInput{}, fiber.NewError(fiber.StatusBadRequest, "at least one line is required")
	}

	in := services.QuoteInput{
		CustomerID:   uint(cid),
		CurrencyCode: strings.ToUpper(strings.TrimSpace(c.FormValue("currency_code"))),
		QuoteDate:    quoteDate,
		ExpiryDate:   expiryDate,
		Notes:        strings.TrimSpace(c.FormValue("notes")),
		Memo:         strings.TrimSpace(c.FormValue("memo")),
	}

	for _, l := range lines {
		in.Lines = append(in.Lines, services.QuoteLineInput{
			TaxCodeID:   l.TaxCodeID,
			Description: l.Description,
			Quantity:    l.Quantity,
			UnitPrice:   l.UnitPrice,
		})
	}
	return in, nil
}

// documentLine is an internal helper for parsing form line items.
type documentLine struct {
	Description string
	Quantity    decimal.Decimal
	UnitPrice   decimal.Decimal
	TaxCodeID   *uint
}

// parseDocumentLines scans form values for line_description_N / line_qty_N / line_price_N / line_tax_N.
func parseDocumentLines(c *fiber.Ctx) []documentLine {
	var lines []documentLine
	for i := 0; i < 200; i++ {
		desc := strings.TrimSpace(c.FormValue("line_description_" + strconv.Itoa(i)))
		qtyStr := strings.TrimSpace(c.FormValue("line_qty_" + strconv.Itoa(i)))
		priceStr := strings.TrimSpace(c.FormValue("line_price_" + strconv.Itoa(i)))
		if desc == "" && qtyStr == "" && priceStr == "" {
			// Gap in numbering — skip but continue (could be a removed row)
			if i > 0 {
				// Check if there are any more rows; simple heuristic: break after 10 consecutive empties
			}
			continue
		}

		qty, err := decimal.NewFromString(qtyStr)
		if err != nil || qty.IsZero() {
			qty = decimal.NewFromInt(1)
		}
		price, err := decimal.NewFromString(priceStr)
		if err != nil {
			price = decimal.Zero
		}

		var taxCodeID *uint
		if tc := strings.TrimSpace(c.FormValue("line_tax_" + strconv.Itoa(i))); tc != "" {
			if id, err := strconv.ParseUint(tc, 10, 64); err == nil && id > 0 {
				uid := uint(id)
				taxCodeID = &uid
			}
		}

		lines = append(lines, documentLine{
			Description: desc,
			Quantity:    qty,
			UnitPrice:   price,
			TaxCodeID:   taxCodeID,
		})
	}
	return lines
}
