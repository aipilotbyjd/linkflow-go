-- ============================================================================
-- Migration: 000014_storage_tables (ROLLBACK)
-- Description: Drop file storage tables
-- ============================================================================

BEGIN;

DROP TRIGGER IF EXISTS trg_files_updated_at ON storage.files;
DROP TRIGGER IF EXISTS trg_buckets_updated_at ON storage.buckets;

DROP TABLE IF EXISTS storage.file_access_logs;
DROP TABLE IF EXISTS storage.file_shares;
DROP TABLE IF EXISTS storage.files;
DROP TABLE IF EXISTS storage.buckets;

COMMIT;
