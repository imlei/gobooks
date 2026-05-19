ALTER TABLE cheque_bank_accounts
    ADD COLUMN IF NOT EXISTS ledger_account_id BIGINT REFERENCES accounts(id);

CREATE INDEX IF NOT EXISTS idx_cheque_bank_accounts_ledger_account
    ON cheque_bank_accounts(ledger_account_id);
