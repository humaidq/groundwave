-- +goose Up
-- Migration: Add source tracking for CardDAV synced emails and phones
-- This allows the no_phone and no_email filters to correctly include CardDAV data

-- Add source column to contact_emails
ALTER TABLE contact_emails ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'local';

-- Add source column to contact_phones
ALTER TABLE contact_phones ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'local';

-- Drop the global unique email constraint and replace with per-contact unique
-- This allows the same email to exist on multiple contacts (e.g., shared work emails)
DROP INDEX IF EXISTS idx_contact_emails_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_contact_emails_unique ON contact_emails(contact_id, lower(email));

-- Add index for efficient source-based queries
CREATE INDEX IF NOT EXISTS idx_contact_emails_source ON contact_emails(source);
CREATE INDEX IF NOT EXISTS idx_contact_phones_source ON contact_phones(source);

-- +goose Down
DROP INDEX IF EXISTS idx_contact_phones_source;
DROP INDEX IF EXISTS idx_contact_emails_source;

-- Restore original global unique constraint
DROP INDEX IF EXISTS idx_contact_emails_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_contact_emails_unique ON contact_emails(lower(email));

ALTER TABLE contact_phones DROP COLUMN IF EXISTS source;
ALTER TABLE contact_emails DROP COLUMN IF EXISTS source;
