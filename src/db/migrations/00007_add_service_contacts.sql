-- +goose Up
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS is_service BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_contacts_is_service ON contacts(is_service) WHERE is_service = true;

-- +goose Down
DROP INDEX IF EXISTS idx_contacts_is_service;
ALTER TABLE contacts DROP COLUMN IF EXISTS is_service;
