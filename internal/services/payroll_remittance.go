package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"balanciz/internal/models"
)

var (
	ErrPayrollRemittanceRequired   = errors.New("payroll remittance is required")
	ErrPayrollRemittanceNoAmounts  = errors.New("payroll remittance has no statutory amounts")
	ErrPayrollRemittanceCannotPay  = errors.New("payroll remittance cannot be paid")
	ErrPayrollRemittanceCannotVoid = errors.New("payroll remittance cannot be voided")
)

type PayrollRemittanceListFilter struct {
	CompanyID uint
	Status    *models.PayrollRemittanceStatus
	Limit     int
}

type PayrollRemittanceVoidResult struct {
	OriginalJournalEntryID uint
	ReversalJournalEntryID uint
}

func PayrollRemittanceForRun(db *gorm.DB, companyID, runID uint) (models.PayrollRemittance, bool, error) {
	if db == nil {
		return models.PayrollRemittance{}, false, fmt.Errorf("PayrollRemittanceForRun: db is required")
	}
	if companyID == 0 || runID == 0 {
		return models.PayrollRemittance{}, false, ErrPayrollRemittanceRequired
	}
	var remittance models.PayrollRemittance
	err := db.Preload("PayrollRun").
		Preload("BankLedgerAccount").
		Preload("JournalEntry").
		Where("company_id = ? AND payroll_run_id = ?", companyID, runID).
		First(&remittance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollRemittance{}, false, nil
		}
		return models.PayrollRemittance{}, false, err
	}
	return remittance, true, nil
}

func ListPayrollRemittances(db *gorm.DB, filter PayrollRemittanceListFilter) ([]models.PayrollRemittance, error) {
	if db == nil {
		return nil, fmt.Errorf("ListPayrollRemittances: db is required")
	}
	if filter.CompanyID == 0 {
		return nil, fmt.Errorf("ListPayrollRemittances: CompanyID is required")
	}
	q := db.Preload("PayrollRun").
		Preload("BankLedgerAccount").
		Preload("JournalEntry").
		Where("company_id = ?", filter.CompanyID)
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var remittances []models.PayrollRemittance
	if err := q.Order("due_date desc, id desc").Limit(limit).Find(&remittances).Error; err != nil {
		return nil, err
	}
	return remittances, nil
}

func CreatePayrollRemittanceForRun(db *gorm.DB, companyID, runID uint, dueDate time.Time) (models.PayrollRemittance, error) {
	if db == nil {
		return models.PayrollRemittance{}, fmt.Errorf("CreatePayrollRemittanceForRun: db is required")
	}
	if companyID == 0 || runID == 0 {
		return models.PayrollRemittance{}, ErrPayrollRemittanceRequired
	}

	var out models.PayrollRemittance
	err := db.Transaction(func(tx *gorm.DB) error {
		if existing, found, err := PayrollRemittanceForRun(tx, companyID, runID); err != nil {
			return err
		} else if found {
			out = existing
			return nil
		}

		var run models.PayrollRun
		if err := tx.Where("id = ? AND company_id = ?", runID, companyID).First(&run).Error; err != nil {
			return err
		}
		if run.Status != models.PayrollRunFinalized {
			return ErrPayrollRunNotFinalized
		}
		if _, found, err := PayrollRunJournalEntry(tx, companyID, runID); err != nil {
			return err
		} else if !found {
			return ErrChequePayrollRunNotPosted
		}
		if dueDate.IsZero() {
			dueDate = run.PayDate
		}

		cppAmount := run.TotalEmployeeCPP.Add(run.TotalEmployeeCPP2).Add(run.TotalEmployerCPP).Add(run.TotalEmployerCPP2).Round(2)
		eiAmount := run.TotalEmployeeEI.Add(run.TotalEmployerEI).Round(2)
		taxAmount := run.TotalEmployeeTax.Round(2)
		totalAmount := cppAmount.Add(eiAmount).Add(taxAmount).Round(2)
		if !totalAmount.GreaterThan(decimal.Zero) {
			return ErrPayrollRemittanceNoAmounts
		}

		remittance := models.PayrollRemittance{
			CompanyID:        companyID,
			PayrollRunID:     run.ID,
			RemittanceNumber: payrollRemittanceNumber(run),
			Status:           models.PayrollRemittanceDraft,
			PeriodStart:      run.PeriodStart,
			PeriodEnd:        run.PeriodEnd,
			DueDate:          dueDate,
			CPPAmount:        cppAmount,
			EIAmount:         eiAmount,
			TaxAmount:        taxAmount,
			TotalAmount:      totalAmount,
		}
		if err := tx.Create(&remittance).Error; err != nil {
			return err
		}
		out = remittance
		return nil
	})
	if err != nil {
		return models.PayrollRemittance{}, err
	}
	return out, nil
}

func PayPayrollRemittance(db *gorm.DB, companyID, remittanceID, bankLedgerAccountID uint, paymentDate time.Time) (models.JournalEntry, error) {
	if db == nil {
		return models.JournalEntry{}, fmt.Errorf("PayPayrollRemittance: db is required")
	}
	if companyID == 0 || remittanceID == 0 {
		return models.JournalEntry{}, ErrPayrollRemittanceRequired
	}
	if bankLedgerAccountID == 0 {
		return models.JournalEntry{}, ErrChequeBankLedgerRequired
	}

	var posted models.JournalEntry
	err := db.Transaction(func(tx *gorm.DB) error {
		var remittance models.PayrollRemittance
		if err := tx.Preload("PayrollRun").
			Where("id = ? AND company_id = ?", remittanceID, companyID).
			First(&remittance).Error; err != nil {
			return err
		}
		if remittance.Status == models.PayrollRemittancePaid && remittance.JournalEntryID != nil {
			var existing models.JournalEntry
			if err := tx.Where("id = ? AND company_id = ?", *remittance.JournalEntryID, companyID).First(&existing).Error; err != nil {
				return err
			}
			posted = existing
			return nil
		}
		if remittance.Status != models.PayrollRemittanceDraft {
			return ErrPayrollRemittanceCannotPay
		}
		if !remittance.TotalAmount.GreaterThan(decimal.Zero) {
			return ErrPayrollRemittanceNoAmounts
		}
		if err := requireChequeLedgerAccount(tx, companyID, bankLedgerAccountID); err != nil {
			return err
		}
		payrollJE, found, err := PayrollRunJournalEntry(tx, companyID, remittance.PayrollRunID)
		if err != nil {
			return err
		}
		if !found {
			return ErrChequePayrollRunNotPosted
		}
		if paymentDate.IsZero() {
			paymentDate = time.Now().UTC()
		}

		cppAccountID, err := payrollPostingLineAccountID(tx, companyID, payrollJE.ID, "CPP payable", "2210")
		if err != nil {
			return err
		}
		eiAccountID, err := payrollPostingLineAccountID(tx, companyID, payrollJE.ID, "EI payable", "2220")
		if err != nil {
			return err
		}
		taxAccountID, err := payrollPostingLineAccountID(tx, companyID, payrollJE.ID, "Income tax withheld payable", "2230")
		if err != nil {
			return err
		}

		company, err := loadPostingCompany(tx, companyID)
		if err != nil {
			return err
		}
		currency := strings.ToUpper(strings.TrimSpace(company.BaseCurrencyCode))
		if currency == "" {
			currency = "CAD"
		}

		je := models.JournalEntry{
			CompanyID:               companyID,
			EntryDate:               paymentDate,
			JournalNo:               "REMIT-" + remittance.RemittanceNumber,
			Status:                  models.JournalEntryStatusPosted,
			TransactionCurrencyCode: currency,
			ExchangeRate:            decimal.NewFromInt(1),
			ExchangeRateDate:        paymentDate,
			ExchangeRateSource:      "identity",
			SourceType:              models.LedgerSourcePayrollRemittance,
			SourceID:                remittance.ID,
		}
		if err := tx.Create(&je).Error; err != nil {
			return err
		}

		lines := make([]models.JournalLine, 0, 4)
		lines = appendPayrollDebitLine(lines, companyID, je.ID, cppAccountID, remittance.CPPAmount, "CPP remittance")
		lines = appendPayrollDebitLine(lines, companyID, je.ID, eiAccountID, remittance.EIAmount, "EI remittance")
		lines = appendPayrollDebitLine(lines, companyID, je.ID, taxAccountID, remittance.TaxAmount, "Income tax remittance")
		lines = append(lines, payrollJournalLine(companyID, je.ID, bankLedgerAccountID, decimal.Zero, remittance.TotalAmount.Round(2), "Payroll remittance payment"))
		if err := tx.Create(&lines).Error; err != nil {
			return err
		}
		if err := WriteSecondaryBookAmounts(tx, companyID, lines, currency, paymentDate, models.FXPostingReasonTransaction); err != nil {
			return err
		}
		if err := ProjectToLedger(tx, companyID, LedgerPostInput{
			JournalEntry: je,
			Lines:        lines,
			SourceType:   models.LedgerSourcePayrollRemittance,
			SourceID:     remittance.ID,
		}); err != nil {
			return err
		}

		if err := tx.Model(&models.PayrollRemittance{}).
			Where("id = ? AND company_id = ?", remittance.ID, companyID).
			Updates(map[string]any{
				"status":                 models.PayrollRemittancePaid,
				"payment_date":           paymentDate,
				"bank_ledger_account_id": bankLedgerAccountID,
				"journal_entry_id":       je.ID,
			}).Error; err != nil {
			return err
		}
		posted = je
		return nil
	})
	if err != nil {
		return models.JournalEntry{}, err
	}
	return posted, nil
}

func VoidPayrollRemittance(db *gorm.DB, companyID, remittanceID uint, voidDate time.Time) (PayrollRemittanceVoidResult, error) {
	if db == nil {
		return PayrollRemittanceVoidResult{}, fmt.Errorf("VoidPayrollRemittance: db is required")
	}
	if companyID == 0 || remittanceID == 0 {
		return PayrollRemittanceVoidResult{}, ErrPayrollRemittanceRequired
	}

	var result PayrollRemittanceVoidResult
	err := db.Transaction(func(tx *gorm.DB) error {
		var remittance models.PayrollRemittance
		if err := tx.Where("id = ? AND company_id = ?", remittanceID, companyID).First(&remittance).Error; err != nil {
			return err
		}
		if remittance.Status == models.PayrollRemittanceVoided {
			return ErrPayrollRemittanceCannotVoid
		}
		if voidDate.IsZero() {
			voidDate = time.Now().UTC()
		}

		updates := map[string]any{
			"status":    models.PayrollRemittanceVoided,
			"voided_at": voidDate,
		}
		if remittance.Status == models.PayrollRemittancePaid {
			if remittance.JournalEntryID == nil || *remittance.JournalEntryID == 0 {
				return ErrPayrollRemittanceCannotVoid
			}
			result.OriginalJournalEntryID = *remittance.JournalEntryID
			reversalID, err := ReverseJournalEntry(tx, companyID, *remittance.JournalEntryID, voidDate)
			if errors.Is(err, ErrJournalEntryAlreadyReversed) {
				reversalID, err = existingReversalJournalEntryID(tx, companyID, *remittance.JournalEntryID)
			}
			if err != nil {
				return err
			}
			result.ReversalJournalEntryID = reversalID
			updates["reversal_journal_entry_id"] = reversalID
		} else if remittance.Status != models.PayrollRemittanceDraft {
			return ErrPayrollRemittanceCannotVoid
		}

		return tx.Model(&models.PayrollRemittance{}).
			Where("id = ? AND company_id = ?", remittance.ID, companyID).
			Updates(updates).Error
	})
	if err != nil {
		return PayrollRemittanceVoidResult{}, err
	}
	return result, nil
}

func payrollPostingLineAccountID(db *gorm.DB, companyID, journalEntryID uint, memo string, fallbackCode string) (uint, error) {
	var line models.JournalLine
	if err := db.Where("company_id = ? AND journal_entry_id = ? AND memo = ?", companyID, journalEntryID, memo).
		Order("id asc").
		First(&line).Error; err == nil {
		return line.AccountID, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	account, err := findPayrollPostingAccount(db, companyID, models.RootLiability, models.DetailPayrollLiability, fallbackCode)
	if err != nil {
		return 0, err
	}
	return account.ID, nil
}

func existingReversalJournalEntryID(db *gorm.DB, companyID, originalJournalEntryID uint) (uint, error) {
	var reversal models.JournalEntry
	if err := db.Select("id").
		Where("company_id = ? AND reversed_from_id = ?", companyID, originalJournalEntryID).
		First(&reversal).Error; err != nil {
		return 0, err
	}
	return reversal.ID, nil
}

func appendPayrollDebitLine(lines []models.JournalLine, companyID, journalEntryID, accountID uint, amount decimal.Decimal, memo string) []models.JournalLine {
	amount = amount.Round(2)
	if amount.IsZero() {
		return lines
	}
	return append(lines, payrollJournalLine(companyID, journalEntryID, accountID, amount, decimal.Zero, memo))
}

func payrollRemittanceNumber(run models.PayrollRun) string {
	number := strings.TrimSpace(run.RunNumber)
	if number != "" {
		return number
	}
	return fmt.Sprintf("RUN-%d", run.ID)
}
