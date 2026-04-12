-- Migration 049: exchange-rate snapshots become append-only.
--
-- Journal-entry save-time validation now binds to an exact stored snapshot row.
-- To keep previously-shown snapshots valid even after a newer same-day refresh,
-- exchange_rates can no longer enforce one mutable row per scope/day.

DROP INDEX IF EXISTS uq_exchange_rates_system;
DROP INDEX IF EXISTS uq_exchange_rates_company;

CREATE INDEX IF NOT EXISTS idx_exchange_rates_system_lookup
  ON exchange_rates (base_currency_code, target_currency_code, rate_type, effective_date DESC, id DESC)
  WHERE company_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_exchange_rates_company_lookup
  ON exchange_rates (company_id, base_currency_code, target_currency_code, rate_type, effective_date DESC, id DESC)
  WHERE company_id IS NOT NULL;
