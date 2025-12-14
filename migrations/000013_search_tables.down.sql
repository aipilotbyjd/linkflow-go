-- ============================================================================
-- Migration: 000013_search_tables (ROLLBACK)
-- Description: Drop search indexing tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_workflow_index_updated_at ON search.workflow_index;
DROP TRIGGER IF EXISTS trg_workflow_index_search_vector ON search.workflow_index;

DROP FUNCTION IF EXISTS search.search_workflows(TEXT, UUID, INTEGER, INTEGER);
DROP FUNCTION IF EXISTS search.update_workflow_search_vector();

DROP TABLE IF EXISTS search.popular_searches;
DROP TABLE IF EXISTS search.search_history;
DROP TABLE IF EXISTS search.workflow_index;

COMMIT;
