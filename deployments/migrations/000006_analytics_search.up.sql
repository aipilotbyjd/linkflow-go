-- Analytics schema
CREATE SCHEMA IF NOT EXISTS analytics;

-- Workflow analytics table
CREATE TABLE IF NOT EXISTS analytics.workflow_stats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    total_executions INTEGER DEFAULT 0,
    successful_executions INTEGER DEFAULT 0,
    failed_executions INTEGER DEFAULT 0,
    avg_execution_time BIGINT DEFAULT 0,
    min_execution_time BIGINT,
    max_execution_time BIGINT,
    total_nodes_executed INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, date)
);

-- User activity analytics
CREATE TABLE IF NOT EXISTS analytics.user_activity (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    workflows_created INTEGER DEFAULT 0,
    workflows_executed INTEGER DEFAULT 0,
    api_calls INTEGER DEFAULT 0,
    login_count INTEGER DEFAULT 0,
    session_duration BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, date)
);

-- System metrics table
CREATE TABLE IF NOT EXISTS analytics.system_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMP NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    metric_value DOUBLE PRECISION NOT NULL,
    labels JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Node usage analytics
CREATE TABLE IF NOT EXISTS analytics.node_usage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    node_type VARCHAR(100) NOT NULL,
    date DATE NOT NULL,
    usage_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    avg_execution_time BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(node_type, date)
);

-- Search schema
CREATE SCHEMA IF NOT EXISTS search;

-- Search index table (for full-text search without Elasticsearch)
CREATE TABLE IF NOT EXISTS search.workflow_index (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID UNIQUE NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tags TEXT[],
    node_types TEXT[],
    search_vector TSVECTOR,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Search history
CREATE TABLE IF NOT EXISTS search.search_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    query TEXT NOT NULL,
    filters JSONB DEFAULT '{}',
    results_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_workflow_stats_workflow_id ON analytics.workflow_stats(workflow_id);
CREATE INDEX idx_workflow_stats_date ON analytics.workflow_stats(date);
CREATE INDEX idx_user_activity_user_id ON analytics.user_activity(user_id);
CREATE INDEX idx_user_activity_date ON analytics.user_activity(date);
CREATE INDEX idx_system_metrics_timestamp ON analytics.system_metrics(timestamp);
CREATE INDEX idx_system_metrics_name ON analytics.system_metrics(metric_name);
CREATE INDEX idx_node_usage_node_type ON analytics.node_usage(node_type);
CREATE INDEX idx_node_usage_date ON analytics.node_usage(date);
CREATE INDEX idx_workflow_index_search ON search.workflow_index USING GIN(search_vector);
CREATE INDEX idx_workflow_index_user_id ON search.workflow_index(user_id);
CREATE INDEX idx_search_history_user_id ON search.search_history(user_id);

-- Function to update search vector
CREATE OR REPLACE FUNCTION search.update_workflow_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('english', COALESCE(NEW.name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for search vector
CREATE TRIGGER update_workflow_search_vector_trigger
    BEFORE INSERT OR UPDATE ON search.workflow_index
    FOR EACH ROW EXECUTE FUNCTION search.update_workflow_search_vector();

-- Trigger for updated_at
CREATE TRIGGER update_workflow_index_updated_at BEFORE UPDATE ON search.workflow_index
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Partitioning for system_metrics (by month)
-- Note: In production, you'd want to set up proper partitioning
