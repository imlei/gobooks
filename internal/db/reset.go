// 遵循project_guide.md
package db

import (
	"fmt"

	"gorm.io/gorm"
)

// ResetAllApplicationData removes every row from all application tables and resets
// primary key sequences. After this, setup.Guard will send users to /setup again.
//
// Intended for local/dev or when you want to wipe the company and re-run setup.
// Uses PostgreSQL TRUNCATE ... RESTART IDENTITY CASCADE (matches db.Connect driver).
func ResetAllApplicationData(db *gorm.DB) error {
	// Table order: children first is not required when CASCADE is used, but listing
	// all tables in one TRUNCATE is valid in PostgreSQL.
	sql := `
TRUNCATE TABLE
	journal_lines,
	journal_entries,
	reconciliations,
	invoices,
	bills,
	customers,
	vendors,
	accounts,
	audit_logs,
	companies
RESTART IDENTITY CASCADE;
`
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("truncate application tables: %w", err)
	}
	return nil
}
