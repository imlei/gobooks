// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/google/uuid"
)

// Session stores an opaque session token as a hash; the raw token lives only in the client.
type Session struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TokenHash        string     `gorm:"not null;uniqueIndex"`
	UserID           uuid.UUID  `gorm:"type:uuid;not null;index"`
	ActiveCompanyID  *uint      `gorm:"index"`
	ExpiresAt        time.Time  `gorm:"not null;index"`
	CreatedAt        time.Time  `gorm:"not null"`
	RevokedAt        *time.Time `gorm:"index"`
}
