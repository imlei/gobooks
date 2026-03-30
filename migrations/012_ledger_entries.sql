-- 012_ledger_entries.sql
-- Ledger entries: the accounting fact layer.
--
-- Each row is a 1:1 projection of a posted journal_line into the general ledger.
-- ledger_entries is NOT a replacement for journal_lines — journal_lines remain
-- the authoritative double-entry record. ledger_entries exist purely to make
-- account-level queries (account balance, trial balance, P&L, general ledger
-- report) fast without full journal_lines table scans.
--
-- Population rules (enforced at application layer, not by triggers):
--   1. A ledger entry is created for each journal_line when its parent
--      journal_entry is committed (posted).
--   2. Reversal journal entries create their own ledger entries (with swapped
--      debit/credit); the original ledger entries are marked status='reversed'
--      but are never deleted or modified further.
--   3. company_id is stored redundantly (copied from the journal entry) to
--      allow direct account-level queries without joining journal_entries.
--   4. source_type + source_id trace back to the originating business document
--      (invoice, bill, payment, etc.). For manual journal entries, source_type
--      is 'manual' and source_id is 0.
--
-- Reconstruction guarantee: ledger_entries can be fully truncated and rebuilt
-- from journal_entries + journal_lines at any time. It is a projection, not
-- primary data.

CREATE TABLE IF NOT EXISTS ledger_entries (
    id                  BIGSERIAL       PRIMARY KEY,

    -- Company scope: mandatory, indexed as part of every query.
    company_id          BIGINT          NOT NULL
                            REFERENCES companies(id) ON DELETE RESTRICT,

    -- Link back to the journal entry that generated this row.
    journal_entry_id    BIGINT          NOT NULL
                            REFERENCES journal_entries(id) ON DELETE RESTRICT,

    -- Source document that triggered posting (invoice, bill, payment, manual …).
    -- source_id is 0 when there is no originating document (e.g. manual JE).
    source_type         TEXT            NOT NULL DEFAULT '',
    source_id           BIGINT          NOT NULL DEFAULT 0,

    -- GL account affected by this posting.
    account_id          BIGINT          NOT NULL
                            REFERENCES accounts(id) ON DELETE RESTRICT,

    -- Date used for period-based reporting (copied from journal_entries.entry_date).
    posting_date        DATE            NOT NULL,

    -- Amounts. Exactly one of debit_amount / credit_amount will be non-zero
    -- for a given line; both may be zero for rounding/zero-value lines.
    debit_amount        NUMERIC(18,2)   NOT NULL DEFAULT 0,
    credit_amount       NUMERIC(18,2)   NOT NULL DEFAULT 0,

    -- Lifecycle:
    --   active   — normal, live entry contributing to account balances.
    --   reversed — the originating journal entry has been reversed; this row
    --              is retained for audit but excluded from balance calculations.
    status              TEXT            NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'reversed')),

    created_at          TIMESTAMPTZ     NOT NULL DEFAULT now()
);

-- ── Indexes ───────────────────────────────────────────────────────────────────

-- Primary reporting index: account balance and general ledger report queries
-- always filter by company + account and order/range by date.
CREATE INDEX IF NOT EXISTS idx_ledger_entries_company_account_date
    ON ledger_entries (company_id, account_id, posting_date);

-- Lookup by source journal entry (used when posting reversal to find entries
-- to mark as reversed, and for journal entry detail drilldown).
CREATE INDEX IF NOT EXISTS idx_ledger_entries_journal_entry_id
    ON ledger_entries (journal_entry_id);

-- Source document lookup (used to find all GL postings for a given invoice,
-- bill, or payment without joining journal_entries).
CREATE INDEX IF NOT EXISTS idx_ledger_entries_source
    ON ledger_entries (source_type, source_id)
    WHERE source_id > 0;
