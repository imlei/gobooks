ALTER TABLE payroll_remittances
    ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ;

ALTER TABLE payroll_remittances
    ADD COLUMN IF NOT EXISTS reversal_journal_entry_id BIGINT REFERENCES journal_entries(id);

CREATE INDEX IF NOT EXISTS idx_payroll_remittances_reversal_journal_entry
    ON payroll_remittances(reversal_journal_entry_id);
