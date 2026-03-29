package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:sql_migrate_test_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestApplySQLMigrationsIsIdempotent(t *testing.T) {
	db := openMigrationTestDB(t)
	dir := t.TempDir()

	files := map[string]string{
		"001_create_demo.sql": "CREATE TABLE demo_items (id INTEGER PRIMARY KEY, name TEXT NOT NULL);",
		"002_seed_demo.sql":   "INSERT INTO demo_items (id, name) VALUES (1, 'first');",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := ApplySQLMigrations(db, dir); err != nil {
		t.Fatal(err)
	}
	if err := ApplySQLMigrations(db, dir); err != nil {
		t.Fatal(err)
	}

	var migrationCount int64
	if err := db.Table("schema_migrations").Count(&migrationCount).Error; err != nil {
		t.Fatal(err)
	}
	if migrationCount != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", migrationCount)
	}

	var itemCount int64
	if err := db.Table("demo_items").Count(&itemCount).Error; err != nil {
		t.Fatal(err)
	}
	if itemCount != 1 {
		t.Fatalf("expected 1 seeded row after rerun, got %d", itemCount)
	}
}

func TestApplySQLMigrationsErrorsWhenDirectoryHasNoSQLFiles(t *testing.T) {
	db := openMigrationTestDB(t)
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a migration"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ApplySQLMigrations(db, dir); err == nil {
		t.Fatal("expected error when no sql migration files exist")
	}
}
