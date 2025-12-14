-- ============================================================================
-- Migration: 000017_performance_indexes
-- Description: Create performance optimization indexes
-- Note: Removed CONCURRENTLY as it cannot run inside migration transactions
-- ============================================================================

-- ---------------------------------------------------------------------------
-- Auth Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_users_email_lower 
    ON auth.users(LOWER(email));

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at 
    ON auth.sessions(expires_at) WHERE expires_at > NOW();

-- ---------------------------------------------------------------------------
-- Workflow Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_workflows_user_active 
    ON workflow.workflows(user_id, is_active) WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_workflows_updated_at 
    ON workflow.workflows(updated_at DESC);

-- ---------------------------------------------------------------------------
-- Execution Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_executions_workflow_status 
    ON execution.executions(workflow_id, status);

CREATE INDEX IF NOT EXISTS idx_executions_started_at 
    ON execution.executions(started_at DESC);

CREATE INDEX IF NOT EXISTS idx_node_executions_execution_status 
    ON execution.node_executions(execution_id, status);

-- ---------------------------------------------------------------------------
-- Schedule Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_schedules_next_run 
    ON schedule.schedules(next_run_at) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Webhook Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_webhooks_path 
    ON webhook.webhooks(path);

-- ---------------------------------------------------------------------------
-- Notification Schema Indexes
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread 
    ON notification.notifications(user_id, is_read) WHERE is_read = FALSE;

-- ---------------------------------------------------------------------------
-- Audit Schema Indexes (BRIN for time-series data)
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at_brin 
    ON audit.audit_logs USING BRIN(created_at);

-- ---------------------------------------------------------------------------
-- Analytics Schema Indexes (BRIN for time-series data)
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_events_timestamp_brin 
    ON analytics.events USING BRIN(timestamp);

-- ---------------------------------------------------------------------------
-- Search Schema Indexes (GIN for full-text search)
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_search_index_content_gin 
    ON search.search_index USING GIN(content);
