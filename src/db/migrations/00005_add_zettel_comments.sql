-- +goose Up
-- Add zettel comments table for temporary annotations on org-roam notes

CREATE TABLE IF NOT EXISTS zettel_comments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zettel_id       TEXT NOT NULL,                        -- org-mode ID (e.g., "075915aa-f7b9-499c-9858-8167d6b1e11b")

    content         TEXT NOT NULL,                        -- comment content

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_zettel_comments_zettel ON zettel_comments(zettel_id);
CREATE INDEX IF NOT EXISTS idx_zettel_comments_created ON zettel_comments(created_at DESC);

-- Trigger for updated_at timestamp
DROP TRIGGER IF EXISTS zettel_comments_updated_at ON zettel_comments;
CREATE TRIGGER zettel_comments_updated_at
    BEFORE UPDATE ON zettel_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TABLE IF EXISTS zettel_comments;
