-- Allow 'fdx' as a valid provider in the institutions constraint.
ALTER TABLE institutions
    DROP CONSTRAINT IF EXISTS institutions_provider_check;
ALTER TABLE institutions
    ADD CONSTRAINT institutions_provider_check
    CHECK (provider IN ('akoya', 'finicity', 'sandbox', 'fdx'));

-- Store PKCE code verifier alongside the OAuth state so the callback
-- handler can pass it to ExchangeCode without a round-trip.
ALTER TABLE link_sessions
    ADD COLUMN IF NOT EXISTS pkce_code_verifier TEXT NOT NULL DEFAULT '';
