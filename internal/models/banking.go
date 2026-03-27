// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Reconciliation represents a bank reconciliation session for one account.
//
// For MVP we reconcile against a GL account (usually an Asset account like "Bank").
// We store the statement ending balance and the cleared balance from selected lines.
type Reconciliation struct {
	ID uint `gorm:"primaryKey"`

	AccountID uint   `gorm:"not null;index"`
	Account   Account `gorm:"foreignKey:AccountID"`

	StatementDate  time.Time       `gorm:"not null"`
	EndingBalance  decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	ClearedBalance decimal.Decimal `gorm:"type:numeric(18,2);not null"`

	CreatedAt time.Time
}

