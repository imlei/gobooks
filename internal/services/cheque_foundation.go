package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

var (
	ErrChequeBankAccountRequired = errors.New("cheque bank account is required")
	ErrChequePayeeRequired       = errors.New("cheque payee is required")
	ErrChequeAmountRequired      = errors.New("cheque amount must be greater than zero")
	ErrChequePayrollRunRequired  = errors.New("finalized payroll run is required")
	ErrChequeCannotPrint         = errors.New("cheque cannot be marked printed")
	ErrChequeCannotVoid          = errors.New("cheque cannot be voided")
	ErrChequeBankLedgerRequired  = errors.New("cheque bank account must be linked to a ledger bank account")
	ErrChequePayrollRunNotPosted = errors.New("payroll run must be posted before payroll cheques can be printed")
)

func CreateChequeBankAccount(db *gorm.DB, account *models.ChequeBankAccount) error {
	if db == nil {
		return fmt.Errorf("CreateChequeBankAccount: db is required")
	}
	if account == nil {
		return fmt.Errorf("CreateChequeBankAccount: account is required")
	}
	if account.CompanyID == 0 {
		return fmt.Errorf("CreateChequeBankAccount: CompanyID is required")
	}
	account.Label = strings.TrimSpace(account.Label)
	account.BankName = strings.TrimSpace(account.BankName)
	account.NextChequeNumber = strings.TrimSpace(account.NextChequeNumber)
	account.DefaultCurrencyCode = strings.ToUpper(strings.TrimSpace(account.DefaultCurrencyCode))
	if account.Label == "" {
		return ErrChequeBankAccountRequired
	}
	if account.DefaultCurrencyCode == "" {
		account.DefaultCurrencyCode = "CAD"
	}
	if account.LedgerAccountID != nil {
		if err := requireChequeLedgerAccount(db, account.CompanyID, *account.LedgerAccountID); err != nil {
			return err
		}
	}
	account.MICRCountry = strings.ToUpper(strings.TrimSpace(account.MICRCountry))
	if account.MICRCountry == "" {
		account.MICRCountry = "CA"
	}
	account.IsActive = true
	return db.Create(account).Error
}

func ListChequeBankAccounts(db *gorm.DB, companyID uint, includeInactive bool) ([]models.ChequeBankAccount, error) {
	if db == nil {
		return nil, fmt.Errorf("ListChequeBankAccounts: db is required")
	}
	if companyID == 0 {
		return nil, fmt.Errorf("ListChequeBankAccounts: CompanyID is required")
	}
	q := db.Preload("LedgerAccount").Where("company_id = ?", companyID)
	if !includeInactive {
		q = q.Where("is_active = ?", true)
	}
	var accounts []models.ChequeBankAccount
	if err := q.Order("is_active desc, label asc, id asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

type ChequeListFilter struct {
	CompanyID uint
	Status    *models.ChequeStatus
	Query     string
	Limit     int
}

func ListCheques(db *gorm.DB, filter ChequeListFilter) ([]models.Cheque, error) {
	if db == nil {
		return nil, fmt.Errorf("ListCheques: db is required")
	}
	if filter.CompanyID == 0 {
		return nil, fmt.Errorf("ListCheques: CompanyID is required")
	}
	q := db.Preload("BankAccount").Where("company_id = ?", filter.CompanyID)
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if s := strings.TrimSpace(filter.Query); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("LOWER(cheque_number) LIKE ? OR LOWER(payee_name) LIKE ? OR LOWER(memo) LIKE ?", like, like, like)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var cheques []models.Cheque
	if err := q.Order("cheque_date desc, id desc").Limit(limit).Find(&cheques).Error; err != nil {
		return nil, err
	}
	return cheques, nil
}

func CreateCheque(db *gorm.DB, cheque *models.Cheque) error {
	if db == nil {
		return fmt.Errorf("CreateCheque: db is required")
	}
	if cheque == nil {
		return fmt.Errorf("CreateCheque: cheque is required")
	}
	if cheque.CompanyID == 0 || cheque.BankAccountID == 0 {
		return ErrChequeBankAccountRequired
	}
	cheque.ChequeNumber = strings.TrimSpace(cheque.ChequeNumber)
	cheque.PayeeType = strings.TrimSpace(cheque.PayeeType)
	cheque.PayeeName = strings.TrimSpace(cheque.PayeeName)
	cheque.CurrencyCode = strings.ToUpper(strings.TrimSpace(cheque.CurrencyCode))
	cheque.Memo = strings.TrimSpace(cheque.Memo)
	if cheque.PayeeName == "" {
		return ErrChequePayeeRequired
	}
	if !cheque.Amount.GreaterThan(decimal.Zero) {
		return ErrChequeAmountRequired
	}
	if cheque.ChequeDate.IsZero() {
		cheque.ChequeDate = time.Now().UTC()
	}
	if cheque.CurrencyCode == "" {
		cheque.CurrencyCode = "CAD"
	}
	if cheque.PayeeType == "" {
		cheque.PayeeType = "other"
	}
	if cheque.Status == "" {
		cheque.Status = models.ChequeStatusDraft
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var account models.ChequeBankAccount
		if err := tx.Where("id = ? AND company_id = ? AND is_active = ?", cheque.BankAccountID, cheque.CompanyID, true).
			First(&account).Error; err != nil {
			return err
		}
		if cheque.ChequeNumber == "" {
			cheque.ChequeNumber = strings.TrimSpace(account.NextChequeNumber)
		}
		if cheque.CurrencyCode == "" {
			cheque.CurrencyCode = account.DefaultCurrencyCode
		}
		return tx.Create(cheque).Error
	})
}

func GeneratePayrollChequeDrafts(db *gorm.DB, companyID, runID, bankAccountID uint) ([]models.Cheque, error) {
	if db == nil {
		return nil, fmt.Errorf("GeneratePayrollChequeDrafts: db is required")
	}
	if companyID == 0 || runID == 0 || bankAccountID == 0 {
		return nil, ErrChequePayrollRunRequired
	}

	var created []models.Cheque
	err := db.Transaction(func(tx *gorm.DB) error {
		var run models.PayrollRun
		if err := tx.Where("id = ? AND company_id = ?", runID, companyID).First(&run).Error; err != nil {
			return err
		}
		if run.Status != models.PayrollRunFinalized {
			return ErrChequePayrollRunRequired
		}

		var account models.ChequeBankAccount
		if err := tx.Where("id = ? AND company_id = ? AND is_active = ?", bankAccountID, companyID, true).
			First(&account).Error; err != nil {
			return err
		}

		var entries []models.PayrollEntry
		if err := tx.Preload("Employee").
			Where("company_id = ? AND payroll_run_id = ?", companyID, runID).
			Where("net_pay > 0").
			Order("employee_id asc, id asc").
			Find(&entries).Error; err != nil {
			return err
		}
		if len(entries) == 0 {
			return ErrPayrollRunNoEntries
		}

		nextNumber, width, hasNumericNumber := parseChequeNumberSeed(account.NextChequeNumber)
		currency := strings.ToUpper(strings.TrimSpace(account.DefaultCurrencyCode))
		if currency == "" {
			currency = "CAD"
		}
		for _, entry := range entries {
			if entry.Employee.ID == 0 {
				continue
			}

			var existing int64
			if err := tx.Model(&models.Cheque{}).
				Where("company_id = ? AND payroll_entry_id = ? AND status <> ?", companyID, entry.ID, models.ChequeStatusVoided).
				Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				continue
			}

			entryID := entry.ID
			employeeID := entry.EmployeeID
			payrollRunID := run.ID
			chequeNumber := ""
			if hasNumericNumber {
				chequeNumber = formatChequeNumber(nextNumber, width)
				nextNumber++
			}
			cheque := models.Cheque{
				CompanyID:      companyID,
				BankAccountID:  account.ID,
				ChequeNumber:   chequeNumber,
				PayeeType:      "employee",
				PayeeName:      entry.Employee.SearchName(),
				EmployeeID:     &employeeID,
				PayrollRunID:   &payrollRunID,
				PayrollEntryID: &entryID,
				ChequeDate:     run.PayDate,
				CurrencyCode:   currency,
				Amount:         entry.NetPay.Round(2),
				Memo:           "Payroll " + payrollRunLabel(run),
				Status:         models.ChequeStatusDraft,
			}
			if cheque.PayeeName == "" {
				cheque.PayeeName = "Employee " + strconv.FormatUint(uint64(entry.EmployeeID), 10)
			}
			if err := tx.Create(&cheque).Error; err != nil {
				return err
			}
			created = append(created, cheque)
		}

		if hasNumericNumber && len(created) > 0 {
			if err := tx.Model(&models.ChequeBankAccount{}).
				Where("id = ? AND company_id = ?", account.ID, companyID).
				Update("next_cheque_number", formatChequeNumber(nextNumber, width)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func MarkChequePrinted(db *gorm.DB, companyID, chequeID uint) error {
	if db == nil {
		return fmt.Errorf("MarkChequePrinted: db is required")
	}
	if companyID == 0 || chequeID == 0 {
		return ErrChequeBankAccountRequired
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var cheque models.Cheque
		if err := tx.Preload("BankAccount").Where("id = ? AND company_id = ?", chequeID, companyID).First(&cheque).Error; err != nil {
			return err
		}
		if cheque.Status != models.ChequeStatusDraft {
			return ErrChequeCannotPrint
		}
		if isPayrollCheque(cheque) {
			if _, err := postPayrollChequePayment(tx, companyID, cheque); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		return tx.Model(&models.Cheque{}).
			Where("id = ? AND company_id = ?", chequeID, companyID).
			Updates(map[string]any{
				"status":     models.ChequeStatusPrinted,
				"printed_at": &now,
			}).Error
	})
}

func VoidCheque(db *gorm.DB, companyID, chequeID uint) error {
	if db == nil {
		return fmt.Errorf("VoidCheque: db is required")
	}
	if companyID == 0 || chequeID == 0 {
		return ErrChequeBankAccountRequired
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var cheque models.Cheque
		if err := tx.Where("id = ? AND company_id = ?", chequeID, companyID).First(&cheque).Error; err != nil {
			return err
		}
		if cheque.Status == models.ChequeStatusVoided {
			return ErrChequeCannotVoid
		}
		if cheque.Status == models.ChequeStatusPrinted {
			if err := reverseChequePaymentIfPosted(tx, companyID, cheque); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		return tx.Model(&models.Cheque{}).
			Where("id = ? AND company_id = ?", chequeID, companyID).
			Updates(map[string]any{
				"status":    models.ChequeStatusVoided,
				"voided_at": &now,
			}).Error
	})
}

func postPayrollChequePayment(tx *gorm.DB, companyID uint, cheque models.Cheque) (models.JournalEntry, error) {
	if cheque.BankAccount.LedgerAccountID == nil || *cheque.BankAccount.LedgerAccountID == 0 {
		return models.JournalEntry{}, ErrChequeBankLedgerRequired
	}
	if err := requireChequeLedgerAccount(tx, companyID, *cheque.BankAccount.LedgerAccountID); err != nil {
		return models.JournalEntry{}, err
	}
	if cheque.PayrollRunID == nil || *cheque.PayrollRunID == 0 {
		return models.JournalEntry{}, ErrChequePayrollRunRequired
	}
	payrollJE, found, err := PayrollRunJournalEntry(tx, companyID, *cheque.PayrollRunID)
	if err != nil {
		return models.JournalEntry{}, err
	}
	if !found {
		return models.JournalEntry{}, ErrChequePayrollRunNotPosted
	}
	if existing, found, err := chequePaymentJournalEntry(tx, companyID, cheque.ID); err != nil {
		return models.JournalEntry{}, err
	} else if found {
		return existing, nil
	}

	liabilityAccountID, err := payrollNetPayLiabilityAccountID(tx, companyID, payrollJE.ID)
	if err != nil {
		return models.JournalEntry{}, err
	}
	company, err := loadPostingCompany(tx, companyID)
	if err != nil {
		return models.JournalEntry{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(cheque.CurrencyCode))
	if currency == "" {
		currency = strings.ToUpper(strings.TrimSpace(company.BaseCurrencyCode))
	}
	if currency == "" {
		currency = "CAD"
	}
	amount := cheque.Amount.Round(2)
	if !amount.GreaterThan(decimal.Zero) {
		return models.JournalEntry{}, ErrChequeAmountRequired
	}

	je := models.JournalEntry{
		CompanyID:               companyID,
		EntryDate:               cheque.ChequeDate,
		JournalNo:               chequeJournalNo(cheque),
		Status:                  models.JournalEntryStatusPosted,
		TransactionCurrencyCode: currency,
		ExchangeRate:            decimal.NewFromInt(1),
		ExchangeRateDate:        cheque.ChequeDate,
		ExchangeRateSource:      "identity",
		SourceType:              models.LedgerSourceCheque,
		SourceID:                cheque.ID,
	}
	if err := tx.Create(&je).Error; err != nil {
		return models.JournalEntry{}, err
	}
	lines := []models.JournalLine{
		payrollJournalLine(companyID, je.ID, liabilityAccountID, amount, decimal.Zero, "Payroll cheque payment"),
		payrollJournalLine(companyID, je.ID, *cheque.BankAccount.LedgerAccountID, decimal.Zero, amount, "Cheque "+chequeDisplayNumberForJournal(cheque)),
	}
	if err := tx.Create(&lines).Error; err != nil {
		return models.JournalEntry{}, err
	}
	if err := WriteSecondaryBookAmounts(tx, companyID, lines, currency, cheque.ChequeDate, models.FXPostingReasonTransaction); err != nil {
		return models.JournalEntry{}, err
	}
	if err := ProjectToLedger(tx, companyID, LedgerPostInput{
		JournalEntry: je,
		Lines:        lines,
		SourceType:   models.LedgerSourceCheque,
		SourceID:     cheque.ID,
	}); err != nil {
		return models.JournalEntry{}, err
	}
	return je, nil
}

func reverseChequePaymentIfPosted(tx *gorm.DB, companyID uint, cheque models.Cheque) error {
	je, found, err := chequePaymentJournalEntry(tx, companyID, cheque.ID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if _, err := ReverseJournalEntry(tx, companyID, je.ID, time.Now().UTC()); err != nil {
		if errors.Is(err, ErrJournalEntryAlreadyReversed) {
			return nil
		}
		return err
	}
	return nil
}

func chequePaymentJournalEntry(db *gorm.DB, companyID, chequeID uint) (models.JournalEntry, bool, error) {
	var je models.JournalEntry
	err := db.Where("company_id = ? AND source_type = ? AND source_id = ? AND status = ?",
		companyID, models.LedgerSourceCheque, chequeID, models.JournalEntryStatusPosted).
		First(&je).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.JournalEntry{}, false, nil
		}
		return models.JournalEntry{}, false, err
	}
	return je, true, nil
}

func payrollNetPayLiabilityAccountID(db *gorm.DB, companyID, payrollJournalEntryID uint) (uint, error) {
	var line models.JournalLine
	if err := db.Where("company_id = ? AND journal_entry_id = ? AND memo = ?", companyID, payrollJournalEntryID, "Net pay payable").
		Order("id asc").
		First(&line).Error; err == nil {
		return line.AccountID, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	account, err := findPayrollPostingAccount(db, companyID, models.RootLiability, models.DetailPayrollLiability, "2200")
	if err != nil {
		return 0, err
	}
	return account.ID, nil
}

func requireChequeLedgerAccount(db *gorm.DB, companyID, accountID uint) error {
	var account models.Account
	if err := db.Where("id = ? AND company_id = ? AND is_active = ? AND root_account_type = ? AND detail_account_type = ?",
		accountID, companyID, true, models.RootAsset, models.DetailBank).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChequeBankLedgerRequired
		}
		return err
	}
	return nil
}

func isPayrollCheque(cheque models.Cheque) bool {
	return cheque.PayrollRunID != nil || cheque.PayrollEntryID != nil
}

func chequeJournalNo(cheque models.Cheque) string {
	return "CHEQUE-" + chequeDisplayNumberForJournal(cheque)
}

func chequeDisplayNumberForJournal(cheque models.Cheque) string {
	if strings.TrimSpace(cheque.ChequeNumber) != "" {
		return strings.TrimSpace(cheque.ChequeNumber)
	}
	return strconv.FormatUint(uint64(cheque.ID), 10)
}

func parseChequeNumberSeed(seed string) (int64, int, bool) {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return 0, 0, false
	}
	n, err := strconv.ParseInt(seed, 10, 64)
	if err != nil || n < 0 {
		return 0, 0, false
	}
	return n, len(seed), true
}

func formatChequeNumber(n int64, width int) string {
	if width <= 0 {
		return strconv.FormatInt(n, 10)
	}
	return fmt.Sprintf("%0*d", width, n)
}

func payrollRunLabel(run models.PayrollRun) string {
	if run.RunNumber != "" {
		return run.RunNumber
	}
	return "run " + strconv.FormatUint(uint64(run.ID), 10)
}
