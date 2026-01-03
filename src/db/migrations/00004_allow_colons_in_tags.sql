-- +goose Up
-- Allow colons in tag names for namespacing (e.g., event:conf2024)
ALTER TABLE tags DROP CONSTRAINT tag_format;
ALTER TABLE tags ADD CONSTRAINT tag_format CHECK (name ~ '^[a-z0-9._:-]+$');

-- +goose Down
-- Revert to original constraint (no colons allowed)
ALTER TABLE tags DROP CONSTRAINT tag_format;
ALTER TABLE tags ADD CONSTRAINT tag_format CHECK (name ~ '^[a-z0-9._-]+$');
