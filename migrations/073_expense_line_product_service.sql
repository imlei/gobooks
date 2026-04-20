-- 073_expense_line_product_service.sql
-- Add ProductService linkage at the expense-line level so expenses,
-- like bill lines and PO lines, can flow catalog-aware data into
-- downstream surfaces (Task reinvoice, future inventory reporting,
-- etc.). Without this column, expense lines were blind to the
-- ProductService catalog — the only catalog hint on an expense was
-- the GL expense account.
--
-- Nullable: existing rows stay untouched; only new entries that
-- explicitly pick a product/service carry the link.

ALTER TABLE expense_lines
    ADD COLUMN IF NOT EXISTS product_service_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_expense_lines_product_service_id
    ON expense_lines(product_service_id);
