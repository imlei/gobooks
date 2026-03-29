-- 006_product_services.sql
-- Product/Service catalogue: items that appear on invoice lines.
-- All rows are company-scoped. Intended for manual application on existing DBs.
-- Fresh databases are handled by GORM AutoMigrate in internal/db/migrate.go.

CREATE TABLE IF NOT EXISTS product_services (
    id                   BIGSERIAL    PRIMARY KEY,
    company_id           BIGINT       NOT NULL REFERENCES companies(id)      ON DELETE RESTRICT,
    name                 TEXT         NOT NULL,
    type                 TEXT         NOT NULL DEFAULT 'service',  -- service | non_inventory
    description          TEXT         NOT NULL DEFAULT '',
    default_price        NUMERIC(18,4) NOT NULL DEFAULT 0,
    revenue_account_id   BIGINT       NOT NULL REFERENCES accounts(id)       ON DELETE RESTRICT,
    default_tax_code_id  BIGINT       REFERENCES tax_codes(id)               ON DELETE SET NULL,
    is_active            BOOLEAN      NOT NULL DEFAULT true,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_product_services_company_name UNIQUE (company_id, name)
);
CREATE INDEX IF NOT EXISTS idx_product_services_company ON product_services(company_id);
CREATE INDEX IF NOT EXISTS idx_product_services_revenue ON product_services(revenue_account_id);
