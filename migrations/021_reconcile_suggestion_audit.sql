-- Migration 021: extend reconciliation match engine tables with full audit trail
-- and lifecycle fields missing from the initial 020 schema.
--
-- What this adds:
--   reconciliation_match_suggestions:
--     • reconciliation_id        — set when Finish Now links accepted suggestions
--                                  to the completed reconciliation record
--     • accepted_by_user_id/at   — who accepted and when (separate from reject)
--     • rejected_by_user_id/at   — who rejected and when (separate from accept)
--     • updated_at               — GORM auto-managed update timestamp
--   reconciliation_match_suggestion_lines:
--     • company_id               — multi-company isolation at the line level
--     • amount_applied           — portion of line amount used (full for 1:1, partial for split)
--     • role                     — 'match' | 'split' | 'context'
--   reconciliation_memory:
--     • updated_at               — for future confidence decay / maintenance

ALTER TABLE reconciliation_match_suggestions
    ADD COLUMN IF NOT EXISTS reconciliation_id      INTEGER,
    ADD COLUMN IF NOT EXISTS accepted_by_user_id    UUID,
    ADD COLUMN IF NOT EXISTS accepted_at            TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS rejected_by_user_id    UUID,
    ADD COLUMN IF NOT EXISTS rejected_at            TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Efficiently find all suggestions that belong to a completed reconciliation
-- (needed for archive-on-void).
CREATE INDEX IF NOT EXISTS idx_recon_suggestions_reconciliation_id
    ON reconciliation_match_suggestions (reconciliation_id)
    WHERE reconciliation_id IS NOT NULL;

ALTER TABLE reconciliation_match_suggestion_lines
    ADD COLUMN IF NOT EXISTS company_id     INTEGER,
    ADD COLUMN IF NOT EXISTS amount_applied NUMERIC(18,2),
    ADD COLUMN IF NOT EXISTS role           TEXT NOT NULL DEFAULT 'match';

ALTER TABLE reconciliation_memory
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
