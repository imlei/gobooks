// 遵循project_guide.md
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InvitationStatusPending is stored in company_invitations.status.
const InvitationStatusPending = "pending"

// CompanyInvitation is a pending invite to join a company (acceptance flow optional in later steps).
type CompanyInvitation struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	CompanyID uint `gorm:"not null;index"`
	Email     string `gorm:"not null"`
	Role      CompanyRole `gorm:"type:company_role;not null"`

	TokenHash       string    `gorm:"not null;uniqueIndex"`
	InvitedByUserID uuid.UUID `gorm:"type:uuid;not null;index"`
	InvitedBy       User      `gorm:"foreignKey:InvitedByUserID"`

	Status    string    `gorm:"not null;default:pending"`
	ExpiresAt time.Time `gorm:"not null"`

	CreatedAt time.Time
}

func (CompanyInvitation) TableName() string {
	return "company_invitations"
}

func (n *CompanyInvitation) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
