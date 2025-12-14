-- ============================================================================
-- Migration: 000003_workflow_tables
-- Description: Create workflow management tables
-- Schema: workflow
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Workflows table - Core workflow definitions
-- ---------------------------------------------------------------------------
CREATE TABLE workflow.workflows (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES auth.teams(id) ON DELETE SET NULL,
    
    -- Workflow definition
    nodes           JSONB DEFAULT '[]',
    connections     JSONB DEFAULT '[]',
    settings        JSONB DEFAULT '{}',
    
    -- Status
    status          VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'inactive', 'archived')),
    is_active       BOOLEAN DEFAULT FALSE,
    
    -- Versioning
    version         INTEGER DEFAULT 1,
    published_at    TIMESTAMP,
    
    -- Metadata
    tags            TEXT[] DEFAULT '{}',
    meta            JSONB DEFAULT '{}',
    
    -- Timestamps
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMP
);

CREATE INDEX idx_workflows_user_id ON workflow.workflows(user_id);
CREATE INDEX idx_workflows_team_id ON workflow.workflows(team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_workflows_status ON workflow.workflows(status);
CREATE INDEX idx_workflows_is_active ON workflow.workflows(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_workflows_tags ON workflow.workflows USING GIN(tags);
CREATE INDEX idx_workflows_deleted_at ON workflow.workflows(deleted_at) WHERE deleted_at IS NULL;

-- Full-text search
CREATE INDEX idx_workflows_name_trgm ON workflow.workflows USING GIN(name gin_trgm_ops);
CREATE INDEX idx_workflows_description_trgm ON workflow.workflows USING GIN(description gin_trgm_ops);

-- ---------------------------------------------------------------------------
-- Workflow Versions table - Version history
-- ---------------------------------------------------------------------------
CREATE TABLE workflow.workflow_versions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    
    -- Snapshot of workflow at this version
    nodes           JSONB NOT NULL,
    connections     JSONB NOT NULL,
    settings        JSONB NOT NULL,
    
    -- Change tracking
    change_summary  TEXT,
    changed_by      UUID REFERENCES auth.users(id),
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT workflow_versions_unique UNIQUE (workflow_id, version)
);

CREATE INDEX idx_workflow_versions_workflow_id ON workflow.workflow_versions(workflow_id);

-- ---------------------------------------------------------------------------
-- Workflow Shares table - Sharing workflows
-- ---------------------------------------------------------------------------
CREATE TABLE workflow.workflow_shares (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    
    -- Share target (user or team)
    shared_with_user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    shared_with_team_id UUID REFERENCES auth.teams(id) ON DELETE CASCADE,
    
    -- Permissions
    permission      VARCHAR(20) DEFAULT 'view' CHECK (permission IN ('view', 'edit', 'execute', 'admin')),
    
    shared_by       UUID NOT NULL REFERENCES auth.users(id),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT workflow_shares_target_check CHECK (
        (shared_with_user_id IS NOT NULL AND shared_with_team_id IS NULL) OR
        (shared_with_user_id IS NULL AND shared_with_team_id IS NOT NULL)
    )
);

CREATE INDEX idx_workflow_shares_workflow_id ON workflow.workflow_shares(workflow_id);
CREATE INDEX idx_workflow_shares_user_id ON workflow.workflow_shares(shared_with_user_id);
CREATE INDEX idx_workflow_shares_team_id ON workflow.workflow_shares(shared_with_team_id);

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_workflows_updated_at 
    BEFORE UPDATE ON workflow.workflows
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
