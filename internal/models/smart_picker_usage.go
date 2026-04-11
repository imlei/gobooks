// 遵循project_guide.md
package models

import "time"

// SmartPickerUsage records every selection event from the SmartPicker UI.
// Used as a ranking/popularity signal; does not affect correctness or auth.
// One row per user selection — high-volume, append-only, best-effort.
type SmartPickerUsage struct {
	ID uint `gorm:"primaryKey"`

	CompanyID uint   `gorm:"not null;index"`
	Entity    string `gorm:"not null;index"` // e.g. "account", "customer"
	Context   string `gorm:"not null"`       // e.g. "expense_form_category"
	ItemID    uint   `gorm:"not null;index"`

	// RequestID correlates the selection with the search result set it came from.
	RequestID string `gorm:"not null;default:''"`

	CreatedAt time.Time `gorm:"index"`
}
