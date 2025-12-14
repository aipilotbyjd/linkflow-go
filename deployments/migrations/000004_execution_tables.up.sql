-- ============================================================================
-- Migration: 000004_execution_tables
-- Description: Create workflow execution tables
-- Schema: execution
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Workflow Executions table - Execution instances
-- ---------------------------------------------------------------------------
CREATE TABLE execution.workflow_executions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    workflow_version INTEGER,
    
    -- Execution status
    status          VARCHAR(20) DEFAULT 'pending' CHECK (status IN (
        'pending', 'queued', 'running', 'paused', 'completed', 'failed', 'cancelled', 'timeout'
    )),
    
    -- Trigger info
    trigger_type    VARCHAR(30) CHECK (trigger_type IN ('manual', 'schedule', 'webhook', 'api', 'retry')),
    triggered_by    UUID REFERENCES auth.users(id),
    
    -- Timing
    started_at      TIMESTAMP,
    finished_at     TIMESTAMP,
    duration_ms     BIGINT,
    
    -- Data
    input_data      JSONB DEFAULT '{}',
    output_data     JSONB DEFAULT '{}',
    
    -- Error handling
    error_message   TEXT,
    error_code      VARCHAR(50),
    error_details   JSONB,
    retry_count     INTEGER DEFAULT 0,
    max_retries     INTEGER DEFAULT 3,
    
    -- Metadata
    correlation_id  VARCHAR(100),
    parent_execution_id UUID REFERENCES execution.workflow_executions(id),
    meta            JSONB DEFAULT '{}',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_executions_workflow_id ON execution.workflow_executions(workflow_id);
CREATE INDEX idx_executions_status ON execution.workflow_executions(status);
CREATE INDEX idx_executions_started_at ON execution.workflow_executions(started_at DESC);
CREATE INDEX idx_executions_correlation_id ON execution.workflow_executions(correlation_id) WHERE correlation_id IS NOT NULL;
CREATE INDEX idx_executions_workflow_status ON execution.workflow_executions(workflow_id, status);
CREATE INDEX idx_executions_created_at ON execution.workflow_executions(created_at DESC);

-- Partial indexes for active executions
CREATE INDEX idx_executions_running ON execution.workflow_executions(id) 
    WHERE status IN ('pending', 'queued', 'running', 'paused');

-- ---------------------------------------------------------------------------
-- Node Executions table - Individual node execution records
-- ---------------------------------------------------------------------------
CREATE TABLE execution.node_executions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id    UUID NOT NULL REFERENCES execution.workflow_executions(id) ON DELETE CASCADE,
    node_id         VARCHAR(100) NOT NULL,
    node_type       VARCHAR(100) NOT NULL,
    node_name       VARCHAR(255),
    
    -- Status
    status          VARCHAR(20) DEFAULT 'pending' CHECK (status IN (
        'pending', 'running', 'completed', 'failed', 'skipped', 'cancelled'
    )),
    
    -- Timing
    started_at      TIMESTAMP,
    finished_at     TIMESTAMP,
    duration_ms     BIGINT,
    
    -- Data
    input_data      JSONB DEFAULT '{}',
    output_data     JSONB DEFAULT '{}',
    
    -- Error handling
    error_message   TEXT,
    error_code      VARCHAR(50),
    retry_count     INTEGER DEFAULT 0,
    
    -- Execution order
    execution_order INTEGER,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_node_executions_execution_id ON execution.node_executions(execution_id);
CREATE INDEX idx_node_executions_node_id ON execution.node_executions(node_id);
CREATE INDEX idx_node_executions_status ON execution.node_executions(status);

-- ---------------------------------------------------------------------------
-- Execution Queue table - For distributed execution
-- ---------------------------------------------------------------------------
CREATE TABLE execution.execution_queue (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id    UUID NOT NULL REFERENCES execution.workflow_executions(id) ON DELETE CASCADE,
    priority        INTEGER DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
    
    -- Queue status
    status          VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    
    -- Worker assignment
    worker_id       VARCHAR(100),
    locked_at       TIMESTAMP,
    lock_expires_at TIMESTAMP,
    
    -- Scheduling
    scheduled_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at      TIMESTAMP,
    completed_at    TIMESTAMP,
    
    -- Retry
    attempts        INTEGER DEFAULT 0,
    max_attempts    INTEGER DEFAULT 3,
    last_error      TEXT,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_execution_queue_status ON execution.execution_queue(status, priority DESC, scheduled_at);
CREATE INDEX idx_execution_queue_worker ON execution.execution_queue(worker_id) WHERE worker_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Execution Checkpoints table - For resumable executions
-- ---------------------------------------------------------------------------
CREATE TABLE execution.execution_checkpoints (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id    UUID NOT NULL REFERENCES execution.workflow_executions(id) ON DELETE CASCADE,
    node_id         VARCHAR(100) NOT NULL,
    
    -- Checkpoint data
    state           JSONB NOT NULL,
    checkpoint_type VARCHAR(20) DEFAULT 'auto' CHECK (checkpoint_type IN ('auto', 'manual', 'error')),
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT execution_checkpoints_unique UNIQUE (execution_id, node_id)
);

CREATE INDEX idx_checkpoints_execution_id ON execution.execution_checkpoints(execution_id);

-- ---------------------------------------------------------------------------
-- Execution Metrics table - Performance metrics
-- ---------------------------------------------------------------------------
CREATE TABLE execution.execution_metrics (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id    UUID NOT NULL REFERENCES execution.workflow_executions(id) ON DELETE CASCADE,
    node_id         VARCHAR(100),
    
    metric_name     VARCHAR(100) NOT NULL,
    metric_value    DOUBLE PRECISION NOT NULL,
    metric_unit     VARCHAR(20),
    
    recorded_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_execution_metrics_execution_id ON execution.execution_metrics(execution_id);
CREATE INDEX idx_execution_metrics_name ON execution.execution_metrics(metric_name);
CREATE INDEX idx_execution_metrics_recorded_at ON execution.execution_metrics(recorded_at DESC);

COMMIT;
