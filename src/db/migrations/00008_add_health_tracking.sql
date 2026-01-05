-- +goose Up
-- Health tracking system for managing medical test results

-- Health Profiles table - tracks people (family members)
CREATE TABLE IF NOT EXISTS health_profiles (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_health_profiles_name ON health_profiles(name);

-- Health Follow-ups table - medical visits/reports
CREATE TABLE IF NOT EXISTS health_followups (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id    UUID NOT NULL REFERENCES health_profiles(id) ON DELETE CASCADE,
    followup_date DATE NOT NULL,
    hospital_name TEXT NOT NULL,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_health_followups_profile ON health_followups(profile_id);
CREATE INDEX IF NOT EXISTS idx_health_followups_date ON health_followups(followup_date DESC);

-- Health Lab Results table - individual test results
CREATE TABLE IF NOT EXISTS health_lab_results (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    followup_id UUID NOT NULL REFERENCES health_followups(id) ON DELETE CASCADE,
    test_name   TEXT NOT NULL,
    test_unit   TEXT,
    test_value  NUMERIC(12,3) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_health_lab_results_followup ON health_lab_results(followup_id);
CREATE INDEX IF NOT EXISTS idx_health_lab_results_test_name ON health_lab_results(test_name);

-- Trigger for auto-updating updated_at on health_profiles
CREATE TRIGGER health_profiles_updated_at
    BEFORE UPDATE ON health_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Trigger for auto-updating updated_at on health_followups
CREATE TRIGGER health_followups_updated_at
    BEFORE UPDATE ON health_followups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- View for health profiles with follow-up counts
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

-- View for health follow-ups with result counts
CREATE OR REPLACE VIEW health_followups_summary AS
SELECT
    f.id,
    f.profile_id,
    f.followup_date,
    f.hospital_name,
    f.notes,
    f.created_at,
    f.updated_at,
    COUNT(r.id) AS result_count
FROM health_followups f
LEFT JOIN health_lab_results r ON f.id = r.followup_id
GROUP BY f.id, f.profile_id, f.followup_date, f.hospital_name, f.notes, f.created_at, f.updated_at;

-- +goose Down
DROP VIEW IF EXISTS health_followups_summary;
DROP VIEW IF EXISTS health_profiles_summary;
DROP TRIGGER IF EXISTS health_followups_updated_at ON health_followups;
DROP TRIGGER IF EXISTS health_profiles_updated_at ON health_profiles;
DROP TABLE IF EXISTS health_lab_results;
DROP TABLE IF EXISTS health_followups;
DROP TABLE IF EXISTS health_profiles;
