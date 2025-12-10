-- Additional performance indexes for LinkFlow database

-- Composite indexes for common query patterns
CREATE INDEX idx_workflows_user_status ON workflow.workflows(user_id, status);
CREATE INDEX idx_workflows_team_status ON workflow.workflows(team_id, status) WHERE team_id IS NOT NULL;
CREATE INDEX idx_executions_workflow_status ON execution.workflow_executions(workflow_id, status);
CREATE INDEX idx_executions_workflow_created ON execution.workflow_executions(workflow_id, created_at DESC);

-- Partial indexes for active records
CREATE INDEX idx_workflows_active ON workflow.workflows(id) WHERE is_active = TRUE;
CREATE INDEX idx_schedules_active ON schedule.schedules(id) WHERE is_active = TRUE;
CREATE INDEX idx_credentials_active ON workflow.credentials(id) WHERE is_active = TRUE;
CREATE INDEX idx_api_keys_active ON auth.api_keys(id) WHERE is_active = TRUE;

-- Indexes for time-based queries
CREATE INDEX idx_executions_started_at ON execution.workflow_executions(started_at DESC) WHERE started_at IS NOT NULL;
CREATE INDEX idx_executions_finished_at ON execution.workflow_executions(finished_at DESC) WHERE finished_at IS NOT NULL;
CREATE INDEX idx_node_executions_started_at ON execution.node_executions(started_at DESC) WHERE started_at IS NOT NULL;
CREATE INDEX idx_audit_logs_created_at_desc ON audit.audit_logs(created_at DESC);
CREATE INDEX idx_schedule_next_run ON schedule.schedules(next_run_at) WHERE is_active = TRUE;

-- Indexes for foreign key lookups
CREATE INDEX idx_user_roles_role_id ON auth.user_roles(role_id);
CREATE INDEX idx_role_permissions_permission_id ON auth.role_permissions(permission_id);
CREATE INDEX idx_team_members_user_id ON auth.team_members(user_id);
CREATE INDEX idx_oauth_tokens_user_provider ON auth.oauth_tokens(user_id, provider);

-- Indexes for webhook performance
CREATE INDEX idx_webhooks_path ON workflow.webhooks(path) WHERE is_active = TRUE;
CREATE INDEX idx_webhook_logs_webhook_id ON workflow.webhook_logs(webhook_id, processed_at DESC);

-- Indexes for node execution queries
CREATE INDEX idx_node_executions_node_id ON execution.node_executions(node_id);
CREATE INDEX idx_node_executions_status ON execution.node_executions(status);
CREATE INDEX idx_node_executions_retry ON execution.node_executions(retry_count) WHERE retry_count > 0;

-- Indexes for notification queries
CREATE INDEX idx_notifications_user_unread ON workflow.notifications(user_id, created_at DESC) WHERE read = FALSE;
CREATE INDEX idx_notifications_type ON workflow.notifications(type);

-- Covering indexes for common queries
CREATE INDEX idx_sessions_user_token_expires ON auth.sessions(user_id, token, expires_at);
CREATE INDEX idx_workflows_list ON workflow.workflows(user_id, status, created_at DESC) INCLUDE (name, description);

-- Array field indexes
CREATE INDEX idx_workflows_tags ON workflow.workflows USING GIN(tags) WHERE tags IS NOT NULL;
CREATE INDEX idx_node_types_tags ON node.node_types USING GIN(tags) WHERE tags IS NOT NULL;
CREATE INDEX idx_credentials_tags ON workflow.credentials USING GIN(tags) WHERE tags IS NOT NULL;
CREATE INDEX idx_schedules_tags ON schedule.schedules USING GIN(tags) WHERE tags IS NOT NULL;

-- Text search optimization
CREATE INDEX idx_node_types_name_trgm ON node.node_types USING GIN (name gin_trgm_ops);
CREATE INDEX idx_credentials_name_trgm ON workflow.credentials USING GIN (name gin_trgm_ops);
CREATE INDEX idx_teams_name_trgm ON auth.teams USING GIN (name gin_trgm_ops);

-- Statistics for query planner
ANALYZE auth.users;
ANALYZE auth.sessions;
ANALYZE workflow.workflows;
ANALYZE execution.workflow_executions;
ANALYZE execution.node_executions;
ANALYZE schedule.schedules;
