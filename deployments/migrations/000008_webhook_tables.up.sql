-- ============================================================================
-- Migration: 000008_webhook_tables
-- Description: Create webhook handling tables
-- Schema: webhook
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Webhooks table - Webhook endpoints
-- ---------------------------------------------------------------------------
CREATE TABLE webhook.webhooks (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    node_id         VARCHAR(100),
    
    name            VARCHAR(255),
    path            VARCHAR(255) NOT NULL,
    
    -- HTTP configuration
    method          VARCHAR(10) DEFAULT 'POST' CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE')),
    
    -- Authentication
    require_auth    BOOLEAN DEFAULT FALSE,
    auth_type       VARCHAR(20) CHECK (auth_type IN ('none', 'basic', 'bearer', 'api_key', 'hmac')),
    auth_config     JSONB DEFAULT '{}',
    secret          VARCHAR(255),
    
    -- Request handling
    headers_config  JSONB DEFAULT '{}',
    response_mode   VARCHAR(20) DEFAULT 'on_received' CHECK (response_mode IN ('on_received', 'last_node', 'custom')),
    response_data   JSONB,
    
    -- Rate limiting
    rate_limit      INTEGER DEFAULT 100,
    rate_window     INTEGER DEFAULT 60,
    
    -- Status
    is_active       BOOLEAN DEFAULT TRUE,
    
    -- Stats
    call_count      BIGINT DEFAULT 0,
    last_called_at  TIMESTAMP,
    
    -- Expiration
    expires_at      TIMESTAMP,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT webhooks_path_unique UNIQUE (path)
);

CREATE INDEX idx_webhooks_workflow_id ON webhook.webhooks(workflow_id);
CREATE INDEX idx_webhooks_user_id ON webhook.webhooks(user_id);
CREATE INDEX idx_webhooks_path ON webhook.webhooks(path) WHERE is_active = TRUE;
CREATE INDEX idx_webhooks_is_active ON webhook.webhooks(is_active) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Webhook Logs table - Request/response logs
-- ---------------------------------------------------------------------------
CREATE TABLE webhook.webhook_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    webhook_id      UUID NOT NULL REFERENCES webhook.webhooks(id) ON DELETE CASCADE,
    execution_id    UUID REFERENCES execution.workflow_executions(id) ON DELETE SET NULL,
    
    -- Request info
    request_method  VARCHAR(10),
    request_headers JSONB,
    request_body    TEXT,
    request_query   JSONB,
    
    -- Response info
    response_status INTEGER,
    response_headers JSONB,
    response_body   TEXT,
    
    -- Client info
    ip_address      VARCHAR(45),
    user_agent      TEXT,
    
    -- Timing
    duration_ms     INTEGER,
    
    -- Status
    status          VARCHAR(20) DEFAULT 'received' CHECK (status IN ('received', 'processed', 'failed', 'rejected')),
    error_message   TEXT,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhook_logs_webhook_id ON webhook.webhook_logs(webhook_id);
CREATE INDEX idx_webhook_logs_created_at ON webhook.webhook_logs(created_at DESC);
CREATE INDEX idx_webhook_logs_status ON webhook.webhook_logs(status);

-- Partition by month for large-scale deployments (optional)
-- CREATE INDEX idx_webhook_logs_created_month ON webhook.webhook_logs(date_trunc('month', created_at));

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_webhooks_updated_at 
    BEFORE UPDATE ON webhook.webhooks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to update webhook stats
CREATE OR REPLACE FUNCTION webhook.update_webhook_stats()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE webhook.webhooks
    SET call_count = call_count + 1,
        last_called_at = NEW.created_at
    WHERE id = NEW.webhook_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_webhook_logs_update_stats
    AFTER INSERT ON webhook.webhook_logs
    FOR EACH ROW EXECUTE FUNCTION webhook.update_webhook_stats();

COMMIT;
