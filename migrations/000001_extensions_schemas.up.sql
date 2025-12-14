-- ============================================================================
-- Migration: 000001_extensions_schemas
-- Description: Initialize PostgreSQL extensions and create database schemas
-- ============================================================================

BEGIN;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";      -- UUID generation
CREATE EXTENSION IF NOT EXISTS "pg_trgm";        -- Trigram text search
CREATE EXTENSION IF NOT EXISTS "btree_gin";      -- GIN index support

-- Create schemas for each service domain
CREATE SCHEMA IF NOT EXISTS auth;          -- Authentication & authorization
CREATE SCHEMA IF NOT EXISTS workflow;      -- Workflow management
CREATE SCHEMA IF NOT EXISTS execution;     -- Workflow execution
CREATE SCHEMA IF NOT EXISTS node;          -- Node registry
CREATE SCHEMA IF NOT EXISTS schedule;      -- Scheduling
CREATE SCHEMA IF NOT EXISTS credential;    -- Credentials management
CREATE SCHEMA IF NOT EXISTS webhook;       -- Webhook handling
CREATE SCHEMA IF NOT EXISTS variable;      -- Variables storage
CREATE SCHEMA IF NOT EXISTS notification;  -- Notifications
CREATE SCHEMA IF NOT EXISTS audit;         -- Audit logging
CREATE SCHEMA IF NOT EXISTS analytics;     -- Analytics & metrics
CREATE SCHEMA IF NOT EXISTS search;        -- Search indexing
CREATE SCHEMA IF NOT EXISTS storage;       -- File storage
CREATE SCHEMA IF NOT EXISTS billing;       -- Billing & subscriptions
CREATE SCHEMA IF NOT EXISTS template;      -- Workflow templates

-- Create common function for updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_updated_at_column() IS 'Automatically updates updated_at column on row update';

COMMIT;
