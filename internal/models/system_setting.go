// 遵循产品需求 v1.0
package models

import "time"

// SystemSetting 是键值对形式的系统级配置，持久化到数据库。
// 当前用于存储维护模式状态（key="maintenance_mode"），
// 未来可扩展为更多运行时可调节的系统参数。
//
// 读路径（每次请求）：直接读取内存中的 atomic.Bool 缓存，零 DB 开销。
// 写路径（SysAdmin 操作）：先写 DB，再更新缓存，保证重启后状态一致。
type SystemSetting struct {
	Key       string    `gorm:"primaryKey"`
	Value     string    `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}
