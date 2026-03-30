-- Migration 020: AI-assisted reconciliation match engine
-- Three tables: suggestion headers, suggestion lines (book side), and memory layer.

CREATE TABLE IF NOT EXISTS reconciliation_match_suggestions (
    id                  BIGSERIAL   PRIMARY KEY,
    company_id          INTEGER     NOT NULL,
    account_id          INTEGER     NOT NULL,
    suggestion_type     TEXT        NOT NULL DEFAULT 'one_to_one',
    status              TEXT        NOT NULL DEFAULT 'pending',
    confidence_score    NUMERIC(5,4) NOT NULL DEFAULT 0,
    ranking_score       NUMERIC(10,4) NOT NULL DEFAULT 0,
    explanation_json    TEXT        NOT NULL DEFAULT '{}',
    generated_by        TEXT        NOT NULL DEFAULT 'engine_v1',
    generated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at         TIMESTAMPTZ,
    reviewed_by_user_id UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recon_suggestions_company_account_status
    ON reconciliation_match_suggestions (company_id, account_id, status);

-- Each suggestion references one or more book-side journal lines.
CREATE TABLE IF NOT EXISTS reconciliation_match_suggestion_lines (
    id              BIGSERIAL   PRIMARY KEY,
    suggestion_id   BIGINT      NOT NULL REFERENCES reconciliation_match_suggestions(id) ON DELETE CASCADE,
    journal_line_id INTEGER     NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recon_suggestion_lines_suggestion
    ON reconciliation_match_suggestion_lines (suggestion_id);

-- Memory layer: learns from accepted suggestions to improve future scoring.
-- Keyed on (company, account, normalized_book_memo, source_type) — unique per pattern.
CREATE TABLE IF NOT EXISTS reconciliation_memory (
    id                      BIGSERIAL    PRIMARY KEY,
    company_id              INTEGER      NOT NULL,
    account_id              INTEGER      NOT NULL,
    normalized_book_memo    TEXT         NOT NULL DEFAULT '',
    source_type             TEXT         NOT NULL DEFAULT '',
    vendor_id               INTEGER,
    customer_id             INTEGER,
    matched_count           INTEGER      NOT NULL DEFAULT 1,
    last_matched_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    confidence_boost        NUMERIC(5,4) NOT NULL DEFAULT 0.0500,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_recon_memory_pattern
        UNIQUE (company_id, account_id, normalized_book_memo, source_type)
);

CREATE INDEX IF NOT EXISTS idx_recon_memory_company_account
    ON reconciliation_memory (company_id, account_id);
