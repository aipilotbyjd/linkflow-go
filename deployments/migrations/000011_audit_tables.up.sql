-- ============================================================================
-- Migration: 000011_audit_tables
-- Description: Create audit logging tables
-- Schema: audit
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Audit Logs table - Main audit trail
-- ---------------------------------------------------------------------------
CREATE TABLE audit.logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Actor
    user_id         UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    user_email      VARCHAR(255),
    api_key_id      UUID REFERENCES auth.api_keys(id) ON DELETE SET NULL,
    
    -- Action
    action          VARCHAR(100) NOT NULL,
    resource_type   VARCHAR(50) NOT NULL,
    resource_id     VARCHAR(100),
    
    -- Details
    old_values      JSONB,
    new_values      JSONB,
    changes         JSONB,
    
    -- Context
    ip_address      VARCHAR(45),
    user_agent      TEXT,
    request_id      VARCHAR(100),
    
    -- Metadata
    metadata        JSONB DEFAULT '{}',
    
    -- Status
    status          VARCHAR(20) DEFAULT 'success' CHECK (status IN ('success', 'failure', 'error')),
    error_message   TEXT,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX idx_audit_logs_user_id ON audit.logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit.logs(action);
CREATE INDEX idx_audit_logs_resource ON audit.logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_created_at ON audit.logs(created_at DESC);
CREATE INDEX idx_audit_logs_request_id ON audit.logs(request_id) WHERE request_id IS NOT NULL;

-- Composite index for filtering
CREATE INDEX idx_audit_logs_user_action_date ON audit.logs(user_id, action, created_at DESC);

-- ---------------------------------------------------------------------------
-- Security Events table - Security-specific events
-- ---------------------------------------------------------------------------
CREATE TABLE audit.security_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Event type
    event_type      VARCHAR(50) NOT NULL CHECK (event_type IN (
        'login_success', 'login_failure', 'logout',
        'password_change', 'password_reset',
        'mfa_enabled', 'mfa_disabled', 'mfa_challenge',
        'api_key_created', 'api_key_revoked',
        'permission_change', 'role_change',
        'suspicious_activity', 'rate_limit_exceeded'
    )),
    
    -- Actor
    user_id         UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    user_email      VARCHAR(255),
    
    -- Details
    details         JSONB DEFAULT '{}',
    
    -- Context
    ip_address      VARCHAR(45),
    user_agent      TEXT,
    location        JSONB,
    
    -- Risk assessment
    risk_level      VARCHAR(10) DEFAULT 'low' CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_security_events_user_id ON audit.security_events(user_id);
CREATE INDEX idx_security_events_type ON audit.security_events(event_type);
CREATE INDEX idx_security_events_risk ON audit.security_events(risk_level) WHERE risk_level IN ('high', 'critical');
CREATE INDEX idx_security_events_created_at ON audit.security_events(created_at DESC);

-- ---------------------------------------------------------------------------
-- Data Retention Policy table
-- ---------------------------------------------------------------------------
CREATE TABLE audit.retention_policies (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    table_name      VARCHAR(100) NOT NULL,
    retention_days  INTEGER NOT NULL,
    
    is_active       BOOLEAN DEFAULT TRUE,
    last_cleanup_at TIMESTAMP,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT retention_policies_table_unique UNIQUE (table_name)
);

-- Insert default retention policies
INSERT INTO audit.retention_policies (table_name, retention_days) VALUES
    ('audit.logs', 365),
    ('audit.security_events', 730),
    ('webhook.webhook_logs', 30),
    ('notification.queue', 7),
    ('execution.execution_metrics', 90)
ON CONFLICT (table_name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_retention_policies_updated_at 
    BEFORE UPDATE ON audit.retention_policies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
