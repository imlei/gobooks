CREATE TABLE IF NOT EXISTS payroll_remittances (
    id                     BIGSERIAL PRIMARY KEY,
    company_id             BIGINT NOT NULL REFERENCES companies(id),
    payroll_run_id         BIGINT NOT NULL REFERENCES payroll_runs(id),
    remittance_number      VARCHAR(64) NOT NULL DEFAULT '',
    status                 TEXT NOT NULL DEFAULT 'draft',
    period_start           TIMESTAMPTZ NOT NULL,
    period_end             TIMESTAMPTZ NOT NULL,
    due_date               TIMESTAMPTZ NOT NULL,
    payment_date           TIMESTAMPTZ,
    cpp_amount             NUMERIC(18,2) NOT NULL DEFAULT 0,
    ei_amount              NUMERIC(18,2) NOT NULL DEFAULT 0,
    tax_amount             NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_amount           NUMERIC(18,2) NOT NULL DEFAULT 0,
    bank_ledger_account_id BIGINT REFERENCES accounts(id),
    journal_entry_id       BIGINT REFERENCES journal_entries(id),
    voided_at              TIMESTAMPTZ,
    reversal_journal_entry_id BIGINT REFERENCES journal_entries(id),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payroll_remittances_run UNIQUE (payroll_run_id)
);

CREATE INDEX IF NOT EXISTS idx_payroll_remittances_company ON payroll_remittances(company_id);
CREATE INDEX IF NOT EXISTS idx_payroll_remittances_status ON payroll_remittances(status);
CREATE INDEX IF NOT EXISTS idx_payroll_remittances_due_date ON payroll_remittances(due_date);
CREATE INDEX IF NOT EXISTS idx_payroll_remittances_bank_account ON payroll_remittances(bank_ledger_account_id);
CREATE INDEX IF NOT EXISTS idx_payroll_remittances_reversal_journal_entry ON payroll_remittances(reversal_journal_entry_id);
