package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"gobooks/internal/models"
)

const journalEntryFXMigrationVersion = "048_journal_entry_fx_support.sql"

type JournalEntryReadFXState struct {
	TransactionCurrencyCode   string
	BaseCurrencyCode          string
	ExchangeRate              decimal.Decimal
	ExchangeRateDate          time.Time
	ExchangeRateSource        string
	ExchangeRateSourceLabel   string
	IsForeignCurrency         bool
	TransactionAmountsPresent bool
	SnapshotNote              string
}

// BuildJournalEntryReadFXState returns the immutable FX read model for a posted
// JE. New JEs read their persisted snapshot directly. Legacy JEs created before
// migration 048 are handled honestly: base-only history stays identity, linked
// foreign-source documents are reconstructed when possible, and otherwise the
// read path marks FX details unavailable instead of fabricating identity state.
func BuildJournalEntryReadFXState(db *gorm.DB, baseCurrencyCode string, je models.JournalEntry) (JournalEntryReadFXState, error) {
	baseCurrencyCode = normalizeCurrencyCode(baseCurrencyCode)
	if baseCurrencyCode == "" {
		baseCurrencyCode = normalizeCurrencyCode(je.TransactionCurrencyCode)
	}

	state := JournalEntryReadFXState{
		TransactionCurrencyCode:   normalizeCurrencyCode(strings.TrimSpace(je.TransactionCurrencyCode)),
		BaseCurrencyCode:          baseCurrencyCode,
		ExchangeRate:              je.ExchangeRate.RoundBank(8),
		ExchangeRateDate:          je.ExchangeRateDate,
		ExchangeRateSource:        strings.TrimSpace(je.ExchangeRateSource),
		TransactionAmountsPresent: true,
	}
	if state.TransactionCurrencyCode == "" {
		state.TransactionCurrencyCode = baseCurrencyCode
	}
	if state.ExchangeRate.IsZero() {
		state.ExchangeRate = decimal.NewFromInt(1)
	}
	if state.ExchangeRateDate.IsZero() {
		state.ExchangeRateDate = je.EntryDate
	}
	state.ExchangeRateDate = normalizeDate(state.ExchangeRateDate)
	if state.ExchangeRateSource == "" {
		state.ExchangeRateSource = JournalEntryExchangeRateSourceIdentity
	}
	state.ExchangeRateSourceLabel = ExchangeRateSourceLabel(state.ExchangeRateSource)
	state.IsForeignCurrency = state.TransactionCurrencyCode != "" && state.TransactionCurrencyCode != baseCurrencyCode

	legacy, err := journalEntryPredatesFXMigration(db, je)
	if err != nil {
		return JournalEntryReadFXState{}, err
	}
	if !legacy {
		return state, nil
	}

	if strings.TrimSpace(string(je.SourceType)) == "" {
		// Manual legacy JEs predate foreign-currency JE support and are base-only.
		return state, nil
	}

	legacyState, err := buildLegacyJournalEntryReadFXState(db, je, baseCurrencyCode)
	if err != nil {
		return JournalEntryReadFXState{}, err
	}
	if legacyState != nil {
		return *legacyState, nil
	}
	return state, nil
}

func journalEntryPredatesFXMigration(db *gorm.DB, je models.JournalEntry) (bool, error) {
	if je.CreatedAt.IsZero() {
		return false, nil
	}
	var appliedAtRaw string
	err := db.Raw(
		`SELECT applied_at FROM schema_migrations WHERE version = ? LIMIT 1`,
		journalEntryFXMigrationVersion,
	).Scan(&appliedAtRaw).Error
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "schema_migrations") {
			return false, nil
		}
		return false, err
	}
	appliedAtRaw = strings.TrimSpace(appliedAtRaw)
	if appliedAtRaw == "" {
		return false, nil
	}
	appliedAt, err := parseSchemaMigrationAppliedAt(appliedAtRaw)
	if err != nil {
		return false, err
	}
	return je.CreatedAt.Before(appliedAt), nil
}

func buildLegacyJournalEntryReadFXState(db *gorm.DB, je models.JournalEntry, baseCurrencyCode string) (*JournalEntryReadFXState, error) {
	switch je.SourceType {
	case models.LedgerSourceInvoice:
		var invoice models.Invoice
		err := db.Select("id", "company_id", "invoice_date", "currency_code", "exchange_rate", "amount", "amount_base").
			Where("id = ? AND company_id = ?", je.SourceID, je.CompanyID).
			First(&invoice).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				state := legacyUnavailableJournalEntryReadFXState(
					baseCurrencyCode,
					"This journal entry predates Gobooks FX snapshots and its linked invoice is no longer available. Historical FX details are unavailable.",
				)
				return &state, nil
			}
			return nil, err
		}
		return buildLegacyDocumentJournalEntryReadFXState(
			baseCurrencyCode,
			normalizeCurrencyCode(invoice.CurrencyCode),
			invoice.ExchangeRate,
			invoice.Amount,
			invoice.AmountBase,
			invoice.InvoiceDate,
			"invoice",
		), nil
	case models.LedgerSourceBill:
		var bill models.Bill
		err := db.Select("id", "company_id", "bill_date", "currency_code", "exchange_rate", "amount", "amount_base").
			Where("id = ? AND company_id = ?", je.SourceID, je.CompanyID).
			First(&bill).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				state := legacyUnavailableJournalEntryReadFXState(
					baseCurrencyCode,
					"This journal entry predates Gobooks FX snapshots and its linked bill is no longer available. Historical FX details are unavailable.",
				)
				return &state, nil
			}
			return nil, err
		}
		return buildLegacyDocumentJournalEntryReadFXState(
			baseCurrencyCode,
			normalizeCurrencyCode(bill.CurrencyCode),
			bill.ExchangeRate,
			bill.Amount,
			bill.AmountBase,
			bill.BillDate,
			"bill",
		), nil
	default:
		state := legacyUnavailableJournalEntryReadFXState(
			baseCurrencyCode,
			fmt.Sprintf("This %s journal entry predates Gobooks FX snapshots and no reliable historical FX source is available.", strings.TrimSpace(string(je.SourceType))),
		)
		return &state, nil
	}
}

func buildLegacyDocumentJournalEntryReadFXState(baseCurrencyCode, transactionCurrencyCode string, exchangeRate, amount, amountBase decimal.Decimal, date time.Time, documentLabel string) *JournalEntryReadFXState {
	if transactionCurrencyCode == "" || transactionCurrencyCode == baseCurrencyCode {
		return nil
	}

	resolvedRate := exchangeRate.RoundBank(8)
	if !resolvedRate.GreaterThan(decimal.Zero) && amount.GreaterThan(decimal.Zero) && amountBase.GreaterThan(decimal.Zero) {
		resolvedRate = amountBase.Div(amount).RoundBank(8)
	}

	note := fmt.Sprintf(
		"Legacy foreign-currency journal entry reconstructed from its linked %s. Original transaction-currency line amounts were not persisted before FX snapshots were introduced.",
		documentLabel,
	)
	if !resolvedRate.GreaterThan(decimal.Zero) {
		state := legacyUnavailableJournalEntryReadFXState(
			baseCurrencyCode,
			fmt.Sprintf(
				"This legacy journal entry was linked to a %s in %s, but Gobooks could not reconstruct a reliable historical FX rate. Transaction-currency line amounts were not persisted.",
				documentLabel,
				transactionCurrencyCode,
			),
		)
		state.TransactionCurrencyCode = transactionCurrencyCode
		state.ExchangeRateDate = normalizeDate(date)
		state.IsForeignCurrency = transactionCurrencyCode != baseCurrencyCode
		return &state
	}

	state := JournalEntryReadFXState{
		TransactionCurrencyCode:   transactionCurrencyCode,
		BaseCurrencyCode:          baseCurrencyCode,
		ExchangeRate:              resolvedRate,
		ExchangeRateDate:          normalizeDate(date),
		ExchangeRateSource:        JournalEntryExchangeRateSourceLegacyUnavailable,
		ExchangeRateSourceLabel:   ExchangeRateSourceLabel(JournalEntryExchangeRateSourceLegacyUnavailable),
		IsForeignCurrency:         true,
		TransactionAmountsPresent: false,
		SnapshotNote:              note,
	}
	return &state
}

func legacyUnavailableJournalEntryReadFXState(baseCurrencyCode, note string) JournalEntryReadFXState {
	return JournalEntryReadFXState{
		BaseCurrencyCode:          baseCurrencyCode,
		ExchangeRateSource:        JournalEntryExchangeRateSourceLegacyUnavailable,
		ExchangeRateSourceLabel:   ExchangeRateSourceLabel(JournalEntryExchangeRateSourceLegacyUnavailable),
		TransactionAmountsPresent: false,
		SnapshotNote:              note,
	}
}

func parseSchemaMigrationAppliedAt(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse schema_migrations applied_at %q", raw)
}
