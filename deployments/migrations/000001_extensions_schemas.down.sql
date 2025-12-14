-- ============================================================================
-- Migration: 000001_extensions_schemas (ROLLBACK)
-- Description: Drop all schemas and extensions
-- WARNING: This will delete ALL data in all schemas!
-- ============================================================================

BEGIN;

-- Drop schemas (CASCADE will drop all objects within)
DROP SCHEMA IF EXISTS template CASCADE;
DROP SCHEMA IF EXISTS billing CASCADE;
DROP SCHEMA IF EXISTS storage CASCADE;
DROP SCHEMA IF EXISTS search CASCADE;
DROP SCHEMA IF EXISTS analytics CASCADE;
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS notification CASCADE;
DROP SCHEMA IF EXISTS variable CASCADE;
DROP SCHEMA IF EXISTS webhook CASCADE;
DROP SCHEMA IF EXISTS credential CASCADE;
DROP SCHEMA IF EXISTS schedule CASCADE;
DROP SCHEMA IF EXISTS node CASCADE;
DROP SCHEMA IF EXISTS execution CASCADE;
DROP SCHEMA IF EXISTS workflow CASCADE;
DROP SCHEMA IF EXISTS auth CASCADE;

-- Drop common function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop extensions
DROP EXTENSION IF EXISTS "btree_gin";
DROP EXTENSION IF EXISTS "pg_trgm";
DROP EXTENSION IF EXISTS "uuid-ossp";

COMMIT;
