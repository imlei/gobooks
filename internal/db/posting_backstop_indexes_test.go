package db

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEnsurePostingBackstopIndexes(t *testing.T) {
	dsn := fmt.Sprintf("file:posting_backstops_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.Exec(`CREATE TABLE journal_entries (
id integer primary key autoincrement,
company_id integer not null,
status text not null default 'posted',
source_type text not null default '',
source_id integer not null default 0
);`).Error; err != nil {
		t.Fatalf("create journal_entries: %v", err)
	}
	if err := testDB.Exec(`CREATE TABLE inventory_movements (
id integer primary key autoincrement,
company_id integer not null,
idempotency_key text
);`).Error; err != nil {
		t.Fatalf("create inventory_movements: %v", err)
	}
	if err := ensurePostingBackstopIndexes(testDB); err != nil {
		t.Fatalf("ensurePostingBackstopIndexes: %v", err)
	}

	if err := testDB.Exec(`INSERT INTO journal_entries (company_id, status, source_type, source_id) VALUES (1, 'posted', 'invoice', 42);`).Error; err != nil {
		t.Fatalf("insert first journal entry: %v", err)
	}
	if err := testDB.Exec(`INSERT INTO journal_entries (company_id, status, source_type, source_id) VALUES (1, 'posted', 'invoice', 42);`).Error; err == nil {
		t.Fatal("expected duplicate posted source journal entry to be rejected")
	}
	if err := testDB.Exec(`INSERT INTO journal_entries (company_id, status, source_type, source_id) VALUES (1, 'draft', 'invoice', 42);`).Error; err != nil {
		t.Fatalf("draft journal entry should be excluded from posted-source index: %v", err)
	}
	if err := testDB.Exec(`INSERT INTO journal_entries (company_id, status, source_type, source_id) VALUES (1, 'posted', '', 0);`).Error; err != nil {
		t.Fatalf("manual journal entry should be excluded from posted-source index: %v", err)
	}

	if err := testDB.Exec(`INSERT INTO inventory_movements (company_id, idempotency_key) VALUES (1, 'bill:9:line:1:v1');`).Error; err != nil {
		t.Fatalf("insert first inventory movement: %v", err)
	}
	if err := testDB.Exec(`INSERT INTO inventory_movements (company_id, idempotency_key) VALUES (1, 'bill:9:line:1:v1');`).Error; err == nil {
		t.Fatal("expected duplicate inventory movement idempotency key to be rejected")
	}
	if err := testDB.Exec(`INSERT INTO inventory_movements (company_id, idempotency_key) VALUES (1, '');`).Error; err != nil {
		t.Fatalf("empty idempotency key should be excluded from index: %v", err)
	}
}
