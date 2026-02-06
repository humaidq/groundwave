-- +goose Up
ALTER TABLE inventory_items
ADD COLUMN inspection_date DATE;

CREATE INDEX IF NOT EXISTS idx_inventory_items_inspection_date ON inventory_items(inspection_date);

-- +goose Down
DROP INDEX IF EXISTS idx_inventory_items_inspection_date;
ALTER TABLE inventory_items DROP COLUMN IF EXISTS inspection_date;
