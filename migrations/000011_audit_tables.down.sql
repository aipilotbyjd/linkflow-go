-- ============================================================================
-- Migration: 000011_audit_tables (ROLLBACK)
-- Description: Drop audit logging tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_retention_policies_updated_at ON audit.retention_policies;

DROP TABLE IF EXISTS audit.retention_policies;
DROP TABLE IF EXISTS audit.security_events;
DROP TABLE IF EXISTS audit.logs;

COMMIT;
