// 遵循project_guide.md
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Reconciliation represents a bank reconciliation session for one account.
//
// For MVP we reconcile against a GL account (usually an Asset account like "Bank").
// We store the statement ending balance and the cleared balance from selected lines.
type Reconciliation struct {
	ID uint `gorm:"primaryKey"`

	CompanyID uint    `gorm:"not null;index"`
	AccountID uint    `gorm:"not null;index"`
	Account   Account `gorm:"foreignKey:AccountID"`

	StatementDate  time.Time       `gorm:"not null"`
	EndingBalance  decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	ClearedBalance decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	// Void fields — only the latest active reconciliation may be voided.
	IsVoided       bool       `gorm:"not null;default:false"`
	VoidReason     string     `gorm:"type:text;not null;default:''"`
	VoidedAt       *time.Time
	VoidedByUserID *uuid.UUID `gorm:"type:uuid"`

	CreatedAt time.Time
}
