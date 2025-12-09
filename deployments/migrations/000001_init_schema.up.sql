-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Create schemas
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS workflow;
CREATE SCHEMA IF NOT EXISTS execution;
CREATE SCHEMA IF NOT EXISTS node;
CREATE SCHEMA IF NOT EXISTS schedule;
CREATE SCHEMA IF NOT EXISTS audit;

-- Users table
CREATE TABLE IF NOT EXISTS auth.users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE,
    password VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    avatar VARCHAR(500),
    email_verified BOOLEAN DEFAULT FALSE,
    email_verify_token VARCHAR(100),
    two_factor_enabled BOOLEAN DEFAULT FALSE,
    two_factor_secret VARCHAR(100),
    status VARCHAR(50) DEFAULT 'active',
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Roles table
CREATE TABLE IF NOT EXISTS auth.roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Permissions table
CREATE TABLE IF NOT EXISTS auth.permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL,
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User roles junction table
CREATE TABLE IF NOT EXISTS auth.user_roles (
    user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES auth.roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id)
);

-- Role permissions junction table
CREATE TABLE IF NOT EXISTS auth.role_permissions (
    role_id UUID REFERENCES auth.roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES auth.permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Sessions table
CREATE TABLE IF NOT EXISTS auth.sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token VARCHAR(500) UNIQUE NOT NULL,
    refresh_token VARCHAR(500) UNIQUE,
    ip_address VARCHAR(50),
    user_agent TEXT,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- OAuth tokens table
CREATE TABLE IF NOT EXISTS auth.oauth_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, provider)
);

-- Teams table
CREATE TABLE IF NOT EXISTS auth.teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL REFERENCES auth.users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Team members table
CREATE TABLE IF NOT EXISTS auth.team_members (
    team_id UUID REFERENCES auth.teams(id) ON DELETE CASCADE,
    user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    role VARCHAR(50) DEFAULT 'member',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (team_id, user_id)
);

-- Workflows table
CREATE TABLE IF NOT EXISTS workflow.workflows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    team_id UUID REFERENCES auth.teams(id),
    nodes JSONB,
    connections JSONB,
    settings JSONB,
    status VARCHAR(50) DEFAULT 'inactive',
    is_active BOOLEAN DEFAULT FALSE,
    version INTEGER DEFAULT 1,
    tags TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Workflow versions table
CREATE TABLE IF NOT EXISTS workflow.workflow_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    data JSONB NOT NULL,
    changed_by UUID REFERENCES auth.users(id),
    change_note TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, version)
);

-- Workflow executions table
CREATE TABLE IF NOT EXISTS execution.workflow_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflow.workflows(id),
    version INTEGER,
    status VARCHAR(50) DEFAULT 'pending',
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    execution_time BIGINT,
    data JSONB,
    error TEXT,
    created_by UUID REFERENCES auth.users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Node executions table
CREATE TABLE IF NOT EXISTS execution.node_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id UUID NOT NULL REFERENCES execution.workflow_executions(id) ON DELETE CASCADE,
    node_id VARCHAR(100) NOT NULL,
    node_type VARCHAR(100),
    status VARCHAR(50),
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    execution_time BIGINT,
    input_data JSONB,
    output_data JSONB,
    error TEXT,
    retry_count INTEGER DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Node types table
CREATE TABLE IF NOT EXISTS node.node_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    icon VARCHAR(100),
    color VARCHAR(7),
    version VARCHAR(20),
    author VARCHAR(255),
    schema JSONB,
    config JSONB,
    status VARCHAR(50) DEFAULT 'active',
    is_builtin BOOLEAN DEFAULT FALSE,
    is_public BOOLEAN DEFAULT FALSE,
    downloads INTEGER DEFAULT 0,
    rating DECIMAL(3,2) DEFAULT 0,
    tags TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Credentials table
CREATE TABLE IF NOT EXISTS workflow.credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    team_id UUID REFERENCES auth.teams(id),
    data JSONB NOT NULL,
    description TEXT,
    tags TEXT[],
    is_shared BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Schedules table
CREATE TABLE IF NOT EXISTS schedule.schedules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    workflow_id UUID NOT NULL REFERENCES workflow.workflows(id),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    team_id UUID REFERENCES auth.teams(id),
    cron_expression VARCHAR(100) NOT NULL,
    timezone VARCHAR(50) DEFAULT 'UTC',
    data JSONB,
    is_active BOOLEAN DEFAULT TRUE,
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    last_run_at TIMESTAMP,
    next_run_at TIMESTAMP,
    misfire_policy VARCHAR(50) DEFAULT 'skip',
    tags TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Schedule executions table
CREATE TABLE IF NOT EXISTS schedule.schedule_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    schedule_id UUID NOT NULL REFERENCES schedule.schedules(id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES workflow.workflows(id),
    execution_id UUID REFERENCES execution.workflow_executions(id),
    triggered_at TIMESTAMP NOT NULL,
    status VARCHAR(50),
    error TEXT,
    data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Webhooks table
CREATE TABLE IF NOT EXISTS workflow.webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    path VARCHAR(255) UNIQUE NOT NULL,
    method VARCHAR(10) DEFAULT 'POST',
    is_active BOOLEAN DEFAULT TRUE,
    secret VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Webhook logs table
CREATE TABLE IF NOT EXISTS workflow.webhook_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id UUID NOT NULL REFERENCES workflow.webhooks(id) ON DELETE CASCADE,
    execution_id UUID REFERENCES execution.workflow_executions(id),
    request_headers JSONB,
    request_body TEXT,
    response_status INTEGER,
    response_body TEXT,
    ip_address VARCHAR(50),
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit logs table
CREATE TABLE IF NOT EXISTS audit.audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES auth.users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(100),
    changes JSONB,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Notifications table
CREATE TABLE IF NOT EXISTS workflow.notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT,
    data JSONB,
    read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- API keys table
CREATE TABLE IF NOT EXISTS auth.api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX idx_users_email ON auth.users(email);
CREATE INDEX idx_users_status ON auth.users(status);
CREATE INDEX idx_sessions_user_id ON auth.sessions(user_id);
CREATE INDEX idx_sessions_token ON auth.sessions(token);
CREATE INDEX idx_workflows_user_id ON workflow.workflows(user_id);
CREATE INDEX idx_workflows_team_id ON workflow.workflows(team_id);
CREATE INDEX idx_workflows_status ON workflow.workflows(status);
CREATE INDEX idx_workflows_is_active ON workflow.workflows(is_active);
CREATE INDEX idx_executions_workflow_id ON execution.workflow_executions(workflow_id);
CREATE INDEX idx_executions_status ON execution.workflow_executions(status);
CREATE INDEX idx_executions_created_at ON execution.workflow_executions(created_at);
CREATE INDEX idx_node_executions_execution_id ON execution.node_executions(execution_id);
CREATE INDEX idx_schedules_workflow_id ON schedule.schedules(workflow_id);
CREATE INDEX idx_schedules_is_active ON schedule.schedules(is_active);
CREATE INDEX idx_audit_logs_user_id ON audit.audit_logs(user_id);
CREATE INDEX idx_audit_logs_created_at ON audit.audit_logs(created_at);
CREATE INDEX idx_notifications_user_id ON workflow.notifications(user_id);
CREATE INDEX idx_notifications_read ON workflow.notifications(read);

-- Create GIN indexes for JSONB columns
CREATE INDEX idx_workflows_nodes_gin ON workflow.workflows USING GIN (nodes);
CREATE INDEX idx_workflows_connections_gin ON workflow.workflows USING GIN (connections);
CREATE INDEX idx_executions_data_gin ON execution.workflow_executions USING GIN (data);

-- Create text search indexes
CREATE INDEX idx_workflows_name_trgm ON workflow.workflows USING GIN (name gin_trgm_ops);
CREATE INDEX idx_workflows_description_trgm ON workflow.workflows USING GIN (description gin_trgm_ops);

-- Create update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply update timestamp trigger to all tables with updated_at column
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON auth.users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    
CREATE TRIGGER update_workflows_updated_at BEFORE UPDATE ON workflow.workflows
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    
CREATE TRIGGER update_credentials_updated_at BEFORE UPDATE ON workflow.credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    
CREATE TRIGGER update_schedules_updated_at BEFORE UPDATE ON schedule.schedules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert default roles
INSERT INTO auth.roles (name, description) VALUES
    ('admin', 'Full system access'),
    ('editor', 'Can create and edit workflows'),
    ('viewer', 'Can view workflows and executions'),
    ('user', 'Basic user role')
ON CONFLICT (name) DO NOTHING;

-- Insert default permissions
INSERT INTO auth.permissions (name, resource, action) VALUES
    ('workflows.create', 'workflows', 'create'),
    ('workflows.read', 'workflows', 'read'),
    ('workflows.update', 'workflows', 'update'),
    ('workflows.delete', 'workflows', 'delete'),
    ('workflows.execute', 'workflows', 'execute'),
    ('executions.read', 'executions', 'read'),
    ('executions.stop', 'executions', 'stop'),
    ('users.read', 'users', 'read'),
    ('users.update', 'users', 'update'),
    ('users.delete', 'users', 'delete'),
    ('teams.create', 'teams', 'create'),
    ('teams.manage', 'teams', 'manage')
ON CONFLICT (name) DO NOTHING;
