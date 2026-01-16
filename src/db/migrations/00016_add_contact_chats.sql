-- Add contact chat history tracking

-- +goose Up
-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE chat_platform AS ENUM ('manual', 'email', 'whatsapp', 'signal', 'wechat', 'teams', 'slack', 'other');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE chat_sender AS ENUM ('me', 'them', 'mix');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS contact_chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    platform chat_platform NOT NULL DEFAULT 'manual',
    sender chat_sender NOT NULL DEFAULT 'them',
    message TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contact_chats_contact_id ON contact_chats(contact_id);
CREATE INDEX IF NOT EXISTS idx_contact_chats_sent_at ON contact_chats(contact_id, sent_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_contact_chats_sent_at;
DROP INDEX IF EXISTS idx_contact_chats_contact_id;
DROP TABLE IF EXISTS contact_chats;
DROP TYPE IF EXISTS chat_sender;
DROP TYPE IF EXISTS chat_platform;
