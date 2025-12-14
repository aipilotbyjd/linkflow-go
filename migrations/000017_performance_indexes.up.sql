-- ============================================================================
-- Migration: 000017_performance_indexes
-- Description: Create performance optimization indexes
-- Note: Removed CONCURRENTLY as it cannot run inside migration transactions
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Auth Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_users_email_lower 
    ON auth.users(LOWER(email));

CREATE INDEX IF NOT EXISTS idx_perf_sessions_expires_active 
    ON auth.sessions(expires_at) WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_perf_api_keys_active 
    ON auth.api_keys(user_id, is_active) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Workflow Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_workflows_user_active 
    ON workflow.workflows(user_id, is_active) WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_perf_workflows_updated_at 
    ON workflow.workflows(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_perf_workflows_team_status 
    ON workflow.workflows(team_id, status) WHERE team_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Execution Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_executions_workflow_status 
    ON execution.workflow_executions(workflow_id, status);

CREATE INDEX IF NOT EXISTS idx_perf_executions_started_at 
    ON execution.workflow_executions(started_at DESC);

CREATE INDEX IF NOT EXISTS idx_perf_executions_status_created 
    ON execution.workflow_executions(status, created_at DESC) 
    WHERE status IN ('pending', 'queued', 'running');

CREATE INDEX IF NOT EXISTS idx_perf_node_executions_exec_status 
    ON execution.node_executions(execution_id, status);

CREATE INDEX IF NOT EXISTS idx_perf_queue_pending 
    ON execution.execution_queue(status, priority DESC, scheduled_at) 
    WHERE status = 'pending';

-- ---------------------------------------------------------------------------
-- Schedule Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_schedules_next_run 
    ON schedule.schedules(next_run_at) WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_perf_schedules_workflow_active 
    ON schedule.schedules(workflow_id, is_active) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Webhook Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_webhooks_path_method 
    ON webhook.webhooks(path, method) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Credential Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_credentials_user_type 
    ON credential.credentials(user_id, type) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Notification Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_notifications_user_unread 
    ON notification.notifications(user_id, created_at DESC) WHERE is_read = FALSE;

CREATE INDEX IF NOT EXISTS idx_perf_queue_pending_retry 
    ON notification.queue(next_retry_at) WHERE status = 'pending';

-- ---------------------------------------------------------------------------
-- Audit Schema Indexes (BRIN for time-series data)
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_audit_logs_created_brin 
    ON audit.logs USING BRIN(created_at);

CREATE INDEX IF NOT EXISTS idx_perf_security_events_created_brin 
    ON audit.security_events USING BRIN(created_at);

-- ---------------------------------------------------------------------------
-- Analytics Schema Indexes (BRIN for time-series data)
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_system_metrics_recorded_brin 
    ON analytics.system_metrics USING BRIN(recorded_at);

CREATE INDEX IF NOT EXISTS idx_perf_workflow_stats_date 
    ON analytics.workflow_stats(date DESC);

CREATE INDEX IF NOT EXISTS idx_perf_user_activity_date 
    ON analytics.user_activity(date DESC);

-- ---------------------------------------------------------------------------
-- Search Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_workflow_index_search_gin 
    ON search.workflow_index USING GIN(search_vector);

CREATE INDEX IF NOT EXISTS idx_perf_search_history_user_created 
    ON search.search_history(user_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- Storage Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_files_user_created 
    ON storage.files(user_id, created_at DESC) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Billing Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_subscriptions_status_period 
    ON billing.subscriptions(status, current_period_end) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_perf_invoices_user_status 
    ON billing.invoices(user_id, status, created_at DESC);

-- ---------------------------------------------------------------------------
-- Template Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_perf_templates_status_featured 
    ON template.templates(status, is_featured, use_count DESC) WHERE status = 'published';

COMMIT;
