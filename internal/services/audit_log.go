// 遵循产品需求 v1.0
package services

import (
	"encoding/json"

	"gobooks/internal/models"

	"gorm.io/gorm"
)

// WriteAuditLog saves one audit row.
// details can be any small map/struct that can be marshaled to JSON.
func WriteAuditLog(tx *gorm.DB, action, entityType string, entityID uint, actor string, details any) error {
	if actor == "" {
		actor = "system"
	}

	raw := "{}"
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			raw = string(b)
		}
	}

	row := models.AuditLog{
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		Actor:       actor,
		DetailsJSON: raw,
	}
	return tx.Create(&row).Error
}

