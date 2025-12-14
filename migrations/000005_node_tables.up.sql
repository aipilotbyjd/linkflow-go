-- ============================================================================
-- Migration: 000005_node_tables
-- Description: Create node registry tables
-- Schema: node
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Node Types table - Registry of available node types
-- ---------------------------------------------------------------------------
CREATE TABLE node.node_types (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type            VARCHAR(100) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    
    -- Categorization
    category        VARCHAR(50) NOT NULL,
    subcategory     VARCHAR(50),
    
    -- Display
    icon            VARCHAR(100),
    color           VARCHAR(7),
    
    -- Version info
    version         VARCHAR(20) DEFAULT '1.0.0',
    min_version     VARCHAR(20),
    
    -- Schema definition
    input_schema    JSONB DEFAULT '{}',
    output_schema   JSONB DEFAULT '{}',
    config_schema   JSONB DEFAULT '{}',
    
    -- Metadata
    author          VARCHAR(255),
    documentation_url VARCHAR(500),
    source_url      VARCHAR(500),
    
    -- Status
    status          VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'deprecated', 'disabled')),
    is_builtin      BOOLEAN DEFAULT FALSE,
    is_public       BOOLEAN DEFAULT TRUE,
    
    -- Stats
    usage_count     BIGINT DEFAULT 0,
    rating          DECIMAL(3,2) DEFAULT 0,
    ratings_count   INTEGER DEFAULT 0,
    
    -- Tags
    tags            TEXT[] DEFAULT '{}',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT node_types_type_unique UNIQUE (type)
);

CREATE INDEX idx_node_types_category ON node.node_types(category);
CREATE INDEX idx_node_types_status ON node.node_types(status);
CREATE INDEX idx_node_types_is_builtin ON node.node_types(is_builtin);
CREATE INDEX idx_node_types_tags ON node.node_types USING GIN(tags);
CREATE INDEX idx_node_types_name_trgm ON node.node_types USING GIN(name gin_trgm_ops);

-- ---------------------------------------------------------------------------
-- Custom Nodes table - User-created custom nodes
-- ---------------------------------------------------------------------------
CREATE TABLE node.custom_nodes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES auth.teams(id) ON DELETE SET NULL,
    
    type            VARCHAR(100) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    
    -- Code
    code            TEXT NOT NULL,
    language        VARCHAR(20) DEFAULT 'javascript',
    
    -- Schema
    input_schema    JSONB DEFAULT '{}',
    output_schema   JSONB DEFAULT '{}',
    config_schema   JSONB DEFAULT '{}',
    
    -- Display
    icon            VARCHAR(100),
    color           VARCHAR(7),
    
    -- Status
    is_public       BOOLEAN DEFAULT FALSE,
    is_approved     BOOLEAN DEFAULT FALSE,
    
    version         VARCHAR(20) DEFAULT '1.0.0',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT custom_nodes_type_unique UNIQUE (type)
);

CREATE INDEX idx_custom_nodes_user_id ON node.custom_nodes(user_id);
CREATE INDEX idx_custom_nodes_team_id ON node.custom_nodes(team_id);
CREATE INDEX idx_custom_nodes_is_public ON node.custom_nodes(is_public) WHERE is_public = TRUE;

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_node_types_updated_at 
    BEFORE UPDATE ON node.node_types
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_custom_nodes_updated_at 
    BEFORE UPDATE ON node.custom_nodes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
