-- Employee / Payroll / Cheque foundation for the SimpleTask merge.
-- Balanciz remains the source architecture: company-scoped tables, uint IDs,
-- PostgreSQL numeric money fields, and sensitive fields kept out of search.

CREATE TABLE IF NOT EXISTS employees (
    id                      BIGSERIAL PRIMARY KEY,
    company_id              BIGINT NOT NULL REFERENCES companies(id),
    employee_no             VARCHAR(64) NOT NULL DEFAULT '',
    legal_name              TEXT NOT NULL DEFAULT '',
    display_name            TEXT NOT NULL DEFAULT '',
    email                   TEXT NOT NULL DEFAULT '',
    mobile                  TEXT NOT NULL DEFAULT '',
    position                TEXT NOT NULL DEFAULT '',
    notes                   TEXT NOT NULL DEFAULT '',
    addr_street1            TEXT NOT NULL DEFAULT '',
    addr_street2            TEXT NOT NULL DEFAULT '',
    addr_city               TEXT NOT NULL DEFAULT '',
    addr_province           TEXT NOT NULL DEFAULT '',
    addr_postal_code        TEXT NOT NULL DEFAULT '',
    addr_country            TEXT NOT NULL DEFAULT 'CA',
    province_of_employment  VARCHAR(16) NOT NULL DEFAULT '',
    sin_ciphertext          TEXT NOT NULL DEFAULT '',
    sin_last4               VARCHAR(4) NOT NULL DEFAULT '',
    date_of_birth           TIMESTAMPTZ,
    hire_date               TIMESTAMPTZ,
    termination_date        TIMESTAMPTZ,
    member_type             TEXT NOT NULL DEFAULT 'employee',
    salary_type             TEXT NOT NULL DEFAULT 'time_based',
    status                  TEXT NOT NULL DEFAULT 'active',
    pay_rate                NUMERIC(18,6) NOT NULL DEFAULT 0,
    pay_rate_unit           TEXT NOT NULL DEFAULT 'hourly',
    pays_per_year           INTEGER NOT NULL DEFAULT 26,
    pay_frequency           TEXT NOT NULL DEFAULT 'biweekly',
    hours_per_week          NUMERIC(18,4) NOT NULL DEFAULT 0,
    td1_federal             NUMERIC(18,2) NOT NULL DEFAULT 0,
    td1_provincial          NUMERIC(18,2) NOT NULL DEFAULT 0,
    paid_ytd_other_payroll  BOOLEAN NOT NULL DEFAULT false,
    auto_vacation           BOOLEAN NOT NULL DEFAULT false,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_employees_company ON employees(company_id);
CREATE INDEX IF NOT EXISTS idx_employees_status ON employees(status);
CREATE INDEX IF NOT EXISTS idx_employees_employee_no ON employees(employee_no);
CREATE UNIQUE INDEX IF NOT EXISTS uq_employees_company_employee_no_present
    ON employees(company_id, employee_no)
    WHERE employee_no <> '';

CREATE TABLE IF NOT EXISTS payroll_runs (
    id                      BIGSERIAL PRIMARY KEY,
    company_id              BIGINT NOT NULL REFERENCES companies(id),
    run_number              VARCHAR(64) NOT NULL DEFAULT '',
    period_start            TIMESTAMPTZ NOT NULL,
    period_end              TIMESTAMPTZ NOT NULL,
    pay_date                TIMESTAMPTZ NOT NULL,
    pays_per_year           INTEGER NOT NULL DEFAULT 26,
    pay_frequency           TEXT NOT NULL DEFAULT 'biweekly',
    payroll_type            TEXT NOT NULL DEFAULT 'regular',
    status                  TEXT NOT NULL DEFAULT 'draft',
    total_gross             NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_employee_tax      NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_employee_cpp      NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_employee_cpp2     NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_employee_ei       NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_employer_cpp      NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_employer_cpp2     NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_employer_ei       NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_deductions        NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_net_pay           NUMERIC(18,2) NOT NULL DEFAULT 0,
    calculation_snapshot    JSONB,
    finalized_at            TIMESTAMPTZ,
    finalized_by_user_id    UUID,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payroll_runs_company ON payroll_runs(company_id);
CREATE INDEX IF NOT EXISTS idx_payroll_runs_pay_date ON payroll_runs(pay_date);
CREATE INDEX IF NOT EXISTS idx_payroll_runs_status ON payroll_runs(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_payroll_runs_company_run_number_present
    ON payroll_runs(company_id, run_number)
    WHERE run_number <> '';

CREATE TABLE IF NOT EXISTS payroll_entries (
    id                      BIGSERIAL PRIMARY KEY,
    company_id              BIGINT NOT NULL REFERENCES companies(id),
    payroll_run_id          BIGINT NOT NULL REFERENCES payroll_runs(id),
    employee_id             BIGINT NOT NULL REFERENCES employees(id),
    hours                   NUMERIC(18,4) NOT NULL DEFAULT 0,
    pay_rate                NUMERIC(18,6) NOT NULL DEFAULT 0,
    gross_pay               NUMERIC(18,2) NOT NULL DEFAULT 0,
    cpp_employee            NUMERIC(18,2) NOT NULL DEFAULT 0,
    cpp2_employee           NUMERIC(18,2) NOT NULL DEFAULT 0,
    ei_employee             NUMERIC(18,2) NOT NULL DEFAULT 0,
    federal_tax             NUMERIC(18,2) NOT NULL DEFAULT 0,
    provincial_tax          NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_deductions        NUMERIC(18,2) NOT NULL DEFAULT 0,
    net_pay                 NUMERIC(18,2) NOT NULL DEFAULT 0,
    cpp_employer            NUMERIC(18,2) NOT NULL DEFAULT 0,
    cpp2_employer           NUMERIC(18,2) NOT NULL DEFAULT 0,
    ei_employer             NUMERIC(18,2) NOT NULL DEFAULT 0,
    ytd_gross               NUMERIC(18,2) NOT NULL DEFAULT 0,
    ytd_cpp_employee        NUMERIC(18,2) NOT NULL DEFAULT 0,
    ytd_cpp2_employee       NUMERIC(18,2) NOT NULL DEFAULT 0,
    ytd_ei_employee         NUMERIC(18,2) NOT NULL DEFAULT 0,
    calculation_snapshot    JSONB,
    payment_type            TEXT NOT NULL DEFAULT 'cheque',
    status                  TEXT NOT NULL DEFAULT 'draft',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payroll_entry_run_employee UNIQUE(payroll_run_id, employee_id)
);

CREATE INDEX IF NOT EXISTS idx_payroll_entries_company ON payroll_entries(company_id);
CREATE INDEX IF NOT EXISTS idx_payroll_entries_run ON payroll_entries(payroll_run_id);
CREATE INDEX IF NOT EXISTS idx_payroll_entries_employee ON payroll_entries(employee_id);

CREATE TABLE IF NOT EXISTS payroll_earning_codes (
    id              BIGSERIAL PRIMARY KEY,
    company_id      BIGINT NOT NULL REFERENCES companies(id),
    code            VARCHAR(64) NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    enabled         BOOLEAN NOT NULL DEFAULT true,
    cpp             BOOLEAN NOT NULL DEFAULT true,
    ei              BOOLEAN NOT NULL DEFAULT true,
    tax_federal     BOOLEAN NOT NULL DEFAULT true,
    tax_provincial  BOOLEAN NOT NULL DEFAULT true,
    non_cash        BOOLEAN NOT NULL DEFAULT false,
    vacationable    BOOLEAN NOT NULL DEFAULT true,
    multiplier      NUMERIC(9,4) NOT NULL DEFAULT 1,
    is_system       BOOLEAN NOT NULL DEFAULT false,
    t4_box          VARCHAR(16) NOT NULL DEFAULT '',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payroll_earning_codes_company_code UNIQUE(company_id, code)
);

CREATE TABLE IF NOT EXISTS payroll_entry_earnings (
    id                 BIGSERIAL PRIMARY KEY,
    payroll_entry_id   BIGINT NOT NULL REFERENCES payroll_entries(id),
    earning_code_id    BIGINT NOT NULL REFERENCES payroll_earning_codes(id),
    hours              NUMERIC(18,4) NOT NULL DEFAULT 0,
    rate               NUMERIC(18,6) NOT NULL DEFAULT 0,
    amount             NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payroll_entry_earnings_entry ON payroll_entry_earnings(payroll_entry_id);

CREATE TABLE IF NOT EXISTS cheque_bank_accounts (
    id                       BIGSERIAL PRIMARY KEY,
    company_id               BIGINT NOT NULL REFERENCES companies(id),
    label                    TEXT NOT NULL DEFAULT '',
    bank_name                TEXT NOT NULL DEFAULT '',
    bank_address             TEXT NOT NULL DEFAULT '',
    bank_city                TEXT NOT NULL DEFAULT '',
    bank_province            TEXT NOT NULL DEFAULT '',
    bank_postal_code         TEXT NOT NULL DEFAULT '',
    micr_country             VARCHAR(8) NOT NULL DEFAULT 'CA',
    bank_institution         TEXT NOT NULL DEFAULT '',
    bank_transit             TEXT NOT NULL DEFAULT '',
    bank_routing_aba         TEXT NOT NULL DEFAULT '',
    bank_account_ciphertext  TEXT NOT NULL DEFAULT '',
    bank_account_last4       VARCHAR(4) NOT NULL DEFAULT '',
    bank_iban_ciphertext     TEXT NOT NULL DEFAULT '',
    bank_swift               TEXT NOT NULL DEFAULT '',
    ledger_account_id        BIGINT REFERENCES accounts(id),
    next_cheque_number       VARCHAR(64) NOT NULL DEFAULT '',
    default_currency_code    VARCHAR(3) NOT NULL DEFAULT 'CAD',
    is_active                BOOLEAN NOT NULL DEFAULT true,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cheque_bank_accounts_company ON cheque_bank_accounts(company_id);
CREATE INDEX IF NOT EXISTS idx_cheque_bank_accounts_ledger_account ON cheque_bank_accounts(ledger_account_id);

CREATE TABLE IF NOT EXISTS cheques (
    id                  BIGSERIAL PRIMARY KEY,
    company_id          BIGINT NOT NULL REFERENCES companies(id),
    bank_account_id     BIGINT NOT NULL REFERENCES cheque_bank_accounts(id),
    cheque_number       VARCHAR(64) NOT NULL DEFAULT '',
    payee_type          TEXT NOT NULL DEFAULT 'other',
    payee_name          TEXT NOT NULL DEFAULT '',
    vendor_id           BIGINT REFERENCES vendors(id),
    employee_id         BIGINT REFERENCES employees(id),
    payroll_run_id      BIGINT REFERENCES payroll_runs(id),
    payroll_entry_id    BIGINT REFERENCES payroll_entries(id),
    cheque_date         TIMESTAMPTZ NOT NULL,
    currency_code       VARCHAR(3) NOT NULL DEFAULT 'CAD',
    amount              NUMERIC(18,2) NOT NULL DEFAULT 0,
    memo                TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'draft',
    printed_at          TIMESTAMPTZ,
    voided_at           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cheques_company ON cheques(company_id);
CREATE INDEX IF NOT EXISTS idx_cheques_status ON cheques(status);
CREATE INDEX IF NOT EXISTS idx_cheques_employee ON cheques(employee_id);
CREATE INDEX IF NOT EXISTS idx_cheques_vendor ON cheques(vendor_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_cheques_company_number_present
    ON cheques(company_id, cheque_number)
    WHERE cheque_number <> '';
