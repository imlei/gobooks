// 遵循产品需求 v1.0
package models

import "time"

// PartyType is used in Journal Lines to reference either a customer or a vendor.
// Keep this minimal for now (MVP).
type PartyType string

const (
	PartyTypeNone     PartyType = ""
	PartyTypeCustomer PartyType = "customer"
	PartyTypeVendor   PartyType = "vendor"
)

func (t PartyType) Valid() bool {
	switch t {
	case PartyTypeNone, PartyTypeCustomer, PartyTypeVendor:
		return true
	default:
		return false
	}
}

// Customer is a minimal name record (for Journal Entry "Name" selection), scoped to one company.
type Customer struct {
	ID        uint   `gorm:"primaryKey"`
	CompanyID uint   `gorm:"not null;index"`
	Name      string `gorm:"not null"`
	CreatedAt time.Time
}

// Vendor is a minimal name record (for Journal Entry "Name" selection), scoped to one company.
type Vendor struct {
	ID        uint   `gorm:"primaryKey"`
	CompanyID uint   `gorm:"not null;index"`
	Name      string `gorm:"not null"`
	CreatedAt time.Time
}

