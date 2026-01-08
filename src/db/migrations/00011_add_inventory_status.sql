-- +goose Up
-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE inventory_status AS ENUM (
        'active',
        'stored',
        'damaged',
        'given',
        'disposed',
        'lost'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
-- +goose StatementEnd

ALTER TABLE inventory_items
ADD COLUMN status inventory_status NOT NULL DEFAULT 'active';

-- +goose Down
ALTER TABLE inventory_items DROP COLUMN IF EXISTS status;

-- +goose StatementBegin
DROP TYPE IF EXISTS inventory_status;
-- +goose StatementEnd
