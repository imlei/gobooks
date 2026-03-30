-- 015_notification_readiness.sql
-- Adds delivery readiness tracking columns to company_notification_settings
-- and system_notification_settings.
--
-- All columns use IF NOT EXISTS and safe defaults so this migration is
-- idempotent and zero-risk on existing data.
--
-- Readiness rule (enforced by the service layer):
--   *_verification_ready = *_enabled
--     AND config is complete
--     AND *_test_status = 'success'
--     AND *_config_hash = *_tested_config_hash   (config unchanged since last success)

-- ── company_notification_settings ─────────────────────────────────────────────

ALTER TABLE company_notification_settings
    ADD COLUMN IF NOT EXISTS email_test_status       TEXT        NOT NULL DEFAULT 'never'
        CHECK (email_test_status IN ('never', 'success', 'failed')),
    ADD COLUMN IF NOT EXISTS email_last_tested_at    TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS email_last_tested_by    TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_last_success_at   TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS email_last_failure_at   TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS email_last_error        TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_config_hash       TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_tested_config_hash TEXT       NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_verification_ready BOOLEAN    NOT NULL DEFAULT false,

    ADD COLUMN IF NOT EXISTS sms_test_status         TEXT        NOT NULL DEFAULT 'never'
        CHECK (sms_test_status IN ('never', 'success', 'failed')),
    ADD COLUMN IF NOT EXISTS sms_last_tested_at      TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS sms_last_tested_by      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_last_success_at     TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS sms_last_failure_at     TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS sms_last_error          TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_config_hash         TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_tested_config_hash  TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_verification_ready  BOOLEAN     NOT NULL DEFAULT false;

-- ── system_notification_settings ──────────────────────────────────────────────

ALTER TABLE system_notification_settings
    ADD COLUMN IF NOT EXISTS email_test_status       TEXT        NOT NULL DEFAULT 'never'
        CHECK (email_test_status IN ('never', 'success', 'failed')),
    ADD COLUMN IF NOT EXISTS email_last_tested_at    TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS email_last_tested_by    TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_last_success_at   TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS email_last_failure_at   TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS email_last_error        TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_config_hash       TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_tested_config_hash TEXT       NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_verification_ready BOOLEAN    NOT NULL DEFAULT false,

    ADD COLUMN IF NOT EXISTS sms_test_status         TEXT        NOT NULL DEFAULT 'never'
        CHECK (sms_test_status IN ('never', 'success', 'failed')),
    ADD COLUMN IF NOT EXISTS sms_last_tested_at      TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS sms_last_tested_by      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_last_success_at     TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS sms_last_failure_at     TIMESTAMPTZ          DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS sms_last_error          TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_config_hash         TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_tested_config_hash  TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sms_verification_ready  BOOLEAN     NOT NULL DEFAULT false;
