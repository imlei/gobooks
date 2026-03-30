-- Migration 022: company logo upload
-- Stores the relative path to the company's uploaded logo file.
-- Empty string means no logo has been uploaded.

ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS logo_path TEXT NOT NULL DEFAULT '';
