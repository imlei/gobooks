// 遵循project_guide.md
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// TranslationRunStatus is the lifecycle state of a period-end translation run.
type TranslationRunStatus string

const (
	// TranslationRunStatusPosted means the run has been recorded and lines are final.
	TranslationRunStatusPosted TranslationRunStatus = "posted"

	// TranslationRunStatusReversed means this run has been superseded by a reversal.
	// (Phase 4: reversal + re-run workflow.)
	TranslationRunStatusReversed TranslationRunStatus = "reversed"
)

// TranslationRateType identifies which rate was applied to a translated account.
type TranslationRateType string

const (
	// TranslationRateTypeClosing — period-end closing rate, used for balance sheet items
	// (assets and liabilities) per IAS 21.39(a).
	TranslationRateTypeClosing TranslationRateType = "closing"

	// TranslationRateTypeAverage — period average rate, used for income and expense items
	// as a practical approximation per IAS 21.40.
	TranslationRateTypeAverage TranslationRateType = "average"
)

// TranslationRun records a single period-end IAS 21 translation event.
//
// A translation run converts a secondary book's accounted amounts (stored in
// JournalLineBookAmount rows) from the book's functional currency into a
// presentation currency for a reporting period.
//
// IAS 21 methodology applied:
//   - Assets and liabilities → closing rate (balance sheet date rate).
//   - Revenue and expenses   → average rate (period approximation).
//   - CTA                    → the residual that makes the translated trial balance
//     balance; recognised in OCI (Other Comprehensive Income).
//
// One run per company/book/period combination. Re-running a period requires
// reversing the existing run first (Phase 4 feature).
//
// CTA sign convention:
//
//	CTAAmount > 0: translation loss  → DR CTA account (reduces equity)
//	CTAAmount < 0: translation gain  → CR CTA account (increases equity)
type TranslationRun struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// BookID references the secondary AccountingBook whose amounts are being translated.
	BookID uint `gorm:"not null;index"`

	// PeriodStart and PeriodEnd are the inclusive date range for this translation.
	PeriodStart time.Time `gorm:"type:date;not null"`
	PeriodEnd   time.Time `gorm:"type:date;not null"` // balance sheet / closing date

	// RunDate is the date this translation was computed.
	RunDate time.Time `gorm:"type:date;not null"`

	// FunctionalCurrency is the book's functional currency at run time (snapshot).
	FunctionalCurrency string `gorm:"type:varchar(3);not null"`

	// PresentationCurrency is the target (reporting) currency.
	PresentationCurrency string `gorm:"type:varchar(3);not null"`

	// ClosingRate is the period-end rate: 1 FunctionalCurrency unit = ClosingRate PresentationCurrency units.
	ClosingRate decimal.Decimal `gorm:"type:numeric(20,8);not null"`

	// AverageRate is the period average rate used for P&L items.
	AverageRate decimal.Decimal `gorm:"type:numeric(20,8);not null"`

	// CTAAmount is the net Cumulative Translation Adjustment for this run.
	// Positive = translation loss (DR OCI); negative = translation gain (CR OCI).
	CTAAmount decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	// CTAAccountID is the equity account that absorbs the CTA (system_key="fx_cta").
	// Null when CTAAmount is zero.
	CTAAccountID *uint `gorm:"index"`

	Status TranslationRunStatus `gorm:"type:text;not null;default:'posted'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TranslationLine records the per-account detail of a TranslationRun.
// One row per account that had activity in the translation period.
type TranslationLine struct {
	ID               uint `gorm:"primaryKey"`
	TranslationRunID uint `gorm:"not null;index"`
	CompanyID        uint `gorm:"not null;index"`

	// AccountID is the chart of accounts account this line covers.
	AccountID uint `gorm:"not null;index"`

	// FunctionalDebit and FunctionalCredit are the summed JournalLineBookAmount
	// amounts for this account over the period, in the book's functional currency.
	FunctionalDebit  decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	FunctionalCredit decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	// RateApplied is the exchange rate used to convert this account's balance.
	RateApplied decimal.Decimal `gorm:"type:numeric(20,8);not null"`

	// RateType indicates whether the closing or average rate was applied.
	RateType TranslationRateType `gorm:"type:text;not null"`

	// TranslatedDebit and TranslatedCredit are the presentation-currency amounts.
	TranslatedDebit  decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	TranslatedCredit decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	CreatedAt time.Time
}
