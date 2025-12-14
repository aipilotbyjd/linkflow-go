-- ============================================================================
-- Migration: 000005_node_tables (ROLLBACK)
-- Description: Drop node registry tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_custom_nodes_updated_at ON node.custom_nodes;
DROP TRIGGER IF EXISTS trg_node_types_updated_at ON node.node_types;

DROP TABLE IF EXISTS node.custom_nodes;
DROP TABLE IF EXISTS node.node_types;

COMMIT;
