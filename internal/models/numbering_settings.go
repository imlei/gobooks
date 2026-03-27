// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NumberingSetting stores company-scoped display numbering rules (rules_json = JSON array of rules).
type NumberingSetting struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	CompanyID uint      `gorm:"not null;uniqueIndex"`
	Version   int       `gorm:"not null;default:1"`
	RulesJSON []byte    `gorm:"type:jsonb;column:rules_json"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (NumberingSetting) TableName() string {
	return "numbering_settings"
}

// BeforeCreate assigns a UUID when the DB default is not used by GORM.
func (n *NumberingSetting) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
