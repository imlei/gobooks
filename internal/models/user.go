// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/google/uuid"
)

// User is an authenticated account (email + password hash).
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"not null;uniqueIndex"`
	PasswordHash string    `gorm:"not null"`
	DisplayName  string    `gorm:"not null;default:''"`
	IsActive     bool      `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
