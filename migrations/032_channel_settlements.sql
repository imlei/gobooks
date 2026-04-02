-- Migration 032: Channel settlement raw layer.
-- Settlement/payout reports from external channels land here before any
-- GL posting. Platform-agnostic; one row per settlement period.

CREATE TABLE IF NOT EXISTS channel_settlements (
    id                       BIGSERIAL      PRIMARY KEY,
    company_id               BIGINT         NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    channel_account_id       BIGINT         NOT NULL REFERENCES sales_channel_accounts(id) ON DELETE RESTRICT,
    external_settlement_id   TEXT           NOT NULL DEFAULT '',
    settlement_date          DATE,
    currency_code            TEXT           NOT NULL DEFAULT '',
    gross_amount             NUMERIC(18,2)  NOT NULL DEFAULT 0,
    fee_amount               NUMERIC(18,2)  NOT NULL DEFAULT 0,
    net_amount               NUMERIC(18,2)  NOT NULL DEFAULT 0,
    raw_payload              JSONB          NOT NULL DEFAULT '{}',
    created_at               TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_channel_settlements_company ON channel_settlements(company_id);
CREATE INDEX IF NOT EXISTS idx_channel_settlements_account ON channel_settlements(company_id, channel_account_id);

CREATE TABLE IF NOT EXISTS channel_settlement_lines (
    id               BIGSERIAL      PRIMARY KEY,
    company_id       BIGINT         NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    settlement_id    BIGINT         NOT NULL REFERENCES channel_settlements(id) ON DELETE CASCADE,
    line_type        TEXT           NOT NULL,
    description      TEXT           NOT NULL DEFAULT '',
    external_ref     TEXT           NOT NULL DEFAULT '',
    amount           NUMERIC(18,2)  NOT NULL DEFAULT 0,
    mapped_account_id BIGINT        REFERENCES accounts(id) ON DELETE SET NULL,
    raw_payload      JSONB          NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_channel_settlement_lines_settlement ON channel_settlement_lines(settlement_id);
CREATE INDEX IF NOT EXISTS idx_channel_settlement_lines_company    ON channel_settlement_lines(company_id);
