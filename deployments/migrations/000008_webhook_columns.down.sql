-- Remove added columns from webhooks table
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS node_id;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS user_id;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS name;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS require_auth;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS auth_type;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS auth_config;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS headers;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS rate_limit;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS expires_at;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS last_called_at;
ALTER TABLE workflow.webhooks DROP COLUMN IF EXISTS call_count;

-- Drop indexes
DROP INDEX IF EXISTS workflow.idx_webhooks_user_id;
DROP INDEX IF EXISTS workflow.idx_webhooks_workflow_id;
