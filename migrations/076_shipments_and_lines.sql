-- 076_shipments_and_lines.sql
-- Phase I slice I.2: Shipment and ShipmentLine as first-class documents
-- (sell-side mirror of Phase H slice H.2's receipts / receipt_lines).
--
-- What this migration installs
-- ----------------------------
-- Two new tables, `shipments` and `shipment_lines`, persisting the
-- Shipment document layer. Each Shipment represents an outbound event
-- (goods leaving a warehouse to a customer). Each ShipmentLine is one
-- item / qty on that Shipment, with source-identity reservation fields
-- (sales_order_id on header, sales_order_line_id on line) for the
-- SO → Shipment → Invoice identity chain completed in I.5.
--
-- What this migration does NOT do (deliberately — I.2 scope lock)
-- ---------------------------------------------------------------
-- No inventory movement is produced by a Shipment yet. No journal
-- entry. No COGS. No Invoice coupling. The `status` column can hold
-- 'draft', 'posted', or 'voided', but `posted` is purely a document-
-- layer state in I.2 — it does not drive any issue truth into
-- inventory_movements / inventory_cost_layers / inventory_balances,
-- and it does not touch the GL. That consumer lands in I.3 (
-- IssueStockFromShipment + Dr COGS / Cr Inventory + the
-- waiting_for_invoice operational item).
--
-- The `shipment_required` capability rail (migration 075) is NOT
-- checked anywhere in I.2. Shipment creation and posting are gate-
-- agnostic. Gate wiring lands with I.4 (Invoice decoupling).
--
-- Source-identity reservation
-- ---------------------------
-- `shipments.sales_order_id` and `shipment_lines.sales_order_line_id`
-- are nullable reservation columns for the Phase I SO → Shipment →
-- Invoice identity chain (mirror of H.2's PO reservation fields).
-- They are accepted on create/update but not read or enforced anywhere
-- in I.2. No FK constraint — enforcement lands with I.5 matching when
-- there is a real consumer.
--
-- Unit cost — not captured on ShipmentLine
-- ----------------------------------------
-- Unlike ReceiptLine.unit_cost (where cost is intrinsic to the inbound
-- event), ShipmentLine deliberately does NOT carry a unit_cost column.
-- Per the authoritative-cost principle in INVENTORY_MODULE_API.md,
-- outbound cost is ALWAYS determined by the inventory module at issue
-- time (FIFO layer peel or moving-average lookup) — never supplied by
-- the business-document layer. I.3's IssueStockFromShipment will
-- return unit_cost_base from the inventory calc, and the JE builder
-- will consume that. Adding a unit_cost column here would invite
-- authority drift and is forbidden.

CREATE TABLE IF NOT EXISTS shipments (
    id                BIGSERIAL PRIMARY KEY,
    company_id        BIGINT       NOT NULL,
    shipment_number   TEXT         NOT NULL DEFAULT '',
    customer_id       BIGINT,
    warehouse_id      BIGINT       NOT NULL,
    ship_date         DATE         NOT NULL,
    status            TEXT         NOT NULL DEFAULT 'draft',
    memo              TEXT         NOT NULL DEFAULT '',
    reference         TEXT         NOT NULL DEFAULT '',
    sales_order_id    BIGINT,
    posted_at         TIMESTAMPTZ,
    voided_at         TIMESTAMPTZ,
    created_at        TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_shipments_company_id        ON shipments(company_id);
CREATE INDEX IF NOT EXISTS idx_shipments_customer_id       ON shipments(customer_id);
CREATE INDEX IF NOT EXISTS idx_shipments_warehouse_id      ON shipments(warehouse_id);
CREATE INDEX IF NOT EXISTS idx_shipments_status            ON shipments(status);
CREATE INDEX IF NOT EXISTS idx_shipments_ship_date         ON shipments(ship_date);
CREATE INDEX IF NOT EXISTS idx_shipments_company_number    ON shipments(company_id, shipment_number);
CREATE INDEX IF NOT EXISTS idx_shipments_sales_order_id    ON shipments(sales_order_id);

CREATE TABLE IF NOT EXISTS shipment_lines (
    id                    BIGSERIAL PRIMARY KEY,
    company_id            BIGINT          NOT NULL,
    shipment_id           BIGINT          NOT NULL,
    sort_order            INTEGER         NOT NULL DEFAULT 0,
    product_service_id    BIGINT          NOT NULL,
    description           TEXT            NOT NULL DEFAULT '',
    qty                   NUMERIC(18,6)   NOT NULL DEFAULT 0,
    unit                  TEXT            NOT NULL DEFAULT '',
    sales_order_line_id   BIGINT,
    created_at            TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_shipment_lines_company_id           ON shipment_lines(company_id);
CREATE INDEX IF NOT EXISTS idx_shipment_lines_shipment_id          ON shipment_lines(shipment_id);
CREATE INDEX IF NOT EXISTS idx_shipment_lines_product_service_id   ON shipment_lines(product_service_id);
CREATE INDEX IF NOT EXISTS idx_shipment_lines_sales_order_line_id  ON shipment_lines(sales_order_line_id);
