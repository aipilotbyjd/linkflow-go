-- Drop all triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON auth.users;
DROP TRIGGER IF EXISTS update_workflows_updated_at ON workflow.workflows;
DROP TRIGGER IF EXISTS update_credentials_updated_at ON workflow.credentials;
DROP TRIGGER IF EXISTS update_schedules_updated_at ON schedule.schedules;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop all indexes
DROP INDEX IF EXISTS workflow.idx_workflows_name_trgm;
DROP INDEX IF EXISTS workflow.idx_workflows_description_trgm;
DROP INDEX IF EXISTS workflow.idx_workflows_nodes_gin;
DROP INDEX IF EXISTS workflow.idx_workflows_connections_gin;
DROP INDEX IF EXISTS execution.idx_executions_data_gin;
DROP INDEX IF EXISTS workflow.idx_notifications_read;
DROP INDEX IF EXISTS workflow.idx_notifications_user_id;
DROP INDEX IF EXISTS audit.idx_audit_logs_created_at;
DROP INDEX IF EXISTS audit.idx_audit_logs_user_id;
DROP INDEX IF EXISTS schedule.idx_schedules_is_active;
DROP INDEX IF EXISTS schedule.idx_schedules_workflow_id;
DROP INDEX IF EXISTS execution.idx_node_executions_execution_id;
DROP INDEX IF EXISTS execution.idx_executions_created_at;
DROP INDEX IF EXISTS execution.idx_executions_status;
DROP INDEX IF EXISTS execution.idx_executions_workflow_id;
DROP INDEX IF EXISTS workflow.idx_workflows_is_active;
DROP INDEX IF EXISTS workflow.idx_workflows_status;
DROP INDEX IF EXISTS workflow.idx_workflows_team_id;
DROP INDEX IF EXISTS workflow.idx_workflows_user_id;
DROP INDEX IF EXISTS auth.idx_sessions_token;
DROP INDEX IF EXISTS auth.idx_sessions_user_id;
DROP INDEX IF EXISTS auth.idx_users_status;
DROP INDEX IF EXISTS auth.idx_users_email;

-- Drop all tables in reverse order of dependencies
DROP TABLE IF EXISTS auth.api_keys CASCADE;
DROP TABLE IF EXISTS workflow.notifications CASCADE;
DROP TABLE IF EXISTS audit.audit_logs CASCADE;
DROP TABLE IF EXISTS workflow.webhook_logs CASCADE;
DROP TABLE IF EXISTS workflow.webhooks CASCADE;
DROP TABLE IF EXISTS schedule.schedule_executions CASCADE;
DROP TABLE IF EXISTS schedule.schedules CASCADE;
DROP TABLE IF EXISTS workflow.credentials CASCADE;
DROP TABLE IF EXISTS node.node_types CASCADE;
DROP TABLE IF EXISTS execution.node_executions CASCADE;
DROP TABLE IF EXISTS execution.workflow_executions CASCADE;
DROP TABLE IF EXISTS workflow.workflow_versions CASCADE;
DROP TABLE IF EXISTS workflow.workflows CASCADE;
DROP TABLE IF EXISTS auth.team_members CASCADE;
DROP TABLE IF EXISTS auth.teams CASCADE;
DROP TABLE IF EXISTS auth.oauth_tokens CASCADE;
DROP TABLE IF EXISTS auth.sessions CASCADE;
DROP TABLE IF EXISTS auth.role_permissions CASCADE;
DROP TABLE IF EXISTS auth.user_roles CASCADE;
DROP TABLE IF EXISTS auth.permissions CASCADE;
DROP TABLE IF EXISTS auth.roles CASCADE;
DROP TABLE IF EXISTS auth.users CASCADE;

-- Drop schemas
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS schedule CASCADE;
DROP SCHEMA IF EXISTS node CASCADE;
DROP SCHEMA IF EXISTS execution CASCADE;
DROP SCHEMA IF EXISTS workflow CASCADE;
DROP SCHEMA IF EXISTS auth CASCADE;

-- Drop extensions
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS "uuid-ossp";
