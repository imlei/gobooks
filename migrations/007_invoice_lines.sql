-- 007_invoice_lines.sql
-- Invoice model refactor: add status/terms/due_date/totals to invoices header;
-- create invoice_lines table for line-item accounting.
-- All rows are company-scoped. Intended for manual application on existing DBs.
-- Fresh databases are handled by GORM AutoMigrate in internal/db/migrate.go.

-- ── Extend invoices header ──────────────────────────────────────────────────

ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS status          TEXT         NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS terms           TEXT         NOT NULL DEFAULT 'net_30',
    ADD COLUMN IF NOT EXISTS due_date        TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS subtotal        NUMERIC(18,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_total       NUMERIC(18,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS journal_entry_id BIGINT REFERENCES journal_entries(id) ON DELETE SET NULL;

-- Backfill: existing invoices are draft with their lump-sum amount as subtotal.
-- tax_total stays 0 (no breakdown available for legacy rows).
UPDATE invoices SET subtotal = amount WHERE subtotal = 0 AND amount > 0;

-- ── invoice_lines ───────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS invoice_lines (
    id                   BIGSERIAL     PRIMARY KEY,
    company_id           BIGINT        NOT NULL REFERENCES companies(id)          ON DELETE RESTRICT,
    invoice_id           BIGINT        NOT NULL REFERENCES invoices(id)           ON DELETE CASCADE,
    sort_order           INT           NOT NULL DEFAULT 1,
    product_service_id   BIGINT                 REFERENCES product_services(id)   ON DELETE SET NULL,
    description          TEXT          NOT NULL,
    qty                  NUMERIC(10,4) NOT NULL DEFAULT 1,
    unit_price           NUMERIC(18,4) NOT NULL DEFAULT 0,
    tax_code_id          BIGINT                 REFERENCES tax_codes(id)          ON DELETE SET NULL,
    line_net             NUMERIC(18,2) NOT NULL DEFAULT 0,
    line_tax             NUMERIC(18,2) NOT NULL DEFAULT 0,
    line_total           NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invoice_lines_company   ON invoice_lines(company_id);
CREATE INDEX IF NOT EXISTS idx_invoice_lines_invoice   ON invoice_lines(invoice_id);
CREATE INDEX IF NOT EXISTS idx_invoice_lines_product   ON invoice_lines(product_service_id);
