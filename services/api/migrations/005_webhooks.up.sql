-- The original webhooks.secret_hash column stored a SHA-256 hash, which can't
-- be reversed for HMAC signing. Add a properly encrypted column instead.
ALTER TABLE webhooks ADD COLUMN IF NOT EXISTS secret_encrypted TEXT;

-- Webhook delivery log: every outgoing HTTP attempt, including retries.
CREATE TABLE webhook_deliveries (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id       UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type       TEXT NOT NULL,
    payload          JSONB NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts         INT NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at     TIMESTAMPTZ,
    last_http_status INT,
    last_error       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only index pending rows — delivered/failed rows don't need fast lookup.
CREATE INDEX idx_webhook_deliveries_pending
    ON webhook_deliveries(next_attempt_at)
    WHERE status = 'pending';
