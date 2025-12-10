-- Create execution_checkpoints table for storing execution state checkpoints
CREATE TABLE IF NOT EXISTS execution_checkpoints (
    id VARCHAR(36) PRIMARY KEY,
    execution_id VARCHAR(36) NOT NULL,
    node_id VARCHAR(36),
    state JSONB NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes for performance
    INDEX idx_execution_checkpoints_execution_id (execution_id),
    INDEX idx_execution_checkpoints_timestamp (timestamp),
    INDEX idx_execution_checkpoints_node_id (node_id),
    INDEX idx_execution_checkpoints_execution_timestamp (execution_id, timestamp DESC)
);

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_execution_checkpoints_updated_at
    BEFORE UPDATE ON execution_checkpoints
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add comments for documentation
COMMENT ON TABLE execution_checkpoints IS 'Stores checkpoints for workflow execution recovery';
COMMENT ON COLUMN execution_checkpoints.id IS 'Unique checkpoint identifier';
COMMENT ON COLUMN execution_checkpoints.execution_id IS 'ID of the workflow execution';
COMMENT ON COLUMN execution_checkpoints.node_id IS 'ID of the node being checkpointed (NULL for full execution checkpoint)';
COMMENT ON COLUMN execution_checkpoints.state IS 'Complete execution state at checkpoint time';
COMMENT ON COLUMN execution_checkpoints.timestamp IS 'When the checkpoint was created';
COMMENT ON COLUMN execution_checkpoints.version IS 'Checkpoint version for conflict resolution';
COMMENT ON COLUMN execution_checkpoints.metadata IS 'Additional metadata about the checkpoint';
