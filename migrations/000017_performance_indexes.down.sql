-- ============================================================================
-- Migration: 000017_performance_indexes (ROLLBACK)
-- Description: Drop performance optimization indexes
-- ============================================================================

BEGIN;

-- Template Schema
DROP INDEX IF EXISTS idx_perf_templates_status_featured;

-- Billing Schema
DROP INDEX IF EXISTS idx_perf_invoices_user_status;
DROP INDEX IF EXISTS idx_perf_subscriptions_status_period;

-- Storage Schema
DROP INDEX IF EXISTS idx_perf_files_user_created;

-- Search Schema
DROP INDEX IF EXISTS idx_perf_search_history_user_created;
DROP INDEX IF EXISTS idx_perf_workflow_index_search_gin;

-- Analytics Schema
DROP INDEX IF EXISTS idx_perf_user_activity_date;
DROP INDEX IF EXISTS idx_perf_workflow_stats_date;
DROP INDEX IF EXISTS idx_perf_system_metrics_recorded_brin;

-- Audit Schema
DROP INDEX IF EXISTS idx_perf_security_events_created_brin;
DROP INDEX IF EXISTS idx_perf_audit_logs_created_brin;

-- Notification Schema
DROP INDEX IF EXISTS idx_perf_queue_pending_retry;
DROP INDEX IF EXISTS idx_perf_notifications_user_unread;

-- Credential Schema
DROP INDEX IF EXISTS idx_perf_credentials_user_type;

-- Webhook Schema
DROP INDEX IF EXISTS idx_perf_webhooks_path_method;

-- Schedule Schema
DROP INDEX IF EXISTS idx_perf_schedules_workflow_active;
DROP INDEX IF EXISTS idx_perf_schedules_next_run;

-- Execution Schema
DROP INDEX IF EXISTS idx_perf_queue_pending;
DROP INDEX IF EXISTS idx_perf_node_executions_exec_status;
DROP INDEX IF EXISTS idx_perf_executions_status_created;
DROP INDEX IF EXISTS idx_perf_executions_started_at;
DROP INDEX IF EXISTS idx_perf_executions_workflow_status;

-- Workflow Schema
DROP INDEX IF EXISTS idx_perf_workflows_team_status;
DROP INDEX IF EXISTS idx_perf_workflows_updated_at;
DROP INDEX IF EXISTS idx_perf_workflows_user_active;

-- Auth Schema
DROP INDEX IF EXISTS idx_perf_api_keys_active;
DROP INDEX IF EXISTS idx_perf_sessions_expires_active;
DROP INDEX IF EXISTS idx_perf_users_email_lower;

COMMIT;
