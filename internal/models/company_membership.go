// 遵循project_guide.md
package models

import (
	"time"

	"github.com/google/uuid"
)

// CompanyMembership ties a user to a company with a role.
type CompanyMembership struct {
	ID        uuid.UUID   `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID   `gorm:"type:uuid;not null;index"`
	User      User        `gorm:"foreignKey:UserID"`
	CompanyID uint        `gorm:"not null;index"`
	Role      CompanyRole `gorm:"type:company_role;not null"`
	IsActive  bool        `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
