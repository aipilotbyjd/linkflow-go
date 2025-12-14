-- ============================================================================
-- Migration: 000008_webhook_tables (ROLLBACK)
-- Description: Drop webhook handling tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_webhook_logs_update_stats ON webhook.webhook_logs;
DROP FUNCTION IF EXISTS webhook.update_webhook_stats();
DROP TRIGGER IF EXISTS trg_webhooks_updated_at ON webhook.webhooks;

DROP TABLE IF EXISTS webhook.webhook_logs;
DROP TABLE IF EXISTS webhook.webhooks;

COMMIT;
