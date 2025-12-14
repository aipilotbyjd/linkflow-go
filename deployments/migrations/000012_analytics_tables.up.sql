-- ============================================================================
-- Migration: 000012_analytics_tables
-- Description: Create analytics and metrics tables
-- Schema: analytics
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Workflow Stats table - Daily workflow statistics
-- ---------------------------------------------------------------------------
CREATE TABLE analytics.workflow_stats (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    date            DATE NOT NULL,
    
    -- Execution counts
    total_executions    INTEGER DEFAULT 0,
    successful_executions INTEGER DEFAULT 0,
    failed_executions   INTEGER DEFAULT 0,
    cancelled_executions INTEGER DEFAULT 0,
    
    -- Timing stats (in milliseconds)
    avg_duration_ms     BIGINT,
    min_duration_ms     BIGINT,
    max_duration_ms     BIGINT,
    p50_duration_ms     BIGINT,
    p95_duration_ms     BIGINT,
    p99_duration_ms     BIGINT,
    
    -- Node stats
    total_nodes_executed INTEGER DEFAULT 0,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT workflow_stats_unique UNIQUE (workflow_id, date)
);

CREATE INDEX idx_workflow_stats_workflow_id ON analytics.workflow_stats(workflow_id);
CREATE INDEX idx_workflow_stats_date ON analytics.workflow_stats(date DESC);

-- ---------------------------------------------------------------------------
-- User Activity table - Daily user activity
-- ---------------------------------------------------------------------------
CREATE TABLE analytics.user_activity (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    date            DATE NOT NULL,
    
    -- Activity counts
    workflows_created   INTEGER DEFAULT 0,
    workflows_updated   INTEGER DEFAULT 0,
    workflows_executed  INTEGER DEFAULT 0,
    api_calls           INTEGER DEFAULT 0,
    login_count         INTEGER DEFAULT 0,
    
    -- Session info
    total_session_duration_ms BIGINT DEFAULT 0,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT user_activity_unique UNIQUE (user_id, date)
);

CREATE INDEX idx_user_activity_user_id ON analytics.user_activity(user_id);
CREATE INDEX idx_user_activity_date ON analytics.user_activity(date DESC);

-- ---------------------------------------------------------------------------
-- Node Usage table - Node type usage statistics
-- ---------------------------------------------------------------------------
CREATE TABLE analytics.node_usage (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    node_type       VARCHAR(100) NOT NULL,
    date            DATE NOT NULL,
    
    -- Usage counts
    usage_count         INTEGER DEFAULT 0,
    success_count       INTEGER DEFAULT 0,
    failure_count       INTEGER DEFAULT 0,
    
    -- Timing
    avg_duration_ms     BIGINT,
    
    -- Unique users/workflows
    unique_users        INTEGER DEFAULT 0,
    unique_workflows    INTEGER DEFAULT 0,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT node_usage_unique UNIQUE (node_type, date)
);

CREATE INDEX idx_node_usage_node_type ON analytics.node_usage(node_type);
CREATE INDEX idx_node_usage_date ON analytics.node_usage(date DESC);

-- ---------------------------------------------------------------------------
-- System Metrics table - System-level metrics
-- ---------------------------------------------------------------------------
CREATE TABLE analytics.system_metrics (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    metric_name     VARCHAR(100) NOT NULL,
    metric_value    DOUBLE PRECISION NOT NULL,
    metric_unit     VARCHAR(20),
    
    -- Dimensions
    labels          JSONB DEFAULT '{}',
    
    recorded_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_system_metrics_name ON analytics.system_metrics(metric_name);
CREATE INDEX idx_system_metrics_recorded_at ON analytics.system_metrics(recorded_at DESC);
CREATE INDEX idx_system_metrics_name_time ON analytics.system_metrics(metric_name, recorded_at DESC);

-- ---------------------------------------------------------------------------
-- Platform Stats table - Overall platform statistics
-- ---------------------------------------------------------------------------
CREATE TABLE analytics.platform_stats (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    date            DATE NOT NULL,
    
    -- User stats
    total_users         INTEGER DEFAULT 0,
    active_users        INTEGER DEFAULT 0,
    new_users           INTEGER DEFAULT 0,
    
    -- Workflow stats
    total_workflows     INTEGER DEFAULT 0,
    active_workflows    INTEGER DEFAULT 0,
    new_workflows       INTEGER DEFAULT 0,
    
    -- Execution stats
    total_executions    INTEGER DEFAULT 0,
    successful_executions INTEGER DEFAULT 0,
    failed_executions   INTEGER DEFAULT 0,
    
    -- Resource usage
    total_api_calls     BIGINT DEFAULT 0,
    total_storage_bytes BIGINT DEFAULT 0,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT platform_stats_date_unique UNIQUE (date)
);

CREATE INDEX idx_platform_stats_date ON analytics.platform_stats(date DESC);

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_workflow_stats_updated_at 
    BEFORE UPDATE ON analytics.workflow_stats
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_user_activity_updated_at 
    BEFORE UPDATE ON analytics.user_activity
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
