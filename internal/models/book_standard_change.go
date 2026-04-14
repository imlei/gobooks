// 遵循project_guide.md
package models

import "time"

// BookStandardChangeMethod records how the standard change was performed.
type BookStandardChangeMethod string

const (
	// BookStandardChangeMethodDirect — changed directly (AllowDirect policy).
	BookStandardChangeMethodDirect BookStandardChangeMethod = "direct"

	// BookStandardChangeMethodWizard — changed via the migration wizard (RequireWizard policy).
	// Requires a CutoverDate.
	BookStandardChangeMethodWizard BookStandardChangeMethod = "wizard"
)

// BookStandardChange is an immutable, append-only audit record of every
// accounting-standard change applied to an AccountingBook. The current
// standard is always the StandardProfileID column on the AccountingBook row;
// this table provides the full history trail.
type BookStandardChange struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`
	BookID    uint `gorm:"not null;index"`

	OldProfileID uint                    `gorm:"not null"`
	OldProfile   AccountingStandardProfile `gorm:"foreignKey:OldProfileID"`
	NewProfileID uint                    `gorm:"not null"`
	NewProfile   AccountingStandardProfile `gorm:"foreignKey:NewProfileID"`

	// Method records whether this was a direct change or wizard-guided.
	Method BookStandardChangeMethod `gorm:"type:text;not null"`

	// CutoverDate is required for wizard changes; nil for direct changes.
	// It is the first day from which the new standard applies.
	CutoverDate *time.Time `gorm:"type:date"`

	// Notes captures any free-text rationale recorded during the wizard.
	Notes string `gorm:"type:text;not null;default:''"`

	// ChangedBy is the email / actor who performed the change.
	ChangedBy string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time
}
