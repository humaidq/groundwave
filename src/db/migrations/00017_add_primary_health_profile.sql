-- +goose Up
-- Add primary flag to health profiles

ALTER TABLE health_profiles
ADD COLUMN is_primary BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN health_profiles.is_primary IS 'Marks the primary health profile for timeline display';

CREATE UNIQUE INDEX IF NOT EXISTS idx_health_profiles_primary
ON health_profiles (is_primary)
WHERE is_primary;

-- Drop and recreate health_profiles_summary view to include is_primary
DROP VIEW IF EXISTS health_profiles_summary;

CREATE VIEW health_profiles_summary AS
SELECT
    p.id,
    p.name,
    p.date_of_birth,
    p.gender,
    p.description,
    p.is_primary,
    p.created_at,
    p.updated_at,
    COUNT(f.id) AS followup_count,
    MAX(f.followup_date) AS last_followup_date
FROM health_profiles p
LEFT JOIN health_followups f ON p.id = f.profile_id
GROUP BY p.id, p.name, p.date_of_birth, p.gender, p.description, p.is_primary, p.created_at, p.updated_at;

-- +goose Down
DROP VIEW IF EXISTS health_profiles_summary;

DROP INDEX IF EXISTS idx_health_profiles_primary;

ALTER TABLE health_profiles
DROP COLUMN IF EXISTS is_primary;

CREATE VIEW health_profiles_summary AS
SELECT
    p.id,
    p.name,
    p.date_of_birth,
    p.gender,
    p.description,
    p.created_at,
    p.updated_at,
    COUNT(f.id) AS followup_count,
    MAX(f.followup_date) AS last_followup_date
FROM health_profiles p
LEFT JOIN health_followups f ON p.id = f.profile_id
GROUP BY p.id, p.name, p.date_of_birth, p.gender, p.description, p.created_at, p.updated_at;
