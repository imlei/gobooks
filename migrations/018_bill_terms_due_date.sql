-- 018_bill_terms_due_date.sql
-- Add payment terms and due date to bills, matching the invoice pattern.
-- AutoMigrate handles fresh databases; this file is for existing deployments.

ALTER TABLE bills
    ADD COLUMN IF NOT EXISTS terms    TEXT        NOT NULL DEFAULT 'net_30',
    ADD COLUMN IF NOT EXISTS due_date TIMESTAMPTZ;
