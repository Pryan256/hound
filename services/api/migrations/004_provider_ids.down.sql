DROP INDEX IF EXISTS idx_transactions_provider;
ALTER TABLE transactions DROP COLUMN IF EXISTS provider_transaction_id;

DROP INDEX IF EXISTS idx_accounts_provider;
ALTER TABLE accounts DROP COLUMN IF EXISTS provider_account_id;
