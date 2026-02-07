-- +goose Up
-- WebAuthn-based authentication

CREATE TABLE IF NOT EXISTS users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name TEXT NOT NULL,
    is_admin     BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_passkeys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id   BYTEA NOT NULL UNIQUE,
    credential_data BYTEA NOT NULL,
    label           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_passkeys_user_id ON user_passkeys(user_id);
CREATE INDEX IF NOT EXISTS idx_user_passkeys_last_used ON user_passkeys(last_used_at);

-- +goose Down
DROP TABLE IF EXISTS user_passkeys;
DROP TABLE IF EXISTS users;
