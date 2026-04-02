-- Migration 035: Channel clearing recognition + settlement/payout reversal state.

-- 1. Link invoices to their source channel order for clearing account resolution.
ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS channel_order_id BIGINT REFERENCES channel_orders(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_invoices_channel_order
    ON invoices(channel_order_id) WHERE channel_order_id IS NOT NULL;

-- 2. Settlement fee reversal JE link.
ALTER TABLE channel_settlements
    ADD COLUMN IF NOT EXISTS posted_reversal_je_id BIGINT REFERENCES journal_entries(id) ON DELETE SET NULL;

-- 3. Payout reversal JE link.
ALTER TABLE channel_settlements
    ADD COLUMN IF NOT EXISTS payout_reversal_je_id BIGINT REFERENCES journal_entries(id) ON DELETE SET NULL;
