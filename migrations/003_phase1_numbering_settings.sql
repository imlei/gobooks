-- Phase 1 — Company-scoped numbering settings (replaces file-based storage in app; table only here).
-- One row per company; rules stored as JSONB (compatible with existing numbering JSON shape).
--
-- Requires: extension pgcrypto (created in 001) for gen_random_uuid() on id.

BEGIN;

CREATE TABLE IF NOT EXISTS numbering_settings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  version INT NOT NULL DEFAULT 1,
  rules_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (company_id)
);

COMMIT;
