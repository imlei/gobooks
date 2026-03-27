-- Phase 1 — READ-ONLY pre-flight checks (does not modify the database).
-- Run these in psql or any SQL client against your target database before 001–004.
-- Fix any issues reported before applying migrations.

-- -----------------------------------------------------------------------------
-- 1) Companies: must have exactly one row for typical single-tenant backfill,
--    or at least one row if you intend to map all data to companies.id = MIN(id).
-- -----------------------------------------------------------------------------
SELECT id, name, created_at FROM companies ORDER BY id;

SELECT
  CASE
    WHEN (SELECT count(*) FROM companies) = 0 THEN 'FAIL: no companies row — 002/004 cannot backfill company_id'
    WHEN (SELECT count(*) FROM companies) > 1 THEN 'WARN: multiple companies — 002/004 only assign MIN(id); review before run'
    ELSE 'OK: companies present'
  END AS companies_check;

-- -----------------------------------------------------------------------------
-- 2) Duplicate account codes (global). Old schema enforced unique(code); if this
--    returns rows, data is already inconsistent OR the unique constraint is missing.
--    uq_accounts_company_code will fail until duplicates are resolved.
-- -----------------------------------------------------------------------------
SELECT code, count(*) AS cnt
FROM accounts
GROUP BY code
HAVING count(*) > 1;

-- -----------------------------------------------------------------------------
-- 3) Orphan journal_lines: lines whose journal_entry_id does not exist (broken FK
--    or manual deletes). Migration still backfills company_id via fallback, but you
--    should investigate orphans.
-- -----------------------------------------------------------------------------
SELECT jl.id AS journal_line_id, jl.journal_entry_id
FROM journal_lines jl
LEFT JOIN journal_entries je ON je.id = jl.journal_entry_id
WHERE je.id IS NULL;

-- -----------------------------------------------------------------------------
-- 4) Journal lines referencing entries that exist but had NULL company_id before
--    migration is N/A pre-migration; post-002 you can verify jl.company_id = je.company_id.
-- -----------------------------------------------------------------------------

-- -----------------------------------------------------------------------------
-- 5) Existing index/constraint names on accounts(code) — helps if 002 manual fix needed.
-- -----------------------------------------------------------------------------
SELECT indexname, indexdef
FROM pg_indexes
WHERE schemaname = current_schema() AND tablename = 'accounts'
ORDER BY indexname;

SELECT conname, pg_get_constraintdef(oid) AS def
FROM pg_constraint
WHERE conrelid = 'accounts'::regclass
  AND contype IN ('u', 'p')
ORDER BY conname;

-- -----------------------------------------------------------------------------
-- 6) Row counts (sanity)
-- -----------------------------------------------------------------------------
SELECT 'accounts' AS tbl, count(*) FROM accounts
UNION ALL SELECT 'customers', count(*) FROM customers
UNION ALL SELECT 'vendors', count(*) FROM vendors
UNION ALL SELECT 'invoices', count(*) FROM invoices
UNION ALL SELECT 'bills', count(*) FROM bills
UNION ALL SELECT 'journal_entries', count(*) FROM journal_entries
UNION ALL SELECT 'journal_lines', count(*) FROM journal_lines
UNION ALL SELECT 'reconciliations', count(*) FROM reconciliations
UNION ALL SELECT 'audit_logs', count(*) FROM audit_logs;
