package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

func ApplySQLMigrations(db *gorm.DB, dir string) error {
	if dir == "" {
		dir = "migrations"
	}

	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL
)`).Error; err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no sql migrations found in %s", dir)
	}

	for _, name := range files {
		var count int64
		if err := db.Raw(
			`SELECT count(*) FROM schema_migrations WHERE version = ?`,
			name,
		).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		path := filepath.Join(dir, name)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(sqlBytes)).Error; err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return tx.Exec(
				`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
				name, time.Now().UTC(),
			).Error
		}); err != nil {
			return err
		}
	}

	return nil
}
