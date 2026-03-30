// 遵循project_guide.md
// Package logging 封装应用全局结构化日志。
// 使用 Go 标准库 log/slog，JSON 输出到 stdout，无数据库依赖。
package logging

import (
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger

// Init 初始化全局结构化日志器（JSON 格式，stdout）。
// 日志级别通过 LOG_LEVEL 环境变量控制（DEBUG / INFO / WARN / ERROR）。
// 未设置时默认 INFO，适合生产环境。
// 必须在启动服务器前调用一次。
// 注意：如果 LOG_LEVEL 通过 .env 文件加载，应在 config.Load() 后调用 SetLevel。
func Init() {
	level := parseLogLevel(os.Getenv("LOG_LEVEL"))
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// SetLevel reinitialises the logger with the given level string.
// Call this after config.Load() so that LOG_LEVEL values from .env take effect.
func SetLevel(level string) {
	l := parseLogLevel(level)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: l,
	})
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// parseLogLevel converts a LOG_LEVEL string to slog.Level. Defaults to INFO.
func parseLogLevel(s string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// L 返回全局日志器。若 Init 未被调用，则自动初始化（兜底，避免 nil panic）。
func L() *slog.Logger {
	if logger == nil {
		Init()
	}
	return logger
}
