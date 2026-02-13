-- +goose Up

DROP VIEW IF EXISTS dxcc_worked;
DROP VIEW IF EXISTS qso_not_uploaded;
DROP VIEW IF EXISTS qso_pending_paper;
DROP VIEW IF EXISTS qso_pending_lotw;
DROP VIEW IF EXISTS qso_log;

ALTER TABLE qsos
    ALTER COLUMN distance TYPE NUMERIC(20,15) USING distance::NUMERIC(20,15);

ALTER TABLE qsos
    ALTER COLUMN app_fields SET DEFAULT '{}'::jsonb,
    ALTER COLUMN user_fields SET DEFAULT '{}'::jsonb;

UPDATE qsos
SET app_fields = '{}'::jsonb
WHERE app_fields IS NULL;

UPDATE qsos
SET user_fields = '{}'::jsonb
WHERE user_fields IS NULL;

CREATE OR REPLACE VIEW qso_log AS
SELECT
    q.*,
    c.name_display AS contact_name,
    c.call_sign AS contact_call_sign,
    COALESCE(q.lotw_qsl_rcvd = 'Y', false) AS confirmed_lotw,
    COALESCE(q.qsl_rcvd = 'Y', false) AS confirmed_paper,
    COALESCE(q.eqsl_qsl_rcvd = 'Y', false) AS confirmed_eqsl,
    (COALESCE(q.lotw_qsl_rcvd = 'Y', false) OR
     COALESCE(q.qsl_rcvd = 'Y', false) OR
     COALESCE(q.eqsl_qsl_rcvd = 'Y', false)) AS is_confirmed
FROM qsos q
LEFT JOIN contacts c ON c.id = q.contact_id
ORDER BY q.qso_date DESC, q.time_on DESC;

CREATE OR REPLACE VIEW qso_pending_lotw AS
SELECT * FROM qso_log
WHERE lotw_qsl_sent = 'Y' AND NOT confirmed_lotw
ORDER BY qso_date DESC, time_on DESC;

CREATE OR REPLACE VIEW qso_pending_paper AS
SELECT * FROM qso_log
WHERE qsl_sent = 'Y' AND NOT confirmed_paper
ORDER BY qso_date DESC, time_on DESC;

CREATE OR REPLACE VIEW qso_not_uploaded AS
SELECT * FROM qso_log
WHERE lotw_qsl_sent IS DISTINCT FROM 'Y'
  AND eqsl_qsl_sent IS DISTINCT FROM 'Y'
  AND clublog_qso_upload_status IS DISTINCT FROM 'Y'
ORDER BY qso_date DESC, time_on DESC;

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

-- +goose Down

DROP VIEW IF EXISTS dxcc_worked;
DROP VIEW IF EXISTS qso_not_uploaded;
DROP VIEW IF EXISTS qso_pending_paper;
DROP VIEW IF EXISTS qso_pending_lotw;
DROP VIEW IF EXISTS qso_log;

ALTER TABLE qsos
    ALTER COLUMN app_fields DROP DEFAULT,
    ALTER COLUMN user_fields DROP DEFAULT;

ALTER TABLE qsos
    ALTER COLUMN distance TYPE INTEGER USING ROUND(distance)::INTEGER;

CREATE OR REPLACE VIEW qso_log AS
SELECT
    q.*,
    c.name_display AS contact_name,
    c.call_sign AS contact_call_sign,
    COALESCE(q.lotw_qsl_rcvd = 'Y', false) AS confirmed_lotw,
    COALESCE(q.qsl_rcvd = 'Y', false) AS confirmed_paper,
    COALESCE(q.eqsl_qsl_rcvd = 'Y', false) AS confirmed_eqsl,
    (COALESCE(q.lotw_qsl_rcvd = 'Y', false) OR
     COALESCE(q.qsl_rcvd = 'Y', false) OR
     COALESCE(q.eqsl_qsl_rcvd = 'Y', false)) AS is_confirmed
FROM qsos q
LEFT JOIN contacts c ON c.id = q.contact_id
ORDER BY q.qso_date DESC, q.time_on DESC;

CREATE OR REPLACE VIEW qso_pending_lotw AS
SELECT * FROM qso_log
WHERE lotw_qsl_sent = 'Y' AND NOT confirmed_lotw
ORDER BY qso_date DESC, time_on DESC;

CREATE OR REPLACE VIEW qso_pending_paper AS
SELECT * FROM qso_log
WHERE qsl_sent = 'Y' AND NOT confirmed_paper
ORDER BY qso_date DESC, time_on DESC;

CREATE OR REPLACE VIEW qso_not_uploaded AS
SELECT * FROM qso_log
WHERE lotw_qsl_sent IS DISTINCT FROM 'Y'
  AND eqsl_qsl_sent IS DISTINCT FROM 'Y'
  AND clublog_qso_upload_status IS DISTINCT FROM 'Y'
ORDER BY qso_date DESC, time_on DESC;

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
