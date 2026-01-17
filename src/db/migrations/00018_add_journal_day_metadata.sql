-- Add journal day metadata table for timeline augmentation

-- +goose Up
CREATE TABLE IF NOT EXISTS journal_day_metadata (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    day             DATE NOT NULL,
    location_lat    DECIMAL(9,6) NOT NULL,
    location_lon    DECIMAL(9,6) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT journal_day_location_lat_range CHECK (location_lat >= -90 AND location_lat <= 90),
    CONSTRAINT journal_day_location_lon_range CHECK (location_lon >= -180 AND location_lon <= 180)
);

CREATE INDEX IF NOT EXISTS idx_journal_day_metadata_day ON journal_day_metadata(day);

DROP TRIGGER IF EXISTS journal_day_metadata_updated_at ON journal_day_metadata;
CREATE TRIGGER journal_day_metadata_updated_at
    BEFORE UPDATE ON journal_day_metadata
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TABLE IF EXISTS journal_day_metadata;
