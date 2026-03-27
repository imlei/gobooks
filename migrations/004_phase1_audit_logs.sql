-- Phase 1 — Extend audit_logs for company + real user actor + before/after payloads.
-- Assumptions (GORM default column names): action, entity_type, entity_id, actor, details_json, created_at
-- Keeps legacy `actor` / `details_json` for backward compatibility until app is updated.
--
-- Requires: 001 applied first (users.id for fk_audit_logs_actor_user).
-- Backfill: company_id = MIN(companies.id) for existing rows (single-tenant); actor_label = legacy-import
-- where NULL (large tables: UPDATE may take time).

BEGIN;

ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS company_id BIGINT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS actor_user_id UUID;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS actor_label TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS before_json JSONB;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS after_json JSONB;

-- Optional FK: user may be deleted; keep audit row
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS fk_audit_logs_actor_user;
ALTER TABLE audit_logs
  ADD CONSTRAINT fk_audit_logs_actor_user
  FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS fk_audit_logs_company;
ALTER TABLE audit_logs
  ADD CONSTRAINT fk_audit_logs_company
  FOREIGN KEY (company_id) REFERENCES companies(id) ON DELETE SET NULL;

-- Backfill company_id for existing rows (single-tenant Phase 1)
DO $$
DECLARE
  cid BIGINT;
BEGIN
  SELECT id INTO cid FROM companies ORDER BY id ASC LIMIT 1;
  IF cid IS NULL THEN
    RETURN;
  END IF;
  UPDATE audit_logs SET company_id = cid WHERE company_id IS NULL;
END $$;

-- Legacy rows: distinguish from future user-driven actions
UPDATE audit_logs
SET actor_label = COALESCE(NULLIF(actor_label, ''), 'legacy-import')
WHERE actor_label IS NULL;

CREATE INDEX IF NOT EXISTS idx_audit_logs_company_created_at ON audit_logs (company_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_user_id ON audit_logs (actor_user_id);

COMMIT;
