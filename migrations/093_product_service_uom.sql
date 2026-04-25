-- 093_product_service_uom.sql
-- UOM (Unit of Measure) phase U1 — see UOM_DESIGN.md.
--
-- Adds three UOM columns + two factor columns to product_services so a
-- single item can be stocked in one unit (e.g. BOTTLE), sold in another
-- (e.g. PACK_6), and purchased in yet another (e.g. CASE).  Factors are
-- always "how many StockUOMs equal one X UOM".
--
-- Defaults are EA / EA / EA / 1 / 1 — every existing item keeps working
-- without operator action (factor=1 means no conversion).  Operators
-- opt in by editing the item.  Stock UOM is immutable while inventory
-- on-hand > 0 (enforced by services.ChangeStockUOM, parallels the
-- existing TrackingMode rule).
--
-- See UOM_DESIGN.md §3.1 for column semantics, §6.8 for back-compat
-- guarantees on existing data.

ALTER TABLE product_services
    ADD COLUMN IF NOT EXISTS stock_uom            VARCHAR(16)   NOT NULL DEFAULT 'EA';
ALTER TABLE product_services
    ADD COLUMN IF NOT EXISTS sell_uom             VARCHAR(16)   NOT NULL DEFAULT 'EA';
ALTER TABLE product_services
    ADD COLUMN IF NOT EXISTS sell_uom_factor      NUMERIC(18,6) NOT NULL DEFAULT 1;
ALTER TABLE product_services
    ADD COLUMN IF NOT EXISTS purchase_uom         VARCHAR(16)   NOT NULL DEFAULT 'EA';
ALTER TABLE product_services
    ADD COLUMN IF NOT EXISTS purchase_uom_factor  NUMERIC(18,6) NOT NULL DEFAULT 1;
