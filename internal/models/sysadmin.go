// 遵循project_guide.md
package models

import "time"

// SysadminUser 是完全独立于业务用户（User）的系统管理员账户。
// 两套账户系统共享同一个数据库但表名不同，认证流程完全分离。
type SysadminUser struct {
	ID           uint   `gorm:"primaryKey"`
	Email        string `gorm:"not null;uniqueIndex"`
	PasswordHash string `gorm:"not null"`
	IsActive     bool   `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SysadminSession 存储系统管理员的不透明会话令牌（哈希形式）。
// 与业务用户 sessions 表完全分离：不同的表、不同的 cookie 名称、不同的令牌命名空间。
type SysadminSession struct {
	ID             uint      `gorm:"primaryKey"`
	SysadminUserID uint      `gorm:"not null;index"`
	TokenHash      string    `gorm:"not null;uniqueIndex"`
	ExpiresAt      time.Time `gorm:"not null;index"`
	CreatedAt      time.Time `gorm:"not null"`
}
