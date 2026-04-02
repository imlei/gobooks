-- Migration 033: Settlement posting state.
-- Tracks which JE was generated from a settlement and when.

ALTER TABLE channel_settlements
    ADD COLUMN IF NOT EXISTS posted_journal_entry_id BIGINT REFERENCES journal_entries(id) ON DELETE SET NULL;

ALTER TABLE channel_settlements
    ADD COLUMN IF NOT EXISTS posted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_channel_settlements_posted
    ON channel_settlements(posted_journal_entry_id) WHERE posted_journal_entry_id IS NOT NULL;
