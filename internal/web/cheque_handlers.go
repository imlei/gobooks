package web

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"balanciz/internal/models"
	"balanciz/internal/searchprojection/producers"
	"balanciz/internal/services"
	"balanciz/internal/web/templates/pages"
)

func (s *Server) handleCheques(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	q := strings.TrimSpace(c.Query("q"))
	statusRaw := strings.TrimSpace(c.Query("status"))
	filter := services.ChequeListFilter{CompanyID: companyID, Query: q, Limit: 100}
	if statusRaw != "" {
		status := models.ChequeStatus(statusRaw)
		if isChequeStatusAllowed(status) {
			filter.Status = &status
		} else {
			statusRaw = ""
		}
	}

	cheques, err := services.ListCheques(s.DB, filter)
	if err != nil {
		return pages.Cheques(pages.ChequesVM{
			HasCompany: true,
			FormError:  "Could not load cheques.",
		}).Render(c.Context(), c)
	}
	accounts, err := services.ListChequeBankAccounts(s.DB, companyID, false)
	if err != nil {
		return pages.Cheques(pages.ChequesVM{
			HasCompany: true,
			Cheques:    cheques,
			FormError:  "Could not load cheque bank accounts.",
		}).Render(c.Context(), c)
	}
	glBankAccounts, err := s.chequeLedgerBankAccounts(companyID)
	if err != nil {
		return pages.Cheques(pages.ChequesVM{
			HasCompany:   true,
			Cheques:      cheques,
			BankAccounts: accounts,
			FormError:    "Could not load ledger bank accounts.",
		}).Render(c.Context(), c)
	}

	return pages.Cheques(pages.ChequesVM{
		HasCompany:     true,
		Cheques:        cheques,
		BankAccounts:   accounts,
		GLBankAccounts: glBankAccounts,
		Query:          q,
		Status:         statusRaw,
		Created:        c.Query("created") == "1",
		AccountCreated: c.Query("account_created") == "1",
		Printed:        c.Query("printed") == "1",
		Voided:         c.Query("voided") == "1",
		FormError:      strings.TrimSpace(c.Query("error")),
		CanPrint:       CanFromCtx(c, ActionChequePrint),
		CanManageBank:  CanFromCtx(c, ActionChequeManageBank),
	}).Render(c.Context(), c)
}

func (s *Server) handleChequeBankAccountCreate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	var ledgerAccountID *uint
	if raw := strings.TrimSpace(c.FormValue("ledger_account_id")); raw != "" {
		id64, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id64 == 0 {
			return redirectErr(c, "/cheques", "Ledger bank account is invalid.")
		}
		id := uint(id64)
		ledgerAccountID = &id
	}

	account := models.ChequeBankAccount{
		CompanyID:           companyID,
		Label:               strings.TrimSpace(c.FormValue("label")),
		BankName:            strings.TrimSpace(c.FormValue("bank_name")),
		LedgerAccountID:     ledgerAccountID,
		NextChequeNumber:    strings.TrimSpace(c.FormValue("next_cheque_number")),
		DefaultCurrencyCode: strings.ToUpper(strings.TrimSpace(c.FormValue("currency"))),
		MICRCountry:         "CA",
		IsActive:            true,
	}
	if err := services.CreateChequeBankAccount(s.DB, &account); err != nil {
		return redirectErr(c, "/cheques", friendlyPayrollError(err, "Could not create cheque bank account."))
	}

	return redirectTo(c, "/cheques?account_created=1")
}

func (s *Server) handleChequeCreate(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}

	bankAccountID64, err := strconv.ParseUint(strings.TrimSpace(c.FormValue("bank_account_id")), 10, 64)
	if err != nil || bankAccountID64 == 0 {
		return redirectErr(c, "/cheques", "Bank account is required.")
	}
	chequeDate := time.Now().UTC()
	if raw := strings.TrimSpace(c.FormValue("cheque_date")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return redirectErr(c, "/cheques", "Cheque date must use YYYY-MM-DD.")
		}
		chequeDate = parsed
	}

	cheque := models.Cheque{
		CompanyID:     companyID,
		BankAccountID: uint(bankAccountID64),
		ChequeNumber:  strings.TrimSpace(c.FormValue("cheque_number")),
		PayeeType:     "other",
		PayeeName:     strings.TrimSpace(c.FormValue("payee_name")),
		ChequeDate:    chequeDate,
		CurrencyCode:  strings.ToUpper(strings.TrimSpace(c.FormValue("currency"))),
		Amount:        decimalFormValue(c, "amount", decimal.Zero),
		Memo:          strings.TrimSpace(c.FormValue("memo")),
		Status:        models.ChequeStatusDraft,
	}
	if err := services.CreateCheque(s.DB, &cheque); err != nil {
		return redirectErr(c, "/cheques", friendlyPayrollError(err, "Could not create cheque draft."))
	}
	_ = producers.ProjectCheque(c.Context(), s.DB, s.SearchProjector, companyID, cheque.ID)

	return redirectTo(c, "/cheques?created=1")
}

func (s *Server) handleChequeMarkPrinted(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	chequeID, err := parseChequeID(c)
	if err != nil {
		return redirectErr(c, "/cheques", "Invalid cheque.")
	}
	if err := services.MarkChequePrinted(s.DB, companyID, chequeID); err != nil {
		return redirectErr(c, "/cheques", friendlyPayrollError(err, "Could not mark cheque printed."))
	}
	_ = producers.ProjectCheque(c.Context(), s.DB, s.SearchProjector, companyID, chequeID)

	return redirectTo(c, "/cheques?printed=1")
}

func (s *Server) handleChequeVoid(c *fiber.Ctx) error {
	companyID, ok := ActiveCompanyIDFromCtx(c)
	if !ok {
		return c.Redirect("/select-company", fiber.StatusSeeOther)
	}
	chequeID, err := parseChequeID(c)
	if err != nil {
		return redirectErr(c, "/cheques", "Invalid cheque.")
	}
	if err := services.VoidCheque(s.DB, companyID, chequeID); err != nil {
		return redirectErr(c, "/cheques", friendlyPayrollError(err, "Could not void cheque."))
	}
	_ = producers.ProjectCheque(c.Context(), s.DB, s.SearchProjector, companyID, chequeID)

	return redirectTo(c, "/cheques?voided=1")
}

func parseChequeID(c *fiber.Ctx) (uint, error) {
	id64, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id64 == 0 {
		return 0, err
	}
	return uint(id64), nil
}

func isChequeStatusAllowed(status models.ChequeStatus) bool {
	switch status {
	case models.ChequeStatusDraft, models.ChequeStatusPrinted, models.ChequeStatusVoided:
		return true
	default:
		return false
	}
}

func (s *Server) chequeLedgerBankAccounts(companyID uint) ([]models.Account, error) {
	var accounts []models.Account
	err := s.DB.Where("company_id = ? AND is_active = ? AND root_account_type = ? AND detail_account_type = ?",
		companyID, true, models.RootAsset, models.DetailBank).
		Order("code asc, id asc").
		Find(&accounts).Error
	return accounts, err
}
