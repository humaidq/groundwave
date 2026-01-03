-- +goose Up
-- +goose StatementBegin
ALTER TABLE contacts ADD COLUMN carddav_uuid TEXT;
CREATE INDEX idx_contacts_carddav_uuid ON contacts(carddav_uuid) WHERE carddav_uuid IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_contacts_carddav_uuid;
ALTER TABLE contacts DROP COLUMN carddav_uuid;
-- +goose StatementEnd
