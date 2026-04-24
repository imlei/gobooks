// 遵循project_guide.md
package models

import (
	"time"

	"github.com/google/uuid"
)

// ReportFavourite is one row in the per-user, per-company list of
// reports the operator has starred. Acts as a join row between users,
// companies, and report keys (where ReportKey is a stable string ID
// like "balance-sheet" or "income-statement" — see
// services.ReportRegistry).
//
// Uniqueness is enforced by a composite unique index so a single
// (user, company, report) tuple can't accumulate duplicate stars; that
// also lets the toggle endpoint use UPSERT/Delete semantics.
type ReportFavourite struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_report_favourites_user_company_key,priority:1"`
	CompanyID uint      `gorm:"not null;uniqueIndex:idx_report_favourites_user_company_key,priority:2"`
	ReportKey string    `gorm:"type:text;not null;uniqueIndex:idx_report_favourites_user_company_key,priority:3"`
	CreatedAt time.Time
}
