-- ============================================================================
-- Migration: 000003_workflow_tables (ROLLBACK)
-- Description: Drop workflow management tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_workflows_updated_at ON workflow.workflows;

DROP TABLE IF EXISTS workflow.workflow_shares;
DROP TABLE IF EXISTS workflow.workflow_versions;
DROP TABLE IF EXISTS workflow.workflows;

COMMIT;
