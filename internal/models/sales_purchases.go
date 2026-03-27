// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Invoice is a simple sales invoice header for MVP.
// Duplicate invoice numbers are allowed only when user confirms conflict.
type Invoice struct {
	ID uint `gorm:"primaryKey"`

	InvoiceNumber string `gorm:"not null;index"`
	CustomerID    uint   `gorm:"not null;index"`
	Customer      Customer `gorm:"foreignKey:CustomerID"`

	InvoiceDate time.Time       `gorm:"not null"`
	Amount      decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	Memo        string          `gorm:"not null;default:''"`

	CreatedAt time.Time
}

// Bill is a simple purchase bill header for MVP.
// Duplicate detection rule: (vendor_id + bill_number, case-insensitive) conflict.
type Bill struct {
	ID uint `gorm:"primaryKey"`

	BillNumber string `gorm:"not null;index"`
	VendorID   uint   `gorm:"not null;index"`
	Vendor     Vendor `gorm:"foreignKey:VendorID"`

	BillDate time.Time       `gorm:"not null"`
	Amount   decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	Memo     string          `gorm:"not null;default:''"`

	CreatedAt time.Time
}

