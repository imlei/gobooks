// 遵循project_guide.md
package models

import (
	"time"

	"github.com/google/uuid"
)

// UserCompanyPermission is a schema-ready extension point for future per-user,
// per-company permission overrides. It is NOT wired into CanPerformAction yet.
//
// Design intent: each row either grants or revokes a named permission for a
// specific user within a specific company, enabling fine-grained control on top
// of the existing role-based system defined in CompanyRole / CompanyMembership.
type UserCompanyPermission struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index:idx_ucp_user_company,priority:1"`
	CompanyID  uint      `gorm:"not null;index:idx_ucp_user_company,priority:2"`
	Permission string    `gorm:"not null;index:idx_ucp_user_company,priority:3"`
	Granted    bool      `gorm:"not null"` // true = grant override, false = deny override
	GrantedBy  uuid.UUID `gorm:"type:uuid;not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
