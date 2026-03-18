DROP TABLE IF EXISTS webhook_deliveries;
ALTER TABLE webhooks DROP COLUMN IF EXISTS secret_encrypted;
