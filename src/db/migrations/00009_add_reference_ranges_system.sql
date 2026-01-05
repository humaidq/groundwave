-- +goose Up
-- Add DOB and gender to health profiles, plus reference ranges tables

-- Add DOB and gender to existing health_profiles table
ALTER TABLE health_profiles
ADD COLUMN date_of_birth DATE,
ADD COLUMN gender TEXT CHECK (gender IN ('Male', 'Female', NULL));

CREATE INDEX IF NOT EXISTS idx_health_profiles_dob ON health_profiles(date_of_birth);
CREATE INDEX IF NOT EXISTS idx_health_profiles_gender ON health_profiles(gender);

COMMENT ON COLUMN health_profiles.date_of_birth IS 'Date of birth for calculating age-based reference ranges';
COMMENT ON COLUMN health_profiles.gender IS 'Gender for reference ranges (Male, Female, or NULL)';

-- Reference Ranges table - stores age/gender-based normal ranges for lab tests
CREATE TABLE IF NOT EXISTS reference_ranges (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_name     TEXT NOT NULL,
    age_range     TEXT NOT NULL CHECK (age_range IN ('Pediatric', 'Adult', 'MiddleAge', 'Senior')),
    gender        TEXT NOT NULL CHECK (gender IN ('Male', 'Female', 'Unisex')),
    reference_min NUMERIC(12,3),
    reference_max NUMERIC(12,3),
    optimal_min   NUMERIC(12,3),
    optimal_max   NUMERIC(12,3),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Ensure one range per test/age/gender combination
    UNIQUE (test_name, age_range, gender)
);

CREATE INDEX IF NOT EXISTS idx_reference_ranges_lookup ON reference_ranges(test_name, age_range, gender);

COMMENT ON TABLE reference_ranges IS 'Reference and optimal ranges for lab tests, synced from Go code on startup';
COMMENT ON COLUMN reference_ranges.test_name IS 'Lab test name (must match predefined test names)';
COMMENT ON COLUMN reference_ranges.age_range IS 'Age category: Pediatric (0-17), Adult (18-49), MiddleAge (50-64), Senior (65+)';
COMMENT ON COLUMN reference_ranges.gender IS 'Gender category: Male, Female, or Unisex (applies to all)';
COMMENT ON COLUMN reference_ranges.reference_min IS 'Minimum normal reference range value';
COMMENT ON COLUMN reference_ranges.reference_max IS 'Maximum normal reference range value';
COMMENT ON COLUMN reference_ranges.optimal_min IS 'Optional minimum optimal range (narrower than reference)';
COMMENT ON COLUMN reference_ranges.optimal_max IS 'Optional maximum optimal range (narrower than reference)';

-- Trigger for auto-updating updated_at on reference_ranges
CREATE TRIGGER reference_ranges_updated_at
    BEFORE UPDATE ON reference_ranges
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Drop and recreate health_profiles_summary view to include new fields
DROP VIEW IF EXISTS health_profiles_summary;

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

-- +goose Down
DROP VIEW IF EXISTS health_profiles_summary;
DROP TRIGGER IF EXISTS reference_ranges_updated_at ON reference_ranges;
DROP TABLE IF EXISTS reference_ranges;
DROP INDEX IF EXISTS idx_health_profiles_gender;
DROP INDEX IF EXISTS idx_health_profiles_dob;

ALTER TABLE health_profiles
DROP COLUMN IF EXISTS gender,
DROP COLUMN IF EXISTS date_of_birth;

-- Recreate original view without DOB/gender
CREATE OR REPLACE VIEW health_profiles_summary AS
SELECT
    p.id,
    p.name,
    p.created_at,
    p.updated_at,
    COUNT(f.id) AS followup_count,
    MAX(f.followup_date) AS last_followup_date
FROM health_profiles p
LEFT JOIN health_followups f ON p.id = f.profile_id
GROUP BY p.id, p.name, p.created_at, p.updated_at;
