// 遵循产品需求 v1.0
package web

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

func (s *Server) handleBankReconcileForm(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	accounts, err := s.activeAccountsForCompany(companyID)
	if err != nil {
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
		HasCompany:        true,
		Accounts:          accounts,
		AccountID:         accountIDStr,
		StatementDate:     statementDateStr,
		EndingBalance:     endingBalanceStr,
		Active:            "Bank Reconcile",
		Saved:             c.Query("saved") == "1",
		PreviouslyCleared: "0.00",
		Candidates:        []services.ReconcileCandidate{},
	}

	if accountIDStr == "" {
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}

	accountIDU64, err := services.ParseUint(accountIDStr)
	if err != nil || accountIDU64 == 0 {
		vm.FormError = "Invalid account selected."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}
	accountID := uint(accountIDU64)

	var accRow models.Account
	if err := s.DB.Where("id = ? AND company_id = ?", accountID, companyID).First(&accRow).Error; err != nil {
		vm.FormError = "Invalid account selected."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}

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

	if endingBalanceStr == "" {
		endingBalanceStr = "0.00"
		vm.EndingBalance = endingBalanceStr
	}
	if _, err := services.ParseDecimalMoney(endingBalanceStr); err != nil {
		vm.FormError = "Ending Balance must be a number."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}

	prev, err := services.ClearedBalance(s.DB, companyID, accountID, statementDate)
	if err != nil {
		vm.FormError = "Could not load cleared balance."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}
	vm.PreviouslyCleared = pages.Money(prev)

	cands, err := services.ListReconcileCandidates(s.DB, companyID, accountID, statementDate)
	if err != nil {
		vm.FormError = "Could not load unreconciled transactions."
		return pages.BankReconcile(vm).Render(c.Context(), c)
	}
	vm.Candidates = cands

	return pages.BankReconcile(vm).Render(c.Context(), c)
}

func (s *Server) handleBankReconcileSubmit(c *fiber.Ctx) error {
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	accountIDStr := strings.TrimSpace(c.FormValue("account_id"))
	statementDateStr := strings.TrimSpace(c.FormValue("statement_date"))
	endingBalanceStr := strings.TrimSpace(c.FormValue("ending_balance"))

	accountIDU64, err := services.ParseUint(accountIDStr)
	if err != nil || accountIDU64 == 0 {
		return c.Redirect("/banking/reconcile", fiber.StatusSeeOther)
	}
	accountID := uint(accountIDU64)

	if err := s.DB.Where("id = ? AND company_id = ?", accountID, companyID).First(new(models.Account)).Error; err != nil {
		return c.Redirect("/banking/reconcile", fiber.StatusSeeOther)
	}

	statementDate, err := time.Parse("2006-01-02", statementDateStr)
	if err != nil {
		return c.Redirect("/banking/reconcile?account_id=" + accountIDStr, fiber.StatusSeeOther)
	}

	endingBalance, err := services.ParseDecimalMoney(endingBalanceStr)
	if err != nil {
		return c.Redirect("/banking/reconcile?account_id=" + accountIDStr + "&statement_date=" + statementDateStr, fiber.StatusSeeOther)
	}

	lineIDBytes := c.Context().PostArgs().PeekMulti("line_ids")
	lineIDs := make([]string, 0, len(lineIDBytes))
	for _, b := range lineIDBytes {
		lineIDs = append(lineIDs, string(b))
	}
	if len(lineIDs) == 0 {
		return c.Redirect("/banking/reconcile?account_id=" + accountIDStr + "&statement_date=" + statementDateStr + "&ending_balance=" + endingBalanceStr, fiber.StatusSeeOther)
	}

	var ids []uint
	for _, sID := range lineIDs {
		u, err := services.ParseUint(sID)
		if err != nil || u == 0 {
			continue
		}
		ids = append(ids, uint(u))
	}
	if len(ids) == 0 {
		return c.Redirect("/banking/reconcile?account_id=" + accountIDStr + "&statement_date=" + statementDateStr + "&ending_balance=" + endingBalanceStr, fiber.StatusSeeOther)
	}

	decimalZero := decimal.NewFromInt(0)

	var savedRecID uint
	var clearedSnapshot decimal.Decimal
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		prevCleared, err := services.ClearedBalance(tx, companyID, accountID, statementDate)
		if err != nil {
			return err
		}

		type row struct{ Amount decimal.Decimal }
		var r row
		if err := tx.Raw(
			`
SELECT COALESCE(SUM(jl.debit - jl.credit), 0) AS amount
FROM journal_lines jl
JOIN journal_entries je ON je.id = jl.journal_entry_id
WHERE jl.id IN ?
  AND jl.account_id = ?
  AND jl.company_id = ?
  AND jl.reconciliation_id IS NULL
  AND je.entry_date <= ?
  AND je.company_id = ?
`,
			ids, accountID, companyID, statementDate, companyID,
		).Scan(&r).Error; err != nil {
			return err
		}

		cleared := prevCleared.Add(r.Amount)
		clearedSnapshot = cleared
		diff := endingBalance.Sub(cleared)
		if !diff.Equal(decimalZero) {
			return errors.New("difference not zero")
		}

		rec := models.Reconciliation{
			CompanyID:      companyID,
			AccountID:      accountID,
			StatementDate:  statementDate,
			EndingBalance:  endingBalance,
			ClearedBalance: cleared,
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		savedRecID = rec.ID

		now := time.Now()
		if err := tx.Model(&models.JournalLine{}).
			Where("id IN ?", ids).
			Where("account_id = ?", accountID).
			Where("company_id = ?", companyID).
			Where("reconciliation_id IS NULL").
			Updates(map[string]any{
				"reconciliation_id": rec.ID,
				"reconciled_at":     &now,
			}).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return c.Redirect("/banking/reconcile?account_id="+accountIDStr+"&statement_date="+statementDateStr+"&ending_balance="+endingBalanceStr, fiber.StatusSeeOther)
	}

	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	cid := companyID
	uid := user.ID
	_ = services.WriteAuditLogWithContext(s.DB, "banking.reconciliation.completed", "reconciliation", savedRecID, actor, map[string]any{
		"account_id":      accountID,
		"statement_date":  statementDateStr,
		"line_count":      len(ids),
		"ending_balance":  endingBalance.StringFixed(2),
		"cleared_balance": clearedSnapshot.StringFixed(2),
		"company_id":      companyID,
	}, &cid, &uid)

	return c.Redirect("/banking/reconcile?account_id=" + accountIDStr + "&statement_date=" + statementDateStr + "&ending_balance=" + endingBalanceStr + "&saved=1", fiber.StatusSeeOther)
}

func (s *Server) handleReceivePaymentForm(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	var customers []models.Customer
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&customers).Error

	accounts, _ := s.activeAccountsForCompany(companyID)

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
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	var customers []models.Customer
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&customers).Error
	accounts, _ := s.activeAccountsForCompany(companyID)

	customerIDRaw := strings.TrimSpace(c.FormValue("customer_id"))
	entryDateRaw := strings.TrimSpace(c.FormValue("entry_date"))
	bankIDRaw := strings.TrimSpace(c.FormValue("bank_account_id"))
	arIDRaw := strings.TrimSpace(c.FormValue("ar_account_id"))
	amountRaw := strings.TrimSpace(c.FormValue("amount"))
	memo := strings.TrimSpace(c.FormValue("memo"))

	vm := pages.ReceivePaymentVM{
		HasCompany:    true,
		Customers:     customers,
		Accounts:      accounts,
		CustomerID:    customerIDRaw,
		EntryDate:     entryDateRaw,
		BankAccountID: bankIDRaw,
		ARAccountID:   arIDRaw,
		Amount:        amountRaw,
		Memo:          memo,
	}

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

	var jeID uint
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		jeID, err = services.RecordReceivePayment(tx, services.ReceivePaymentInput{
			CompanyID:     companyID,
			CustomerID:    uint(custU64),
			EntryDate:     entryDate,
			BankAccountID: uint(bankU64),
			ARAccountID:   uint(arU64),
			Amount:        amount,
			Memo:          memo,
		})
		return err
	}); err != nil {
		vm.FormError = "Could not record payment. Please try again."
		return pages.ReceivePayment(vm).Render(c.Context(), c)
	}

	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	cid := companyID
	uid := user.ID
	_ = services.WriteAuditLogWithContext(s.DB, "payment.received", "journal_entry", jeID, actor, map[string]any{
		"customer_id": customerIDRaw,
		"amount":      amount.StringFixed(2),
		"entry_date":  entryDateRaw,
		"company_id":  companyID,
	}, &cid, &uid)

	return c.Redirect("/banking/receive-payment?saved=1", fiber.StatusSeeOther)
}

func (s *Server) handlePayBillsForm(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	var vendors []models.Vendor
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&vendors).Error

	accounts, _ := s.activeAccountsForCompany(companyID)

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
	user := UserFromCtx(c)
	if user == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	var vendors []models.Vendor
	_ = s.DB.Where("company_id = ?", companyID).Order("name asc").Find(&vendors).Error
	accounts, _ := s.activeAccountsForCompany(companyID)

	vendorIDRaw := strings.TrimSpace(c.FormValue("vendor_id"))
	entryDateRaw := strings.TrimSpace(c.FormValue("entry_date"))
	bankIDRaw := strings.TrimSpace(c.FormValue("bank_account_id"))
	apIDRaw := strings.TrimSpace(c.FormValue("ap_account_id"))
	amountRaw := strings.TrimSpace(c.FormValue("amount"))
	memo := strings.TrimSpace(c.FormValue("memo"))

	vm := pages.PayBillsVM{
		HasCompany:    true,
		Vendors:       vendors,
		Accounts:      accounts,
		VendorID:      vendorIDRaw,
		EntryDate:     entryDateRaw,
		BankAccountID: bankIDRaw,
		APAccountID:   apIDRaw,
		Amount:        amountRaw,
		Memo:          memo,
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

	var jeID uint
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		jeID, err = services.RecordPayBills(tx, services.PayBillsInput{
			CompanyID:     companyID,
			VendorID:      uint(venU64),
			EntryDate:     entryDate,
			BankAccountID: uint(bankU64),
			APAccountID:   uint(apU64),
			Amount:        amount,
			Memo:          memo,
		})
		return err
	}); err != nil {
		vm.FormError = "Could not record payment. Please try again."
		return pages.PayBills(vm).Render(c.Context(), c)
	}

	actor := user.Email
	if actor == "" {
		actor = "user"
	}
	cid := companyID
	uid := user.ID
	_ = services.WriteAuditLogWithContext(s.DB, "bills.paid", "journal_entry", jeID, actor, map[string]any{
		"vendor_id":  vendorIDRaw,
		"amount":     amount.StringFixed(2),
		"entry_date": entryDateRaw,
		"company_id": companyID,
	}, &cid, &uid)

	return c.Redirect("/banking/pay-bills?saved=1", fiber.StatusSeeOther)
}
