-- Phase 2: user verification challenges for email/password change flows.
-- Idempotent: uses IF NOT EXISTS throughout.

CREATE TABLE IF NOT EXISTS user_verification_challenges (
    id            UUID        PRIMARY KEY,
    user_id       UUID        NOT NULL,
    type          TEXT        NOT NULL,
    code_hash     TEXT        NOT NULL,
    new_email     TEXT        NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ NOT NULL,
    attempt_count INTEGER     NOT NULL DEFAULT 0,
    max_attempts  INTEGER     NOT NULL DEFAULT 5,
    used_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_verification_challenges_user_id
    ON user_verification_challenges (user_id);

CREATE INDEX IF NOT EXISTS idx_user_verification_challenges_expires_at
    ON user_verification_challenges (expires_at);
