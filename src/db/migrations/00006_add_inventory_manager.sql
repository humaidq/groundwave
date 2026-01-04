-- +goose Up
-- Add inventory manager tables for tracking physical items

--------------------------------------------------------------------------------
-- SEQUENCE for incremental inventory IDs
--------------------------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS inventory_id_seq START WITH 1;

--------------------------------------------------------------------------------
-- INVENTORY ITEMS
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS inventory_items (
    id              SERIAL PRIMARY KEY,                -- Auto-incrementing numeric ID (1, 2, 3...)
    inventory_id    TEXT UNIQUE NOT NULL               -- Formatted ID (GW-00001, GW-00002...)
                    DEFAULT ('GW-' || LPAD(nextval('inventory_id_seq')::TEXT, 5, '0')),

    -- Core Fields
    name            TEXT NOT NULL                      -- Required field
                    CONSTRAINT name_not_empty CHECK (length(trim(name)) > 0),
    location        TEXT,                              -- Optional field, used for autocomplete
    description     TEXT,                              -- Optional field

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_inventory_items_inventory_id ON inventory_items(inventory_id);
CREATE INDEX IF NOT EXISTS idx_inventory_items_location ON inventory_items(location) WHERE location IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_inventory_items_created ON inventory_items(created_at DESC);

-- Trigger for updated_at timestamp
DROP TRIGGER IF EXISTS inventory_items_updated_at ON inventory_items;
CREATE TRIGGER inventory_items_updated_at
    BEFORE UPDATE ON inventory_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

--------------------------------------------------------------------------------
-- INVENTORY COMMENTS
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS inventory_comments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id         INTEGER NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,

    content         TEXT NOT NULL                      -- Comment content
                    CONSTRAINT content_not_empty CHECK (length(trim(content)) > 0),

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_inventory_comments_item ON inventory_comments(item_id);
CREATE INDEX IF NOT EXISTS idx_inventory_comments_created ON inventory_comments(created_at DESC);

-- Trigger for updated_at timestamp
DROP TRIGGER IF EXISTS inventory_comments_updated_at ON inventory_comments;
CREATE TRIGGER inventory_comments_updated_at
    BEFORE UPDATE ON inventory_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TABLE IF EXISTS inventory_comments;
DROP TABLE IF EXISTS inventory_items;
DROP SEQUENCE IF EXISTS inventory_id_seq;
