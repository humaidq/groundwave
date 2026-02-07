-- +goose Up
-- User invite tokens for provisioning additional accounts

CREATE TABLE IF NOT EXISTS user_invites (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token        TEXT NOT NULL UNIQUE,
    display_name TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at      TIMESTAMPTZ,
    created_by   UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_user_invites_used_at ON user_invites(used_at);
CREATE INDEX IF NOT EXISTS idx_user_invites_created_at ON user_invites(created_at DESC);

-- Health profile shares for non-admin access

CREATE TABLE IF NOT EXISTS health_profile_shares (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id UUID NOT NULL REFERENCES health_profiles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, profile_id)
);

CREATE INDEX IF NOT EXISTS idx_health_profile_shares_user ON health_profile_shares(user_id);
CREATE INDEX IF NOT EXISTS idx_health_profile_shares_profile ON health_profile_shares(profile_id);

-- +goose Down
DROP TABLE IF EXISTS health_profile_shares;
DROP TABLE IF EXISTS user_invites;
