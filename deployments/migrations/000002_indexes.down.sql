-- Drop additional performance indexes

-- Drop composite indexes
DROP INDEX IF EXISTS workflow.idx_workflows_user_status;
DROP INDEX IF EXISTS workflow.idx_workflows_team_status;
DROP INDEX IF EXISTS execution.idx_executions_workflow_status;
DROP INDEX IF EXISTS execution.idx_executions_workflow_created;

-- Drop partial indexes
DROP INDEX IF EXISTS workflow.idx_workflows_active;
DROP INDEX IF EXISTS schedule.idx_schedules_active;
DROP INDEX IF EXISTS workflow.idx_credentials_active;
DROP INDEX IF EXISTS auth.idx_api_keys_active;

-- Drop time-based indexes
DROP INDEX IF EXISTS execution.idx_executions_started_at;
DROP INDEX IF EXISTS execution.idx_executions_finished_at;
DROP INDEX IF EXISTS execution.idx_node_executions_started_at;
DROP INDEX IF EXISTS audit.idx_audit_logs_created_at_desc;
DROP INDEX IF EXISTS schedule.idx_schedule_next_run;

-- Drop foreign key indexes
DROP INDEX IF EXISTS auth.idx_user_roles_role_id;
DROP INDEX IF EXISTS auth.idx_role_permissions_permission_id;
DROP INDEX IF EXISTS auth.idx_team_members_user_id;
DROP INDEX IF EXISTS auth.idx_oauth_tokens_user_provider;

-- Drop webhook indexes
DROP INDEX IF EXISTS workflow.idx_webhooks_path;
DROP INDEX IF EXISTS workflow.idx_webhook_logs_webhook_id;

-- Drop node execution indexes
DROP INDEX IF EXISTS execution.idx_node_executions_node_id;
DROP INDEX IF EXISTS execution.idx_node_executions_status;
DROP INDEX IF EXISTS execution.idx_node_executions_retry;

-- Drop notification indexes
DROP INDEX IF EXISTS workflow.idx_notifications_user_unread;
DROP INDEX IF EXISTS workflow.idx_notifications_type;

-- Drop covering indexes
DROP INDEX IF EXISTS auth.idx_sessions_user_token_expires;
DROP INDEX IF EXISTS workflow.idx_workflows_list;

-- Drop array field indexes
DROP INDEX IF EXISTS workflow.idx_workflows_tags;
DROP INDEX IF EXISTS node.idx_node_types_tags;
DROP INDEX IF EXISTS workflow.idx_credentials_tags;
DROP INDEX IF EXISTS schedule.idx_schedules_tags;

-- Drop text search indexes
DROP INDEX IF EXISTS node.idx_node_types_name_trgm;
DROP INDEX IF EXISTS workflow.idx_credentials_name_trgm;
DROP INDEX IF EXISTS auth.idx_teams_name_trgm;
