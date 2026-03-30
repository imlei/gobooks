-- Migration 019: add void fields to reconciliations
ALTER TABLE reconciliations
    ADD COLUMN IF NOT EXISTS is_voided          BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS void_reason        TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS voided_at          TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS voided_by_user_id  UUID;
