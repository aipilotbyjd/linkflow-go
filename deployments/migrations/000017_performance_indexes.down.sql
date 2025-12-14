-- ============================================================================
-- Migration: 000017_performance_indexes (ROLLBACK)
-- Description: Drop performance optimization indexes
-- ============================================================================

BEGIN;

-- Auth Schema
DROP INDEX CONCURRENTLY IF EXISTS auth.idx_users_email_lower;
DROP INDEX CONCURRENTLY IF EXISTS auth.idx_sessions_expires_at;

-- Workflow Schema
DROP INDEX CONCURRENTLY IF EXISTS workflow.idx_workflows_user_active;
DROP INDEX CONCURRENTLY IF EXISTS workflow.idx_workflows_updated_at;

-- Execution Schema
DROP INDEX CONCURRENTLY IF EXISTS execution.idx_executions_workflow_status;
DROP INDEX CONCURRENTLY IF EXISTS execution.idx_executions_started_at;
DROP INDEX CONCURRENTLY IF EXISTS execution.idx_node_executions_execution_status;

-- Schedule Schema
DROP INDEX CONCURRENTLY IF EXISTS schedule.idx_schedules_next_run;

-- Webhook Schema
DROP INDEX CONCURRENTLY IF EXISTS webhook.idx_webhooks_path;

-- Notification Schema
DROP INDEX CONCURRENTLY IF EXISTS notification.idx_notifications_user_unread;

-- Audit Schema
DROP INDEX CONCURRENTLY IF EXISTS audit.idx_audit_logs_created_at_brin;

-- Analytics Schema
DROP INDEX CONCURRENTLY IF EXISTS analytics.idx_events_timestamp_brin;

-- Search Schema
DROP INDEX CONCURRENTLY IF EXISTS search.idx_search_index_content_gin;

COMMIT;
