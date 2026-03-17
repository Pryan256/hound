DROP TABLE IF EXISTS finicity_customers;
DROP TABLE IF EXISTS provider_tokens;
DROP INDEX IF EXISTS idx_link_sessions_state;
ALTER TABLE link_sessions
    DROP COLUMN IF EXISTS institution_id,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS oauth_state,
    DROP COLUMN IF EXISTS oauth_state_expires_at;
ALTER TABLE link_tokens DROP COLUMN IF EXISTS redirect_uri;
