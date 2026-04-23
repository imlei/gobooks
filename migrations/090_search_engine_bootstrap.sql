-- 090_search_engine_bootstrap.sql
-- Phase 0 of the Sales Transactions / Global Search project.
--
-- Why
-- ---
-- The current SmartPicker (`/api/smart-picker/search`) fan-outs to per-entity
-- providers (customer / vendor / product_service / account / payment_account).
-- That works at SmartPicker's original scale — single-entity row pickers in
-- transaction editors — but doesn't compose into a unified search surface
-- ("global search"), and as more entity types are added the per-provider
-- pattern duplicates ranking/recency/context logic.
--
-- The plan is to introduce a denormalized projection table (search_documents)
-- that every entity domain feeds into; SmartPicker then becomes a single
-- query over this table with grouped results. ent owns this subdomain only —
-- GORM continues to own all canonical business writes.
--
-- Phase 0 (this migration + accompanying ent schemas + skeleton packages)
-- introduces ONLY the storage + interface scaffolding. Zero user-visible
-- behaviour changes; the projector / dual-run / engine swap arrive in
-- subsequent phases.
--
-- What this migration does
-- ------------------------
--   1. Enables the `pg_trgm` extension (substring + fuzzy matching).
--   2. Creates `search_documents`     — main projection table.
--   3. Creates `search_recent_queries` — per-user query history.
--   4. Creates `search_usage_stats`    — per-(company, entity) click counter.
--   5. Adds GIN trigram indexes for substring/fuzzy search on the title fields.
--   6. Adds a `search_tsv` generated column + GIN index for fulltext search.
--      Uses `simple` config (no stemming) — multi-language safe; per-language
--      stemming can be layered in later without a column rewrite.
--
-- Idempotent — safe to re-run.

-- ── Extension ─────────────────────────────────────────────────────────────
-- pg_trgm enables `gin_trgm_ops` GIN indexes for fast ILIKE / similarity
-- queries on the normalized text fields. RDS / supabase / managed Postgres
-- all support this extension by default.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ── search_documents — the main projection table ───────────────────────
CREATE TABLE IF NOT EXISTS search_documents (
    id                 BIGSERIAL PRIMARY KEY,
    company_id         BIGINT       NOT NULL,
    entity_type        VARCHAR(32)  NOT NULL,
    entity_id          BIGINT       NOT NULL,
    doc_number         VARCHAR(64)  NOT NULL DEFAULT '',
    title              TEXT         NOT NULL,
    subtitle           TEXT         NOT NULL DEFAULT '',
    -- Normalized fields populated by searchprojection.Normalizer.
    title_native       TEXT         NOT NULL,
    title_latin        TEXT         NOT NULL DEFAULT '',
    title_initials     TEXT         NOT NULL DEFAULT '',
    memo_native        TEXT         NOT NULL DEFAULT '',
    -- Display / ranking signals.
    doc_date           TIMESTAMPTZ  NULL,
    amount             VARCHAR(32)  NOT NULL DEFAULT '',
    currency           VARCHAR(3)   NOT NULL DEFAULT '',
    status             VARCHAR(32)  NOT NULL DEFAULT '',
    -- Routing.
    url_path           VARCHAR(255) NOT NULL,
    -- Housekeeping.
    projector_version  INT          NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Idempotency key — projector upserts on this triple.
CREATE UNIQUE INDEX IF NOT EXISTS search_documents_company_entity_idx
    ON search_documents (company_id, entity_type, entity_id);

-- Recency-ordered list (Phase 4 advanced search default ordering).
CREATE INDEX IF NOT EXISTS search_documents_company_doc_date_idx
    ON search_documents (company_id, doc_date DESC NULLS LAST);

-- Exact-code lookup (first-tier match: "INV-202604" → 1 row).
CREATE INDEX IF NOT EXISTS search_documents_company_type_number_idx
    ON search_documents (company_id, entity_type, doc_number);

-- Status filter (e.g. exclude voided in default view).
CREATE INDEX IF NOT EXISTS search_documents_company_type_status_idx
    ON search_documents (company_id, entity_type, status);

-- Trigram GIN indexes for substring + similarity matching on the
-- normalized text fields. These are the hot path for "type 'Li' →
-- match Lighting Geek + Liu + Li Hu" style queries.
CREATE INDEX IF NOT EXISTS search_documents_title_native_trgm_idx
    ON search_documents USING gin (title_native gin_trgm_ops);

CREATE INDEX IF NOT EXISTS search_documents_title_latin_trgm_idx
    ON search_documents USING gin (title_latin gin_trgm_ops);

CREATE INDEX IF NOT EXISTS search_documents_title_initials_trgm_idx
    ON search_documents USING gin (title_initials gin_trgm_ops);

CREATE INDEX IF NOT EXISTS search_documents_memo_native_trgm_idx
    ON search_documents USING gin (memo_native gin_trgm_ops);

-- Fulltext search column. Generated from title + subtitle + memo using
-- the `simple` text-search config (no stemming, no stopwords) — safe
-- for any language and good enough for prefix/word-boundary matching.
-- Per-language stemming can be added in a later migration without a
-- table rewrite (just ALTER COLUMN expression).
ALTER TABLE search_documents
    ADD COLUMN IF NOT EXISTS search_tsv tsvector
    GENERATED ALWAYS AS (
        to_tsvector('simple',
            coalesce(title, '') || ' ' ||
            coalesce(subtitle, '') || ' ' ||
            coalesce(memo_native, '')
        )
    ) STORED;

CREATE INDEX IF NOT EXISTS search_documents_search_tsv_idx
    ON search_documents USING gin (search_tsv);

-- ── search_recent_queries — user search history ─────────────────────────
-- Powers the "Recent searches" section of the empty-query dropdown state.
-- Trim job (~50 most recent per user/company) is async; no DB constraint.
CREATE TABLE IF NOT EXISTS search_recent_queries (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID         NOT NULL,
    company_id  BIGINT       NOT NULL,
    query       TEXT         NOT NULL,
    queried_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS search_recent_queries_user_company_time_idx
    ON search_recent_queries (user_id, company_id, queried_at DESC);

-- ── search_usage_stats — click-through ranking signal ───────────────────
-- Per-(company, entity_type, entity_id) click counter. Bumped on dropdown
-- selection. Read by the ranker as a recency + popularity tie-breaker.
CREATE TABLE IF NOT EXISTS search_usage_stats (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT       NOT NULL,
    entity_type     VARCHAR(32)  NOT NULL,
    entity_id       BIGINT       NOT NULL,
    click_count     INT          NOT NULL DEFAULT 0,
    last_clicked_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS search_usage_stats_company_entity_idx
    ON search_usage_stats (company_id, entity_type, entity_id);

CREATE INDEX IF NOT EXISTS search_usage_stats_company_type_idx
    ON search_usage_stats (company_id, entity_type);
