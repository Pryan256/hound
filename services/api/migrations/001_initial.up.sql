-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Applications (developer accounts)
CREATE TABLE applications (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- API keys (hashed — we never store raw keys)
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    key_hash        TEXT NOT NULL UNIQUE,  -- SHA-256 of the raw key
    env             TEXT NOT NULL CHECK (env IN ('test', 'live')),
    label           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash) WHERE revoked_at IS NULL;

-- Link tokens (short-lived, used to initialize Link UI)
CREATE TABLE link_tokens (
    token           TEXT PRIMARY KEY,
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL,
    products        TEXT[] NOT NULL,
    env             TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL
);

-- Items (a user<>institution connection)
CREATE TABLE items (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id   UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL CHECK (provider IN ('akoya', 'finicity', 'sandbox')),
    provider_item_id TEXT NOT NULL,  -- encrypted at app level before storage
    institution_id   TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('active', 'pending', 'error', 'revoked')),
    consent_expiry   TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_items_application ON items(application_id) WHERE status != 'revoked';

-- Link sessions (tracks the OAuth/Link flow, stores public token before exchange)
CREATE TABLE link_sessions (
    id                       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id           UUID NOT NULL REFERENCES applications(id),
    link_token               TEXT NOT NULL REFERENCES link_tokens(token),
    item_id                  UUID REFERENCES items(id),
    public_token             TEXT UNIQUE,
    public_token_expires_at  TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Access tokens (durable, hashed)
CREATE TABLE access_tokens (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_hash      TEXT NOT NULL UNIQUE,  -- SHA-256 of raw token
    item_id         UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    application_id  UUID NOT NULL REFERENCES applications(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);

CREATE INDEX idx_access_tokens_hash ON access_tokens(token_hash) WHERE revoked_at IS NULL;

-- Accounts
CREATE TABLE accounts (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_id        UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    official_name  TEXT,
    type           TEXT NOT NULL,
    subtype        TEXT,
    mask           TEXT,
    balances       JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_item ON accounts(item_id);

-- Transactions (hypertable for time-series performance)
CREATE TABLE transactions (
    id                        UUID NOT NULL DEFAULT uuid_generate_v4(),
    account_id                UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    amount                    NUMERIC(12, 2) NOT NULL,
    currency                  CHAR(3) NOT NULL DEFAULT 'USD',
    date                      DATE NOT NULL,
    authorized_date           DATE,
    name                      TEXT NOT NULL,
    merchant_name             TEXT,
    category                  TEXT[],
    category_id               TEXT,
    pending                   BOOLEAN NOT NULL DEFAULT false,
    payment_channel           TEXT,
    location                  JSONB,
    personal_finance_category JSONB,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, date)
);

-- Convert to TimescaleDB hypertable partitioned by date
SELECT create_hypertable('transactions', 'date', chunk_time_interval => INTERVAL '1 month');

CREATE INDEX idx_transactions_account_date ON transactions(account_id, date DESC);

-- Webhooks configuration
CREATE TABLE webhooks (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    url             TEXT NOT NULL,
    events          TEXT[] NOT NULL DEFAULT '{}',
    secret_hash     TEXT NOT NULL,  -- HMAC signing secret (hashed for storage)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
