-- ============================================================================
-- Migration: 000004_execution_tables (ROLLBACK)
-- Description: Drop workflow execution tables
-- ============================================================================

BEGIN;

DROP TABLE IF EXISTS execution.execution_metrics;
DROP TABLE IF EXISTS execution.execution_checkpoints;
DROP TABLE IF EXISTS execution.execution_queue;
DROP TABLE IF EXISTS execution.node_executions;
DROP TABLE IF EXISTS execution.workflow_executions;

COMMIT;
