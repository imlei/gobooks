-- 005_tax_engine.sql
-- Tax infrastructure: agencies, components, codes, code_components.
-- All tables are company-scoped. Intended for manual application on existing DBs.
-- Fresh databases are handled by GORM AutoMigrate in internal/db/migrate.go.

-- Tax agencies (CRA, Revenu Québec, provincial ministries).
CREATE TABLE IF NOT EXISTS tax_agencies (
    id         BIGSERIAL   PRIMARY KEY,
    company_id BIGINT      NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    name       TEXT        NOT NULL,
    short_code TEXT        NOT NULL,
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tax_agencies_company_code UNIQUE (company_id, short_code)
);
CREATE INDEX IF NOT EXISTS idx_tax_agencies_company ON tax_agencies(company_id);

-- Tax components: individual rate elements (GST 5%, ON-HST 13%, QST 9.975%, BC-PST 7%).
-- Each component maps to one liability account; one JE credit line per component when posting.
-- tax_family: gst | hst | qst | pst | rst
-- province: empty = federal (GST/HST), province code = provincial (QST/PST/RST).
CREATE TABLE IF NOT EXISTS tax_components (
    id                   BIGSERIAL    PRIMARY KEY,
    company_id           BIGINT       NOT NULL REFERENCES companies(id)  ON DELETE RESTRICT,
    name                 TEXT         NOT NULL,
    tax_family           TEXT         NOT NULL,
    tax_agency_id        BIGINT       NOT NULL REFERENCES tax_agencies(id) ON DELETE RESTRICT,
    rate                 NUMERIC(8,6) NOT NULL,
    liability_account_id BIGINT       NOT NULL REFERENCES accounts(id)   ON DELETE RESTRICT,
    province             TEXT         NOT NULL DEFAULT '',
    is_recoverable       BOOLEAN      NOT NULL DEFAULT true,
    is_active            BOOLEAN      NOT NULL DEFAULT true,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_tax_components_company_name UNIQUE (company_id, name)
);
CREATE INDEX IF NOT EXISTS idx_tax_components_company ON tax_components(company_id);

-- Tax codes: user-visible codes assigned to invoice lines.
-- tax_type: taxable | exempt | zero_rated | out_of_scope
-- Non-taxable codes produce no tax amounts; no JE tax lines are generated.
CREATE TABLE IF NOT EXISTS tax_codes (
    id         BIGSERIAL   PRIMARY KEY,
    company_id BIGINT      NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    code       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    tax_type   TEXT        NOT NULL DEFAULT 'taxable',
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tax_codes_company_code UNIQUE (company_id, code)
);
CREATE INDEX IF NOT EXISTS idx_tax_codes_company ON tax_codes(company_id);

-- Bridge: which components belong to each tax code.
-- One code can map to multiple components (e.g. QC-GST+QST → GST component + QST component).
-- ON DELETE CASCADE: removing a tax code removes its component links.
CREATE TABLE IF NOT EXISTS tax_code_components (
    id               BIGSERIAL   PRIMARY KEY,
    tax_code_id      BIGINT      NOT NULL REFERENCES tax_codes(id)      ON DELETE CASCADE,
    tax_component_id BIGINT      NOT NULL REFERENCES tax_components(id) ON DELETE RESTRICT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tax_code_components UNIQUE (tax_code_id, tax_component_id)
);
CREATE INDEX IF NOT EXISTS idx_tax_code_components_code ON tax_code_components(tax_code_id);
