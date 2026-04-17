-- 055_customer_vendor_is_active.sql
-- Add soft-delete flag to customers and vendors so parties with historical
-- records can be archived from pickers without losing their data. Full
-- deletion is gated at the application layer: only parties with zero
-- references can be truly deleted.
--
-- Pre-existing rows default to active so the migration is a no-op for
-- live data.

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE vendors
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
