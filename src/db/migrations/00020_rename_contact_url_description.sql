-- +goose Up
ALTER TABLE contact_urls
    RENAME COLUMN label TO description;

ALTER TABLE contact_urls
    DROP COLUMN username;

-- +goose Down
ALTER TABLE contact_urls
    ADD COLUMN username TEXT;

ALTER TABLE contact_urls
    RENAME COLUMN description TO label;
