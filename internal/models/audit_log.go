// 遵循project_guide.md
package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog records important user/system actions.
// Actor string remains for display; actor_user_id links to users when present.
type AuditLog struct {
	ID uint `gorm:"primaryKey"`

	Action     string `gorm:"not null;index"` // e.g. "journal.posted"
	EntityType string `gorm:"not null;index"` // e.g. "journal_entry"
	EntityID   uint   `gorm:"not null;default:0;index"`

	Actor string `gorm:"not null;default:'system'"`

	CompanyID   *uint      `gorm:"index"`
	ActorUserID *uuid.UUID `gorm:"type:uuid;index"`

	DetailsJSON string `gorm:"type:text;not null;default:'{}'"`

	CreatedAt time.Time `gorm:"index"`
}

