-- ============================================================================
-- Migration: 000002_auth_tables
-- Description: Create authentication and authorization tables
-- Schema: auth
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Users table - Core user accounts
-- ---------------------------------------------------------------------------
CREATE TABLE auth.users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email           VARCHAR(255) NOT NULL,
    username        VARCHAR(100),
    password_hash   VARCHAR(255) NOT NULL,
    first_name      VARCHAR(100),
    last_name       VARCHAR(100),
    avatar_url      VARCHAR(500),
    phone           VARCHAR(20),
    
    -- Email verification
    email_verified      BOOLEAN DEFAULT FALSE,
    email_verify_token  VARCHAR(100),
    email_verify_sent_at TIMESTAMP,
    
    -- Two-factor authentication
    two_factor_enabled  BOOLEAN DEFAULT FALSE,
    two_factor_secret   VARCHAR(100),
    
    -- Account status
    status          VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'suspended', 'deleted')),
    
    -- Timestamps
    last_login_at   TIMESTAMP,
    last_login_ip   VARCHAR(45),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMP,
    
    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_username_unique UNIQUE (username)
);

CREATE INDEX idx_users_email ON auth.users(email);
CREATE INDEX idx_users_status ON auth.users(status) WHERE status = 'active';
CREATE INDEX idx_users_deleted_at ON auth.users(deleted_at) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Roles table - User roles for RBAC
-- ---------------------------------------------------------------------------
CREATE TABLE auth.roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(50) NOT NULL,
    description TEXT,
    is_system   BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT roles_name_unique UNIQUE (name)
);

-- ---------------------------------------------------------------------------
-- Permissions table - Granular permissions
-- ---------------------------------------------------------------------------
CREATE TABLE auth.permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) NOT NULL,
    resource    VARCHAR(50) NOT NULL,
    action      VARCHAR(50) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT permissions_name_unique UNIQUE (name),
    CONSTRAINT permissions_resource_action_unique UNIQUE (resource, action)
);

-- ---------------------------------------------------------------------------
-- User-Role junction table
-- ---------------------------------------------------------------------------
CREATE TABLE auth.user_roles (
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES auth.roles(id) ON DELETE CASCADE,
    granted_by  UUID REFERENCES auth.users(id),
    granted_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_user_roles_role_id ON auth.user_roles(role_id);

-- ---------------------------------------------------------------------------
-- Role-Permission junction table
-- ---------------------------------------------------------------------------
CREATE TABLE auth.role_permissions (
    role_id         UUID NOT NULL REFERENCES auth.roles(id) ON DELETE CASCADE,
    permission_id   UUID NOT NULL REFERENCES auth.permissions(id) ON DELETE CASCADE,
    
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_permission_id ON auth.role_permissions(permission_id);

-- ---------------------------------------------------------------------------
-- Sessions table - User sessions
-- ---------------------------------------------------------------------------
CREATE TABLE auth.sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash      VARCHAR(255) NOT NULL,
    refresh_token_hash VARCHAR(255),
    ip_address      VARCHAR(45),
    user_agent      TEXT,
    device_info     JSONB DEFAULT '{}',
    expires_at      TIMESTAMP NOT NULL,
    revoked_at      TIMESTAMP,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT sessions_token_unique UNIQUE (token_hash)
);

CREATE INDEX idx_sessions_user_id ON auth.sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON auth.sessions(expires_at) WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- API Keys table - For programmatic access
-- ---------------------------------------------------------------------------
CREATE TABLE auth.api_keys (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    key_hash    VARCHAR(255) NOT NULL,
    key_prefix  VARCHAR(10) NOT NULL,
    permissions JSONB DEFAULT '[]',
    rate_limit  INTEGER DEFAULT 1000,
    is_active   BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP,
    expires_at  TIMESTAMP,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT api_keys_hash_unique UNIQUE (key_hash)
);

CREATE INDEX idx_api_keys_user_id ON auth.api_keys(user_id);
CREATE INDEX idx_api_keys_prefix ON auth.api_keys(key_prefix);

-- ---------------------------------------------------------------------------
-- OAuth Connections table - Third-party OAuth
-- ---------------------------------------------------------------------------
CREATE TABLE auth.oauth_connections (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    provider        VARCHAR(50) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    access_token    TEXT,
    refresh_token   TEXT,
    token_expires_at TIMESTAMP,
    profile_data    JSONB DEFAULT '{}',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT oauth_user_provider_unique UNIQUE (user_id, provider),
    CONSTRAINT oauth_provider_id_unique UNIQUE (provider, provider_user_id)
);

-- ---------------------------------------------------------------------------
-- Teams table - Team/Organization management
-- ---------------------------------------------------------------------------
CREATE TABLE auth.teams (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) NOT NULL,
    slug        VARCHAR(100) NOT NULL,
    description TEXT,
    owner_id    UUID NOT NULL REFERENCES auth.users(id),
    settings    JSONB DEFAULT '{}',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT teams_slug_unique UNIQUE (slug)
);

CREATE INDEX idx_teams_owner_id ON auth.teams(owner_id);

-- ---------------------------------------------------------------------------
-- Team Members table
-- ---------------------------------------------------------------------------
CREATE TABLE auth.team_members (
    team_id     UUID NOT NULL REFERENCES auth.teams(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role        VARCHAR(20) DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    invited_by  UUID REFERENCES auth.users(id),
    joined_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX idx_team_members_user_id ON auth.team_members(user_id);

-- ---------------------------------------------------------------------------
-- Password Reset Tokens table
-- ---------------------------------------------------------------------------
CREATE TABLE auth.password_resets (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL,
    expires_at  TIMESTAMP NOT NULL,
    used_at     TIMESTAMP,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT password_resets_token_unique UNIQUE (token_hash)
);

CREATE INDEX idx_password_resets_user_id ON auth.password_resets(user_id);

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_users_updated_at 
    BEFORE UPDATE ON auth.users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_roles_updated_at 
    BEFORE UPDATE ON auth.roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_oauth_connections_updated_at 
    BEFORE UPDATE ON auth.oauth_connections
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_teams_updated_at 
    BEFORE UPDATE ON auth.teams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
