-- Migration 047: Add payment settlement fields to expenses table.
--
-- PaymentAccountID  → the GL account used to pay (bank, credit card, petty cash).
-- PaymentMethod     → instrument used (check, wire, cash, credit_card, debit_card, other).
-- PaymentReference  → user-supplied memo or cheque number.
--
-- All three columns are optional (NULL / empty allowed); validation is enforced
-- at the service layer, not the DB schema.

ALTER TABLE expenses
    ADD COLUMN IF NOT EXISTS payment_account_id BIGINT REFERENCES accounts(id),
    ADD COLUMN IF NOT EXISTS payment_method     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_reference  TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_expenses_payment_account_id ON expenses(payment_account_id);
