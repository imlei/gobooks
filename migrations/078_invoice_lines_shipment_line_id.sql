-- 078_invoice_lines_shipment_line_id.sql
-- Phase I slice I.5: link invoice lines back to the shipment line
-- they invoice, so the waiting_for_invoice operational queue can
-- close atomically at Invoice post time.
--
-- Identity chain (sell-side, under shipment_required=true)
-- --------------------------------------------------------
--   SalesOrder.line ──► ShipmentLine ──► InvoiceLine
--
-- Nullable for two reasons
-- ------------------------
-- 1. Companies on shipment_required=false never use it. Legacy
--    Invoice-forms-COGS carries no shipment linkage, and this column
--    must remain unused for them.
-- 2. Under shipment_required=true, an Invoice MAY cover non-stock
--    lines (services, fees) that were never shipped and therefore
--    have no ShipmentLine. Those lines leave shipment_line_id NULL
--    and fall through to AR + Revenue without waiting_for_invoice
--    interaction.
--
-- No FK in schema (same convention as existing source-identity
-- reservation fields). The service layer enforces (company, posted
-- Shipment, open WFI row for the line) in I.5's validation path.
--
-- Scope lock
-- ----------
-- I.5 does NOT support partial invoicing of a shipment line. The
-- invoice line's qty is not cross-checked against the shipment
-- line's qty by this column alone; the matching-side guard in the
-- service layer enforces 1:1 closure of the waiting_for_invoice row
-- (fail-loud on mismatch). Partial invoicing is out of Phase I
-- scope.

ALTER TABLE invoice_lines
    ADD COLUMN IF NOT EXISTS shipment_line_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_invoice_lines_shipment_line_id ON invoice_lines(shipment_line_id);
