-- +goose Up

ALTER TABLE contact_exchange_links
ADD COLUMN IF NOT EXISTS additional_note TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE contact_exchange_links
DROP COLUMN IF EXISTS additional_note;
