-- Migration 029: Link inventory movements to journal entries.
-- When a posting creates both a JE and inventory movements (sale/purchase),
-- the movement records the JE ID for bidirectional traceability.

ALTER TABLE inventory_movements
    ADD COLUMN IF NOT EXISTS journal_entry_id BIGINT REFERENCES journal_entries(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_inv_movements_je ON inventory_movements(journal_entry_id) WHERE journal_entry_id IS NOT NULL;
