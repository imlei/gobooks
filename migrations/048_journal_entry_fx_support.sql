-- Migration 048: journal entry FX snapshot support.
--
-- Adds immutable transaction-currency and FX snapshot fields to journal_entries,
-- plus transaction-currency source amounts to journal_lines.
--
-- Backfill policy for legacy rows:
--   - transaction_currency_code  -> company base currency
--   - exchange_rate              -> 1
--   - exchange_rate_date         -> entry_date
--   - exchange_rate_source       -> identity
--   - tx_debit / tx_credit       -> existing base debit / credit

ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS transaction_currency_code TEXT NOT NULL DEFAULT '';
ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(20,8) NOT NULL DEFAULT 1;
ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS exchange_rate_date DATE;
ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS exchange_rate_source TEXT NOT NULL DEFAULT 'identity';

ALTER TABLE journal_lines ADD COLUMN IF NOT EXISTS tx_debit NUMERIC(18,2) NOT NULL DEFAULT 0;
ALTER TABLE journal_lines ADD COLUMN IF NOT EXISTS tx_credit NUMERIC(18,2) NOT NULL DEFAULT 0;

UPDATE journal_entries
SET transaction_currency_code = COALESCE(
    NULLIF(transaction_currency_code, ''),
    (
        SELECT COALESCE(NULLIF(base_currency_code, ''), 'CAD')
        FROM companies
        WHERE companies.id = journal_entries.company_id
    )
)
WHERE COALESCE(transaction_currency_code, '') = '';

UPDATE journal_entries
SET exchange_rate = 1
WHERE exchange_rate IS NULL OR exchange_rate = 0;

UPDATE journal_entries
SET exchange_rate_date = COALESCE(exchange_rate_date, entry_date)
WHERE exchange_rate_date IS NULL;

UPDATE journal_entries
SET exchange_rate_source = 'identity'
WHERE COALESCE(exchange_rate_source, '') = '';

UPDATE journal_lines
SET tx_debit = debit
WHERE (tx_debit IS NULL OR tx_debit = 0) AND debit <> 0;

UPDATE journal_lines
SET tx_credit = credit
WHERE (tx_credit IS NULL OR tx_credit = 0) AND credit <> 0;
