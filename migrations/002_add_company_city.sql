-- Optional manual migration when GORM AutoMigrate is not used or to align existing DBs.
-- Safe for non-empty databases: adds NOT NULL column with default '' for existing rows.
-- Idempotent: skip if column already exists.

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'companies'
      AND column_name = 'city'
  ) THEN
    ALTER TABLE companies ADD COLUMN city text NOT NULL DEFAULT '';
  END IF;
END $$;
