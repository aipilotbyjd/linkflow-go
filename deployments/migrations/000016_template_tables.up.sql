-- ============================================================================
-- Migration: 000016_template_tables
-- Description: Create workflow template tables
-- Schema: template
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Categories table - Template categories
-- ---------------------------------------------------------------------------
CREATE TABLE template.categories (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(100) NOT NULL,
    slug            VARCHAR(100) NOT NULL,
    description     TEXT,
    icon            VARCHAR(50),
    color           VARCHAR(7),
    
    -- Hierarchy
    parent_id       UUID REFERENCES template.categories(id) ON DELETE SET NULL,
    
    -- Ordering
    sort_order      INTEGER DEFAULT 0,
    
    is_active       BOOLEAN DEFAULT TRUE,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT categories_slug_unique UNIQUE (slug)
);

CREATE INDEX idx_template_categories_parent ON template.categories(parent_id);
CREATE INDEX idx_template_categories_slug ON template.categories(slug);

-- ---------------------------------------------------------------------------
-- Templates table - Workflow templates
-- ---------------------------------------------------------------------------
CREATE TABLE template.templates (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Basic info
    name            VARCHAR(200) NOT NULL,
    slug            VARCHAR(200) NOT NULL,
    description     TEXT,
    
    -- Content
    workflow_json   JSONB NOT NULL,
    
    -- Categorization
    category_id     UUID REFERENCES template.categories(id) ON DELETE SET NULL,
    tags            TEXT[] DEFAULT '{}',
    
    -- Author
    author_id       UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    author_name     VARCHAR(100),
    
    -- Media
    thumbnail_url   TEXT,
    preview_images  TEXT[] DEFAULT '{}',
    
    -- Stats
    use_count       INTEGER DEFAULT 0,
    rating_sum      INTEGER DEFAULT 0,
    rating_count    INTEGER DEFAULT 0,
    
    -- Status
    status          VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'pending', 'published', 'rejected', 'archived')),
    is_featured     BOOLEAN DEFAULT FALSE,
    is_official     BOOLEAN DEFAULT FALSE,
    
    -- Versioning
    version         VARCHAR(20) DEFAULT '1.0.0',
    
    -- Requirements
    required_nodes      TEXT[] DEFAULT '{}',
    required_credentials TEXT[] DEFAULT '{}',
    
    -- Metadata
    metadata        JSONB DEFAULT '{}',
    
    published_at    TIMESTAMP,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT templates_slug_unique UNIQUE (slug)
);

CREATE INDEX idx_templates_category_id ON template.templates(category_id);
CREATE INDEX idx_templates_author_id ON template.templates(author_id);
CREATE INDEX idx_templates_status ON template.templates(status);
CREATE INDEX idx_templates_tags ON template.templates USING GIN(tags);
CREATE INDEX idx_templates_featured ON template.templates(is_featured) WHERE is_featured = TRUE;
CREATE INDEX idx_templates_use_count ON template.templates(use_count DESC);

-- ---------------------------------------------------------------------------
-- Template Versions table - Version history
-- ---------------------------------------------------------------------------
CREATE TABLE template.template_versions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id     UUID NOT NULL REFERENCES template.templates(id) ON DELETE CASCADE,
    
    version         VARCHAR(20) NOT NULL,
    workflow_json   JSONB NOT NULL,
    changelog       TEXT,
    
    created_by      UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_template_versions_template_id ON template.template_versions(template_id);

-- ---------------------------------------------------------------------------
-- Template Reviews table - User reviews
-- ---------------------------------------------------------------------------
CREATE TABLE template.reviews (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id     UUID NOT NULL REFERENCES template.templates(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    
    rating          INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    title           VARCHAR(200),
    comment         TEXT,
    
    -- Moderation
    is_approved     BOOLEAN DEFAULT TRUE,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT reviews_user_template_unique UNIQUE (template_id, user_id)
);

CREATE INDEX idx_template_reviews_template_id ON template.reviews(template_id);
CREATE INDEX idx_template_reviews_user_id ON template.reviews(user_id);

-- ---------------------------------------------------------------------------
-- Template Uses table - Track template usage
-- ---------------------------------------------------------------------------
CREATE TABLE template.uses (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id     UUID NOT NULL REFERENCES template.templates(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    workflow_id     UUID REFERENCES workflow.workflows(id) ON DELETE SET NULL,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_template_uses_template_id ON template.uses(template_id);
CREATE INDEX idx_template_uses_user_id ON template.uses(user_id);
CREATE INDEX idx_template_uses_created_at ON template.uses(created_at DESC);

-- ---------------------------------------------------------------------------
-- Template Collections table - Curated collections
-- ---------------------------------------------------------------------------
CREATE TABLE template.collections (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(200) NOT NULL,
    slug            VARCHAR(200) NOT NULL,
    description     TEXT,
    
    -- Media
    cover_image_url TEXT,
    
    -- Curator
    curator_id      UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    
    is_featured     BOOLEAN DEFAULT FALSE,
    is_active       BOOLEAN DEFAULT TRUE,
    
    sort_order      INTEGER DEFAULT 0,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT collections_slug_unique UNIQUE (slug)
);

-- ---------------------------------------------------------------------------
-- Collection Templates table - Templates in collections
-- ---------------------------------------------------------------------------
CREATE TABLE template.collection_templates (
    collection_id   UUID NOT NULL REFERENCES template.collections(id) ON DELETE CASCADE,
    template_id     UUID NOT NULL REFERENCES template.templates(id) ON DELETE CASCADE,
    sort_order      INTEGER DEFAULT 0,
    added_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (collection_id, template_id)
);

CREATE INDEX idx_collection_templates_template ON template.collection_templates(template_id);

-- ---------------------------------------------------------------------------
-- Insert default categories
-- ---------------------------------------------------------------------------
INSERT INTO template.categories (name, slug, description, icon, sort_order) VALUES
    ('Marketing', 'marketing', 'Marketing automation workflows', 'megaphone', 1),
    ('Sales', 'sales', 'Sales and CRM workflows', 'chart-line', 2),
    ('DevOps', 'devops', 'Development and operations workflows', 'code', 3),
    ('Data', 'data', 'Data processing and ETL workflows', 'database', 4),
    ('Communication', 'communication', 'Email, chat, and notification workflows', 'message-circle', 5),
    ('Productivity', 'productivity', 'Task and project management workflows', 'check-square', 6),
    ('Finance', 'finance', 'Accounting and finance workflows', 'dollar-sign', 7),
    ('HR', 'hr', 'Human resources workflows', 'users', 8)
ON CONFLICT (slug) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_template_categories_updated_at 
    BEFORE UPDATE ON template.categories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_templates_updated_at 
    BEFORE UPDATE ON template.templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_template_reviews_updated_at 
    BEFORE UPDATE ON template.reviews
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_template_collections_updated_at 
    BEFORE UPDATE ON template.collections
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ---------------------------------------------------------------------------
-- Function to update template rating
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION template.update_template_rating()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE template.templates 
        SET rating_sum = rating_sum + NEW.rating,
            rating_count = rating_count + 1
        WHERE id = NEW.template_id;
    ELSIF TG_OP = 'UPDATE' THEN
        UPDATE template.templates 
        SET rating_sum = rating_sum - OLD.rating + NEW.rating
        WHERE id = NEW.template_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE template.templates 
        SET rating_sum = rating_sum - OLD.rating,
            rating_count = rating_count - 1
        WHERE id = OLD.template_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_template_rating
    AFTER INSERT OR UPDATE OR DELETE ON template.reviews
    FOR EACH ROW EXECUTE FUNCTION template.update_template_rating();

-- ---------------------------------------------------------------------------
-- Function to increment template use count
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION template.increment_use_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE template.templates 
    SET use_count = use_count + 1
    WHERE id = NEW.template_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_increment_template_use_count
    AFTER INSERT ON template.uses
    FOR EACH ROW EXECUTE FUNCTION template.increment_use_count();

COMMIT;
