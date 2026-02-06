-- +goose Up
-- +goose StatementBegin
ALTER TYPE inventory_status ADD VALUE IF NOT EXISTS 'maintenance_required';
-- +goose StatementEnd

-- +goose Down
-- Note: PostgreSQL doesn't support removing enum values directly
-- The value will remain but won't be used
