-- ============================================================================
-- Migration: 000007_credential_tables
-- Description: Create credential management tables
-- Schema: credential
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Credential Types table - Types of credentials supported
-- ---------------------------------------------------------------------------
CREATE TABLE credential.credential_types (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type            VARCHAR(100) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    
    -- Schema for credential fields
    schema          JSONB NOT NULL DEFAULT '{}',
    
    -- Display
    icon            VARCHAR(100),
    category        VARCHAR(50),
    
    -- OAuth config (if applicable)
    oauth_config    JSONB,
    
    is_builtin      BOOLEAN DEFAULT FALSE,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT credential_types_type_unique UNIQUE (type)
);

CREATE INDEX idx_credential_types_category ON credential.credential_types(category);

-- ---------------------------------------------------------------------------
-- Credentials table - User credentials
-- ---------------------------------------------------------------------------
CREATE TABLE credential.credentials (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES auth.teams(id) ON DELETE SET NULL,
    
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(100) NOT NULL,
    
    -- Encrypted credential data
    data_encrypted  BYTEA NOT NULL,
    encryption_key_id VARCHAR(100),
    
    -- OAuth tokens (encrypted)
    oauth_tokens    BYTEA,
    oauth_expires_at TIMESTAMP,
    
    -- Status
    is_active       BOOLEAN DEFAULT TRUE,
    is_shared       BOOLEAN DEFAULT FALSE,
    
    -- Usage tracking
    last_used_at    TIMESTAMP,
    use_count       BIGINT DEFAULT 0,
    
    -- Expiration
    expires_at      TIMESTAMP,
    
    -- Metadata
    description     TEXT,
    tags            TEXT[] DEFAULT '{}',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_credentials_user_id ON credential.credentials(user_id);
CREATE INDEX idx_credentials_team_id ON credential.credentials(team_id);
CREATE INDEX idx_credentials_type ON credential.credentials(type);
CREATE INDEX idx_credentials_is_active ON credential.credentials(is_active) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Credential Shares table - Sharing credentials
-- ---------------------------------------------------------------------------
CREATE TABLE credential.credential_shares (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    credential_id   UUID NOT NULL REFERENCES credential.credentials(id) ON DELETE CASCADE,
    
    shared_with_user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    shared_with_team_id UUID REFERENCES auth.teams(id) ON DELETE CASCADE,
    
    permission      VARCHAR(20) DEFAULT 'use' CHECK (permission IN ('use', 'view', 'manage')),
    
    shared_by       UUID NOT NULL REFERENCES auth.users(id),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT credential_shares_target_check CHECK (
        (shared_with_user_id IS NOT NULL AND shared_with_team_id IS NULL) OR
        (shared_with_user_id IS NULL AND shared_with_team_id IS NOT NULL)
    )
);

CREATE INDEX idx_credential_shares_credential_id ON credential.credential_shares(credential_id);
CREATE INDEX idx_credential_shares_user_id ON credential.credential_shares(shared_with_user_id);

-- ---------------------------------------------------------------------------
-- Credential Usage Log table - Audit trail
-- ---------------------------------------------------------------------------
CREATE TABLE credential.credential_usage_log (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    credential_id   UUID NOT NULL REFERENCES credential.credentials(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES auth.users(id),
    workflow_id     UUID REFERENCES workflow.workflows(id),
    execution_id    UUID,
    
    action          VARCHAR(50) NOT NULL,
    ip_address      VARCHAR(45),
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_credential_usage_credential_id ON credential.credential_usage_log(credential_id);
CREATE INDEX idx_credential_usage_created_at ON credential.credential_usage_log(created_at DESC);

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_credential_types_updated_at 
    BEFORE UPDATE ON credential.credential_types
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_credentials_updated_at 
    BEFORE UPDATE ON credential.credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
