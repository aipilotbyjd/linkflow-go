-- Add missing columns to webhooks table
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS node_id VARCHAR(255);
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS user_id UUID;
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS name VARCHAR(255);
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS require_auth BOOLEAN DEFAULT FALSE;
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS auth_type VARCHAR(50);
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS auth_config JSONB DEFAULT '{}';
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS headers JSONB DEFAULT '{}';
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS rate_limit INTEGER DEFAULT 100;
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP;
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS last_called_at TIMESTAMP;
ALTER TABLE workflow.webhooks ADD COLUMN IF NOT EXISTS call_count BIGINT DEFAULT 0;

-- Add indexes
CREATE INDEX IF NOT EXISTS idx_webhooks_user_id ON workflow.webhooks(user_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_workflow_id ON workflow.webhooks(workflow_id);
