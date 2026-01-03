-- Personal CRM Schema
-- Inspired by Derek Sivers' approach: PostgreSQL as the brain
-- Compatible with vCard 4.0 (RFC 6350)

-- Extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;  -- for gen_random_uuid()

--------------------------------------------------------------------------------
-- TYPES
--------------------------------------------------------------------------------

DO $$ BEGIN
    CREATE TYPE tier AS ENUM ('A', 'B', 'C', 'D', 'E', 'F');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE phone_type AS ENUM ('cell', 'home', 'work', 'fax', 'pager', 'other');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE email_type AS ENUM ('personal', 'work', 'other');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE address_type AS ENUM ('home', 'work', 'other');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE url_type AS ENUM (
        'website', 'blog',
        'twitter', 'mastodon', 'bluesky', 'threads', 'facebook', 'instagram',
        'linkedin', 'github', 'gitlab', 'codeberg',
        'youtube', 'twitch', 'tiktok',
        'signal', 'telegram', 'whatsapp', 'matrix',
        'qrz',  -- amateur radio profile
        'other'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

--------------------------------------------------------------------------------
-- CONTACTS (core vCard fields)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS contacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- vCard: N (name components)
    name_given      TEXT,                          -- first name
    name_additional TEXT,                          -- middle name(s)
    name_family     TEXT,                          -- last name

    -- vCard: FN (formatted name) - auto-generated if null
    name_display    TEXT NOT NULL
                    CONSTRAINT name_not_empty CHECK (length(trim(name_display)) > 0),

    -- vCard: NICKNAME
    nickname        TEXT,

    -- vCard: ORG, TITLE, ROLE
    organization    TEXT,
    title           TEXT,                          -- job title
    role            TEXT,                          -- function/role

    -- vCard: BDAY, ANNIVERSARY
    birthday        DATE,
    anniversary     DATE,

    -- vCard: GENDER (M, F, O, N, U per spec, but free text is fine)
    gender          TEXT,

    -- vCard: TZ, GEO
    timezone        TEXT,                          -- e.g., 'America/New_York'
    geo_lat         DECIMAL(9,6),
    geo_lon         DECIMAL(9,6),

    -- vCard: LANG (preferred language)
    language        TEXT,                          -- e.g., 'en', 'ja'

    -- vCard: NOTE
    notes           TEXT,

    -- vCard: PHOTO (store URL or base64 reference)
    photo_url       TEXT,

    -- vCard: UID (for CardDAV sync - use our id)
    -- We use the UUID id for this

    -- CRM extensions
    tier            tier DEFAULT 'C',
    call_sign       TEXT                           -- amateur radio
                    CONSTRAINT call_sign_format
                    CHECK (call_sign IS NULL OR call_sign ~ '^[A-Z0-9]{3,8}$'),

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contacts_tier ON contacts(tier);
CREATE INDEX IF NOT EXISTS idx_contacts_call_sign ON contacts(call_sign) WHERE call_sign IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(name_family, name_given);
CREATE INDEX IF NOT EXISTS idx_contacts_updated ON contacts(updated_at);

--------------------------------------------------------------------------------
-- EMAILS (multiple per contact)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS contact_emails (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,

    email           TEXT NOT NULL
                    CONSTRAINT valid_email CHECK (email ~ '^\S+@\S+\.\S+$'),
    email_type      email_type DEFAULT 'personal',
    is_primary      BOOLEAN DEFAULT false,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_emails_contact ON contact_emails(contact_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_contact_emails_unique ON contact_emails(lower(email));

--------------------------------------------------------------------------------
-- PHONES (multiple per contact)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS contact_phones (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,

    phone           TEXT NOT NULL,
    phone_type      phone_type DEFAULT 'cell',
    is_primary      BOOLEAN DEFAULT false,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_phones_contact ON contact_phones(contact_id);

--------------------------------------------------------------------------------
-- ADDRESSES (multiple per contact)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS contact_addresses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,

    -- vCard: ADR components
    street          TEXT,                          -- street address (can include unit)
    locality        TEXT,                          -- city
    region          TEXT,                          -- state/province
    postal_code     TEXT,
    country         TEXT,                          -- ISO 3166-1 alpha-2 preferred

    address_type    address_type DEFAULT 'home',
    is_primary      BOOLEAN DEFAULT false,

    -- Optional: structured for mail
    po_box          TEXT,
    extended        TEXT,                          -- apartment, suite, etc.

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_addresses_contact ON contact_addresses(contact_id);

--------------------------------------------------------------------------------
-- URLS & SOCIAL (multiple per contact)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS contact_urls (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,

    url             TEXT NOT NULL,
    url_type        url_type DEFAULT 'website',
    label           TEXT,                          -- optional display label
    username        TEXT,                          -- e.g., @handle for social

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_urls_contact ON contact_urls(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_urls_type ON contact_urls(url_type);

--------------------------------------------------------------------------------
-- TAGS (many-to-many)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS tags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE
                    CONSTRAINT tag_format CHECK (name ~ '^[a-z0-9._-]+$'),
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS contact_tags (
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id          UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (contact_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_contact_tags_tag ON contact_tags(tag_id);

--------------------------------------------------------------------------------
-- CONTACT LOG (CRM interactions - NOT for QSOs)
--------------------------------------------------------------------------------

DO $$ BEGIN
    CREATE TYPE log_type AS ENUM (
        'note',           -- general note
        'email_sent',     -- sent them an email
        'email_received', -- received email from them
        'call',           -- phone/video call
        'meeting',        -- in-person meeting
        'message',        -- text/chat message
        'gift_sent',      -- sent a gift
        'gift_received',  -- received a gift
        'intro',          -- introduction made
        'other'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS contact_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,

    log_type        log_type NOT NULL DEFAULT 'note',
    logged_at       TIMESTAMPTZ NOT NULL DEFAULT now(),  -- when it happened

    subject         TEXT,                                 -- brief summary
    content         TEXT,                                 -- full notes

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_logs_contact ON contact_logs(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_logs_date ON contact_logs(logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_contact_logs_type ON contact_logs(log_type);

--------------------------------------------------------------------------------
-- QSO LOG (ADIF 3.1.6 compatible)
-- Full amateur radio contact log with lossless ADIF import/export
--------------------------------------------------------------------------------

-- ADIF Enumerations as PostgreSQL types
DO $$ BEGIN
    CREATE TYPE qsl_status AS ENUM ('Y', 'N', 'R', 'I', 'V');  -- Yes, No, Requested, Ignore, Verified
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE qsl_sent_status AS ENUM ('Y', 'N', 'R', 'Q', 'I');  -- Yes, No, Requested, Queued, Ignore
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE qsl_via AS ENUM ('B', 'D', 'E', 'M');  -- Bureau, Direct, Electronic, Manager
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE qso_upload_status AS ENUM ('Y', 'N', 'M');  -- Uploaded, Not uploaded, Modified
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE qso_complete AS ENUM ('Y', 'N', 'NIL', '?');  -- Yes, No, Not in log, Unknown
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE ant_path AS ENUM ('G', 'O', 'S', 'L');  -- Grayline, Other, Short path, Long path
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS qsos (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Optional link to contacts table (may be null for one-off QSOs)
    contact_id          UUID REFERENCES contacts(id) ON DELETE SET NULL,

    -------------------------------------------------------------------------
    -- CORE QSO FIELDS (required for valid QSO)
    -------------------------------------------------------------------------
    call                TEXT NOT NULL,                    -- CALL: contacted station
    qso_date            DATE NOT NULL,                    -- QSO_DATE: date UTC
    time_on             TIME NOT NULL,                    -- TIME_ON: start time UTC
    band                TEXT,                             -- BAND: 20m, 40m, etc.
    freq                DECIMAL(12,6),                    -- FREQ: frequency in MHz
    mode                TEXT NOT NULL,                    -- MODE: SSB, CW, FT8, etc.

    -------------------------------------------------------------------------
    -- TIME
    -------------------------------------------------------------------------
    time_off            TIME,                             -- TIME_OFF: end time UTC
    qso_date_off        DATE,                             -- QSO_DATE_OFF: end date if different

    -------------------------------------------------------------------------
    -- FREQUENCY (additional)
    -------------------------------------------------------------------------
    band_rx             TEXT,                             -- BAND_RX: receive band (split)
    freq_rx             DECIMAL(12,6),                    -- FREQ_RX: receive frequency

    -------------------------------------------------------------------------
    -- MODE (additional)
    -------------------------------------------------------------------------
    submode             TEXT,                             -- SUBMODE: e.g., FT8 under MFSK

    -------------------------------------------------------------------------
    -- SIGNAL REPORTS
    -------------------------------------------------------------------------
    rst_sent            TEXT,                             -- RST_SENT: signal report sent
    rst_rcvd            TEXT,                             -- RST_RCVD: signal report received

    -------------------------------------------------------------------------
    -- POWER
    -------------------------------------------------------------------------
    tx_pwr              DECIMAL(10,2),                    -- TX_PWR: transmit power (watts)
    rx_pwr              DECIMAL(10,2),                    -- RX_PWR: other station's power

    -------------------------------------------------------------------------
    -- CONTACTED STATION INFO
    -------------------------------------------------------------------------
    name                TEXT,                             -- NAME: operator name
    qth                 TEXT,                             -- QTH: city/location
    gridsquare          TEXT,                             -- GRIDSQUARE: Maidenhead (4-8 char)
    gridsquare_ext      TEXT,                             -- GRIDSQUARE_EXT: extended (chars 9-12)
    vucc_grids          TEXT,                             -- VUCC_GRIDS: comma-separated grids
    lat                 TEXT,                             -- LAT: latitude
    lon                 TEXT,                             -- LON: longitude
    altitude            DECIMAL(10,2),                    -- ALTITUDE: meters
    country             TEXT,                             -- COUNTRY: country name
    dxcc                INTEGER,                          -- DXCC: entity code
    cqz                 INTEGER,                          -- CQZ: CQ zone
    ituz                INTEGER,                          -- ITUZ: ITU zone
    cont                TEXT,                             -- CONT: continent
    state               TEXT,                             -- STATE: state/province
    cnty                TEXT,                             -- CNTY: county (US)
    cnty_alt            TEXT,                             -- CNTY_ALT: secondary subdivision alt
    pfx                 TEXT,                             -- PFX: WPX prefix
    age                 INTEGER,                          -- AGE: operator age
    email               TEXT,                             -- EMAIL: email address
    web                 TEXT,                             -- WEB: website URL
    address             TEXT,                             -- ADDRESS: mailing address

    -------------------------------------------------------------------------
    -- CONTACTED STATION - AWARDS/PROGRAMS
    -------------------------------------------------------------------------
    iota                TEXT,                             -- IOTA: IOTA reference (e.g., EU-005)
    iota_island_id      TEXT,                             -- IOTA_ISLAND_ID: island ID
    sota_ref            TEXT,                             -- SOTA_REF: SOTA reference
    pota_ref            TEXT,                             -- POTA_REF: POTA reference(s)
    wwff_ref            TEXT,                             -- WWFF_REF: WWFF reference
    sig                 TEXT,                             -- SIG: special interest group
    sig_info            TEXT,                             -- SIG_INFO: SIG details
    darc_dok            TEXT,                             -- DARC_DOK: DARC DOK
    fists               INTEGER,                          -- FISTS: FISTS number
    fists_cc            INTEGER,                          -- FISTS_CC: FISTS CC number
    skcc                TEXT,                             -- SKCC: SKCC number
    ten_ten             INTEGER,                          -- TEN_TEN: 10-10 number
    uksmg               INTEGER,                          -- UKSMG: UKSMG number
    usaca_counties      TEXT,                             -- USACA_COUNTIES: counties list

    -------------------------------------------------------------------------
    -- CONTACTED OPERATOR
    -------------------------------------------------------------------------
    contacted_op        TEXT,                             -- CONTACTED_OP: callsign of operator
    eq_call             TEXT,                             -- EQ_CALL: owner callsign
    guest_op            TEXT,                             -- GUEST_OP: deprecated
    silent_key          BOOLEAN,                          -- SILENT_KEY: operator is SK

    -------------------------------------------------------------------------
    -- MY STATION INFO
    -------------------------------------------------------------------------
    station_callsign    TEXT,                             -- STATION_CALLSIGN: my call used
    operator            TEXT,                             -- OPERATOR: my operator call
    owner_callsign      TEXT,                             -- OWNER_CALLSIGN: station owner
    my_name             TEXT,                             -- MY_NAME
    my_gridsquare       TEXT,                             -- MY_GRIDSQUARE
    my_gridsquare_ext   TEXT,                             -- MY_GRIDSQUARE_EXT
    my_vucc_grids       TEXT,                             -- MY_VUCC_GRIDS
    my_lat              TEXT,                             -- MY_LAT
    my_lon              TEXT,                             -- MY_LON
    my_altitude         DECIMAL(10,2),                    -- MY_ALTITUDE
    my_city             TEXT,                             -- MY_CITY
    my_street           TEXT,                             -- MY_STREET
    my_postal_code      TEXT,                             -- MY_POSTAL_CODE
    my_state            TEXT,                             -- MY_STATE
    my_cnty             TEXT,                             -- MY_CNTY
    my_cnty_alt         TEXT,                             -- MY_CNTY_ALT
    my_country          TEXT,                             -- MY_COUNTRY
    my_dxcc             INTEGER,                          -- MY_DXCC
    my_cq_zone          INTEGER,                          -- MY_CQ_ZONE
    my_itu_zone         INTEGER,                          -- MY_ITU_ZONE

    -------------------------------------------------------------------------
    -- MY STATION - AWARDS/PROGRAMS
    -------------------------------------------------------------------------
    my_iota             TEXT,                             -- MY_IOTA
    my_iota_island_id   TEXT,                             -- MY_IOTA_ISLAND_ID
    my_sota_ref         TEXT,                             -- MY_SOTA_REF
    my_pota_ref         TEXT,                             -- MY_POTA_REF
    my_wwff_ref         TEXT,                             -- MY_WWFF_REF
    my_sig              TEXT,                             -- MY_SIG
    my_sig_info         TEXT,                             -- MY_SIG_INFO
    my_arrl_sect        TEXT,                             -- MY_ARRL_SECT
    my_darc_dok         TEXT,                             -- MY_DARC_DOK
    my_fists            INTEGER,                          -- MY_FISTS
    my_usaca_counties   TEXT,                             -- MY_USACA_COUNTIES

    -------------------------------------------------------------------------
    -- MY EQUIPMENT
    -------------------------------------------------------------------------
    my_rig              TEXT,                             -- MY_RIG
    my_antenna          TEXT,                             -- MY_ANTENNA
    rig                 TEXT,                             -- RIG: contacted station's rig

    -------------------------------------------------------------------------
    -- MORSE KEY INFO
    -------------------------------------------------------------------------
    morse_key_type      TEXT,                             -- MORSE_KEY_TYPE
    morse_key_info      TEXT,                             -- MORSE_KEY_INFO
    my_morse_key_type   TEXT,                             -- MY_MORSE_KEY_TYPE
    my_morse_key_info   TEXT,                             -- MY_MORSE_KEY_INFO

    -------------------------------------------------------------------------
    -- PROPAGATION
    -------------------------------------------------------------------------
    prop_mode           TEXT,                             -- PROP_MODE: propagation mode
    ant_path            ant_path,                         -- ANT_PATH: antenna path
    ant_az              DECIMAL(5,1),                     -- ANT_AZ: antenna azimuth
    ant_el              DECIMAL(5,1),                     -- ANT_EL: antenna elevation
    distance            INTEGER,                          -- DISTANCE: km
    a_index             INTEGER,                          -- A_INDEX
    k_index             INTEGER,                          -- K_INDEX
    sfi                 INTEGER,                          -- SFI: solar flux index

    -------------------------------------------------------------------------
    -- SATELLITE
    -------------------------------------------------------------------------
    sat_name            TEXT,                             -- SAT_NAME
    sat_mode            TEXT,                             -- SAT_MODE

    -------------------------------------------------------------------------
    -- METEOR SCATTER
    -------------------------------------------------------------------------
    ms_shower           TEXT,                             -- MS_SHOWER: meteor shower
    max_bursts          INTEGER,                          -- MAX_BURSTS
    nr_bursts           INTEGER,                          -- NR_BURSTS
    nr_pings            INTEGER,                          -- NR_PINGS

    -------------------------------------------------------------------------
    -- CONTEST
    -------------------------------------------------------------------------
    contest_id          TEXT,                             -- CONTEST_ID
    srx                 INTEGER,                          -- SRX: serial received
    srx_string          TEXT,                             -- SRX_STRING: exchange received
    stx                 INTEGER,                          -- STX: serial sent
    stx_string          TEXT,                             -- STX_STRING: exchange sent
    class               TEXT,                             -- CLASS: contest class
    arrl_sect           TEXT,                             -- ARRL_SECT
    check_field         TEXT,                             -- CHECK: year licensed (renamed from CHECK)
    precedence          TEXT,                             -- PRECEDENCE
    region              TEXT,                             -- REGION

    -------------------------------------------------------------------------
    -- QSO META
    -------------------------------------------------------------------------
    qso_complete        qso_complete,                     -- QSO_COMPLETE
    qso_random          BOOLEAN,                          -- QSO_RANDOM
    force_init          BOOLEAN,                          -- FORCE_INIT
    swl                 BOOLEAN,                          -- SWL: shortwave listener

    -------------------------------------------------------------------------
    -- QSL - PAPER/TRADITIONAL
    -------------------------------------------------------------------------
    qsl_sent            qsl_sent_status,                  -- QSL_SENT: Y/N/R/Q/I
    qslsdate            DATE,                             -- QSLSDATE: date sent
    qsl_sent_via        qsl_via,                          -- QSL_SENT_VIA: B/D/E/M
    qsl_rcvd            qsl_status,                       -- QSL_RCVD: Y/N/R/I/V
    qslrdate            DATE,                             -- QSLRDATE: date received
    qsl_rcvd_via        qsl_via,                          -- QSL_RCVD_VIA
    qsl_via             TEXT,                             -- QSL_VIA: QSL route info
    qslmsg              TEXT,                             -- QSLMSG: message for QSL
    qslmsg_rcvd         TEXT,                             -- QSLMSG_RCVD: message received

    -------------------------------------------------------------------------
    -- QSL - LoTW
    -------------------------------------------------------------------------
    lotw_qsl_sent       qsl_sent_status,                  -- LOTW_QSL_SENT
    lotw_qslsdate       DATE,                             -- LOTW_QSLSDATE
    lotw_qsl_rcvd       qsl_status,                       -- LOTW_QSL_RCVD
    lotw_qslrdate       DATE,                             -- LOTW_QSLRDATE

    -------------------------------------------------------------------------
    -- QSL - eQSL
    -------------------------------------------------------------------------
    eqsl_qsl_sent       qsl_sent_status,                  -- EQSL_QSL_SENT
    eqsl_qslsdate       DATE,                             -- EQSL_QSLSDATE
    eqsl_qsl_rcvd       qsl_status,                       -- EQSL_QSL_RCVD
    eqsl_qslrdate       DATE,                             -- EQSL_QSLRDATE
    eqsl_ag             BOOLEAN,                          -- EQSL_AG: Authenticity Guaranteed

    -------------------------------------------------------------------------
    -- QSL - Club Log
    -------------------------------------------------------------------------
    clublog_qso_upload_date     DATE,                     -- CLUBLOG_QSO_UPLOAD_DATE
    clublog_qso_upload_status   qso_upload_status,        -- CLUBLOG_QSO_UPLOAD_STATUS

    -------------------------------------------------------------------------
    -- QSL - QRZ.com
    -------------------------------------------------------------------------
    qrzcom_qso_upload_date      DATE,                     -- QRZCOM_QSO_UPLOAD_DATE
    qrzcom_qso_upload_status    qso_upload_status,        -- QRZCOM_QSO_UPLOAD_STATUS
    qrzcom_qso_download_date    DATE,                     -- QRZCOM_QSO_DOWNLOAD_DATE
    qrzcom_qso_download_status  qso_upload_status,        -- QRZCOM_QSO_DOWNLOAD_STATUS

    -------------------------------------------------------------------------
    -- QSL - HRDLog
    -------------------------------------------------------------------------
    hrdlog_qso_upload_date      DATE,                     -- HRDLOG_QSO_UPLOAD_DATE
    hrdlog_qso_upload_status    qso_upload_status,        -- HRDLOG_QSO_UPLOAD_STATUS

    -------------------------------------------------------------------------
    -- QSL - HamLog.eu
    -------------------------------------------------------------------------
    hamlogeu_qso_upload_date    DATE,                     -- HAMLOGEU_QSO_UPLOAD_DATE
    hamlogeu_qso_upload_status  qso_upload_status,        -- HAMLOGEU_QSO_UPLOAD_STATUS

    -------------------------------------------------------------------------
    -- QSL - HamQTH
    -------------------------------------------------------------------------
    hamqth_qso_upload_date      DATE,                     -- HAMQTH_QSO_UPLOAD_DATE
    hamqth_qso_upload_status    qso_upload_status,        -- HAMQTH_QSO_UPLOAD_STATUS

    -------------------------------------------------------------------------
    -- QSL - DCL (DARC Community Log)
    -------------------------------------------------------------------------
    dcl_qsl_sent        qsl_sent_status,                  -- DCL_QSL_SENT
    dcl_qslsdate        DATE,                             -- DCL_QSLSDATE
    dcl_qsl_rcvd        qsl_status,                       -- DCL_QSL_RCVD
    dcl_qslrdate        DATE,                             -- DCL_QSLRDATE

    -------------------------------------------------------------------------
    -- AWARDS
    -------------------------------------------------------------------------
    award_submitted     TEXT,                             -- AWARD_SUBMITTED: list
    award_granted       TEXT,                             -- AWARD_GRANTED: list
    credit_submitted    TEXT,                             -- CREDIT_SUBMITTED: credit list
    credit_granted      TEXT,                             -- CREDIT_GRANTED: credit list

    -------------------------------------------------------------------------
    -- NOTES
    -------------------------------------------------------------------------
    comment             TEXT,                             -- COMMENT: single line
    notes               TEXT,                             -- NOTES: multiline
    public_key          TEXT,                             -- PUBLIC_KEY

    -------------------------------------------------------------------------
    -- APPLICATION-DEFINED / USER-DEFINED FIELDS
    -- Store as JSONB for flexibility with app-specific and user-defined fields
    -------------------------------------------------------------------------
    app_fields          JSONB,                            -- APP_* fields
    user_fields         JSONB,                            -- USERDEF* fields

    -------------------------------------------------------------------------
    -- METADATA
    -------------------------------------------------------------------------
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes for QSOs
CREATE INDEX IF NOT EXISTS idx_qsos_contact ON qsos(contact_id) WHERE contact_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qsos_call ON qsos(upper(call));
CREATE INDEX IF NOT EXISTS idx_qsos_date ON qsos(qso_date DESC, time_on DESC);
CREATE INDEX IF NOT EXISTS idx_qsos_band ON qsos(band);
CREATE INDEX IF NOT EXISTS idx_qsos_mode ON qsos(mode);
CREATE INDEX IF NOT EXISTS idx_qsos_dxcc ON qsos(dxcc) WHERE dxcc IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qsos_gridsquare ON qsos(gridsquare) WHERE gridsquare IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qsos_contest ON qsos(contest_id) WHERE contest_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qsos_iota ON qsos(iota) WHERE iota IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qsos_sota ON qsos(sota_ref) WHERE sota_ref IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qsos_pota ON qsos(pota_ref) WHERE pota_ref IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qsos_lotw_pending ON qsos(lotw_qsl_sent) WHERE lotw_qsl_sent = 'Y' AND lotw_qsl_rcvd IS DISTINCT FROM 'Y';
CREATE INDEX IF NOT EXISTS idx_qsos_paper_pending ON qsos(qsl_sent) WHERE qsl_sent = 'Y' AND qsl_rcvd IS DISTINCT FROM 'Y';

--------------------------------------------------------------------------------
-- TRIGGER FUNCTIONS: Define functions before triggers that use them
--------------------------------------------------------------------------------

-- Function to auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to clean/normalize QSO data
CREATE OR REPLACE FUNCTION clean_qso()
RETURNS TRIGGER AS $$
BEGIN
    -- Uppercase callsigns
    NEW.call = upper(btrim(NEW.call));
    NEW.station_callsign = nullif(upper(btrim(NEW.station_callsign)), '');
    NEW.operator = nullif(upper(btrim(NEW.operator)), '');
    NEW.contacted_op = nullif(upper(btrim(NEW.contacted_op)), '');

    -- Uppercase gridsquares
    NEW.gridsquare = nullif(upper(btrim(NEW.gridsquare)), '');
    NEW.my_gridsquare = nullif(upper(btrim(NEW.my_gridsquare)), '');

    -- Normalize band to lowercase
    NEW.band = nullif(lower(btrim(NEW.band)), '');
    NEW.band_rx = nullif(lower(btrim(NEW.band_rx)), '');

    -- Uppercase mode/submode
    NEW.mode = upper(btrim(NEW.mode));
    NEW.submode = nullif(upper(btrim(NEW.submode)), '');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to clean contact data
CREATE OR REPLACE FUNCTION clean_contact()
RETURNS TRIGGER AS $$
BEGIN
    -- Normalize whitespace in names
    NEW.name_given = nullif(btrim(NEW.name_given), '');
    NEW.name_additional = nullif(btrim(NEW.name_additional), '');
    NEW.name_family = nullif(btrim(NEW.name_family), '');
    NEW.name_display = btrim(regexp_replace(NEW.name_display, '\s+', ' ', 'g'));
    NEW.nickname = nullif(btrim(NEW.nickname), '');

    -- Uppercase call sign
    NEW.call_sign = nullif(upper(btrim(NEW.call_sign)), '');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to clean email data
CREATE OR REPLACE FUNCTION clean_email()
RETURNS TRIGGER AS $$
BEGIN
    NEW.email = lower(regexp_replace(NEW.email, '\s', '', 'g'));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

--------------------------------------------------------------------------------
-- TRIGGERS: Apply trigger functions
--------------------------------------------------------------------------------

-- Trigger for QSO updated_at
DROP TRIGGER IF EXISTS qsos_updated_at ON qsos;
CREATE TRIGGER qsos_updated_at
    BEFORE UPDATE ON qsos
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Trigger to clean/normalize QSO data
DROP TRIGGER IF EXISTS clean_qso_trigger ON qsos;
CREATE TRIGGER clean_qso_trigger
    BEFORE INSERT OR UPDATE ON qsos
    FOR EACH ROW EXECUTE FUNCTION clean_qso();

-- Trigger for contacts updated_at
DROP TRIGGER IF EXISTS contacts_updated_at ON contacts;
CREATE TRIGGER contacts_updated_at
    BEFORE UPDATE ON contacts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Trigger to clean contact data
DROP TRIGGER IF EXISTS clean_contact_trigger ON contacts;
CREATE TRIGGER clean_contact_trigger
    BEFORE INSERT OR UPDATE ON contacts
    FOR EACH ROW EXECUTE FUNCTION clean_contact();

-- Trigger to clean email data
DROP TRIGGER IF EXISTS clean_email_trigger ON contact_emails;
CREATE TRIGGER clean_email_trigger
    BEFORE INSERT OR UPDATE ON contact_emails
    FOR EACH ROW EXECUTE FUNCTION clean_email();

--------------------------------------------------------------------------------
-- VIEWS: Useful aggregations
--------------------------------------------------------------------------------

-- Full contact view with primary email/phone
CREATE OR REPLACE VIEW contact_view AS
SELECT
    c.*,
    (SELECT email FROM contact_emails WHERE contact_id = c.id
     ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_email,
    (SELECT phone FROM contact_phones WHERE contact_id = c.id
     ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_phone,
    (SELECT json_agg(t.name) FROM tags t
     JOIN contact_tags ct ON t.id = ct.tag_id
     WHERE ct.contact_id = c.id) AS tags,
    (SELECT MAX(logged_at) FROM contact_logs WHERE contact_id = c.id) AS last_crm_contact,
    (SELECT MAX(qso_date + time_on) FROM qsos WHERE contact_id = c.id) AS last_qso
FROM contacts c;

-- Contacts due for follow-up based on tier
CREATE OR REPLACE VIEW contacts_due AS
SELECT
    c.*,
    cv.primary_email,
    GREATEST(cv.last_crm_contact, cv.last_qso) AS last_contact,
    CASE c.tier
        WHEN 'A' THEN interval '3 weeks'
        WHEN 'B' THEN interval '2 months'
        WHEN 'C' THEN interval '6 months'
        WHEN 'D' THEN interval '1 year'
        WHEN 'E' THEN interval '2 years'
        WHEN 'F' THEN NULL  -- never
    END AS contact_interval,
    CASE
        WHEN c.tier = 'F' THEN false
        WHEN GREATEST(cv.last_crm_contact, cv.last_qso) IS NULL THEN true
        WHEN c.tier = 'A' AND GREATEST(cv.last_crm_contact, cv.last_qso) < now() - interval '3 weeks' THEN true
        WHEN c.tier = 'B' AND GREATEST(cv.last_crm_contact, cv.last_qso) < now() - interval '2 months' THEN true
        WHEN c.tier = 'C' AND GREATEST(cv.last_crm_contact, cv.last_qso) < now() - interval '6 months' THEN true
        WHEN c.tier = 'D' AND GREATEST(cv.last_crm_contact, cv.last_qso) < now() - interval '1 year' THEN true
        WHEN c.tier = 'E' AND GREATEST(cv.last_crm_contact, cv.last_qso) < now() - interval '2 years' THEN true
        ELSE false
    END AS is_due
FROM contacts c
JOIN contact_view cv ON cv.id = c.id
WHERE c.tier != 'F'
ORDER BY
    CASE c.tier WHEN 'A' THEN 1 WHEN 'B' THEN 2 WHEN 'C' THEN 3 WHEN 'D' THEN 4 WHEN 'E' THEN 5 END,
    GREATEST(cv.last_crm_contact, cv.last_qso) NULLS FIRST;

-- QSO log view with contact info (if linked)
CREATE OR REPLACE VIEW qso_log AS
SELECT
    q.*,
    c.name_display AS contact_name,
    c.call_sign AS contact_call_sign,
    -- Confirmation status summary
    COALESCE(q.lotw_qsl_rcvd = 'Y', false) AS confirmed_lotw,
    COALESCE(q.qsl_rcvd = 'Y', false) AS confirmed_paper,
    COALESCE(q.eqsl_qsl_rcvd = 'Y', false) AS confirmed_eqsl,
    (COALESCE(q.lotw_qsl_rcvd = 'Y', false) OR
     COALESCE(q.qsl_rcvd = 'Y', false) OR
     COALESCE(q.eqsl_qsl_rcvd = 'Y', false)) AS is_confirmed
FROM qsos q
LEFT JOIN contacts c ON c.id = q.contact_id
ORDER BY q.qso_date DESC, q.time_on DESC;

-- QSOs pending LoTW confirmation
CREATE OR REPLACE VIEW qso_pending_lotw AS
SELECT * FROM qso_log
WHERE lotw_qsl_sent = 'Y' AND NOT confirmed_lotw
ORDER BY qso_date DESC, time_on DESC;

-- QSOs pending paper QSL
CREATE OR REPLACE VIEW qso_pending_paper AS
SELECT * FROM qso_log
WHERE qsl_sent = 'Y' AND NOT confirmed_paper
ORDER BY qso_date DESC, time_on DESC;

-- QSOs not yet uploaded anywhere
CREATE OR REPLACE VIEW qso_not_uploaded AS
SELECT * FROM qso_log
WHERE lotw_qsl_sent IS DISTINCT FROM 'Y'
  AND eqsl_qsl_sent IS DISTINCT FROM 'Y'
  AND clublog_qso_upload_status IS DISTINCT FROM 'Y'
ORDER BY qso_date DESC, time_on DESC;

-- DXCC progress view
CREATE OR REPLACE VIEW dxcc_worked AS
SELECT
    dxcc,
    country,
    COUNT(*) AS qso_count,
    COUNT(*) FILTER (WHERE is_confirmed) AS confirmed_count,
    MIN(qso_date) AS first_qso,
    MAX(qso_date) AS last_qso,
    array_agg(DISTINCT band ORDER BY band) AS bands_worked,
    array_agg(DISTINCT mode ORDER BY mode) AS modes_worked
FROM qso_log
WHERE dxcc IS NOT NULL AND dxcc > 0
GROUP BY dxcc, country
ORDER BY country;

--------------------------------------------------------------------------------
-- HELPER FUNCTIONS
--------------------------------------------------------------------------------

-- Get or create a tag
CREATE OR REPLACE FUNCTION get_or_create_tag(tag_name TEXT)
RETURNS UUID AS $$
DECLARE
    tag_id UUID;
BEGIN
    SELECT id INTO tag_id FROM tags WHERE name = lower(tag_name);
    IF tag_id IS NULL THEN
        INSERT INTO tags (name) VALUES (lower(tag_name)) RETURNING id INTO tag_id;
    END IF;
    RETURN tag_id;
END;
$$ LANGUAGE plpgsql;

-- Generate vCard UID from contact id (for CardDAV)
CREATE OR REPLACE FUNCTION vcard_uid(contact_id UUID)
RETURNS TEXT AS $$
BEGIN
    RETURN contact_id::text || '@crm.local';
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Generate ETag from updated_at (for CardDAV sync)
CREATE OR REPLACE FUNCTION contact_etag(contact_id UUID)
RETURNS TEXT AS $$
DECLARE
    ts TIMESTAMPTZ;
BEGIN
    SELECT updated_at INTO ts FROM contacts WHERE id = contact_id;
    RETURN encode(digest(ts::text, 'sha1'), 'hex');
END;
$$ LANGUAGE plpgsql STABLE;

--------------------------------------------------------------------------------
-- ADIF HELPER FUNCTIONS
--------------------------------------------------------------------------------

-- Convert QSO to ADIF record (for export)
CREATE OR REPLACE FUNCTION qso_to_adif(qso_id UUID)
RETURNS TEXT AS $$
DECLARE
    q qsos%ROWTYPE;
    adif TEXT := '';
BEGIN
    SELECT * INTO q FROM qsos WHERE id = qso_id;
    IF NOT FOUND THEN RETURN NULL; END IF;

    -- Helper to add ADIF field
    -- Format: <FIELD:length>value

    -- Required fields
    adif := adif || '<CALL:' || length(q.call) || '>' || q.call;
    adif := adif || '<QSO_DATE:8>' || to_char(q.qso_date, 'YYYYMMDD');
    adif := adif || '<TIME_ON:' || CASE WHEN q.time_on IS NOT NULL THEN '6>' || to_char(q.time_on, 'HH24MISS') ELSE '4>' || to_char(q.time_on, 'HH24MI') END;
    adif := adif || '<MODE:' || length(q.mode) || '>' || q.mode;

    -- Optional fields (only if not null)
    IF q.band IS NOT NULL THEN
        adif := adif || '<BAND:' || length(q.band) || '>' || q.band;
    END IF;
    IF q.freq IS NOT NULL THEN
        adif := adif || '<FREQ:' || length(q.freq::text) || '>' || q.freq;
    END IF;
    IF q.submode IS NOT NULL THEN
        adif := adif || '<SUBMODE:' || length(q.submode) || '>' || q.submode;
    END IF;
    IF q.rst_sent IS NOT NULL THEN
        adif := adif || '<RST_SENT:' || length(q.rst_sent) || '>' || q.rst_sent;
    END IF;
    IF q.rst_rcvd IS NOT NULL THEN
        adif := adif || '<RST_RCVD:' || length(q.rst_rcvd) || '>' || q.rst_rcvd;
    END IF;
    IF q.tx_pwr IS NOT NULL THEN
        adif := adif || '<TX_PWR:' || length(q.tx_pwr::text) || '>' || q.tx_pwr;
    END IF;
    IF q.name IS NOT NULL THEN
        adif := adif || '<NAME:' || length(q.name) || '>' || q.name;
    END IF;
    IF q.qth IS NOT NULL THEN
        adif := adif || '<QTH:' || length(q.qth) || '>' || q.qth;
    END IF;
    IF q.gridsquare IS NOT NULL THEN
        adif := adif || '<GRIDSQUARE:' || length(q.gridsquare) || '>' || q.gridsquare;
    END IF;
    IF q.country IS NOT NULL THEN
        adif := adif || '<COUNTRY:' || length(q.country) || '>' || q.country;
    END IF;
    IF q.dxcc IS NOT NULL THEN
        adif := adif || '<DXCC:' || length(q.dxcc::text) || '>' || q.dxcc;
    END IF;
    IF q.cqz IS NOT NULL THEN
        adif := adif || '<CQZ:' || length(q.cqz::text) || '>' || q.cqz;
    END IF;
    IF q.ituz IS NOT NULL THEN
        adif := adif || '<ITUZ:' || length(q.ituz::text) || '>' || q.ituz;
    END IF;
    IF q.state IS NOT NULL THEN
        adif := adif || '<STATE:' || length(q.state) || '>' || q.state;
    END IF;
    IF q.iota IS NOT NULL THEN
        adif := adif || '<IOTA:' || length(q.iota) || '>' || q.iota;
    END IF;
    IF q.sota_ref IS NOT NULL THEN
        adif := adif || '<SOTA_REF:' || length(q.sota_ref) || '>' || q.sota_ref;
    END IF;
    IF q.pota_ref IS NOT NULL THEN
        adif := adif || '<POTA_REF:' || length(q.pota_ref) || '>' || q.pota_ref;
    END IF;
    IF q.station_callsign IS NOT NULL THEN
        adif := adif || '<STATION_CALLSIGN:' || length(q.station_callsign) || '>' || q.station_callsign;
    END IF;
    IF q.my_gridsquare IS NOT NULL THEN
        adif := adif || '<MY_GRIDSQUARE:' || length(q.my_gridsquare) || '>' || q.my_gridsquare;
    END IF;

    -- QSL fields
    IF q.qsl_sent IS NOT NULL THEN
        adif := adif || '<QSL_SENT:1>' || q.qsl_sent::text;
    END IF;
    IF q.qsl_rcvd IS NOT NULL THEN
        adif := adif || '<QSL_RCVD:1>' || q.qsl_rcvd::text;
    END IF;
    IF q.lotw_qsl_sent IS NOT NULL THEN
        adif := adif || '<LOTW_QSL_SENT:1>' || q.lotw_qsl_sent::text;
    END IF;
    IF q.lotw_qsl_rcvd IS NOT NULL THEN
        adif := adif || '<LOTW_QSL_RCVD:1>' || q.lotw_qsl_rcvd::text;
    END IF;

    -- Contest fields
    IF q.contest_id IS NOT NULL THEN
        adif := adif || '<CONTEST_ID:' || length(q.contest_id) || '>' || q.contest_id;
    END IF;
    IF q.srx IS NOT NULL THEN
        adif := adif || '<SRX:' || length(q.srx::text) || '>' || q.srx;
    END IF;
    IF q.stx IS NOT NULL THEN
        adif := adif || '<STX:' || length(q.stx::text) || '>' || q.stx;
    END IF;

    -- Notes
    IF q.comment IS NOT NULL THEN
        adif := adif || '<COMMENT:' || length(q.comment) || '>' || q.comment;
    END IF;

    adif := adif || '<EOR>';

    RETURN adif;
END;
$$ LANGUAGE plpgsql STABLE;

-- Export all QSOs to ADIF format
CREATE OR REPLACE FUNCTION export_adif(
    from_date DATE DEFAULT NULL,
    to_date DATE DEFAULT NULL
)
RETURNS TEXT AS $$
DECLARE
    header TEXT;
    records TEXT := '';
    q RECORD;
BEGIN
    -- ADIF header
    header := 'ADIF Export from Personal CRM' || E'\n';
    header := header || '<ADIF_VER:5>3.1.6' || E'\n';
    header := header || '<CREATED_TIMESTAMP:15>' || to_char(now() AT TIME ZONE 'UTC', 'YYYYMMDD HH24MISS') || E'\n';
    header := header || '<PROGRAMID:12>PersonalCRM' || E'\n';
    header := header || '<EOH>' || E'\n\n';

    -- Build records
    FOR q IN
        SELECT id FROM qsos
        WHERE (from_date IS NULL OR qso_date >= from_date)
          AND (to_date IS NULL OR qso_date <= to_date)
        ORDER BY qso_date, time_on
    LOOP
        records := records || qso_to_adif(q.id) || E'\n';
    END LOOP;

    RETURN header || records;
END;
$$ LANGUAGE plpgsql STABLE;

-- Try to link a QSO to an existing contact by callsign
CREATE OR REPLACE FUNCTION link_qso_to_contact(qso_id UUID)
RETURNS UUID AS $$
DECLARE
    qso_call TEXT;
    matched_contact_id UUID;
BEGIN
    SELECT call INTO qso_call FROM qsos WHERE id = qso_id;

    -- Find contact with matching call_sign
    SELECT id INTO matched_contact_id
    FROM contacts
    WHERE call_sign = qso_call
    LIMIT 1;

    IF matched_contact_id IS NOT NULL THEN
        UPDATE qsos SET contact_id = matched_contact_id WHERE id = qso_id;
    END IF;

    RETURN matched_contact_id;
END;
$$ LANGUAGE plpgsql;

-- Link all unlinked QSOs to contacts where possible
CREATE OR REPLACE FUNCTION link_all_qsos_to_contacts()
RETURNS INTEGER AS $$
DECLARE
    linked_count INTEGER := 0;
BEGIN
    WITH linked AS (
        UPDATE qsos q
        SET contact_id = c.id
        FROM contacts c
        WHERE q.contact_id IS NULL
          AND c.call_sign IS NOT NULL
          AND upper(q.call) = c.call_sign
        RETURNING q.id
    )
    SELECT COUNT(*) INTO linked_count FROM linked;

    RETURN linked_count;
END;
$$ LANGUAGE plpgsql;

--------------------------------------------------------------------------------
-- SAMPLE DATA (optional - remove in production)
--------------------------------------------------------------------------------

/*
INSERT INTO contacts (name_display, name_given, name_family, tier, call_sign)
VALUES ('John Doe', 'John', 'Doe', 'B', 'W1AW');

INSERT INTO contact_emails (contact_id, email, email_type, is_primary)
SELECT id, 'john@example.com', 'personal', true FROM contacts WHERE name_display = 'John Doe';

INSERT INTO contact_phones (contact_id, phone, phone_type, is_primary)
SELECT id, '+1-555-123-4567', 'cell', true FROM contacts WHERE name_display = 'John Doe';

INSERT INTO contact_urls (contact_id, url, url_type, username)
SELECT id, 'https://mastodon.social/@johndoe', 'mastodon', 'johndoe'
FROM contacts WHERE name_display = 'John Doe';

INSERT INTO contact_logs (contact_id, log_type, subject, content)
SELECT id, 'meeting', 'Coffee catch-up', 'Met at the usual place. Discussed ham radio projects.'
FROM contacts WHERE name_display = 'John Doe';
*/
