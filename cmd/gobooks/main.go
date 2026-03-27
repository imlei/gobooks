// 遵循产品需求 v1.0
package main

import (
	"log"

	"gobooks/internal/config"
	"gobooks/internal/db"
	"gobooks/internal/version"
	"gobooks/internal/web"
)

func main() {
	// Load configuration from .env / environment variables.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	// Connect to PostgreSQL (GORM) and run basic migrations.
	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	if err := db.Migrate(gormDB); err != nil {
		log.Fatalf("db migrate failed: %v", err)
	}

	// Create and start the Fiber web server.
	app := web.NewServer(cfg, gormDB)
	log.Printf("gobooks %s listening on %s", version.Version, cfg.Addr)
	log.Fatal(app.Listen(cfg.Addr))
}

