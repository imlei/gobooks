// 遵循project_guide.md
package main

import (
	"log"
	"time"

	"gobooks/internal/config"
	"gobooks/internal/db"
	"gobooks/internal/logging"
	"gobooks/internal/services"
	"gobooks/internal/version"
	"gobooks/internal/web"
)

func main() {
	// 初始化结构化日志（JSON 输出到 stdout）。必须在所有其他组件之前调用。
	logging.Init()

	// Load configuration from .env / environment variables.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	if err := services.ConfigureAISecretKey(cfg.AISecretKey); err != nil {
		log.Fatalf("AI secret key config failed: %v", err)
	}

	// Connect to PostgreSQL (GORM) and run basic migrations.
	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	if err := db.Migrate(gormDB); err != nil {
		log.Fatalf("db migrate failed: %v", err)
	}

	// Start daily cleanup goroutine for system_logs (retain 30 days).
	go func() {
		// Run once immediately on startup to catch any accumulated old rows.
		if n, err := services.CleanupSystemLogs(gormDB, 30*24*time.Hour); err != nil {
			logging.L().Warn("system_logs cleanup failed", "err", err)
		} else if n > 0 {
			logging.L().Info("system_logs cleanup", "deleted", n)
		}
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if n, err := services.CleanupSystemLogs(gormDB, 30*24*time.Hour); err != nil {
				logging.L().Warn("system_logs cleanup failed", "err", err)
			} else if n > 0 {
				logging.L().Info("system_logs cleanup", "deleted", n)
			}
		}
	}()

	// Create and start the Fiber web server.
	app := web.NewServer(cfg, gormDB)
	logging.L().Info("starting server", "version", version.Version, "addr", cfg.Addr)
	if err := app.Listen(cfg.Addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

