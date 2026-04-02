-- Migration 030: Add inventory costing method to companies table.
-- Default: moving_average. Once inventory movements exist for a company,
-- changing this value is not supported (enforced in application logic).

ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS inventory_costing_method TEXT NOT NULL DEFAULT 'moving_average';
