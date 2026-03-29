package main

import (
	"log"

	"gobooks/internal/config"
	"gobooks/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	if err := db.ApplySQLMigrations(gormDB, "migrations"); err != nil {
		log.Fatalf("sql migrate failed: %v", err)
	}

	log.Print("sql migrations applied successfully")
}
