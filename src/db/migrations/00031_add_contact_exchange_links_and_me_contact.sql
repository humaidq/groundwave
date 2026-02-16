-- +goose Up

ALTER TABLE contacts
ADD COLUMN IF NOT EXISTS is_me BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_single_me
ON contacts (is_me)
WHERE is_me = true;

CREATE TABLE IF NOT EXISTS contact_exchange_links (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id    UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    token         TEXT NOT NULL UNIQUE,
    collect_phone BOOLEAN NOT NULL,
    collect_email BOOLEAN NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    used_at       TIMESTAMPTZ,
    CONSTRAINT contact_exchange_links_collect_check CHECK (collect_phone OR collect_email)
);

CREATE INDEX IF NOT EXISTS idx_contact_exchange_links_contact_id
ON contact_exchange_links(contact_id);

CREATE INDEX IF NOT EXISTS idx_contact_exchange_links_active_contact
ON contact_exchange_links(contact_id, created_at DESC)
WHERE used_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_contact_exchange_links_expires_at
ON contact_exchange_links(expires_at);

-- +goose Down

DROP INDEX IF EXISTS idx_contact_exchange_links_expires_at;
DROP INDEX IF EXISTS idx_contact_exchange_links_active_contact;
DROP INDEX IF EXISTS idx_contact_exchange_links_contact_id;
DROP TABLE IF EXISTS contact_exchange_links;

DROP INDEX IF EXISTS idx_contacts_single_me;

ALTER TABLE contacts
DROP COLUMN IF EXISTS is_me;
