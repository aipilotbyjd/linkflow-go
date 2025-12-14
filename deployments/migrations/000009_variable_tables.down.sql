-- ============================================================================
-- Migration: 000009_variable_tables (ROLLBACK)
-- Description: Drop variable storage tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_variables_track_changes ON variable.variables;
DROP FUNCTION IF EXISTS variable.track_variable_changes();
DROP TRIGGER IF EXISTS trg_variables_updated_at ON variable.variables;

DROP TABLE IF EXISTS variable.variable_history;
DROP TABLE IF EXISTS variable.variables;

COMMIT;
