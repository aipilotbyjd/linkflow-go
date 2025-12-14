-- ============================================================================
-- Migration: 000010_notification_tables (ROLLBACK)
-- Description: Drop notification tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_notification_preferences_updated_at ON notification.preferences;
DROP TRIGGER IF EXISTS trg_notification_channels_updated_at ON notification.channels;

DROP TABLE IF EXISTS notification.queue;
DROP TABLE IF EXISTS notification.notifications;
DROP TABLE IF EXISTS notification.preferences;
DROP TABLE IF EXISTS notification.channels;

COMMIT;
