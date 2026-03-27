// 遵循产品需求 v1.0
package db

import (
	"gobooks/internal/models"

	"gorm.io/gorm"
)

// Migrate runs basic GORM auto-migrations.
// This is intentionally simple for the initial project setup.
func Migrate(db *gorm.DB) error {
	if err := renameJournalEntriesDescriptionToJournalNo(db); err != nil {
		return err
	}
	return db.AutoMigrate(
		&models.Company{},
		&models.AIConnectionSettings{},
		&models.Account{},
		&models.Customer{},
		&models.Vendor{},
		&models.Invoice{},
		&models.Bill{},
		&models.JournalEntry{},
		&models.JournalLine{},
		&models.Reconciliation{},
		&models.AuditLog{},
	)
}

// renameJournalEntriesDescriptionToJournalNo upgrades older databases that used
// journal_entries.description; the application model now maps to journal_no.
func renameJournalEntriesDescriptionToJournalNo(db *gorm.DB) error {
	return db.Exec(`
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = CURRENT_SCHEMA()
      AND table_name = 'journal_entries'
      AND column_name = 'description'
  ) THEN
    ALTER TABLE journal_entries RENAME COLUMN description TO journal_no;
  END IF;
END $$;
`).Error
}

