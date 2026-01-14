-- +goose Up
-- Add description field to health profiles for additional context in AI analysis

ALTER TABLE health_profiles
ADD COLUMN description TEXT;

COMMENT ON COLUMN health_profiles.description IS 'Optional notes about the person health baseline (e.g., normal WBC runs low)';

-- Drop and recreate health_profiles_summary view to include description
DROP VIEW IF EXISTS health_profiles_summary;

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

-- +goose Down
DROP VIEW IF EXISTS health_profiles_summary;

ALTER TABLE health_profiles
DROP COLUMN IF EXISTS description;

-- Recreate view without description
CREATE VIEW health_profiles_summary AS
SELECT
    p.id,
    p.name,
    p.date_of_birth,
    p.gender,
    p.created_at,
    p.updated_at,
    COUNT(f.id) AS followup_count,
    MAX(f.followup_date) AS last_followup_date
FROM health_profiles p
LEFT JOIN health_followups f ON p.id = f.profile_id
GROUP BY p.id, p.name, p.date_of_birth, p.gender, p.created_at, p.updated_at;
