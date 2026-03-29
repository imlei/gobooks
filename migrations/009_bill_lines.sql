-- 009_bill_lines.sql
-- Extend bills with status/totals/JE link; add bill_lines for line-item purchasing.
-- All tables are company-scoped. Intended for manual application on existing DBs.
-- Fresh databases are handled by GORM AutoMigrate in internal/db/migrate.go.

-- ── Extend bills header ──────────────────────────────────────────────────────
-- Existing bills retain their lump-sum amount; subtotal/tax_total default to 0.
-- Status defaults to 'draft'; existing bills are treated as unposted drafts.

ALTER TABLE bills
    ADD COLUMN IF NOT EXISTS status            TEXT          NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS subtotal          NUMERIC(18,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_total         NUMERIC(18,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS journal_entry_id  BIGINT        REFERENCES journal_entries(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now();

-- Backfill: existing bills with a lump-sum amount have subtotal = amount.
UPDATE bills SET subtotal = amount WHERE subtotal = 0 AND amount > 0;

-- ── bill_lines ───────────────────────────────────────────────────────────────
--
-- expense_account_id: required for posting — the GL account debited for this
--   line's cost (base net + non-recoverable tax).
-- tax_code_id: optional; the TaxCode must have scope 'purchase' or 'both'.
--   Recoverable tax is posted to tax_code.purchase_recoverable_account_id.
--   Non-recoverable tax is rolled into the expense_account_id debit.

CREATE TABLE IF NOT EXISTS bill_lines (
    id                   BIGSERIAL     PRIMARY KEY,
    company_id           BIGINT        NOT NULL REFERENCES companies(id)          ON DELETE RESTRICT,
    bill_id              BIGINT        NOT NULL REFERENCES bills(id)              ON DELETE CASCADE,
    sort_order           INT           NOT NULL DEFAULT 1,
    product_service_id   BIGINT                 REFERENCES product_services(id)   ON DELETE SET NULL,
    description          TEXT          NOT NULL,
    qty                  NUMERIC(10,4) NOT NULL DEFAULT 1,
    unit_price           NUMERIC(18,4) NOT NULL DEFAULT 0,
    tax_code_id          BIGINT                 REFERENCES tax_codes(id)          ON DELETE SET NULL,
    expense_account_id   BIGINT                 REFERENCES accounts(id)           ON DELETE SET NULL,
    line_net             NUMERIC(18,2) NOT NULL DEFAULT 0,
    line_tax             NUMERIC(18,2) NOT NULL DEFAULT 0,
    line_total           NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bill_lines_company   ON bill_lines(company_id);
CREATE INDEX IF NOT EXISTS idx_bill_lines_bill      ON bill_lines(bill_id);
CREATE INDEX IF NOT EXISTS idx_bill_lines_product   ON bill_lines(product_service_id);
CREATE INDEX IF NOT EXISTS idx_bill_lines_tax_code  ON bill_lines(tax_code_id);
CREATE INDEX IF NOT EXISTS idx_bill_lines_expense   ON bill_lines(expense_account_id);
