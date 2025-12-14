-- ============================================================================
-- Migration: 000013_search_tables
-- Description: Create search indexing tables
-- Schema: search
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Workflow Search Index table - Full-text search for workflows
-- ---------------------------------------------------------------------------
CREATE TABLE search.workflow_index (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    
    -- Searchable fields
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    tags            TEXT[] DEFAULT '{}',
    node_types      TEXT[] DEFAULT '{}',
    
    -- Full-text search vector
    search_vector   TSVECTOR,
    
    -- Metadata for filtering
    status          VARCHAR(20),
    is_active       BOOLEAN,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT workflow_index_workflow_unique UNIQUE (workflow_id)
);

CREATE INDEX idx_workflow_index_user_id ON search.workflow_index(user_id);
CREATE INDEX idx_workflow_index_search ON search.workflow_index USING GIN(search_vector);
CREATE INDEX idx_workflow_index_tags ON search.workflow_index USING GIN(tags);
CREATE INDEX idx_workflow_index_node_types ON search.workflow_index USING GIN(node_types);

-- ---------------------------------------------------------------------------
-- Search History table - User search history
-- ---------------------------------------------------------------------------
CREATE TABLE search.search_history (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    
    query           TEXT NOT NULL,
    filters         JSONB DEFAULT '{}',
    
    results_count   INTEGER DEFAULT 0,
    clicked_result_id UUID,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_search_history_user_id ON search.search_history(user_id);
CREATE INDEX idx_search_history_created_at ON search.search_history(created_at DESC);

-- ---------------------------------------------------------------------------
-- Popular Searches table - Aggregated popular searches
-- ---------------------------------------------------------------------------
CREATE TABLE search.popular_searches (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    query           VARCHAR(255) NOT NULL,
    search_count    INTEGER DEFAULT 1,
    
    last_searched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT popular_searches_query_unique UNIQUE (query)
);

CREATE INDEX idx_popular_searches_count ON search.popular_searches(search_count DESC);

-- ---------------------------------------------------------------------------
-- Functions for search
-- ---------------------------------------------------------------------------

-- Function to update search vector
CREATE OR REPLACE FUNCTION search.update_workflow_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('english', COALESCE(NEW.name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.node_types, ' '), '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to search workflows
CREATE OR REPLACE FUNCTION search.search_workflows(
    p_query TEXT,
    p_user_id UUID,
    p_limit INTEGER DEFAULT 20,
    p_offset INTEGER DEFAULT 0
)
RETURNS TABLE (
    workflow_id UUID,
    name VARCHAR(255),
    description TEXT,
    rank REAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        wi.workflow_id,
        wi.name,
        wi.description,
        ts_rank(wi.search_vector, plainto_tsquery('english', p_query)) AS rank
    FROM search.workflow_index wi
    WHERE wi.user_id = p_user_id
      AND wi.search_vector @@ plainto_tsquery('english', p_query)
    ORDER BY rank DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_workflow_index_search_vector
    BEFORE INSERT OR UPDATE ON search.workflow_index
    FOR EACH ROW EXECUTE FUNCTION search.update_workflow_search_vector();

CREATE TRIGGER trg_workflow_index_updated_at 
    BEFORE UPDATE ON search.workflow_index
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
