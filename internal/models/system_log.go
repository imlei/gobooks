// 遵循产品需求 v1.0
package models

import (
	"time"

	"github.com/google/uuid"
)

// SystemLog 记录运行时结构化日志（错误、警告）到数据库，供 SysAdmin 查看。
// 与 AuditLog（业务审计）完全分离，专用于技术层面的运行时事件。
//
// Level: "ERROR" | "WARN" | "INFO"
// Stack: 仅在 panic 场景填充（goroutine 堆栈）
type SystemLog struct {
	ID        uint      `gorm:"primaryKey"`
	Level     string    `gorm:"not null;index"`
	Message   string    `gorm:"not null"`
	RequestID string    `gorm:"index"`
	Path      string
	Method    string
	CompanyID *uint      `gorm:"index"`
	UserID    *uuid.UUID `gorm:"type:uuid;index"`
	Stack     string
	CreatedAt time.Time `gorm:"not null;index"`
}
