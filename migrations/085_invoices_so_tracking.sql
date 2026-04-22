-- 085_invoices_so_tracking.sql
-- SO↔Invoice tracking chain: header + line-level back-links from
-- invoices to their originating SalesOrder.
--
-- Why
-- ---
-- SalesOrder has `InvoicedAmount` (header) + `InvoicedQty` (per line)
-- columns pre-populated by earlier AR Phase 13 work with the comment
-- "Invoices raised against this order link back via Invoice.SalesOrderID
-- (set at Phase 2)." The schema column + model field were anticipated
-- but never materialised. Without the link:
--   - SO.InvoicedAmount stays at 0 forever
--   - SO status never advances to `partially_invoiced` / `fully_invoiced`
--   - Operators have no data to answer "how much of this SO have we billed yet?"
--
-- What this installs
-- ------------------
-- `invoices.sales_order_id BIGINT` — nullable header-level FK to the
-- SalesOrder that sourced this invoice. Populated when an invoice is
-- created via the "Create Invoice" shortcut on SO detail; left NULL
-- for standalone invoices.
--
-- `invoice_lines.sales_order_line_id BIGINT` — nullable line-level
-- FK to the specific SalesOrderLine each invoice line bills against.
-- Populated server-side at save time by matching
-- ProductServiceID + FIFO-remaining — no UI affordance required.
-- Line matches don't need to be 1-1 perfect; the link is used to
-- increment SO.InvoicedQty at post time and reverse at void time.
--
-- No DB-level FK constraints. Matches the existing repo convention
-- for invoice cross-document links (`invoices.quote_id`,
-- `invoice_lines.shipment_line_id`). Cross-tenant / existence checks
-- live in the service layer.
--
-- Safety: nullable columns with no backfill. Existing invoices have
-- NULL (semantically correct — they pre-date this tracking).

ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS sales_order_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_invoices_sales_order_id
    ON invoices(sales_order_id);

ALTER TABLE invoice_lines
    ADD COLUMN IF NOT EXISTS sales_order_line_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_invoice_lines_sales_order_line_id
    ON invoice_lines(sales_order_line_id);
