-- 008_tax_code_redesign.sql
-- Replace the component-based tax_codes table with a flat-rate design.
-- The bridge table tax_code_components is no longer needed.
--
-- Fresh databases are handled by GORM AutoMigrate in internal/db/migrate.go.
-- Apply this manually on existing databases.

-- Drop bridge table first (references tax_codes).
DROP TABLE IF EXISTS tax_code_components CASCADE;

-- Drop old tax_codes (was: id, company_id, code, name, tax_type, is_active).
DROP TABLE IF EXISTS tax_codes CASCADE;

-- Recreate tax_codes with flat-rate design.
--
-- Columns:
--   name                            — user-visible label (e.g. "GST 5%", "Exempt")
--   rate                            — flat tax rate as a fraction (e.g. 0.050000 = 5%)
--   scope                           — which direction: sales | purchase | both
--   recovery_mode                   — ITC recoverability: full | partial | none
--   recovery_rate                   — percentage of tax that is recoverable (0–100);
--                                     only meaningful when recovery_mode = partial
--   sales_tax_account_id            — GL liability account credited on sales
--                                     (e.g. "GST/HST Payable")
--   purchase_recoverable_account_id — GL asset/receivable account debited for
--                                     recoverable purchase tax (ITC Receivable);
--                                     NULL when recovery_mode = none or scope = sales
CREATE TABLE tax_codes (
    id                              BIGSERIAL       PRIMARY KEY,
    company_id                      BIGINT          NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    name                            TEXT            NOT NULL,
    rate                            NUMERIC(8,6)    NOT NULL,
    scope                           TEXT            NOT NULL DEFAULT 'both',
    recovery_mode                   TEXT            NOT NULL DEFAULT 'none',
    recovery_rate                   NUMERIC(5,2)    NOT NULL DEFAULT 0,
    sales_tax_account_id            BIGINT          NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    purchase_recoverable_account_id BIGINT          REFERENCES accounts(id) ON DELETE RESTRICT,
    is_active                       BOOLEAN         NOT NULL DEFAULT true,
    created_at                      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT chk_tax_codes_scope
        CHECK (scope IN ('sales', 'purchase', 'both')),
    CONSTRAINT chk_tax_codes_recovery_mode
        CHECK (recovery_mode IN ('full', 'partial', 'none')),
    CONSTRAINT chk_tax_codes_recovery_rate
        CHECK (recovery_rate >= 0 AND recovery_rate <= 100),
    CONSTRAINT chk_tax_codes_rate
        CHECK (rate >= 0)
);

-- Composite index used by every company-scoped query + active filter.
CREATE INDEX idx_tax_codes_company_active
    ON tax_codes(company_id, is_active);

-- FK indexes for join performance.
CREATE INDEX idx_tax_codes_sales_account
    ON tax_codes(sales_tax_account_id);

CREATE INDEX idx_tax_codes_purchase_account
    ON tax_codes(purchase_recoverable_account_id)
    WHERE purchase_recoverable_account_id IS NOT NULL;
