package services

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type FXLineAmounts struct {
	TxDebit  decimal.Decimal
	TxCredit decimal.Decimal
}

type FXLineConversion struct {
	TxDebit  decimal.Decimal
	TxCredit decimal.Decimal
	Debit    decimal.Decimal
	Credit   decimal.Decimal
}

type FXConversionTotals struct {
	TxDebitTotal   decimal.Decimal
	TxCreditTotal  decimal.Decimal
	BaseDebitTotal decimal.Decimal
	BaseCreditTotal decimal.Decimal
}

type FXConversionResult struct {
	Lines  []FXLineConversion
	Totals FXConversionTotals
}

// RoundBankMoney is the phase-1 JE FX rounding rule: banker's rounding to 2 decimals.
func RoundBankMoney(amount decimal.Decimal) decimal.Decimal {
	return amount.RoundBank(2)
}

// ConvertJournalLineAmounts converts each line individually and blocks save if totals do not balance after rounding.
func ConvertJournalLineAmounts(lines []FXLineAmounts, exchangeRate decimal.Decimal) (FXConversionResult, error) {
	if !exchangeRate.GreaterThan(decimal.Zero) {
		return FXConversionResult{}, fmt.Errorf("exchange rate must be greater than 0")
	}

	result := FXConversionResult{
		Lines: make([]FXLineConversion, 0, len(lines)),
		Totals: FXConversionTotals{
			TxDebitTotal:    decimal.Zero,
			TxCreditTotal:   decimal.Zero,
			BaseDebitTotal:  decimal.Zero,
			BaseCreditTotal: decimal.Zero,
		},
	}

	for _, line := range lines {
		if line.TxDebit.GreaterThan(decimal.Zero) && line.TxCredit.GreaterThan(decimal.Zero) {
			return FXConversionResult{}, fmt.Errorf("a line cannot have both debit and credit")
		}

		converted := FXLineConversion{
			TxDebit:  line.TxDebit,
			TxCredit: line.TxCredit,
			Debit:    RoundBankMoney(line.TxDebit.Mul(exchangeRate)),
			Credit:   RoundBankMoney(line.TxCredit.Mul(exchangeRate)),
		}
		result.Lines = append(result.Lines, converted)
		result.Totals.TxDebitTotal = result.Totals.TxDebitTotal.Add(converted.TxDebit)
		result.Totals.TxCreditTotal = result.Totals.TxCreditTotal.Add(converted.TxCredit)
		result.Totals.BaseDebitTotal = result.Totals.BaseDebitTotal.Add(converted.Debit)
		result.Totals.BaseCreditTotal = result.Totals.BaseCreditTotal.Add(converted.Credit)
	}

	if !result.Totals.TxDebitTotal.Equal(result.Totals.TxCreditTotal) {
		return FXConversionResult{}, fmt.Errorf("total debits must equal total credits")
	}
	if !result.Totals.BaseDebitTotal.Equal(result.Totals.BaseCreditTotal) {
		return FXConversionResult{}, fmt.Errorf("converted base-currency totals do not balance exactly under the current exchange rate; phase 1 blocks save instead of inventing an FX rounding line")
	}

	return result, nil
}
