ALTER TABLE link_sessions DROP COLUMN IF EXISTS pkce_code_verifier;

ALTER TABLE institutions DROP CONSTRAINT IF EXISTS institutions_provider_check;
ALTER TABLE institutions ADD CONSTRAINT institutions_provider_check
    CHECK (provider IN ('akoya', 'finicity', 'sandbox'));
