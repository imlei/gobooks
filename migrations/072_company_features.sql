-- 072_company_features.sql
-- Company-scoped feature enablement registry.
--
-- One row per (company, feature) when the company has ever interacted
-- with that feature (enabled or disabled at least once). Companies
-- with no row for a feature are implicitly in the "off" state — absence
-- of a row means "never enabled."
--
-- What this table IS:
--   - Self-serve feature toggle persistence for owner-driven enablement
--     (e.g. Inventory Alpha).
--   - Audit evidence of the ack / reason / typed confirmation captured
--     at enable time.
--
-- What this table IS NOT:
--   - Not a global feature flag registry; keys here are per-company.
--   - Not a replacement for the Phase H capability rails
--     (companies.tracking_enabled, companies.receipt_required) — those
--     are lower-level gates owned by the inventory module. This table
--     sits ONE level above: controls whether a product feature family
--     (Inventory Alpha, Task, …) is available to the company's UI and
--     API surface at all.
--
-- Feature registry policy:
--   - `feature_key` values are enumerated app-side in
--     `internal/models/company_feature.go` (FeatureKey*). A row is only
--     meaningful when its feature_key matches a registered key.
--   - `maturity` is a static property of the feature (alpha / beta / ga
--     / coming_soon), persisted here so historical audit rows preserve
--     the maturity the feature had at enable time.
--   - `status` is the current effective state: 'off' or 'enabled'. A
--     disabled row stays with status='off' and retains enabled_at /
--     enabled_by as history of the prior enablement.
--
-- Acknowledgement audit:
--   - `ack_version` is the identifier of the specific acknowledgement
--     text + checkbox set the user agreed to at enable time. If the
--     prompt later evolves, a new ack_version is used and the old
--     enablements are still attributable to the old version.
--   - `reason_code` / `reason_note` capture why the owner opted in.
--     Free-form note is optional; code is one of a small enumerated
--     set (app-side constants).

CREATE TABLE IF NOT EXISTS company_features (
    id                       BIGSERIAL PRIMARY KEY,
    company_id               BIGINT       NOT NULL,
    feature_key              TEXT         NOT NULL,
    status                   TEXT         NOT NULL DEFAULT 'off',
    maturity                 TEXT         NOT NULL DEFAULT 'alpha',
    enabled_at               TIMESTAMPTZ,
    enabled_by_user_id       UUID,
    acknowledged_at          TIMESTAMPTZ,
    ack_version              TEXT         NOT NULL DEFAULT '',
    reason_code              TEXT         NOT NULL DEFAULT '',
    reason_note              TEXT         NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ,
    updated_at               TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_company_features_company_feature
    ON company_features(company_id, feature_key);

CREATE INDEX IF NOT EXISTS idx_company_features_company_status
    ON company_features(company_id, status);
