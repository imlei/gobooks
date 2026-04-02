-- Migration 031: Channel order conversion marker + mapping uniqueness.
--
-- 1. Add converted_invoice_id to channel_orders to prevent duplicate conversion.
-- 2. Add partial unique index on item_channel_mappings to enforce one active
--    mapping per (company, channel_account, marketplace, external_sku).

ALTER TABLE channel_orders
    ADD COLUMN IF NOT EXISTS converted_invoice_id BIGINT REFERENCES invoices(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_channel_orders_converted
    ON channel_orders(converted_invoice_id) WHERE converted_invoice_id IS NOT NULL;

-- Unique constraint: at most one active mapping per (company, account, marketplace, sku).
-- marketplace_id is nullable; COALESCE normalizes NULL to '' for uniqueness purposes.
CREATE UNIQUE INDEX IF NOT EXISTS uq_item_channel_mappings_active_sku
    ON item_channel_mappings(company_id, channel_account_id, COALESCE(marketplace_id, ''), external_sku)
    WHERE is_active = true;
