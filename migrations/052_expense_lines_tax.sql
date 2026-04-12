-- Migration 052: Per-line sales tax for expense lines.
--
-- Adds tax_code_id, line_tax, and line_total to expense_lines so each
-- cost-category row can carry an optional tax code (purchase-scope).
-- line_total = amount (net) + line_tax.
-- Expense.amount will now reflect the grand total (net + tax).
--
-- Backfill: existing lines have no tax so line_total is backfilled to amount.

ALTER TABLE expense_lines ADD COLUMN IF NOT EXISTS tax_code_id BIGINT REFERENCES tax_codes(id);
ALTER TABLE expense_lines ADD COLUMN IF NOT EXISTS line_tax   NUMERIC(18,2) NOT NULL DEFAULT 0;
ALTER TABLE expense_lines ADD COLUMN IF NOT EXISTS line_total NUMERIC(18,2) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_expense_lines_tax_code_id ON expense_lines(tax_code_id);

-- Backfill line_total from amount for rows with no tax.
UPDATE expense_lines SET line_total = amount WHERE line_total = 0;
