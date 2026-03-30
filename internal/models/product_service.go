// 遵循project_guide.md
package models

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ProductServiceType classifies what the product/service represents.
// inventory is reserved for a future phase; only service and non_inventory are active.
type ProductServiceType string

const (
	ProductServiceTypeService      ProductServiceType = "service"
	ProductServiceTypeNonInventory ProductServiceType = "non_inventory"
)

// AllProductServiceTypes returns the currently supported types in display order.
func AllProductServiceTypes() []ProductServiceType {
	return []ProductServiceType{
		ProductServiceTypeService,
		ProductServiceTypeNonInventory,
	}
}

// ProductServiceTypeLabel returns a human-readable label for a type.
func ProductServiceTypeLabel(t ProductServiceType) string {
	switch t {
	case ProductServiceTypeService:
		return "Service"
	case ProductServiceTypeNonInventory:
		return "Non-Inventory"
	default:
		return string(t)
	}
}

// ParseProductServiceType parses a raw string into a ProductServiceType, returning an error
// if the value is not recognised.
func ParseProductServiceType(s string) (ProductServiceType, error) {
	switch ProductServiceType(s) {
	case ProductServiceTypeService, ProductServiceTypeNonInventory:
		return ProductServiceType(s), nil
	default:
		return "", fmt.Errorf("unknown product/service type: %q", s)
	}
}

// ProductService is a company-scoped item that can be added to invoice lines.
// It links to a revenue account so invoice posting can credit the correct account.
// DefaultTaxCodeID is optional; if set it pre-fills the tax code on new invoice lines.
type ProductService struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	Name        string             `gorm:"not null"`
	Type        ProductServiceType `gorm:"type:text;not null"`
	Description string             `gorm:"not null;default:''"`

	DefaultPrice decimal.Decimal `gorm:"type:numeric(18,4);not null;default:0"`

	RevenueAccountID uint    `gorm:"not null;index"`
	RevenueAccount   Account `gorm:"foreignKey:RevenueAccountID"`

	DefaultTaxCodeID *uint    `gorm:"index"`
	DefaultTaxCode   *TaxCode `gorm:"foreignKey:DefaultTaxCodeID"`

	IsActive bool `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
