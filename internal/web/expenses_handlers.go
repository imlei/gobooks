package web

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"gobooks/internal/models"
	"gobooks/internal/services"
	"gobooks/internal/web/templates/pages"
)

// rehydrateExpenseAccountLabel uses the SmartPicker registry to look up the
// human-readable label for the given account ID, enforcing the same company /
// active / expense-type guards as the search provider. Returns "" if the ID
// is empty or the account is no longer valid under current guards.
func (s *Server) rehydrateExpenseAccountLabel(companyID uint, idStr string) string {
	if idStr == "" {
		return ""
	}
	p, ok := defaultSmartPickerRegistry.get("account")
	if !ok {
		return ""
	}
	item, err := p.GetByID(s.DB, SmartPickerContext{CompanyID: companyID, Context: "expense_form_category"}, idStr)
	if err != nil || item == nil {
		return ""
	}
	return item.Primary
}

// rehydrateVendorLabel uses the VendorProvider to look up the human-readable
// label for the given vendor ID. Returns "" if the ID is empty or the vendor
// is not found within the company scope.
func (s *Server) rehydrateVendorLabel(companyID uint, idStr string) string {
	if idStr == "" {
		return ""
	}
	p, ok := defaultSmartPickerRegistry.get("vendor")
	if !ok {
		return ""
	}
	item, err := p.GetByID(s.DB, SmartPickerContext{CompanyID: companyID, Context: "expense_form_vendor"}, idStr)
	if err != nil || item == nil {
		return ""
	}
	return item.Primary
}

// rehydratePaymentAccountLabel uses the PaymentAccountProvider to look up the
// human-readable label for the given payment account ID. Returns "" if the ID
// is empty or the account no longer satisfies the payment-account guards.
func (s *Server) rehydratePaymentAccountLabel(companyID uint, idStr string) string {
	if idStr == "" {
		return ""
	}
	p, ok := defaultSmartPickerRegistry.get("payment_account")
	if !ok {
		return ""
	}
	item, err := p.GetByID(s.DB, SmartPickerContext{CompanyID: companyID, Context: "expense_form_payment"}, idStr)
	if err != nil || item == nil {
		return ""
	}
	return item.Primary
}

func (s *Server) handleExpenses(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	expenses, err := services.ListExpenses(s.DB, services.ExpenseListFilter{CompanyID: companyID})
	if err != nil {
		return pages.Expenses(pages.ExpenseListVM{
			HasCompany: true,
			FormError:  "Could not load expenses.",
		}).Render(c.Context(), c)
	}

	return pages.Expenses(pages.ExpenseListVM{
		HasCompany: true,
		FormError:  strings.TrimSpace(c.Query("error")),
		Created:    c.Query("created") == "1",
		Updated:    c.Query("updated") == "1",
		CanCreate:  CanFromCtx(c, ActionBillCreate),
		CanUpdate:  CanFromCtx(c, ActionBillUpdate),
		Expenses:   expenses,
	}).Render(c.Context(), c)
}

func (s *Server) handleExpenseNew(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	vm, err := s.newExpenseFormVM(companyID)
	if err != nil {
		return pages.ExpenseForm(pages.ExpenseFormVM{
			HasCompany: true,
			FormError:  "Could not load expense form.",
		}).Render(c.Context(), c)
	}
	return pages.ExpenseForm(vm).Render(c.Context(), c)
}

func (s *Server) handleExpenseCreate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	vm, input, hasErr := s.buildExpenseFormVMFromRequest(c, companyID, nil)
	if hasErr {
		return pages.ExpenseForm(vm).Render(c.Context(), c)
	}

	expense, err := services.CreateExpense(s.DB, input)
	if err != nil {
		s.applyExpenseServiceError(&vm, err)
		return pages.ExpenseForm(vm).Render(c.Context(), c)
	}
	_ = expense
	return redirectTo(c, "/expenses?created=1")
}

func (s *Server) handleExpenseEdit(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	expenseID, err := parseExpenseID(c)
	if err != nil {
		return redirectErr(c, "/expenses", "invalid expense ID")
	}

	expense, err := services.GetExpenseByID(s.DB, companyID, expenseID)
	if err != nil {
		return redirectErr(c, "/expenses", err.Error())
	}

	vm, err := s.expenseFormVMFromExpense(companyID, expense)
	if err != nil {
		return redirectErr(c, "/expenses", "could not load expense form")
	}
	vm.FormError = strings.TrimSpace(c.Query("error"))
	return pages.ExpenseForm(vm).Render(c.Context(), c)
}

func (s *Server) handleExpenseUpdate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	expenseID, err := parseExpenseID(c)
	if err != nil {
		return redirectErr(c, "/expenses", "invalid expense ID")
	}

	existing, err := services.GetExpenseByID(s.DB, companyID, expenseID)
	if err != nil {
		return redirectErr(c, "/expenses", err.Error())
	}
	vm, input, hasErr := s.buildExpenseFormVMFromRequest(c, companyID, existing)
	vm.IsEdit = true
	vm.EditingID = expenseID
	if hasErr {
		return pages.ExpenseForm(vm).Render(c.Context(), c)
	}

	expense, err := services.UpdateExpense(s.DB, companyID, expenseID, input)
	if err != nil {
		s.applyExpenseServiceError(&vm, err)
		return pages.ExpenseForm(vm).Render(c.Context(), c)
	}
	_ = expense
	return redirectTo(c, "/expenses?updated=1")
}

func parseExpenseID(c *fiber.Ctx) (uint, error) {
	idRaw := strings.TrimSpace(c.Params("id"))
	id64, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || id64 == 0 {
		return 0, fmt.Errorf("invalid expense ID")
	}
	return uint(id64), nil
}

func (s *Server) newExpenseFormVM(companyID uint) (pages.ExpenseFormVM, error) {
	vm := pages.ExpenseFormVM{
		HasCompany:  true,
		ExpenseDate: time.Now().Format("2006-01-02"),
		Amount:      "0.00",
	}
	if err := s.loadExpenseFormContext(companyID, &vm); err != nil {
		return vm, err
	}
	if vm.CurrencyCode == "" {
		vm.CurrencyCode = vm.BaseCurrencyCode
	}
	return vm, nil
}

func (s *Server) expenseFormVMFromExpense(companyID uint, exp *models.Expense) (pages.ExpenseFormVM, error) {
	vm := pages.ExpenseFormVM{
		HasCompany:      true,
		IsEdit:          true,
		EditingID:       exp.ID,
		ExpenseDate:     exp.ExpenseDate.Format("2006-01-02"),
		Description:     exp.Description,
		Amount:          exp.Amount.StringFixed(2),
		CurrencyCode:    exp.CurrencyCode,
		VendorID:        optUintStr(exp.VendorID),
		TaskID:          optUintStr(exp.TaskID),
		IsBillable:      exp.IsBillable,
		Notes:           exp.Notes,
		PaymentMethod:   string(exp.PaymentMethod),
		PaymentReference: exp.PaymentReference,
	}

	// Rehydrate vendor label for SmartPicker.
	vm.VendorLabel = s.rehydrateVendorLabel(companyID, vm.VendorID)

	// Rehydrate expense account for SmartPicker. GetByID enforces the same
	// company / active / expense-type guards as the search provider.
	if exp.ExpenseAccountID != nil {
		idStr := fmt.Sprintf("%d", *exp.ExpenseAccountID)
		label := s.rehydrateExpenseAccountLabel(companyID, idStr)
		if label != "" {
			vm.ExpenseAccountID = idStr
			vm.ExpenseAccountLabel = label
		} else {
			// Account existed but is no longer valid under current provider guards.
			// Clear both fields so the picker shows blank and the user must re-select.
			// Hidden input will submit "" which the backend correctly rejects with a clear error.
			vm.ExpenseAccountID = ""
			vm.ExpenseAccountLabel = ""
			vm.ExpenseAccountError = "Previously selected expense account is no longer available. Please choose a new one."
		}
	}

	// Rehydrate payment account for SmartPicker.
	if exp.PaymentAccountID != nil {
		idStr := fmt.Sprintf("%d", *exp.PaymentAccountID)
		label := s.rehydratePaymentAccountLabel(companyID, idStr)
		if label != "" {
			vm.PaymentAccountID = idStr
			vm.PaymentAccountLabel = label
		} else {
			vm.PaymentAccountID = ""
			vm.PaymentAccountLabel = ""
			vm.PaymentAccountError = "Previously selected payment account is no longer available. Please choose a new one."
		}
	}

	if err := s.loadExpenseFormContext(companyID, &vm); err != nil {
		return vm, err
	}
	return vm, nil
}

func (s *Server) buildExpenseFormVMFromRequest(c *fiber.Ctx, companyID uint, existing *models.Expense) (pages.ExpenseFormVM, services.ExpenseInput, bool) {
	vm := pages.ExpenseFormVM{HasCompany: true}
	if existing != nil {
		vm.IsEdit = true
		vm.EditingID = existing.ID
	}
	_ = s.loadExpenseFormContext(companyID, &vm)

	vm.ExpenseDate = strings.TrimSpace(c.FormValue("expense_date"))
	vm.Description = strings.TrimSpace(c.FormValue("description"))
	vm.Amount = strings.TrimSpace(c.FormValue("amount"))
	vm.CurrencyCode = strings.ToUpper(strings.TrimSpace(c.FormValue("currency_code")))
	vm.VendorID = strings.TrimSpace(c.FormValue("vendor_id"))
	vm.ExpenseAccountID = strings.TrimSpace(c.FormValue("expense_account_id"))
	vm.TaskID = strings.TrimSpace(c.FormValue("task_id"))
	vm.PaymentAccountID = strings.TrimSpace(c.FormValue("payment_account_id"))
	vm.PaymentMethod = strings.TrimSpace(c.FormValue("payment_method"))
	vm.PaymentReference = strings.TrimSpace(c.FormValue("payment_reference"))

	// Rehydrate vendor label for error re-render.
	vm.VendorLabel = s.rehydrateVendorLabel(companyID, vm.VendorID)

	// Rehydrate SmartPicker label for error re-render: if the submitted account ID is
	// still valid, show its label so the user sees their selection. If invalid (cross-company,
	// inactive, wrong type), label stays "" — the picker shows blank, matching the empty
	// submission that the backend is about to reject.
	vm.ExpenseAccountLabel = s.rehydrateExpenseAccountLabel(companyID, vm.ExpenseAccountID)
	// If the submitted account ID fails the provider's guards (inactive, cross-company,
	// wrong root type), rehydrate returns "". Clear the ID too so the hidden input and
	// visible picker are consistent — both blank — matching GET invalid-account behaviour.
	if vm.ExpenseAccountID != "" && vm.ExpenseAccountLabel == "" {
		vm.ExpenseAccountID = ""
	}

	// Rehydrate payment account SmartPicker label for error re-render.
	vm.PaymentAccountLabel = s.rehydratePaymentAccountLabel(companyID, vm.PaymentAccountID)

	vm.IsBillable = c.FormValue("is_billable") == "1"
	vm.Notes = strings.TrimSpace(c.FormValue("notes"))

	if vm.Amount == "" {
		vm.Amount = "0.00"
	}
	if vm.CurrencyCode == "" {
		vm.CurrencyCode = vm.BaseCurrencyCode
	}

	var input services.ExpenseInput
	input.CompanyID = companyID
	input.Description = vm.Description
	input.CurrencyCode = vm.CurrencyCode
	input.IsBillable = vm.IsBillable
	input.Notes = vm.Notes

	var hasErr bool
	// If the submitted payment account ID fails the provider's eligibility guards (inactive,
	// cross-company, or ineligible account type such as revenue or A/P), surface an explicit
	// error. Payment account is optional, so without this check the service would skip its
	// validation and silently save without a payment account.
	if vm.PaymentAccountID != "" && vm.PaymentAccountLabel == "" {
		vm.PaymentAccountError = "The selected payment account is not available or is not a valid payment source (must be a bank, credit card, or petty-cash account)."
		vm.PaymentAccountID = ""
		hasErr = true
	}
	if d, err := time.Parse("2006-01-02", vm.ExpenseDate); err == nil {
		input.ExpenseDate = d
	} else {
		vm.ExpenseDateError = "Expense date is required."
		hasErr = true
	}
	if vm.Description == "" {
		vm.DescriptionError = "Description is required."
		hasErr = true
	}
	if amt, err := decimal.NewFromString(vm.Amount); err == nil && amt.GreaterThan(decimal.Zero) {
		input.Amount = amt
	} else {
		vm.AmountError = "Amount must be greater than zero."
		hasErr = true
	}
	if vm.CurrencyCode == "" {
		vm.CurrencyError = "Currency is required."
		hasErr = true
	} else if !containsString(vm.CurrencyOptions, vm.CurrencyCode) {
		vm.CurrencyError = "Currency is not enabled for this company."
		hasErr = true
	}

	if id64, err := services.ParseUint(vm.ExpenseAccountID); err == nil && id64 > 0 {
		id := uint(id64)
		input.ExpenseAccountID = &id
	} else {
		vm.ExpenseAccountError = "Expense account is required."
		hasErr = true
	}
	if id64, err := services.ParseUint(vm.VendorID); err == nil && id64 > 0 {
		id := uint(id64)
		input.VendorID = &id
	}
	if id64, err := services.ParseUint(vm.TaskID); err == nil && id64 > 0 {
		id := uint(id64)
		input.TaskID = &id
	}
	if id64, err := services.ParseUint(vm.PaymentAccountID); err == nil && id64 > 0 {
		id := uint(id64)
		input.PaymentAccountID = &id
	}
	input.PaymentMethod = models.PaymentMethod(vm.PaymentMethod)
	input.PaymentReference = vm.PaymentReference
	return vm, input, hasErr
}

func (s *Server) loadExpenseFormContext(companyID uint, vm *pages.ExpenseFormVM) error {
	// Vendor and expense-account lists are no longer pre-loaded here: both fields
	// use SmartPicker, which fetches candidates on-demand via the search API.
	// Eligibility filtering is enforced by the respective backend providers.
	selectableTasks, err := services.ListSelectableTasks(s.DB, companyID)
	if err != nil {
		return err
	}
	vm.SelectableTasks = selectableTasks

	var company models.Company
	if err := s.DB.Select("id", "base_currency_code", "multi_currency_enabled").First(&company, companyID).Error; err != nil {
		return err
	}
	vm.BaseCurrencyCode = company.BaseCurrencyCode
	vm.MultiCurrency = company.MultiCurrencyEnabled
	vm.CurrencyOptions = []string{company.BaseCurrencyCode}
	if company.MultiCurrencyEnabled {
		ccs, err := services.ListCompanyCurrencies(s.DB, companyID)
		if err != nil {
			return err
		}
		for _, cc := range ccs {
			if !cc.IsActive {
				continue
			}
			code := strings.ToUpper(strings.TrimSpace(cc.CurrencyCode))
			if code == "" || containsString(vm.CurrencyOptions, code) {
				continue
			}
			vm.CurrencyOptions = append(vm.CurrencyOptions, code)
		}
	}
	return nil
}

func (s *Server) applyExpenseServiceError(vm *pages.ExpenseFormVM, err error) {
	switch {
	case errors.Is(err, services.ErrExpenseDateRequired):
		vm.ExpenseDateError = err.Error()
	case errors.Is(err, services.ErrExpenseDescriptionRequired):
		vm.DescriptionError = err.Error()
	case errors.Is(err, services.ErrExpenseAmountInvalid):
		vm.AmountError = err.Error()
	case errors.Is(err, services.ErrExpenseCurrencyRequired):
		vm.CurrencyError = err.Error()
	case errors.Is(err, services.ErrExpenseAccountRequired), errors.Is(err, services.ErrExpenseAccountInvalid):
		vm.ExpenseAccountError = err.Error()
	case errors.Is(err, services.ErrExpenseVendorInvalid):
		vm.VendorError = err.Error()
	case errors.Is(err, services.ErrTaskLinkageTaskNotFound), errors.Is(err, services.ErrTaskLinkageTaskStatusInvalid):
		vm.TaskError = err.Error()
	case errors.Is(err, services.ErrTaskBillableCustomerMismatch):
		vm.BillableCustomerError = err.Error()
	case errors.Is(err, services.ErrExpensePaymentAccountInvalid):
		vm.PaymentAccountError = err.Error()
	case errors.Is(err, services.ErrExpensePaymentMethodRequired), errors.Is(err, services.ErrExpensePaymentMethodInvalid):
		vm.PaymentMethodError = err.Error()
	default:
		vm.FormError = err.Error()
	}
}
