// 遵循产品需求 v1.0
package db

import (
	"gorm.io/gorm"
)

// migrateAccountsRootDetail migrates legacy `accounts.type` to root_account_type + detail_account_type
// and drops `type`. Safe on fresh DBs (no-op when `type` is absent).
func migrateAccountsRootDetail(db *gorm.DB) error {
	return db.Exec(`
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'accounts'
  ) THEN
    RETURN;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'accounts' AND column_name = 'type'
  ) THEN
    RETURN;
  END IF;

  ALTER TABLE accounts ADD COLUMN IF NOT EXISTS root_account_type text;
  ALTER TABLE accounts ADD COLUMN IF NOT EXISTS detail_account_type text;

  UPDATE accounts SET
    root_account_type = CASE "type"
      WHEN 'Bank' THEN 'asset'
      WHEN 'Accounts Receivable' THEN 'asset'
      WHEN 'Other Current Asset' THEN 'asset'
      WHEN 'Fixed Asset' THEN 'asset'
      WHEN 'Other Asset' THEN 'asset'
      WHEN 'Accounts Payable' THEN 'liability'
      WHEN 'Credit Card' THEN 'liability'
      WHEN 'Other Current Liability' THEN 'liability'
      WHEN 'Long Term Liability' THEN 'liability'
      WHEN 'Equity' THEN 'equity'
      WHEN 'Income' THEN 'revenue'
      WHEN 'Cost of Goods Sold' THEN 'cost_of_sales'
      WHEN 'Expense' THEN 'expense'
      WHEN 'Other Income' THEN 'revenue'
      WHEN 'Other Expense' THEN 'expense'
      WHEN 'asset' THEN 'asset'
      WHEN 'liability' THEN 'liability'
      WHEN 'equity' THEN 'equity'
      WHEN 'revenue' THEN 'revenue'
      WHEN 'cost_of_sales' THEN 'cost_of_sales'
      WHEN 'expense' THEN 'expense'
      ELSE NULL
    END,
    detail_account_type = CASE "type"
      WHEN 'Bank' THEN 'bank'
      WHEN 'Accounts Receivable' THEN 'accounts_receivable'
      WHEN 'Other Current Asset' THEN 'other_current_asset'
      WHEN 'Fixed Asset' THEN 'fixed_asset'
      WHEN 'Other Asset' THEN 'other_asset'
      WHEN 'Accounts Payable' THEN 'accounts_payable'
      WHEN 'Credit Card' THEN 'credit_card'
      WHEN 'Other Current Liability' THEN 'other_current_liability'
      WHEN 'Long Term Liability' THEN 'long_term_liability'
      WHEN 'Equity' THEN 'other_equity'
      WHEN 'Income' THEN 'operating_revenue'
      WHEN 'Cost of Goods Sold' THEN 'cost_of_goods_sold'
      WHEN 'Expense' THEN 'operating_expense'
      WHEN 'Other Income' THEN 'other_income'
      WHEN 'Other Expense' THEN 'other_expense'
      WHEN 'asset' THEN 'other_asset'
      WHEN 'liability' THEN 'other_current_liability'
      WHEN 'equity' THEN 'other_equity'
      WHEN 'revenue' THEN 'operating_revenue'
      WHEN 'cost_of_sales' THEN 'cost_of_goods_sold'
      WHEN 'expense' THEN 'operating_expense'
      ELSE NULL
    END
  WHERE root_account_type IS NULL OR root_account_type = ''
     OR detail_account_type IS NULL OR detail_account_type = '';

  IF EXISTS (SELECT 1 FROM accounts WHERE root_account_type IS NULL OR root_account_type = '') THEN
    RAISE EXCEPTION 'migrateAccountsRootDetail: could not map all legacy account type values';
  END IF;

  ALTER TABLE accounts ALTER COLUMN root_account_type SET NOT NULL;
  ALTER TABLE accounts ALTER COLUMN detail_account_type SET NOT NULL;

  ALTER TABLE accounts DROP COLUMN "type";
END $$;
`).Error
}
