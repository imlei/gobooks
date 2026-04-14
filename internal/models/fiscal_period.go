// 遵循project_guide.md
package models

import "time"

// FiscalPeriodStatus represents the lifecycle state of a reporting period.
type FiscalPeriodStatus string

const (
	// FiscalPeriodStatusOpen — period is open; entries may be posted.
	FiscalPeriodStatusOpen FiscalPeriodStatus = "open"

	// FiscalPeriodStatusClosed — period has been closed; no new entries permitted.
	// Closing a period drives the book's StandardChangePolicy to ForbidDirect.
	FiscalPeriodStatusClosed FiscalPeriodStatus = "closed"

	// FiscalPeriodStatusLocked — period is closed and formally filed / locked.
	// Locked periods cannot be re-opened without an explicit override.
	FiscalPeriodStatusLocked FiscalPeriodStatus = "locked"
)

// FiscalPeriod is a named date range for a company (and optionally a specific
// accounting book). Closing or locking a period triggers ForbidDirect policy
// on the affected book via RefreshBookStandardChangePolicy.
//
// BookID == 0 means the period is company-wide (applies to all books).
// BookID >  0 means the period is scoped to that specific book (e.g. a tax
// book may have different close dates from the primary book).
type FiscalPeriod struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	// BookID == 0 means company-wide. Stored as a plain column; no DB-level
	// foreign key because value 0 is not a valid book row.
	BookID uint `gorm:"not null;default:0;index"`

	// Label is a human-readable identifier such as "2024-Q1" or "March 2025".
	Label string `gorm:"type:text;not null"`

	PeriodStart time.Time          `gorm:"type:date;not null"`
	PeriodEnd   time.Time          `gorm:"type:date;not null"`
	Status      FiscalPeriodStatus `gorm:"type:text;not null;default:'open'"`

	ClosedAt *time.Time `gorm:"type:timestamptz"`
	ClosedBy string     `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
