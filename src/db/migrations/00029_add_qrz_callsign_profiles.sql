-- +goose Up
-- Store QRZ XML profile data for unique callsigns worked in the QSO log.

CREATE TABLE IF NOT EXISTS qrz_callsign_profiles (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    callsign          TEXT NOT NULL UNIQUE
                      CONSTRAINT qrz_callsign_profiles_callsign_not_empty CHECK (length(trim(callsign)) > 0),

    -- Commonly displayed profile fields
    call              TEXT,
    xref              TEXT,
    aliases           TEXT,
    dxcc              INTEGER,
    fname             TEXT,
    name              TEXT,
    name_fmt          TEXT,
    nickname          TEXT,
    attn              TEXT,
    addr1             TEXT,
    addr2             TEXT,
    state             TEXT,
    zip               TEXT,
    country           TEXT,
    ccode             INTEGER,
    lat               DOUBLE PRECISION,
    lon               DOUBLE PRECISION,
    grid              TEXT,
    county            TEXT,
    qslmgr            TEXT,
    email             TEXT,
    qrz_user          TEXT,

    -- Full raw payload for forward-compatible storage of all XML fields
    payload_json      JSONB,
    raw_xml           TEXT,

    -- Lookup metadata
    last_lookup_at    TIMESTAMPTZ,
    last_success_at   TIMESTAMPTZ,
    last_lookup_error TEXT,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_qrz_callsign_profiles_callsign ON qrz_callsign_profiles(callsign);
CREATE INDEX IF NOT EXISTS idx_qrz_callsign_profiles_lookup_at ON qrz_callsign_profiles(last_lookup_at);

DROP TRIGGER IF EXISTS qrz_callsign_profiles_updated_at ON qrz_callsign_profiles;
CREATE TRIGGER qrz_callsign_profiles_updated_at
    BEFORE UPDATE ON qrz_callsign_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS qrz_callsign_profiles_updated_at ON qrz_callsign_profiles;
DROP TABLE IF EXISTS qrz_callsign_profiles;
