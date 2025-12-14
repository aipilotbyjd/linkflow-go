-- ============================================================================
-- Migration: 000012_analytics_tables (ROLLBACK)
-- Description: Drop analytics and metrics tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_user_activity_updated_at ON analytics.user_activity;
DROP TRIGGER IF EXISTS trg_workflow_stats_updated_at ON analytics.workflow_stats;

DROP TABLE IF EXISTS analytics.platform_stats;
DROP TABLE IF EXISTS analytics.system_metrics;
DROP TABLE IF EXISTS analytics.node_usage;
DROP TABLE IF EXISTS analytics.user_activity;
DROP TABLE IF EXISTS analytics.workflow_stats;

COMMIT;
