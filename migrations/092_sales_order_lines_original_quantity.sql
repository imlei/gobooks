-- 092_sales_order_lines_original_quantity.sql
-- Capture the contracted (original) quantity on each SO line so the
-- over-shipment buffer cap (S3) stays anchored to a stable baseline as
-- operators raise Qty post-confirm via the partially-invoiced edit path
-- (S2 — 2026-04-25).
--
-- Without this column, every successful adjust would shift the baseline,
-- so buffer would compound: original=8 + buffer=1 → newQty=9 →
-- baseline=9 + buffer=1 → newQty=10 → infinite drift.  With the column
-- the cap stays original + buffer regardless of how many times the line
-- is adjusted.
--
-- Backfill: for existing rows, original_quantity := quantity.  These
-- lines were created before S2 and their current quantity IS their
-- contracted amount.  Safe even on empty tables.

ALTER TABLE sales_order_lines
    ADD COLUMN IF NOT EXISTS original_quantity NUMERIC(18,4) NOT NULL DEFAULT 0;

UPDATE sales_order_lines
   SET original_quantity = quantity
 WHERE original_quantity = 0
   AND quantity > 0;
