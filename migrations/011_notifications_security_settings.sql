-- 011_notifications_security_settings.sql
-- Notification and security configuration tables.
-- company_* tables: one row per company (UNIQUE on company_id).
-- system_* tables: singleton rows enforced by the application layer.
-- Secrets are encrypted at the application layer (AES-256-GCM) before storage;
-- only masked hints are safe to send to the browser.

-- ── company_notification_settings ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS company_notification_settings (
    id                          BIGSERIAL     PRIMARY KEY,
    company_id                  BIGINT        NOT NULL REFERENCES companies(id) ON DELETE CASCADE,

    -- Email / SMTP
    email_enabled               BOOLEAN       NOT NULL DEFAULT false,
    smtp_host                   TEXT          NOT NULL DEFAULT '',
    smtp_port                   INT           NOT NULL DEFAULT 587,
    smtp_username               TEXT          NOT NULL DEFAULT '',
    smtp_password_encrypted     TEXT          NOT NULL DEFAULT '',
    smtp_password_masked_hint   TEXT          NOT NULL DEFAULT '',
    smtp_from_email             TEXT          NOT NULL DEFAULT '',
    smtp_from_name              TEXT          NOT NULL DEFAULT '',
    smtp_encryption             TEXT          NOT NULL DEFAULT 'starttls'
                                    CHECK (smtp_encryption IN ('none', 'ssl_tls', 'starttls')),

    -- SMS
    sms_enabled                 BOOLEAN       NOT NULL DEFAULT false,
    sms_provider                TEXT          NOT NULL DEFAULT '',
    sms_api_key_encrypted       TEXT          NOT NULL DEFAULT '',
    sms_api_key_masked_hint     TEXT          NOT NULL DEFAULT '',
    sms_api_secret_encrypted    TEXT          NOT NULL DEFAULT '',
    sms_api_secret_masked_hint  TEXT          NOT NULL DEFAULT '',
    sms_sender_id               TEXT          NOT NULL DEFAULT '',

    allow_system_fallback       BOOLEAN       NOT NULL DEFAULT true,

    created_at                  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT uq_company_notification_settings UNIQUE (company_id)
);

CREATE INDEX IF NOT EXISTS idx_company_notif_settings_company
    ON company_notification_settings(company_id);

-- ── company_security_settings ─────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS company_security_settings (
    id                                  BIGSERIAL   PRIMARY KEY,
    company_id                          BIGINT      NOT NULL REFERENCES companies(id) ON DELETE CASCADE,

    unusual_ip_login_alert_enabled      BOOLEAN     NOT NULL DEFAULT true,
    unusual_ip_login_alert_channel      TEXT        NOT NULL DEFAULT 'email'
                                            CHECK (unusual_ip_login_alert_channel IN ('email', 'sms', 'both')),
    new_device_login_alert_enabled      BOOLEAN     NOT NULL DEFAULT true,
    password_reset_alert_enabled        BOOLEAN     NOT NULL DEFAULT true,
    failed_login_alert_enabled          BOOLEAN     NOT NULL DEFAULT true,
    future_rules_json                   JSONB                DEFAULT NULL,

    created_at                          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_company_security_settings UNIQUE (company_id)
);

CREATE INDEX IF NOT EXISTS idx_company_security_settings_company
    ON company_security_settings(company_id);

-- ── system_notification_settings ──────────────────────────────────────────────
-- Singleton: only one row, enforced by the application layer.
CREATE TABLE IF NOT EXISTS system_notification_settings (
    id                          BIGSERIAL     PRIMARY KEY,

    -- Email / SMTP
    email_enabled               BOOLEAN       NOT NULL DEFAULT false,
    smtp_host                   TEXT          NOT NULL DEFAULT '',
    smtp_port                   INT           NOT NULL DEFAULT 587,
    smtp_username               TEXT          NOT NULL DEFAULT '',
    smtp_password_encrypted     TEXT          NOT NULL DEFAULT '',
    smtp_password_masked_hint   TEXT          NOT NULL DEFAULT '',
    smtp_from_email             TEXT          NOT NULL DEFAULT '',
    smtp_from_name              TEXT          NOT NULL DEFAULT '',
    smtp_encryption             TEXT          NOT NULL DEFAULT 'starttls'
                                    CHECK (smtp_encryption IN ('none', 'ssl_tls', 'starttls')),

    -- SMS
    sms_enabled                 BOOLEAN       NOT NULL DEFAULT false,
    sms_provider                TEXT          NOT NULL DEFAULT '',
    sms_api_key_encrypted       TEXT          NOT NULL DEFAULT '',
    sms_api_key_masked_hint     TEXT          NOT NULL DEFAULT '',
    sms_api_secret_encrypted    TEXT          NOT NULL DEFAULT '',
    sms_api_secret_masked_hint  TEXT          NOT NULL DEFAULT '',
    sms_sender_id               TEXT          NOT NULL DEFAULT '',

    allow_company_override      BOOLEAN       NOT NULL DEFAULT true,

    created_at                  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- ── system_security_settings ──────────────────────────────────────────────────
-- Singleton: only one row, enforced by the application layer.
CREATE TABLE IF NOT EXISTS system_security_settings (
    id                                          BIGSERIAL   PRIMARY KEY,

    unusual_ip_login_alert_default_enabled      BOOLEAN     NOT NULL DEFAULT true,
    unusual_ip_login_company_override_allowed   BOOLEAN     NOT NULL DEFAULT true,
    new_device_login_alert_default_enabled      BOOLEAN     NOT NULL DEFAULT true,
    password_reset_alert_default_enabled        BOOLEAN     NOT NULL DEFAULT true,
    failed_login_alert_default_enabled          BOOLEAN     NOT NULL DEFAULT true,
    global_security_rules_json                  JSONB                DEFAULT NULL,

    created_at                                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── security_events ───────────────────────────────────────────────────────────
-- Append-only audit log for authentication and security events.
-- company_id / user_id use SET NULL on delete to preserve the audit record.
-- user_id is stored as text to accommodate both regular users (UUID) and
-- sysadmin users without a cross-table foreign key.
CREATE TABLE IF NOT EXISTS security_events (
    id            BIGSERIAL   PRIMARY KEY,
    company_id    BIGINT                  REFERENCES companies(id) ON DELETE SET NULL,
    user_id       TEXT,
    event_type    TEXT        NOT NULL,
    ip_address    TEXT,
    user_agent    TEXT,
    metadata_json JSONB                   DEFAULT NULL,
    created_at    TIMESTAMPTZ NOT NULL    DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_events_company ON security_events(company_id);
CREATE INDEX IF NOT EXISTS idx_security_events_user    ON security_events(user_id);
CREATE INDEX IF NOT EXISTS idx_security_events_type    ON security_events(event_type);
CREATE INDEX IF NOT EXISTS idx_security_events_created ON security_events(created_at);
