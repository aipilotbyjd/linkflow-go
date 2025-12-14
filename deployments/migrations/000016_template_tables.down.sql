-- ============================================================================
-- Migration: 000016_template_tables (ROLLBACK)
-- Description: Drop workflow template tables
-- Schema: template
-- ============================================================================

BEGIN;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_increment_template_use_count ON template.uses;
DROP TRIGGER IF EXISTS trg_update_template_rating ON template.reviews;
DROP TRIGGER IF EXISTS trg_template_collections_updated_at ON template.collections;
DROP TRIGGER IF EXISTS trg_template_reviews_updated_at ON template.reviews;
DROP TRIGGER IF EXISTS trg_templates_updated_at ON template.templates;
DROP TRIGGER IF EXISTS trg_template_categories_updated_at ON template.categories;

-- Drop functions
DROP FUNCTION IF EXISTS template.increment_use_count();
DROP FUNCTION IF EXISTS template.update_template_rating();

-- Drop tables in reverse order of creation
DROP TABLE IF EXISTS template.collection_templates;
DROP TABLE IF EXISTS template.collections;
DROP TABLE IF EXISTS template.uses;
DROP TABLE IF EXISTS template.reviews;
DROP TABLE IF EXISTS template.template_versions;
DROP TABLE IF EXISTS template.templates;
DROP TABLE IF EXISTS template.categories;

COMMIT;
