-- Migration 034: Settlement payout recording state.
-- Tracks the JE that recorded the payout (Dr Bank, Cr Clearing).

ALTER TABLE channel_settlements
    ADD COLUMN IF NOT EXISTS payout_journal_entry_id BIGINT REFERENCES journal_entries(id) ON DELETE SET NULL;

ALTER TABLE channel_settlements
    ADD COLUMN IF NOT EXISTS payout_recorded_at TIMESTAMPTZ;
