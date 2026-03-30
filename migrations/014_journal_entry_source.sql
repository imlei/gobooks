-- 014_journal_entry_source.sql
-- Add source_type + source_id to journal_entries and enforce posting uniqueness.
--
-- Design rationale:
--
--   Every journal entry originates from exactly one business document
--   (invoice, bill, payment, manual entry, or reversal). Storing the source
--   alongside the journal entry enables:
--
--     1. A DB-level unique constraint ensuring at most one *active* (posted)
--        journal entry exists per (company, source_type, source_id). This is the
--        final backstop against duplicate postings that slip past the application-
--        level SELECT FOR UPDATE lock.
--
--     2. Direct JE → source document lookup without joining invoices/bills tables.
--
-- Concurrency model (defence in depth):
--
--   Layer 1 (application):  pre-flight status check before entering the transaction.
--   Layer 2 (application):  SELECT ... FOR UPDATE on the source document row inside
--                            the transaction; status is re-validated after acquiring
--                            the lock. A concurrent posting attempt blocks here until
--                            the first committer either succeeds or rolls back.
--   Layer 3 (database):     unique partial index below. If, against all odds, two
--                            transactions both passed the status re-check, the second
--                            INSERT into journal_entries will fail with a 23505
--                            unique-constraint violation, which the application
--                            surfaces as ErrConcurrentPostingConflict.
--
-- Source type values (from models.LedgerSourceType):
--   invoice  — customer invoice posting (PostInvoice)
--   bill     — purchase bill posting    (PostBill)
--   payment  — payment application      (future)
--   reversal — void or manual reversal  (VoidInvoice, ReverseJournalEntry)
--   manual   — hand-entered JE          (source_id = 0; excluded from index)
--   opening_balance — period opening    (source_id = 0; excluded from index)

ALTER TABLE journal_entries
    ADD COLUMN IF NOT EXISTS source_type TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_id   BIGINT  NOT NULL DEFAULT 0;

-- Lookup index: find all JEs for a given source document (e.g. all JEs for invoice 42).
CREATE INDEX IF NOT EXISTS idx_journal_entries_source
    ON journal_entries (company_id, source_type, source_id)
    WHERE source_type != '' AND source_id > 0;

-- Uniqueness backstop: at most one *posted* JE per (company, source_type, source_id).
-- The partial WHERE clause excludes:
--   • manual / opening_balance entries (source_type = '' or source_id = 0)
--   • reversed JEs (status = 'reversed') — allows the reversal JE to coexist
--     with the original JE for the same source document using a different source_type
CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_posted_source
    ON journal_entries (company_id, source_type, source_id)
    WHERE status = 'posted' AND source_type != '' AND source_id > 0;
