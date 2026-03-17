-- Add redirect_uri to link_tokens (required for OAuth flow)
ALTER TABLE link_tokens ADD COLUMN redirect_uri TEXT;

-- Add OAuth columns to link_sessions
ALTER TABLE link_sessions
    ADD COLUMN institution_id TEXT,
    ADD COLUMN provider       TEXT,
    ADD COLUMN oauth_state    TEXT UNIQUE,   -- random nonce, validated on callback (CSRF)
    ADD COLUMN oauth_state_expires_at TIMESTAMPTZ;

CREATE INDEX idx_link_sessions_state ON link_sessions(oauth_state)
    WHERE oauth_state IS NOT NULL;

-- provider_tokens: encrypted access/refresh tokens per item (separate from items table
-- so we can rotate without touching the item record)
CREATE TABLE provider_tokens (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_id       UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    access_token  TEXT NOT NULL,   -- AES-256-GCM encrypted
    refresh_token TEXT,            -- AES-256-GCM encrypted, nullable
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_provider_tokens_item ON provider_tokens(item_id);

-- finicity_customers: maps our (application_id, user_id) to a Finicity customer ID
-- Finicity requires a customer to be created before generating Connect URLs
CREATE TABLE finicity_customers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id  UUID NOT NULL REFERENCES applications(id),
    user_id         TEXT NOT NULL,
    finicity_id     TEXT NOT NULL,
    env             TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (application_id, user_id, env)
);
