-- +goose Up
-- Add last_auto_contact to contacts for WhatsApp auto-tracking
-- This timestamp is updated automatically when WhatsApp messages are sent/received
-- and is used in the overdue calculation without creating manual log entries
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS last_auto_contact TIMESTAMPTZ;

-- Create index for efficient overdue queries
CREATE INDEX IF NOT EXISTS idx_contacts_last_auto_contact
    ON contacts(last_auto_contact) WHERE last_auto_contact IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_contacts_last_auto_contact;
ALTER TABLE contacts DROP COLUMN IF EXISTS last_auto_contact;
