// 遵循产品需求 v1.0
package web

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"gobooks/internal/models"
	"gobooks/internal/numbering"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"

	"github.com/shopspring/decimal"
)

func (s *Server) handleDashboard(c *fiber.Ctx) error {
	// Dashboard MVP: calculate a lightweight P&L + expenses breakdown and
	// show recent revenue trend. We keep everything simple and non-dense.
	now := time.Now()
	fromDate := now.AddDate(0, 0, -30)
	toDate := now

	toMoneyVM := func(d decimal.Decimal) pages.MoneyVM {
		return pages.MoneyVM{
			Value:      pages.Money(d),
			IsPositive: d.GreaterThanOrEqual(decimal.Zero),
		}
	}

	vm := pages.DashboardVM{
		HasCompany:   true,
		RangeLabel:  "Last 30 days",
		RevenueTrend: []pages.RevenueTrendPointVM{},
	}

	// Profit & Loss summary (and expenses list are derived from the same report).
	if report, err := services.IncomeStatementReport(s.DB, fromDate, toDate); err == nil {
		vm.PnL.Revenue = toMoneyVM(report.TotalRevenue)
		// Expenses are typically outflows; show as negative so we can color red.
		vm.PnL.Expenses = toMoneyVM(report.TotalExpenses.Neg())
		vm.PnL.NetIncome = toMoneyVM(report.NetIncome)

		vm.Expenses.Total = vm.PnL.Expenses
		// Top expense accounts by absolute value (now already positive cost -> we negate for display).
		// Note: report.Expenses is naturally "expense" accounts (expense/cost_of_sales not included).
		top := report.Expenses
		if len(top) > 6 {
			top = top[:6]
		}
		vm.Expenses.TopLines = make([]pages.ExpenseLineVM, 0, len(top))
		for _, l := range top {
			amt := l.Amount.Neg()
			vm.Expenses.TopLines = append(vm.Expenses.TopLines, pages.ExpenseLineVM{
				Account: l.Name,
				Amount:  toMoneyVM(amt),
			})
		}
	}

	// Optional: revenue trend for last 3 calendar months.
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for i := 2; i >= 0; i-- {
		ms := monthStart.AddDate(0, -i, 0)
		me := ms.AddDate(0, 1, -1)
		rep, err := services.IncomeStatementReport(s.DB, ms, me)
		if err != nil {
			continue
		}
		vm.RevenueTrend = append(vm.RevenueTrend, pages.RevenueTrendPointVM{
			Label: ms.Format("2006-01"),
			Revenue: pages.MoneyVM{
				Value:      pages.Money(rep.TotalRevenue),
				IsPositive: rep.TotalRevenue.GreaterThanOrEqual(decimal.Zero),
			},
		})
	}

	// Right column: bank accounts list (best-effort MVP heuristic).
	var assetAccounts []models.Account
	assetTypes := []models.AccountType{
		models.AccountTypeBank,
		models.AccountTypeAccountsReceivable,
		models.AccountTypeOtherCurrentAsset,
		models.AccountTypeFixedAsset,
		models.AccountTypeOtherAsset,
		models.AccountTypeAsset, // legacy
	}
	if err := s.DB.Where("type IN ?", assetTypes).Order("code asc").Limit(50).Find(&assetAccounts).Error; err == nil {
		bankAccounts := make([]models.Account, 0, len(assetAccounts))
		for _, a := range assetAccounts {
			if a.Type == models.AccountTypeBank || strings.Contains(strings.ToLower(a.Name), "bank") {
				bankAccounts = append(bankAccounts, a)
			}
		}
		if len(bankAccounts) == 0 {
			bankAccounts = assetAccounts
		}
		if len(bankAccounts) > 5 {
			bankAccounts = bankAccounts[:5]
		}
		vm.BankAccounts = make([]pages.BankAccountVM, 0, len(bankAccounts))
		for _, a := range bankAccounts {
			vm.BankAccounts = append(vm.BankAccounts, pages.BankAccountVM{
				Code: a.Code,
				Name: a.Name,
			})
		}
	}

	return pages.Dashboard(vm).Render(c.Context(), c)
}

func (s *Server) handleAccounts(c *fiber.Ctx) error {
	var accounts []models.Account
	if err := s.DB.Order("code asc").Find(&accounts).Error; err != nil {
		return pages.Accounts(pages.AccountsVM{
			HasCompany: true,
			Active:     "Accounts",
			FormError:  "Could not load accounts.",
			Accounts:   []models.Account{},
		}).Render(c.Context(), c)
	}

	return pages.Accounts(pages.AccountsVM{
		HasCompany: true,
		Active:     "Accounts",
		Created:    c.Query("created") == "1",
		Accounts:   accounts,
	}).Render(c.Context(), c)
}

func (s *Server) handleAccountCreate(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.FormValue("code"))
	name := strings.TrimSpace(c.FormValue("name"))
	typeRaw := strings.TrimSpace(c.FormValue("type"))

	vm := pages.AccountsVM{
		HasCompany: true,
		Active:     "Accounts",
		Code:       code,
		Name:       name,
		Type:       typeRaw,
	}

	// Validate required fields.
	if code == "" {
		vm.CodeError = "Code is required."
	} else if err := models.ValidateAccountCode(code); err != nil {
		vm.CodeError = err.Error()
	}
	if name == "" {
		vm.NameError = "Name is required."
	}

	accType, err := models.ParseAccountType(typeRaw)
	if err != nil {
		vm.TypeError = "Type is required."
	}

	// Load accounts for the table (even if validation fails).
	var accounts []models.Account
	_ = s.DB.Order("code asc").Find(&accounts).Error
	vm.Accounts = accounts

	if vm.CodeError != "" || vm.NameError != "" || vm.TypeError != "" {
		return pages.Accounts(vm).Render(c.Context(), c)
	}

	// Prevent duplicate codes with a simple pre-check so we can show a friendly message.
	var count int64
	if err := s.DB.Model(&models.Account{}).Where("code = ?", code).Count(&count).Error; err != nil {
		vm.FormError = "Could not validate account code."
		return pages.Accounts(vm).Render(c.Context(), c)
	}
	if count > 0 {
		vm.CodeError = "This code is already in use."
		return pages.Accounts(vm).Render(c.Context(), c)
	}

	acc := models.Account{
		Code: code,
		Name: name,
		Type: accType,
	}

	if err := s.DB.Create(&acc).Error; err != nil {
		vm.FormError = "Could not create account. Please try again."
		return pages.Accounts(vm).Render(c.Context(), c)
	}
	_ = services.WriteAuditLog(s.DB, "account.created", "account", acc.ID, "system", map[string]any{
		"code": acc.Code,
		"name": acc.Name,
		"type": acc.Type,
	})

	// Success: redirect back to /accounts with a small success flag.
	return c.Redirect("/accounts?created=1", fiber.StatusSeeOther)
}

func (s *Server) handleJournalEntryForm(c *fiber.Ctx) error {
	var accounts []models.Account
	if err := s.DB.Order("code asc").Find(&accounts).Error; err != nil {
		return pages.JournalEntry(pages.JournalEntryVM{
			HasCompany: true,
			FormError:  "Could not load accounts.",
		}).Render(c.Context(), c)
	}

	// Customers/Vendors are minimal tables for the Name selector.
	var customers []models.Customer
	_ = s.DB.Order("name asc").Find(&customers).Error
	var vendors []models.Vendor
	_ = s.DB.Order("name asc").Find(&vendors).Error

	return pages.JournalEntry(pages.JournalEntryVM{
		HasCompany: true,
		Accounts:   accounts,
		Customers:  customers,
		Vendors:    vendors,
		Saved:      c.Query("saved") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleInvoices(c *fiber.Ctx) error {
	var customers []models.Customer
	_ = s.DB.Order("name asc").Find(&customers).Error

	filterQ := strings.TrimSpace(c.Query("q"))
	filterCustomerID := strings.TrimSpace(c.Query("customer_id"))
	filterFrom := strings.TrimSpace(c.Query("from"))
	filterTo := strings.TrimSpace(c.Query("to"))

	var invoices []models.Invoice
	qry := s.DB.Preload("Customer").Model(&models.Invoice{})
	if filterQ != "" {
		qry = qry.Where("LOWER(invoice_number) LIKE LOWER(?)", "%"+filterQ+"%")
	}
	if filterCustomerID != "" {
		if id, err := services.ParseUint(filterCustomerID); err == nil && id > 0 {
			qry = qry.Where("customer_id = ?", uint(id))
		}
	}
	if filterFrom != "" {
		if d, err := time.Parse("2006-01-02", filterFrom); err == nil {
			qry = qry.Where("invoice_date >= ?", d)
		}
	}
	if filterTo != "" {
		if d, err := time.Parse("2006-01-02", filterTo); err == nil {
			qry = qry.Where("invoice_date < ?", d.AddDate(0, 0, 1))
		}
	}
	_ = qry.Order("invoice_date desc, id desc").Find(&invoices).Error

	// Derive next invoice number from latest saved invoice.
	nextNo := "IN001"
	var latest models.Invoice
	if err := s.DB.Order("id desc").First(&latest).Error; err == nil {
		nextNo = services.NextDocumentNumber(latest.InvoiceNumber, "IN001")
	}

	return pages.Invoices(pages.InvoicesVM{
		HasCompany:    true,
		Customers:     customers,
		Invoices:      invoices,
		InvoiceDate:   time.Now().Format("2006-01-02"),
		InvoiceNumber: nextNo,
		Created:       c.Query("created") == "1",
		FilterQ:       filterQ,
		FilterCustomerID: filterCustomerID,
		FilterFrom:    filterFrom,
		FilterTo:      filterTo,
	}).Render(c.Context(), c)
}

func (s *Server) handleInvoiceCreate(c *fiber.Ctx) error {
	var customers []models.Customer
	_ = s.DB.Order("name asc").Find(&customers).Error
	var invoices []models.Invoice
	_ = s.DB.Preload("Customer").Order("invoice_date desc, id desc").Find(&invoices).Error

	invoiceNo := strings.TrimSpace(c.FormValue("invoice_number"))
	customerRaw := strings.TrimSpace(c.FormValue("customer_id"))
	dateRaw := strings.TrimSpace(c.FormValue("invoice_date"))
	amountRaw := strings.TrimSpace(c.FormValue("amount"))
	memo := strings.TrimSpace(c.FormValue("memo"))
	forceDuplicate := strings.TrimSpace(c.FormValue("force_duplicate")) == "1"

	vm := pages.InvoicesVM{
		HasCompany:     true,
		Customers:      customers,
		Invoices:       invoices,
		InvoiceNumber:  invoiceNo,
		CustomerID:     customerRaw,
		InvoiceDate:    dateRaw,
		Amount:         amountRaw,
		Memo:           memo,
	}

	if invoiceNo == "" {
		vm.InvoiceNumberError = "Invoice Number is required."
	} else if err := services.ValidateDocumentNumber(invoiceNo); err != nil {
		vm.InvoiceNumberError = err.Error()
	}
	custID, err := services.ParseUint(customerRaw)
	if err != nil || custID == 0 {
		vm.CustomerError = "Customer is required."
	}
	invoiceDate, err := time.Parse("2006-01-02", dateRaw)
	if err != nil {
		vm.DateError = "Invoice Date is required."
	}
	amount, err := services.ParseDecimalMoney(amountRaw)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		vm.AmountError = "Amount must be greater than 0."
	}

	if vm.InvoiceNumberError != "" || vm.CustomerError != "" || vm.DateError != "" || vm.AmountError != "" {
		return pages.Invoices(vm).Render(c.Context(), c)
	}

	// Duplicate check: case-insensitive invoice number.
	var dupCount int64
	if err := s.DB.Model(&models.Invoice{}).
		Where("LOWER(invoice_number) = LOWER(?)", invoiceNo).
		Count(&dupCount).Error; err != nil {
		vm.FormError = "Could not validate Invoice Number."
		return pages.Invoices(vm).Render(c.Context(), c)
	}
	if dupCount > 0 && !forceDuplicate {
		vm.DuplicateWarning = true
		vm.DuplicateMessage = "Invoice Number conflict detected (case-insensitive)."
		return pages.Invoices(vm).Render(c.Context(), c)
	}

	inv := models.Invoice{
		InvoiceNumber: invoiceNo,
		CustomerID:    uint(custID),
		InvoiceDate:   invoiceDate,
		Amount:        amount,
		Memo:          memo,
	}
	if err := s.DB.Create(&inv).Error; err != nil {
		vm.FormError = "Could not create invoice. Please try again."
		return pages.Invoices(vm).Render(c.Context(), c)
	}
	_ = services.WriteAuditLog(s.DB, "invoice.created", "invoice", inv.ID, "system", map[string]any{
		"invoice_number": inv.InvoiceNumber,
		"customer_id":    inv.CustomerID,
		"amount":         inv.Amount.StringFixed(2),
	})

	return c.Redirect("/invoices?created=1", fiber.StatusSeeOther)
}

func (s *Server) handleBills(c *fiber.Ctx) error {
	var vendors []models.Vendor
	_ = s.DB.Order("name asc").Find(&vendors).Error

	filterQ := strings.TrimSpace(c.Query("q"))
	filterVendorID := strings.TrimSpace(c.Query("vendor_id"))
	filterFrom := strings.TrimSpace(c.Query("from"))
	filterTo := strings.TrimSpace(c.Query("to"))

	var bills []models.Bill
	qry := s.DB.Preload("Vendor").Model(&models.Bill{})
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
	_ = qry.Order("bill_date desc, id desc").Find(&bills).Error

	nextNo := "BILL001"
	var latest models.Bill
	if err := s.DB.Order("id desc").First(&latest).Error; err == nil {
		nextNo = services.NextDocumentNumber(latest.BillNumber, "BILL001")
	}

	return pages.Bills(pages.BillsVM{
		HasCompany: true,
		Vendors:    vendors,
		Bills:      bills,
		BillDate:   time.Now().Format("2006-01-02"),
		BillNumber: nextNo,
		Created:    c.Query("created") == "1",
		FilterQ:    filterQ,
		FilterVendorID: filterVendorID,
		FilterFrom: filterFrom,
		FilterTo:   filterTo,
	}).Render(c.Context(), c)
}

func (s *Server) handleBillCreate(c *fiber.Ctx) error {
	var vendors []models.Vendor
	_ = s.DB.Order("name asc").Find(&vendors).Error
	var bills []models.Bill
	_ = s.DB.Preload("Vendor").Order("bill_date desc, id desc").Find(&bills).Error

	billNo := strings.TrimSpace(c.FormValue("bill_number"))
	vendorRaw := strings.TrimSpace(c.FormValue("vendor_id"))
	dateRaw := strings.TrimSpace(c.FormValue("bill_date"))
	amountRaw := strings.TrimSpace(c.FormValue("amount"))
	memo := strings.TrimSpace(c.FormValue("memo"))
	forceDuplicate := strings.TrimSpace(c.FormValue("force_duplicate")) == "1"

	vm := pages.BillsVM{
		HasCompany:  true,
		Vendors:     vendors,
		Bills:       bills,
		BillNumber:  billNo,
		VendorID:    vendorRaw,
		BillDate:    dateRaw,
		Amount:      amountRaw,
		Memo:        memo,
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

	// Duplicate check: vendor + bill number, case-insensitive bill number.
	var dupCount int64
	if err := s.DB.Model(&models.Bill{}).
		Where("vendor_id = ? AND LOWER(bill_number) = LOWER(?)", uint(vendorID), billNo).
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
	_ = services.WriteAuditLog(s.DB, "bill.created", "bill", bill.ID, "system", map[string]any{
		"bill_number": bill.BillNumber,
		"vendor_id":   bill.VendorID,
		"amount":      bill.Amount.StringFixed(2),
	})

	return c.Redirect("/bills?created=1", fiber.StatusSeeOther)
}

type postedLine struct {
	AccountID string
	Debit     string
	Credit    string
	Memo      string
	Party     string
}

func (s *Server) handleJournalEntryPost(c *fiber.Ctx) error {
	// Load dropdown data for re-render on errors.
	var accounts []models.Account
	_ = s.DB.Order("code asc").Find(&accounts).Error
	var customers []models.Customer
	_ = s.DB.Order("name asc").Find(&customers).Error
	var vendors []models.Vendor
	_ = s.DB.Order("name asc").Find(&vendors).Error

	entryDateRaw := strings.TrimSpace(c.FormValue("entry_date"))
	journalNo := strings.TrimSpace(c.FormValue("journal_no"))

	if entryDateRaw == "" {
		return pages.JournalEntry(pages.JournalEntryVM{
			HasCompany: true,
			Accounts:   accounts,
			Customers:  customers,
			Vendors:    vendors,
			FormError:  "Date is required.",
		}).Render(c.Context(), c)
	}

	entryDate, err := time.Parse("2006-01-02", entryDateRaw)
	if err != nil {
		return pages.JournalEntry(pages.JournalEntryVM{
			HasCompany: true,
			Accounts:   accounts,
			Customers:  customers,
			Vendors:    vendors,
			FormError:  "Date must be a valid date.",
		}).Render(c.Context(), c)
	}

	// Parse posted lines from keys like:
	// lines[0][account_id], lines[0][debit], ...
	re := regexp.MustCompile(`^lines\[(\d+)\]\[(account_id|debit|credit|memo|party)\]$`)
	linesMap := map[string]*postedLine{}

	c.Context().PostArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		m := re.FindStringSubmatch(key)
		if len(m) != 3 {
			return
		}

		idx := m[1]
		field := m[2]
		val := strings.TrimSpace(string(v))

		pl := linesMap[idx]
		if pl == nil {
			pl = &postedLine{}
			linesMap[idx] = pl
		}

		switch field {
		case "account_id":
			pl.AccountID = val
		case "debit":
			pl.Debit = val
		case "credit":
			pl.Credit = val
		case "memo":
			pl.Memo = val
		case "party":
			pl.Party = val
		}
	})

	drafts := make([]services.JournalLineDraft, 0, len(linesMap))
	for _, pl := range linesMap {
		drafts = append(drafts, services.JournalLineDraft{
			AccountID: pl.AccountID,
			Debit:     pl.Debit,
			Credit:    pl.Credit,
			Memo:      pl.Memo,
			Party:     pl.Party,
		})
	}

	validLines, err := services.ValidateJournalLines(drafts)
	if err != nil {
		return pages.JournalEntry(pages.JournalEntryVM{
			HasCompany: true,
			Accounts:   accounts,
			Customers:  customers,
			Vendors:    vendors,
			FormError:  err.Error(),
		}).Render(c.Context(), c)
	}

	decimalZero := decimal.NewFromInt(0)

	// Save journal entry + lines in a transaction.
	var postedJEID uint
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		je := models.JournalEntry{
			EntryDate: entryDate,
			JournalNo: journalNo,
		}
		if err := tx.Create(&je).Error; err != nil {
			return err
		}
		postedJEID = je.ID

		for i := range validLines {
			validLines[i].JournalEntryID = je.ID
			// Ensure zero values are explicit.
			if validLines[i].Debit.IsZero() {
				validLines[i].Debit = decimalZero
			}
			if validLines[i].Credit.IsZero() {
				validLines[i].Credit = decimalZero
			}
		}

		return tx.Create(&validLines).Error
	}); err != nil {
		return pages.JournalEntry(pages.JournalEntryVM{
			HasCompany: true,
			Accounts:   accounts,
			Customers:  customers,
			Vendors:    vendors,
			FormError:  "Could not save journal entry. Please try again.",
		}).Render(c.Context(), c)
	}
	_ = services.WriteAuditLog(s.DB, "journal.posted", "journal_entry", postedJEID, "system", map[string]any{
		"journal_no": journalNo,
		"line_count": len(validLines),
		"entry_date": entryDateRaw,
	})

	return c.Redirect("/journal-entry?saved=1", fiber.StatusSeeOther)
}

func (s *Server) handleSetupForm(c *fiber.Ctx) error {
	return pages.Setup(pages.SetupViewModel{
		Active: "Setup",
		Values: pages.SetupFormValues{},
		Errors: pages.SetupFormErrors{},
	}).Render(c.Context(), c)
}

func (s *Server) handleTrialBalance(c *fiber.Ctx) error {
	fromDate, toDate, fromStr, toStr, errMsg := parseReportRange(c.Query("from"), c.Query("to"))
	if errMsg != "" {
		return pages.TrialBalance(pages.TrialBalanceVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			ActiveTab:  "trial",
			Rows:       []services.TrialBalanceRow{},
			TotalDebits:  "0.00",
			TotalCredits: "0.00",
			FormError:  errMsg,
		}).Render(c.Context(), c)
	}

	rows, totalDebits, totalCredits, err := services.TrialBalance(s.DB, fromDate, toDate)
	if err != nil {
		return pages.TrialBalance(pages.TrialBalanceVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			ActiveTab:  "trial",
			Rows:       []services.TrialBalanceRow{},
			TotalDebits:  "0.00",
			TotalCredits: "0.00",
			FormError:  "Could not run report.",
		}).Render(c.Context(), c)
	}

	return pages.TrialBalance(pages.TrialBalanceVM{
		HasCompany: true,
		From:       fromStr,
		To:         toStr,
		ActiveTab:  "trial",
		Rows:       rows,
		TotalDebits:  pages.Money(totalDebits),
		TotalCredits: pages.Money(totalCredits),
	}).Render(c.Context(), c)
}

func (s *Server) handleIncomeStatement(c *fiber.Ctx) error {
	fromDate, toDate, fromStr, toStr, errMsg := parseReportRange(c.Query("from"), c.Query("to"))
	if errMsg != "" {
		return pages.IncomeStatement(pages.IncomeStatementVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			ActiveTab:  "income",
			Report:     services.IncomeStatement{FromDate: fromDate, ToDate: toDate},
			FormError:  errMsg,
		}).Render(c.Context(), c)
	}

	report, err := services.IncomeStatementReport(s.DB, fromDate, toDate)
	if err != nil {
		return pages.IncomeStatement(pages.IncomeStatementVM{
			HasCompany: true,
			From:       fromStr,
			To:         toStr,
			ActiveTab:  "income",
			Report:     services.IncomeStatement{FromDate: fromDate, ToDate: toDate},
			FormError:  "Could not run report.",
		}).Render(c.Context(), c)
	}

	return pages.IncomeStatement(pages.IncomeStatementVM{
		HasCompany: true,
		From:       fromStr,
		To:         toStr,
		ActiveTab:  "income",
		Report:     report,
	}).Render(c.Context(), c)
}

func (s *Server) handleBalanceSheet(c *fiber.Ctx) error {
	asOfStr := strings.TrimSpace(c.Query("as_of"))
	if asOfStr == "" {
		asOfStr = time.Now().Format("2006-01-02")
	}

	asOf, err := time.Parse("2006-01-02", asOfStr)
	if err != nil {
		return pages.BalanceSheet(pages.BalanceSheetVM{
			HasCompany: true,
			AsOf:       asOfStr,
			ActiveTab:  "balance",
			Report:     services.BalanceSheet{AsOf: time.Now()},
			FormError:  "As of date must be a valid date.",
			AsOfTime:   time.Now(),
		}).Render(c.Context(), c)
	}

	report, err := services.BalanceSheetReport(s.DB, asOf)
	if err != nil {
		return pages.BalanceSheet(pages.BalanceSheetVM{
			HasCompany: true,
			AsOf:       asOfStr,
			ActiveTab:  "balance",
			Report:     services.BalanceSheet{AsOf: asOf},
			FormError:  "Could not run report.",
			AsOfTime:   asOf,
		}).Render(c.Context(), c)
	}

	return pages.BalanceSheet(pages.BalanceSheetVM{
		HasCompany: true,
		AsOf:       asOfStr,
		ActiveTab:  "balance",
		Report:     report,
		AsOfTime:   asOf,
	}).Render(c.Context(), c)
}

func parseReportRange(fromRaw, toRaw string) (time.Time, time.Time, string, string, string) {
	// Defaults: last 30 days.
	now := time.Now()
	toStr := strings.TrimSpace(toRaw)
	if toStr == "" {
		toStr = now.Format("2006-01-02")
	}
	toDate, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return time.Time{}, time.Time{}, strings.TrimSpace(fromRaw), toStr, "To date must be a valid date."
	}

	fromStr := strings.TrimSpace(fromRaw)
	if fromStr == "" {
		fromStr = toDate.AddDate(0, 0, -30).Format("2006-01-02")
	}
	fromDate, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fromStr, toStr, "From date must be a valid date."
	}

	if fromDate.After(toDate) {
		return time.Time{}, time.Time{}, fromStr, toStr, "From date must be before To date."
	}

	return fromDate, toDate, fromStr, toStr, ""
}

func defaultBusinessTypeForEntity(entity models.EntityType) models.BusinessType {
	switch entity {
	case models.EntityTypeLLP:
		return models.BusinessTypeProfessionalCorp
	default:
		// Keep setup simple: default to Retail for Personal/Incorporated.
		return models.BusinessTypeRetail
	}
}

func within53Weeks(a, b time.Time) bool {
	days := int(a.Sub(b).Hours() / 24)
	if days < 0 {
		days = -days
	}
	return days <= 371
}

func (s *Server) handleSetupSubmit(c *fiber.Ctx) error {
	// Read form fields.
	name := strings.TrimSpace(c.FormValue("company_name"))
	entityTypeRaw := strings.TrimSpace(c.FormValue("entity_type"))
	addressLine := strings.TrimSpace(c.FormValue("address_line"))
	province := strings.TrimSpace(c.FormValue("province"))
	postalCode := strings.TrimSpace(c.FormValue("postal_code"))
	country := strings.TrimSpace(c.FormValue("country"))
	businessNumber := strings.TrimSpace(c.FormValue("business_number"))
	industry := strings.TrimSpace(c.FormValue("industry"))
	incorporatedDate := strings.TrimSpace(c.FormValue("incorporated_date"))
	fiscalYearEnd := strings.TrimSpace(c.FormValue("fiscal_year_end"))

	values := pages.SetupFormValues{
		CompanyName:    name,
		EntityType:     entityTypeRaw,
		AddressLine:    addressLine,
		Province:       province,
		PostalCode:     postalCode,
		Country:        country,
		BusinessNumber: businessNumber,
		Industry:       industry,
		IncorporatedDate: incorporatedDate,
		FiscalYearEnd:  fiscalYearEnd,
	}

	// Validate required fields.
	var errs pages.SetupFormErrors
	if name == "" {
		errs.CompanyName = "Company Name is required."
	}

	entityType, err := models.ParseEntityType(entityTypeRaw)
	if err != nil {
		errs.EntityType = "Entity Type is required."
	}

	businessType := defaultBusinessTypeForEntity(entityType)

	industryValue, err3 := models.ParseIndustry(industry)
	if err3 != nil {
		errs.Industry = "Industry is required."
	}

	if addressLine == "" {
		errs.AddressLine = "Address Line is required."
	}
	if province == "" {
		errs.Province = "Province is required."
	}
	if postalCode == "" {
		errs.PostalCode = "Postal Code is required."
	}
	if country == "" {
		errs.Country = "Country is required."
	}
	if businessNumber == "" {
		errs.BusinessNumber = "Business Number is required."
	}
	if industry == "" {
		// Keep the message consistent even if it is empty or invalid.
		errs.Industry = "Industry is required."
	}
	var incorporatedDateTime time.Time
	if incorporatedDate == "" {
		errs.IncorporatedDate = "Incorporated Date is required."
	} else if d, err := time.Parse("2006-01-02", incorporatedDate); err != nil {
		errs.IncorporatedDate = "Incorporated Date must be a valid date."
	} else {
		incorporatedDateTime = d
	}

	var fiscalYearEndTime time.Time
	if fiscalYearEnd == "" {
		errs.FiscalYearEnd = "Fiscal Year End is required."
	} else if d, err := time.Parse("2006-01-02", fiscalYearEnd); err != nil {
		errs.FiscalYearEnd = "Fiscal Year End must be a valid date."
	} else {
		fiscalYearEndTime = d
	}

	if errs.IncorporatedDate == "" && errs.FiscalYearEnd == "" && !within53Weeks(incorporatedDateTime, fiscalYearEndTime) {
		errs.FiscalYearEnd = "Fiscal Year End and Incorporated Date must be within 53 weeks."
	}

	if errs.HasAny() {
		// Re-render the form with friendly validation messages.
		return pages.Setup(pages.SetupViewModel{
			Active: "Setup",
			Values: values,
			Errors: errs,
		}).Render(c.Context(), c)
	}

	// Save company + import default COA in one transaction.
	var setupCompanyID uint
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		company := models.Company{
			Name:           name,
			EntityType:     entityType,
			BusinessType:   businessType,
			AddressLine:    addressLine,
			Province:       province,
			PostalCode:     postalCode,
			Country:        country,
			BusinessNumber: businessNumber,
			Industry:       industryValue,
			IncorporatedDate: incorporatedDate,
			FiscalYearEnd:  fiscalYearEnd,
		}

		if err := tx.Create(&company).Error; err != nil {
			return err
		}
		setupCompanyID = company.ID

		return services.ImportDefaultChartOfAccounts(tx, entityType, businessType)
	}); err != nil {
		// Safe error shown to user; details can be logged later.
		return pages.Setup(pages.SetupViewModel{
			Active: "Setup",
			Values: values,
			Errors: pages.SetupFormErrors{
				Form: "Could not save setup. Please try again.",
			},
		}).Render(c.Context(), c)
	}
	_ = services.WriteAuditLog(s.DB, "setup.completed", "company", setupCompanyID, "system", map[string]any{
		"company_name": name,
		"entity_type":  entityTypeRaw,
		"business_type": string(businessType),
	})

	// Setup done. Redirect to dashboard (guard middleware will allow now).
	// Support both normal form submit and potential HTMX submit.
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/")
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.Redirect("/", fiber.StatusSeeOther)
}

func (s *Server) handleCompanySettingsForm(c *fiber.Ctx) error {
	var company models.Company
	if err := s.DB.Order("id asc").First(&company).Error; err != nil {
		return pages.CompanySettings(pages.CompanySettingsVM{
			HasCompany: false,
			Values:     pages.SetupFormValues{},
			Errors: pages.SetupFormErrors{
				Form: "Company not found. Please run setup first.",
			},
		}).Render(c.Context(), c)
	}

	return pages.CompanySettings(pages.CompanySettingsVM{
		HasCompany: true,
		Values: pages.SetupFormValues{
			CompanyName:    company.Name,
			EntityType:     string(company.EntityType),
			BusinessType:   string(company.BusinessType),
			AddressLine:    company.AddressLine,
			Province:       company.Province,
			PostalCode:     company.PostalCode,
			Country:        company.Country,
			BusinessNumber: company.BusinessNumber,
			Industry:       string(company.Industry),
			IncorporatedDate: company.IncorporatedDate,
			FiscalYearEnd:  company.FiscalYearEnd,
		},
		Errors: pages.SetupFormErrors{},
		Saved:  false,
	}).Render(c.Context(), c)
}

func (s *Server) handleCompanySettingsSubmit(c *fiber.Ctx) error {
	// Read form fields.
	name := strings.TrimSpace(c.FormValue("company_name"))
	entityTypeRaw := strings.TrimSpace(c.FormValue("entity_type"))
	businessTypeRaw := strings.TrimSpace(c.FormValue("business_type"))
	addressLine := strings.TrimSpace(c.FormValue("address_line"))
	province := strings.TrimSpace(c.FormValue("province"))
	postalCode := strings.TrimSpace(c.FormValue("postal_code"))
	country := strings.TrimSpace(c.FormValue("country"))
	businessNumber := strings.TrimSpace(c.FormValue("business_number"))
	industry := strings.TrimSpace(c.FormValue("industry"))
	incorporatedDate := strings.TrimSpace(c.FormValue("incorporated_date"))
	fiscalYearEnd := strings.TrimSpace(c.FormValue("fiscal_year_end"))

	values := pages.SetupFormValues{
		CompanyName:    name,
		EntityType:     entityTypeRaw,
		BusinessType:   businessTypeRaw,
		AddressLine:    addressLine,
		Province:       province,
		PostalCode:     postalCode,
		Country:        country,
		BusinessNumber: businessNumber,
		Industry:       industry,
		IncorporatedDate: incorporatedDate,
		FiscalYearEnd:  fiscalYearEnd,
	}

	var errs pages.SetupFormErrors
	if name == "" {
		errs.CompanyName = "Company Name is required."
	}

	entityType, err := models.ParseEntityType(entityTypeRaw)
	if err != nil {
		errs.EntityType = "Entity Type is required."
	}

	businessType, err2 := models.ParseBusinessType(businessTypeRaw)
	if err2 != nil {
		errs.BusinessType = "Business Type is required."
	}

	industryValue, err3 := models.ParseIndustry(industry)
	if err3 != nil {
		errs.Industry = "Industry is required."
	}

	if addressLine == "" {
		errs.AddressLine = "Address Line is required."
	}
	if province == "" {
		errs.Province = "Province is required."
	}
	if postalCode == "" {
		errs.PostalCode = "Postal Code is required."
	}
	if country == "" {
		errs.Country = "Country is required."
	}
	if businessNumber == "" {
		errs.BusinessNumber = "Business Number is required."
	}
	if industry == "" {
		errs.Industry = "Industry is required."
	}
	var incorporatedDateTime time.Time
	if incorporatedDate == "" {
		errs.IncorporatedDate = "Incorporated Date is required."
	} else if d, err := time.Parse("2006-01-02", incorporatedDate); err != nil {
		errs.IncorporatedDate = "Incorporated Date must be a valid date."
	} else {
		incorporatedDateTime = d
	}

	var fiscalYearEndTime time.Time
	if fiscalYearEnd == "" {
		errs.FiscalYearEnd = "Fiscal Year End is required."
	} else if d, err := time.Parse("2006-01-02", fiscalYearEnd); err != nil {
		errs.FiscalYearEnd = "Fiscal Year End must be a valid date."
	} else {
		fiscalYearEndTime = d
	}

	if errs.IncorporatedDate == "" && errs.FiscalYearEnd == "" && !within53Weeks(incorporatedDateTime, fiscalYearEndTime) {
		errs.FiscalYearEnd = "Fiscal Year End and Incorporated Date must be within 53 weeks."
	}

	if errs.HasAny() {
		return pages.CompanySettings(pages.CompanySettingsVM{
			HasCompany: true,
			Values:     values,
			Errors:     errs,
			Saved:      false,
		}).Render(c.Context(), c)
	}

	// Update the first company row (MVP: single-company).
	var company models.Company
	if err := s.DB.Order("id asc").First(&company).Error; err != nil {
		return pages.CompanySettings(pages.CompanySettingsVM{
			HasCompany: false,
			Values:     values,
			Errors: pages.SetupFormErrors{
				Form: "Company not found. Please run setup first.",
			},
			Saved: false,
		}).Render(c.Context(), c)
	}

	company.Name = name
	company.EntityType = entityType
	company.BusinessType = businessType
	company.AddressLine = addressLine
	company.Province = province
	company.PostalCode = postalCode
	company.Country = country
	company.BusinessNumber = businessNumber
	company.Industry = industryValue
	company.IncorporatedDate = incorporatedDate
	company.FiscalYearEnd = fiscalYearEnd

	if err := s.DB.Save(&company).Error; err != nil {
		return pages.CompanySettings(pages.CompanySettingsVM{
			HasCompany: true,
			Values:     values,
			Errors: pages.SetupFormErrors{
				Form: "Could not save. Please try again.",
			},
			Saved: false,
		}).Render(c.Context(), c)
	}

	return pages.CompanySettings(pages.CompanySettingsVM{
		HasCompany: true,
		Values:     values,
		Errors:     pages.SetupFormErrors{},
		Saved:      true,
	}).Render(c.Context(), c)
}

func (s *Server) handleCustomers(c *fiber.Ctx) error {
	var customers []models.Customer
	if err := s.DB.Order("name asc").Find(&customers).Error; err != nil {
		return pages.Customers(pages.CustomersVM{
			HasCompany: true,
			FormError:  "Could not load customers.",
			Customers:  []models.Customer{},
		}).Render(c.Context(), c)
	}

	return pages.Customers(pages.CustomersVM{
		HasCompany: true,
		Customers:  customers,
		Created:    c.Query("created") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleCustomerCreate(c *fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))

	vm := pages.CustomersVM{
		HasCompany: true,
		Name:       name,
	}

	if name == "" {
		vm.NameError = "Name is required."
	}

	var customers []models.Customer
	_ = s.DB.Order("name asc").Find(&customers).Error
	vm.Customers = customers

	if vm.NameError != "" {
		return pages.Customers(vm).Render(c.Context(), c)
	}

	customer := models.Customer{Name: name}
	if err := s.DB.Create(&customer).Error; err != nil {
		vm.FormError = "Could not create customer. Please try again."
		return pages.Customers(vm).Render(c.Context(), c)
	}

	return c.Redirect("/customers?created=1", fiber.StatusSeeOther)
}

func (s *Server) handleVendors(c *fiber.Ctx) error {
	var vendors []models.Vendor
	if err := s.DB.Order("name asc").Find(&vendors).Error; err != nil {
		return pages.Vendors(pages.VendorsVM{
			HasCompany: true,
			FormError:  "Could not load vendors.",
			Vendors:    []models.Vendor{},
		}).Render(c.Context(), c)
	}

	return pages.Vendors(pages.VendorsVM{
		HasCompany: true,
		Vendors:    vendors,
		Created:    c.Query("created") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleVendorCreate(c *fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))

	vm := pages.VendorsVM{
		HasCompany: true,
		Name:       name,
	}

	if name == "" {
		vm.NameError = "Name is required."
	}

	var vendors []models.Vendor
	_ = s.DB.Order("name asc").Find(&vendors).Error
	vm.Vendors = vendors

	if vm.NameError != "" {
		return pages.Vendors(vm).Render(c.Context(), c)
	}

	vendor := models.Vendor{Name: name}
	if err := s.DB.Create(&vendor).Error; err != nil {
		vm.FormError = "Could not create vendor. Please try again."
		return pages.Vendors(vm).Render(c.Context(), c)
	}

	return c.Redirect("/vendors?created=1", fiber.StatusSeeOther)
}

func (s *Server) handleBankReconcileForm(c *fiber.Ctx) error {
	// Load accounts for the bank account dropdown.
	// For MVP we show asset accounts first (typical bank accounts),
	// but keep it simple and include all accounts if needed later.
	var accounts []models.Account
	if err := s.DB.Order("code asc").Find(&accounts).Error; err != nil {
		return pages.BankReconcile(pages.BankReconcileVM{
			HasCompany: true,
			Accounts:   []models.Account{},
			Active:     "Bank Reconcile",
			FormError:  "Could not load accounts.",
		}).Render(c.Context(), c)
	}

	accountIDStr := strings.TrimSpace(c.Query("account_id"))
	statementDateStr := strings.TrimSpace(c.Query("statement_date"))
	endingBalanceStr := strings.TrimSpace(c.Query("ending_balance"))

	vm := pages.BankReconcileVM{
		HasCompany:    true,
		Accounts:      accounts,
		AccountID:     accountIDStr,
		StatementDate: statementDateStr,
		EndingBalance: endingBalanceStr,
		Active:        "Bank Reconcile",
		Saved:         c.Query("saved") == "1",
		PreviouslyCleared: "0.00",
		Candidates:    []services.ReconcileCandidate{},
	}

	// If user hasn't selected an account yet, just render the page.
	if accountIDStr == "" {
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}

	accountIDU64, err := services.ParseUint(accountIDStr)
	if err != nil || accountIDU64 == 0 {
		vm.FormError = "Invalid account selected."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}
	accountID := uint(accountIDU64)

	// Default statement date: today.
	if statementDateStr == "" {
		statementDateStr = time.Now().Format("2006-01-02")
		vm.StatementDate = statementDateStr
	}
	statementDate, err := time.Parse("2006-01-02", statementDateStr)
	if err != nil {
		vm.FormError = "Statement Date must be a valid date."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}
	vm.StatementDateTime = statementDate

	// Default ending balance to 0.00 if empty.
	if endingBalanceStr == "" {
		endingBalanceStr = "0.00"
		vm.EndingBalance = endingBalanceStr
	}
	if _, err := services.ParseDecimalMoney(endingBalanceStr); err != nil {
		vm.FormError = "Ending Balance must be a number."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}

	prev, err := services.ClearedBalance(s.DB, accountID, statementDate)
	if err != nil {
		vm.FormError = "Could not load cleared balance."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}
	vm.PreviouslyCleared = pages.Money(prev)

	cands, err := services.ListReconcileCandidates(s.DB, accountID, statementDate)
	if err != nil {
		vm.FormError = "Could not load unreconciled transactions."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}
	vm.Candidates = cands

	return pages.BankReconcile(vm).Render(c.Context(), c)
}

func (s *Server) handleBankReconcileSubmit(c *fiber.Ctx) error {
	accountIDStr := strings.TrimSpace(c.FormValue("account_id"))
	statementDateStr := strings.TrimSpace(c.FormValue("statement_date"))
	endingBalanceStr := strings.TrimSpace(c.FormValue("ending_balance"))

	accountIDU64, err := services.ParseUint(accountIDStr)
	if err != nil || accountIDU64 == 0 {
		return c.Redirect("/banking/reconcile", fiber.StatusSeeOther)
	}
	accountID := uint(accountIDU64)

	statementDate, err := time.Parse("2006-01-02", statementDateStr)
	if err != nil {
		return c.Redirect("/banking/reconcile?account_id="+accountIDStr, fiber.StatusSeeOther)
	}

	endingBalance, err := services.ParseDecimalMoney(endingBalanceStr)
	if err != nil {
		return c.Redirect("/banking/reconcile?account_id="+accountIDStr+"&statement_date="+statementDateStr, fiber.StatusSeeOther)
	}

	// Read selected line ids.
	lineIDBytes := c.Context().PostArgs().PeekMulti("line_ids")
	lineIDs := make([]string, 0, len(lineIDBytes))
	for _, b := range lineIDBytes {
		lineIDs = append(lineIDs, string(b))
	}
	if len(lineIDs) == 0 {
		// No selection; nothing to reconcile.
		return c.Redirect("/banking/reconcile?account_id="+accountIDStr+"&statement_date="+statementDateStr+"&ending_balance="+endingBalanceStr, fiber.StatusSeeOther)
	}

	// Convert ids to uint slice.
	var ids []uint
	for _, sID := range lineIDs {
		u, err := services.ParseUint(sID)
		if err != nil || u == 0 {
			continue
		}
		ids = append(ids, uint(u))
	}
	if len(ids) == 0 {
		return c.Redirect("/banking/reconcile?account_id="+accountIDStr+"&statement_date="+statementDateStr+"&ending_balance="+endingBalanceStr, fiber.StatusSeeOther)
	}

	decimalZero := decimal.NewFromInt(0)

	// Save reconciliation and mark selected lines in a transaction.
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		prevCleared, err := services.ClearedBalance(tx, accountID, statementDate)
		if err != nil {
			return err
		}

		// Sum selected amounts from DB (do not trust client).
		type row struct{ Amount decimal.Decimal }
		var r row
		if err := tx.Raw(
			`
SELECT COALESCE(SUM(jl.debit - jl.credit), 0) AS amount
FROM journal_lines jl
JOIN journal_entries je ON je.id = jl.journal_entry_id
WHERE jl.id IN ?
  AND jl.account_id = ?
  AND jl.reconciliation_id IS NULL
  AND je.entry_date <= ?
`,
			ids, accountID, statementDate,
		).Scan(&r).Error; err != nil {
			return err
		}

		cleared := prevCleared.Add(r.Amount)
		diff := endingBalance.Sub(cleared)
		if !diff.Equal(decimalZero) {
			return errors.New("difference not zero")
		}

		rec := models.Reconciliation{
			AccountID:      accountID,
			StatementDate:  statementDate,
			EndingBalance:  endingBalance,
			ClearedBalance: cleared,
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}

		now := time.Now()
		if err := tx.Model(&models.JournalLine{}).
			Where("id IN ?", ids).
			Where("account_id = ?", accountID).
			Where("reconciliation_id IS NULL").
			Updates(map[string]any{
				"reconciliation_id": rec.ID,
				"reconciled_at":     &now,
			}).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		// Redirect back with inputs so user can adjust selection.
		return c.Redirect("/banking/reconcile?account_id="+accountIDStr+"&statement_date="+statementDateStr+"&ending_balance="+endingBalanceStr, fiber.StatusSeeOther)
	}

	return c.Redirect("/banking/reconcile?account_id="+accountIDStr+"&statement_date="+statementDateStr+"&ending_balance="+endingBalanceStr+"&saved=1", fiber.StatusSeeOther)
}

func (s *Server) handleReceivePaymentForm(c *fiber.Ctx) error {
	var customers []models.Customer
	_ = s.DB.Order("name asc").Find(&customers).Error

	// For MVP we allow any account in the dropdown, but the service enforces Asset type.
	var accounts []models.Account
	_ = s.DB.Order("code asc").Find(&accounts).Error

	vm := pages.ReceivePaymentVM{
		HasCompany: true,
		Customers:  customers,
		Accounts:   accounts,
		Saved:      c.Query("saved") == "1",
		EntryDate:  time.Now().Format("2006-01-02"),
	}

	return pages.ReceivePayment(vm).Render(c.Context(), c)
}

func (s *Server) handleReceivePaymentSubmit(c *fiber.Ctx) error {
	var customers []models.Customer
	_ = s.DB.Order("name asc").Find(&customers).Error
	var accounts []models.Account
	_ = s.DB.Order("code asc").Find(&accounts).Error

	customerIDRaw := strings.TrimSpace(c.FormValue("customer_id"))
	entryDateRaw := strings.TrimSpace(c.FormValue("entry_date"))
	bankIDRaw := strings.TrimSpace(c.FormValue("bank_account_id"))
	arIDRaw := strings.TrimSpace(c.FormValue("ar_account_id"))
	amountRaw := strings.TrimSpace(c.FormValue("amount"))
	memo := strings.TrimSpace(c.FormValue("memo"))

	vm := pages.ReceivePaymentVM{
		HasCompany:     true,
		Customers:      customers,
		Accounts:       accounts,
		CustomerID:     customerIDRaw,
		EntryDate:      entryDateRaw,
		BankAccountID:  bankIDRaw,
		ARAccountID:    arIDRaw,
		Amount:         amountRaw,
		Memo:           memo,
	}

	// Validate required inputs.
	custU64, err := services.ParseUint(customerIDRaw)
	if err != nil || custU64 == 0 {
		vm.CustomerError = "Customer is required."
	}

	entryDate, err := time.Parse("2006-01-02", entryDateRaw)
	if err != nil {
		vm.DateError = "Date is required."
	}

	bankU64, err := services.ParseUint(bankIDRaw)
	if err != nil || bankU64 == 0 {
		vm.BankError = "Bank account is required."
	}

	arU64, err := services.ParseUint(arIDRaw)
	if err != nil || arU64 == 0 {
		vm.ARError = "A/R account is required."
	}

	amount, err := services.ParseDecimalMoney(amountRaw)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		vm.AmountError = "Amount must be greater than 0."
	}

	if vm.CustomerError != "" || vm.DateError != "" || vm.BankError != "" || vm.ARError != "" || vm.AmountError != "" {
		return pages.ReceivePayment(vm).Render(c.Context(), c)
	}

	// Save as a journal entry in a transaction.
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		return services.RecordReceivePayment(tx, services.ReceivePaymentInput{
			CustomerID:     uint(custU64),
			EntryDate:      entryDate,
			BankAccountID:  uint(bankU64),
			ARAccountID:    uint(arU64),
			Amount:         amount,
			Memo:           memo,
		})
	}); err != nil {
		vm.FormError = "Could not record payment. Please try again."
		return pages.ReceivePayment(vm).Render(c.Context(), c)
	}
	_ = services.WriteAuditLog(s.DB, "payment.received", "journal_entry", 0, "system", map[string]any{
		"customer_id": customerIDRaw,
		"amount":      amount.StringFixed(2),
		"entry_date":  entryDateRaw,
	})

	return c.Redirect("/banking/receive-payment?saved=1", fiber.StatusSeeOther)
}

func (s *Server) handlePayBillsForm(c *fiber.Ctx) error {
	var vendors []models.Vendor
	_ = s.DB.Order("name asc").Find(&vendors).Error

	var accounts []models.Account
	_ = s.DB.Order("code asc").Find(&accounts).Error

	vm := pages.PayBillsVM{
		HasCompany: true,
		Vendors:    vendors,
		Accounts:   accounts,
		Saved:      c.Query("saved") == "1",
		EntryDate:  time.Now().Format("2006-01-02"),
	}

	return pages.PayBills(vm).Render(c.Context(), c)
}

func (s *Server) handlePayBillsSubmit(c *fiber.Ctx) error {
	var vendors []models.Vendor
	_ = s.DB.Order("name asc").Find(&vendors).Error
	var accounts []models.Account
	_ = s.DB.Order("code asc").Find(&accounts).Error

	vendorIDRaw := strings.TrimSpace(c.FormValue("vendor_id"))
	entryDateRaw := strings.TrimSpace(c.FormValue("entry_date"))
	bankIDRaw := strings.TrimSpace(c.FormValue("bank_account_id"))
	apIDRaw := strings.TrimSpace(c.FormValue("ap_account_id"))
	amountRaw := strings.TrimSpace(c.FormValue("amount"))
	memo := strings.TrimSpace(c.FormValue("memo"))

	vm := pages.PayBillsVM{
		HasCompany:     true,
		Vendors:        vendors,
		Accounts:       accounts,
		VendorID:       vendorIDRaw,
		EntryDate:      entryDateRaw,
		BankAccountID:  bankIDRaw,
		APAccountID:    apIDRaw,
		Amount:         amountRaw,
		Memo:           memo,
	}

	venU64, err := services.ParseUint(vendorIDRaw)
	if err != nil || venU64 == 0 {
		vm.VendorError = "Vendor is required."
	}

	entryDate, err := time.Parse("2006-01-02", entryDateRaw)
	if err != nil {
		vm.DateError = "Date is required."
	}

	bankU64, err := services.ParseUint(bankIDRaw)
	if err != nil || bankU64 == 0 {
		vm.BankError = "Bank account is required."
	}

	apU64, err := services.ParseUint(apIDRaw)
	if err != nil || apU64 == 0 {
		vm.APError = "A/P account is required."
	}

	amount, err := services.ParseDecimalMoney(amountRaw)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		vm.AmountError = "Amount must be greater than 0."
	}

	if vm.VendorError != "" || vm.DateError != "" || vm.BankError != "" || vm.APError != "" || vm.AmountError != "" {
		return pages.PayBills(vm).Render(c.Context(), c)
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		return services.RecordPayBills(tx, services.PayBillsInput{
			VendorID:      uint(venU64),
			EntryDate:     entryDate,
			BankAccountID: uint(bankU64),
			APAccountID:   uint(apU64),
			Amount:        amount,
			Memo:          memo,
		})
	}); err != nil {
		vm.FormError = "Could not record payment. Please try again."
		return pages.PayBills(vm).Render(c.Context(), c)
	}
	_ = services.WriteAuditLog(s.DB, "bills.paid", "journal_entry", 0, "system", map[string]any{
		"vendor_id":  vendorIDRaw,
		"amount":     amount.StringFixed(2),
		"entry_date": entryDateRaw,
	})

	return c.Redirect("/banking/pay-bills?saved=1", fiber.StatusSeeOther)
}

func (s *Server) handleAuditLog(c *fiber.Ctx) error {
	filterQ := strings.TrimSpace(c.Query("q"))
	filterAction := strings.TrimSpace(c.Query("action"))
	filterEntity := strings.TrimSpace(c.Query("entity"))
	filterFrom := strings.TrimSpace(c.Query("from"))
	filterTo := strings.TrimSpace(c.Query("to"))

	page := 1
	if pageRaw := strings.TrimSpace(c.Query("page")); pageRaw != "" {
		if p, err := services.ParseUint(pageRaw); err == nil && p > 0 {
			page = int(p)
		}
	}

	const pageSize = 50
	offset := (page - 1) * pageSize

	base := s.DB.Model(&models.AuditLog{})
	if filterQ != "" {
		like := "%" + filterQ + "%"
		base = base.Where(
			"LOWER(action) LIKE LOWER(?) OR LOWER(entity_type) LIKE LOWER(?) OR LOWER(details_json) LIKE LOWER(?)",
			like, like, like,
		)
	}
	if filterAction != "" {
		base = base.Where("action = ?", filterAction)
	}
	if filterEntity != "" {
		base = base.Where("entity_type = ?", filterEntity)
	}
	if filterFrom != "" {
		if d, err := time.Parse("2006-01-02", filterFrom); err == nil {
			base = base.Where("created_at >= ?", d)
		}
	}
	if filterTo != "" {
		if d, err := time.Parse("2006-01-02", filterTo); err == nil {
			base = base.Where("created_at < ?", d.AddDate(0, 0, 1))
		}
	}

	var total int64
	_ = base.Count(&total).Error

	var rows []models.AuditLog
	_ = base.Order("created_at desc, id desc").Offset(offset).Limit(pageSize).Find(&rows).Error

	var actions []string
	_ = s.DB.Model(&models.AuditLog{}).Distinct().Order("action asc").Pluck("action", &actions).Error
	var entities []string
	_ = s.DB.Model(&models.AuditLog{}).Distinct().Order("entity_type asc").Pluck("entity_type", &entities).Error

	vm := pages.AuditLogVM{
		HasCompany: true,
		Items:      rows,

		FilterQ:      filterQ,
		FilterAction: filterAction,
		FilterEntity: filterEntity,
		FilterFrom:   filterFrom,
		FilterTo:     filterTo,
		Actions:      actions,
		Entities:     entities,

		Page:       page,
		PrevPage:   page - 1,
		NextPage:   page + 1,
		HasPrev:    page > 1,
		HasNext:    int64(offset+pageSize) < total,
		TotalCount: total,
	}

	return pages.AuditLog(vm).Render(c.Context(), c)
}

func (s *Server) handleNumberingSettingsGet(c *fiber.Ctx) error {
	var company models.Company
	if err := s.DB.Order("id asc").First(&company).Error; err != nil {
		return pages.NumberingSettings(pages.NumberingSettingsVM{
			HasCompany: false,
			FormError:  "Company not found. Please run setup first.",
			Rules:      numbering.DefaultDisplayRules(),
		}).Render(c.Context(), c)
	}

	rules, err := numbering.LoadMerged(numbering.DefaultStorePath())
	if err != nil {
		return pages.NumberingSettings(pages.NumberingSettingsVM{
			HasCompany: true,
			FormError:  "Could not load numbering settings.",
			Rules:      numbering.DefaultDisplayRules(),
		}).Render(c.Context(), c)
	}

	return pages.NumberingSettings(pages.NumberingSettingsVM{
		HasCompany: true,
		Rules:      rules,
		Saved:      c.Query("saved") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleNumberingSettingsPost(c *fiber.Ctx) error {
	var company models.Company
	if err := s.DB.Order("id asc").First(&company).Error; err != nil {
		return c.Redirect("/setup", fiber.StatusSeeOther)
	}

	rules, err := numbering.ParseRulesPost(c)
	if err != nil {
		return pages.NumberingSettings(pages.NumberingSettingsVM{
			HasCompany: true,
			FormError:  "Invalid form data.",
			Rules:      numbering.DefaultDisplayRules(),
		}).Render(c.Context(), c)
	}

	if err := numbering.Save(numbering.DefaultStorePath(), rules); err != nil {
		return pages.NumberingSettings(pages.NumberingSettingsVM{
			HasCompany: true,
			FormError:  "Could not save numbering settings. Check that the app can write to the data directory.",
			Rules:      rules,
		}).Render(c.Context(), c)
	}

	_ = services.WriteAuditLog(s.DB, "settings.numbering.saved", "settings", company.ID, "system", map[string]any{
		"modules": len(rules),
	})

	return c.Redirect("/settings/numbering?saved=1", fiber.StatusSeeOther)
}

func (s *Server) handleAIConnectGet(c *fiber.Ctx) error {
	var company models.Company
	if err := s.DB.Order("id asc").First(&company).Error; err != nil {
		return pages.AIConnect(pages.AIConnectVM{
			HasCompany: false,
			FormError:  "Company not found. Please run setup first.",
		}).Render(c.Context(), c)
	}

	row, err := services.LoadAIConnectionSettings(s.DB, company.ID)
	if err != nil {
		return pages.AIConnect(pages.AIConnectVM{
			HasCompany: true,
			FormError:  "Could not load AI connection settings.",
		}).Render(c.Context(), c)
	}

	vm := aiConnectVMFromRow(row)
	vm.HasCompany = true
	vm.Saved = c.Query("saved") == "1"
	vm.Tested = c.Query("tested") == "1"
	return pages.AIConnect(vm).Render(c.Context(), c)
}

func (s *Server) handleAIConnectPost(c *fiber.Ctx) error {
	var company models.Company
	if err := s.DB.Order("id asc").First(&company).Error; err != nil {
		return c.Redirect("/setup", fiber.StatusSeeOther)
	}

	provider, err := services.ParseAIProvider(c.FormValue("provider"))
	if err != nil {
		row, _ := services.LoadAIConnectionSettings(s.DB, company.ID)
		vm := aiConnectVMFromRow(row)
		vm.HasCompany = true
		vm.FormError = "Invalid provider."
		return pages.AIConnect(vm).Render(c.Context(), c)
	}

	enabled := c.FormValue("enabled") == "true"
	vision := c.FormValue("vision_enabled") == "true"
	apiKey := strings.TrimSpace(c.FormValue("api_key"))
	baseURL := strings.TrimSpace(c.FormValue("api_base_url"))
	model := strings.TrimSpace(c.FormValue("model_name"))

	if err := services.UpsertAIConnectionSettings(s.DB, company.ID, provider, baseURL, apiKey, model, enabled, vision); err != nil {
		row, _ := services.LoadAIConnectionSettings(s.DB, company.ID)
		vm := aiConnectVMFromRow(row)
		vm.HasCompany = true
		vm.FormError = "Could not save AI connection settings."
		vm.Provider = provider
		vm.APIBaseURL = baseURL
		vm.ModelName = model
		vm.Enabled = enabled
		vm.VisionEnabled = vision
		vm.HasAPIKey = row.APIKey != "" || apiKey != ""
		vm.APIKeyHint = services.MaskAPIKey(row.APIKey)
		return pages.AIConnect(vm).Render(c.Context(), c)
	}

	_ = services.WriteAuditLog(s.DB, "settings.ai_connect.saved", "settings", company.ID, "system", map[string]any{
		"provider":       provider,
		"enabled":        enabled,
		"vision_enabled": vision,
	})

	return c.Redirect("/settings/ai-connect?saved=1", fiber.StatusSeeOther)
}

func (s *Server) handleAIConnectTestPost(c *fiber.Ctx) error {
	var company models.Company
	if err := s.DB.Order("id asc").First(&company).Error; err != nil {
		return c.Redirect("/setup", fiber.StatusSeeOther)
	}

	ok, msg, skipped, err := services.RunAIConnectionTest(s.DB, company.ID)
	if err != nil {
		row, _ := services.LoadAIConnectionSettings(s.DB, company.ID)
		vm := aiConnectVMFromRow(row)
		vm.HasCompany = true
		vm.FormError = "Could not run connection test."
		return pages.AIConnect(vm).Render(c.Context(), c)
	}
	if skipped {
		row, _ := services.LoadAIConnectionSettings(s.DB, company.ID)
		vm := aiConnectVMFromRow(row)
		vm.HasCompany = true
		vm.FormError = msg
		return pages.AIConnect(vm).Render(c.Context(), c)
	}

	_ = services.WriteAuditLog(s.DB, "settings.ai_connect.tested", "settings", company.ID, "system", map[string]any{
		"ok":      ok,
		"message": msg,
	})

	return c.Redirect("/settings/ai-connect?tested=1", fiber.StatusSeeOther)
}

func aiConnectVMFromRow(row models.AIConnectionSettings) pages.AIConnectVM {
	vm := pages.AIConnectVM{
		Provider:      row.Provider,
		APIBaseURL:    row.APIBaseURL,
		ModelName:     row.ModelName,
		Enabled:       row.Enabled,
		VisionEnabled: row.VisionEnabled,
		HasAPIKey:     row.APIKey != "",
		APIKeyHint:    services.MaskAPIKey(row.APIKey),
	}
	if vm.Provider == "" {
		vm.Provider = models.AIProviderOpenAICompatible
	}
	if row.LastTestAt != nil {
		vm.HasLastTest = true
		vm.LastTestAtFormatted = row.LastTestAt.Format(time.RFC3339)
		vm.LastTestOK = row.LastTestOK
		vm.LastTestMessage = row.LastTestMessage
	}
	return vm
}

func (s *Server) handleJournalEntryList(c *fiber.Ctx) error {
	formError := ""
	if c.Query("error") == "already-reversed" {
		formError = "This journal entry is already reversed."
	}

	var entries []models.JournalEntry
	if err := s.DB.Preload("Lines").Order("entry_date desc, id desc").Limit(200).Find(&entries).Error; err != nil {
		return pages.JournalEntryList(pages.JournalEntryListVM{
			HasCompany: true,
			Active:     "Journal Entry",
			Items:      []pages.JournalEntryListItem{},
			FormError:  "Could not load journal entries.",
		}).Render(c.Context(), c)
	}

	reversedFromSet := map[uint]bool{}
	for _, e := range entries {
		if e.ReversedFromID != nil {
			reversedFromSet[*e.ReversedFromID] = true
		}
	}

	items := make([]pages.JournalEntryListItem, 0, len(entries))
	for _, e := range entries {
		totalDebit := decimal.Zero
		totalCredit := decimal.Zero
		for _, l := range e.Lines {
			totalDebit = totalDebit.Add(l.Debit)
			totalCredit = totalCredit.Add(l.Credit)
		}
		canReverse := e.ReversedFromID == nil && !reversedFromSet[e.ID]
		reverseHint := ""
		if e.ReversedFromID != nil {
			reverseHint = "This is already a reversal entry."
		} else if reversedFromSet[e.ID] {
			reverseHint = "Already reversed."
		}
		items = append(items, pages.JournalEntryListItem{
			ID:          e.ID,
			EntryDate:   e.EntryDate.Format("2006-01-02"),
			JournalNo:   e.JournalNo,
			LineCount:   len(e.Lines),
			TotalDebit:  pages.Money(totalDebit),
			TotalCredit: pages.Money(totalCredit),
			CanReverse:  canReverse,
			ReverseHint: reverseHint,
		})
	}

	return pages.JournalEntryList(pages.JournalEntryListVM{
		HasCompany: true,
		Active:     "Journal Entry",
		Items:      items,
		FormError:  formError,
		Reversed:   c.Query("reversed") == "1",
	}).Render(c.Context(), c)
}

func (s *Server) handleJournalEntryReverse(c *fiber.Ctx) error {
	idRaw := strings.TrimSpace(c.Params("id"))
	idU64, err := services.ParseUint(idRaw)
	if err != nil || idU64 == 0 {
		return c.Redirect("/journal-entry/list", fiber.StatusSeeOther)
	}

	reverseDate := time.Now()
	reverseDateRaw := strings.TrimSpace(c.FormValue("reverse_date"))
	if reverseDateRaw != "" {
		if d, err := time.Parse("2006-01-02", reverseDateRaw); err == nil {
			reverseDate = d
		}
	}

	var reversedID uint
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		newID, err := services.ReverseJournalEntry(tx, uint(idU64), reverseDate)
		if err != nil {
			return err
		}
		reversedID = newID
		return nil
	}); err != nil {
		return c.Redirect("/journal-entry/list?error=already-reversed", fiber.StatusSeeOther)
	}

	_ = services.WriteAuditLog(s.DB, "journal.reversed", "journal_entry", reversedID, "system", map[string]any{
		"original_id": idU64,
		"reverse_date": reverseDate.Format("2006-01-02"),
	})

	return c.Redirect("/journal-entry/list?reversed=1", fiber.StatusSeeOther)
}

