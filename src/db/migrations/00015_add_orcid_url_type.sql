-- +goose Up
-- +goose StatementBegin
ALTER TYPE url_type ADD VALUE IF NOT EXISTS 'orcid';
-- +goose StatementEnd

-- +goose Down
-- Note: PostgreSQL doesn't support removing enum values directly
-- The value will remain but won't be used
