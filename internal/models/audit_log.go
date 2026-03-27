// 遵循产品需求 v1.0
package models

import "time"

// AuditLog records important user/system actions.
// For MVP we keep actor simple (string) and store details as JSON text.
type AuditLog struct {
	ID uint `gorm:"primaryKey"`

	Action     string `gorm:"not null;index"` // e.g. "journal.posted"
	EntityType string `gorm:"not null;index"` // e.g. "journal_entry"
	EntityID   uint   `gorm:"not null;default:0;index"`

	Actor string `gorm:"not null;default:'system'"` // for now we only have single-user mode

	DetailsJSON string `gorm:"type:text;not null;default:'{}'"`

	CreatedAt time.Time `gorm:"index"`
}

