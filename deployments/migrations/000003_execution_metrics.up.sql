-- Create tables for execution metrics and state transitions

-- State transitions table
CREATE TABLE IF NOT EXISTS execution.state_transitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id UUID NOT NULL REFERENCES execution.workflow_executions(id) ON DELETE CASCADE,
    from_state VARCHAR(50),
    to_state VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Execution metrics table
CREATE TABLE IF NOT EXISTS execution.execution_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id UUID NOT NULL REFERENCES execution.workflow_executions(id) ON DELETE CASCADE,
    node_id VARCHAR(100),
    metric_type VARCHAR(50) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    unit VARCHAR(20),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Execution metrics archive table (for archived/aggregated data)
CREATE TABLE IF NOT EXISTS execution.execution_metrics_archive (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id UUID NOT NULL,
    node_id VARCHAR(100),
    metric_type VARCHAR(50) NOT NULL,
    avg_value DOUBLE PRECISION,
    min_value DOUBLE PRECISION,
    max_value DOUBLE PRECISION,
    count INTEGER,
    date DATE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Performance summary table (materialized view alternative)
CREATE TABLE IF NOT EXISTS execution.performance_summary (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    total_executions INTEGER DEFAULT 0,
    successful_executions INTEGER DEFAULT 0,
    failed_executions INTEGER DEFAULT 0,
    cancelled_executions INTEGER DEFAULT 0,
    avg_execution_time BIGINT,
    min_execution_time BIGINT,
    max_execution_time BIGINT,
    p50_execution_time BIGINT,
    p95_execution_time BIGINT,
    p99_execution_time BIGINT,
    total_node_executions INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, date)
);

-- Create indexes for better query performance
CREATE INDEX idx_state_transitions_execution_id ON execution.state_transitions(execution_id);
CREATE INDEX idx_state_transitions_timestamp ON execution.state_transitions(timestamp DESC);
CREATE INDEX idx_state_transitions_to_state ON execution.state_transitions(to_state);

CREATE INDEX idx_execution_metrics_execution_id ON execution.execution_metrics(execution_id);
CREATE INDEX idx_execution_metrics_node_id ON execution.execution_metrics(node_id);
CREATE INDEX idx_execution_metrics_metric_type ON execution.execution_metrics(metric_type);
CREATE INDEX idx_execution_metrics_timestamp ON execution.execution_metrics(timestamp DESC);
CREATE INDEX idx_execution_metrics_type_timestamp ON execution.execution_metrics(metric_type, timestamp DESC);

CREATE INDEX idx_metrics_archive_execution_id ON execution.execution_metrics_archive(execution_id);
CREATE INDEX idx_metrics_archive_date ON execution.execution_metrics_archive(date DESC);
CREATE INDEX idx_metrics_archive_metric_type ON execution.execution_metrics_archive(metric_type);

CREATE INDEX idx_performance_summary_workflow_date ON execution.performance_summary(workflow_id, date DESC);
CREATE INDEX idx_performance_summary_date ON execution.performance_summary(date DESC);

-- Create hypertable for time-series data if TimescaleDB is available (optional)
-- SELECT create_hypertable('execution.execution_metrics', 'timestamp', if_not_exists => TRUE);
-- SELECT create_hypertable('execution.state_transitions', 'timestamp', if_not_exists => TRUE);

-- Create function to update performance summary
CREATE OR REPLACE FUNCTION execution.update_performance_summary()
RETURNS TRIGGER AS $$
BEGIN
    -- Update or insert performance summary for the workflow
    INSERT INTO execution.performance_summary (
        id,
        workflow_id,
        date,
        total_executions,
        successful_executions,
        failed_executions,
        cancelled_executions,
        avg_execution_time,
        updated_at
    )
    VALUES (
        uuid_generate_v4(),
        NEW.workflow_id,
        DATE(NEW.started_at),
        1,
        CASE WHEN NEW.status = 'completed' THEN 1 ELSE 0 END,
        CASE WHEN NEW.status = 'failed' THEN 1 ELSE 0 END,
        CASE WHEN NEW.status = 'cancelled' THEN 1 ELSE 0 END,
        NEW.execution_time,
        CURRENT_TIMESTAMP
    )
    ON CONFLICT (workflow_id, date) 
    DO UPDATE SET
        total_executions = performance_summary.total_executions + 1,
        successful_executions = performance_summary.successful_executions + 
            CASE WHEN NEW.status = 'completed' THEN 1 ELSE 0 END,
        failed_executions = performance_summary.failed_executions + 
            CASE WHEN NEW.status = 'failed' THEN 1 ELSE 0 END,
        cancelled_executions = performance_summary.cancelled_executions + 
            CASE WHEN NEW.status = 'cancelled' THEN 1 ELSE 0 END,
        avg_execution_time = (
            (performance_summary.avg_execution_time * performance_summary.total_executions + NEW.execution_time) / 
            (performance_summary.total_executions + 1)
        ),
        updated_at = CURRENT_TIMESTAMP;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for performance summary updates
CREATE TRIGGER update_performance_summary_trigger
AFTER INSERT OR UPDATE OF status ON execution.workflow_executions
FOR EACH ROW
WHEN (NEW.status IN ('completed', 'failed', 'cancelled'))
EXECUTE FUNCTION execution.update_performance_summary();
