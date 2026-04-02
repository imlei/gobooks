// 遵循project_guide.md
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// ItemComponent represents a parent → component relationship for BOM / Bundle / Assembly.
//
// Usage by ItemStructureType:
//   - single:   has zero rows (no components).
//   - bundle:   parent is the sellable package; components are existing items that
//               are listed on the invoice but remain individually stocked.
//   - assembly: parent is the finished good; components are consumed during a
//               build process (future: assembly_build inventory movement).
//
// Both parent_item_id and component_item_id must belong to the same company_id.
// effective_from / effective_to support future BOM versioning (nullable = always active).
type ItemComponent struct {
	ID        uint `gorm:"primaryKey"`
	CompanyID uint `gorm:"not null;index"`

	ParentItemID    uint           `gorm:"not null;index"`
	ParentItem      ProductService `gorm:"foreignKey:ParentItemID"`
	ComponentItemID uint           `gorm:"not null;index"`
	ComponentItem   ProductService `gorm:"foreignKey:ComponentItemID"`

	Quantity  decimal.Decimal `gorm:"type:numeric(10,4);not null;default:1"`
	SortOrder int             `gorm:"not null;default:0"`

	EffectiveFrom *time.Time
	EffectiveTo   *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
