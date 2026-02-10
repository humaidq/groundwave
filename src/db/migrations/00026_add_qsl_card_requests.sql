-- +goose Up
-- Store public physical QSL card requests separately from QSO confirmation state.

CREATE TABLE IF NOT EXISTS qsl_card_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    qso_id          UUID NOT NULL REFERENCES qsos(id) ON DELETE CASCADE,
    requester_name  TEXT,
    mailing_address TEXT NOT NULL
                    CONSTRAINT qsl_card_requests_mailing_address_not_empty CHECK (length(trim(mailing_address)) > 0),
    note            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    dismissed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_qsl_card_requests_qso_id ON qsl_card_requests(qso_id);
CREATE INDEX IF NOT EXISTS idx_qsl_card_requests_open ON qsl_card_requests(created_at DESC) WHERE dismissed_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS qsl_card_requests;
