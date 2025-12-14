-- ============================================================================
-- Migration: 000007_credential_tables (ROLLBACK)
-- Description: Drop credential management tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_credentials_updated_at ON credential.credentials;
DROP TRIGGER IF EXISTS trg_credential_types_updated_at ON credential.credential_types;

DROP TABLE IF EXISTS credential.credential_usage_log;
DROP TABLE IF EXISTS credential.credential_shares;
DROP TABLE IF EXISTS credential.credentials;
DROP TABLE IF EXISTS credential.credential_types;

COMMIT;
