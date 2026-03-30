-- 013_journal_entry_status.sql
-- Add lifecycle status to journal_entries.
--
-- Design rationale:
--   journal_entries.status is stored independently from the source document
--   status (invoices.status, bills.status) but the two must stay synchronized
--   in lifecycle behavior. The posting engine is the only coordinator allowed
--   to transition both together inside a single transaction.
--
-- Status values:
--   draft    — entry created but not yet committed to books; lines may change.
--              The current system does not produce draft JEs; reserved for future
--              approval workflows.
--   posted   — entry is committed; lines are immutable; ledger entries are active.
--   voided   — entry was voided before posting (pre-commit cancel).
--   reversed — a reversal JE has been created and posted; this entry's ledger
--              entries have been marked reversed. The entry itself is retained
--              for audit traceability and must never be deleted.
--
-- Backfill:
--   All existing rows were created by PostInvoice / PostBill, which only write
--   a JE when committing. DEFAULT 'posted' backfills them correctly in one step.
--
-- Synchronization rules (enforced at application layer, not by triggers):
--   • PostInvoice / PostBill   → status = 'posted'
--   • VoidInvoice (reversal)   → original JE status = 'reversed';
--                                 reversal JE status = 'posted'
--   • ReverseJournalEntry      → same as above
--   • Inconsistent states rejected by lifecycle_checks.go helpers

ALTER TABLE journal_entries
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'posted'
        CHECK (status IN ('draft', 'posted', 'voided', 'reversed'));

-- Compound index: used by consistency scans and future report filters that
-- query JEs by lifecycle state within a company.
CREATE INDEX IF NOT EXISTS idx_journal_entries_company_status
    ON journal_entries (company_id, status);
