-- +goose Up
ALTER TABLE inventory_items
ADD COLUMN item_type TEXT,
ADD CONSTRAINT inventory_items_item_type_not_empty CHECK (item_type IS NULL OR length(trim(item_type)) > 0);

CREATE TABLE IF NOT EXISTS inventory_tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE
                CONSTRAINT inventory_tag_format CHECK (name ~ '^[a-z0-9._:/-]+( [a-z0-9._:/-]+)*$'),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory_item_tags (
    item_id     INTEGER NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES inventory_tags(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (item_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_inventory_items_item_type ON inventory_items(item_type)
    WHERE item_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_item_tags_tag ON inventory_item_tags(tag_id);

-- +goose Down
DROP INDEX IF EXISTS idx_inventory_item_tags_tag;
DROP INDEX IF EXISTS idx_inventory_items_item_type;

DROP TABLE IF EXISTS inventory_item_tags;
DROP TABLE IF EXISTS inventory_tags;

ALTER TABLE inventory_items DROP CONSTRAINT IF EXISTS inventory_items_item_type_not_empty;
ALTER TABLE inventory_items DROP COLUMN IF EXISTS item_type;
