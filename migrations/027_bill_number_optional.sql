-- Migration 027: Make bill_number optional.
-- Bill numbers are vendor-provided reference numbers; not all vendors issue them.
-- Replace the old unique index with a partial index that only applies to non-empty values,
-- allowing multiple bills with empty bill_number for the same vendor.

-- Set default for existing NOT NULL column (GORM AutoMigrate handles this, but be explicit).
ALTER TABLE bills ALTER COLUMN bill_number SET DEFAULT '';

-- Drop old unique index that would conflict on empty strings.
DROP INDEX IF EXISTS uq_bills_company_vendor_bill_number_ci;

-- Recreate as partial index: uniqueness only enforced when bill_number is not empty.
CREATE UNIQUE INDEX IF NOT EXISTS uq_bills_company_vendor_bill_number_ci
ON bills (company_id, vendor_id, lower(bill_number))
WHERE bill_number <> '';
