-- Add provider tracking IDs for upsert deduplication.
-- These are the raw IDs from the aggregator (Akoya accountId, transactionId).

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS provider_account_id TEXT;

-- Unique per item so the same bank account is never duplicated.
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_provider
    ON accounts(item_id, provider_account_id)
    WHERE provider_account_id IS NOT NULL;

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS provider_transaction_id TEXT;

-- TimescaleDB requires the partition key (date) in every unique index.
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_provider
    ON transactions(account_id, provider_transaction_id, date)
    WHERE provider_transaction_id IS NOT NULL;
