// 遵循project_guide.md
package services

// posting_errors.go — shared error sentinels and concurrency helpers for the
// posting engine (Phase 6).
//
// Concurrency defence-in-depth summary:
//
//   Layer 1 — pre-flight status check  (outside transaction, fast reject)
//   Layer 2 — SELECT FOR UPDATE lock   (inside transaction; applyLockForUpdate)
//              Status is re-validated after acquiring the row lock. A concurrent
//              posting attempt blocks at the lock until the first transaction
//              commits or rolls back, then re-reads and sees the new status.
//   Layer 3 — unique partial index     (DB backstop; wraps 23505 as
//              ErrConcurrentPostingConflict so callers get a meaningful error)
//
// SQLite note: SQLite serialises all writes at the connection level and does not
// support SELECT ... FOR UPDATE syntax. applyLockForUpdate detects the dialect
// and is a no-op for SQLite. Tests remain unaffected; Postgres production
// environments get full row-level locking.

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ── Error sentinels ───────────────────────────────────────────────────────────

// ErrAlreadyPosted is returned when a posting attempt finds the source document
// already in a non-draft state inside the locked transaction. This indicates
// either a concurrent request beat us to it, or the caller has a stale view of
// the document.
var ErrAlreadyPosted = errors.New("document is already posted — posting rejected")

// ErrConcurrentPostingConflict is returned when the unique-index backstop fires:
// two transactions somehow both passed the FOR UPDATE re-validation and both
// tried to INSERT a journal entry for the same source document. The second
// INSERT fails with a Postgres 23505 unique-constraint violation, which is
// wrapped as this error.
var ErrConcurrentPostingConflict = errors.New("concurrent posting conflict: another request posted this document simultaneously — retry if needed")

// ── DB helpers ────────────────────────────────────────────────────────────────

// applyLockForUpdate adds SELECT ... FOR UPDATE to q for databases that support
// row-level locking (Postgres). SQLite serialises writes at the connection level
// and does not support FOR UPDATE syntax; this function is a no-op for SQLite.
//
// Usage — inside a transaction, before the first write:
//
//	var locked models.Invoice
//	err := applyLockForUpdate(
//	    tx.Select("id", "company_id", "status").
//	        Where("id = ? AND company_id = ?", invoiceID, companyID),
//	).First(&locked).Error
func applyLockForUpdate(q *gorm.DB) *gorm.DB {
	if q.Dialector.Name() == "sqlite" {
		return q
	}
	return q.Clauses(clause.Locking{Strength: "UPDATE"})
}

// wrapUniqueViolation converts a Postgres 23505 unique-constraint error into
// ErrConcurrentPostingConflict. All other errors are returned unchanged.
//
// Call this around any journal_entries INSERT that could race:
//
//	if err := tx.Create(&je).Error; err != nil {
//	    return wrapUniqueViolation(err, "create journal entry")
//	}
func wrapUniqueViolation(err error, op string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConcurrentPostingConflict
	}
	return err
}
