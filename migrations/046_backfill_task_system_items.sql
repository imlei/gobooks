-- Migration 046: Backfill TASK_LABOR and TASK_REIM for companies that were
-- created before the task module was introduced (migrations 042–045).
--
-- EnsureSystemTaskItems is called during company setup, but companies that
-- already existed when the task module shipped never received these rows.
-- Without them, Generate Invoice Draft fails with:
--   "required task billing system items are missing or inactive"
--
-- This migration is idempotent: ON CONFLICT DO NOTHING skips companies
-- that already have the rows (e.g. companies created after migration 042).
--
-- Revenue account priority (mirrors EnsureSystemTaskItems logic):
--   1. detail_account_type = 'service_revenue'   AND is_active
--   2. detail_account_type = 'operating_revenue'  AND is_active
--   3. root_account_type   = 'revenue'            AND is_active
-- Companies with no revenue account at all are skipped with a WARNING.

DO $$
DECLARE
    r              RECORD;
    rev_account_id BIGINT;
BEGIN
    FOR r IN SELECT id FROM companies LOOP

        -- Resolve revenue account using same priority as EnsureSystemTaskItems.
        SELECT id INTO rev_account_id
        FROM accounts
        WHERE company_id = r.id
          AND detail_account_type = 'service_revenue'
          AND is_active = true
        ORDER BY id ASC LIMIT 1;

        IF rev_account_id IS NULL THEN
            SELECT id INTO rev_account_id
            FROM accounts
            WHERE company_id = r.id
              AND detail_account_type = 'operating_revenue'
              AND is_active = true
            ORDER BY id ASC LIMIT 1;
        END IF;

        IF rev_account_id IS NULL THEN
            SELECT id INTO rev_account_id
            FROM accounts
            WHERE company_id = r.id
              AND root_account_type = 'revenue'
              AND is_active = true
            ORDER BY id ASC LIMIT 1;
        END IF;

        IF rev_account_id IS NULL THEN
            RAISE WARNING 'migration 046: skipping company % — no active revenue account found', r.id;
            CONTINUE;
        END IF;

        -- TASK_LABOR: service type, sold only.
        INSERT INTO product_services (
            company_id, name, type, sku, description,
            default_price, purchase_price,
            can_be_sold, can_be_purchased, is_stock_item,
            item_structure_type,
            revenue_account_id,
            is_active, is_system, system_code,
            created_at, updated_at
        ) VALUES (
            r.id, 'Task', 'service', '', '',
            0, 0,
            true, false, false,
            'single',
            rev_account_id,
            true, true, 'TASK_LABOR',
            now(), now()
        )
        ON CONFLICT (company_id, system_code) DO NOTHING;

        -- TASK_REIM: non_inventory type, sold and purchased.
        INSERT INTO product_services (
            company_id, name, type, sku, description,
            default_price, purchase_price,
            can_be_sold, can_be_purchased, is_stock_item,
            item_structure_type,
            revenue_account_id,
            is_active, is_system, system_code,
            created_at, updated_at
        ) VALUES (
            r.id, 'Task Reim', 'non_inventory', '', '',
            0, 0,
            true, true, false,
            'single',
            rev_account_id,
            true, true, 'TASK_REIM',
            now(), now()
        )
        ON CONFLICT (company_id, system_code) DO NOTHING;

    END LOOP;
END $$;
