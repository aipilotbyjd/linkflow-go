-- ============================================================================
-- Migration: 000017_performance_indexes (ROLLBACK)
-- Description: Drop performance optimization indexes
-- ============================================================================

-- Auth Schema
DROP INDEX IF EXISTS auth.idx_users_email_lower;
DROP INDEX IF EXISTS auth.idx_sessions_expires_at;

-- Workflow Schema
DROP INDEX IF EXISTS workflow.idx_workflows_user_active;
DROP INDEX IF EXISTS workflow.idx_workflows_updated_at;

-- Execution Schema
DROP INDEX IF EXISTS execution.idx_executions_workflow_status;
DROP INDEX IF EXISTS execution.idx_executions_started_at;
DROP INDEX IF EXISTS execution.idx_node_executions_execution_status;

-- Schedule Schema
DROP INDEX IF EXISTS schedule.idx_schedules_next_run;

-- Webhook Schema
DROP INDEX IF EXISTS webhook.idx_webhooks_path;

-- Notification Schema
DROP INDEX IF EXISTS notification.idx_notifications_user_unread;

-- Audit Schema
DROP INDEX IF EXISTS audit.idx_audit_logs_created_at_brin;

-- Analytics Schema
DROP INDEX IF EXISTS analytics.idx_events_timestamp_brin;

-- Search Schema
DROP INDEX IF EXISTS search.idx_search_index_content_gin;
