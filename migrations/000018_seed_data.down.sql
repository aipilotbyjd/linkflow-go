-- ============================================================================
-- Migration: 000018_seed_data (ROLLBACK)
-- Description: Remove seed data
-- ============================================================================

BEGIN;

-- Remove system configuration
DELETE FROM variable.variables WHERE scope = 'global' AND key LIKE 'system.%';

-- Remove default storage buckets
DELETE FROM storage.buckets WHERE name IN ('workflows', 'attachments', 'templates', 'avatars');

-- Remove notification templates
DELETE FROM notification.templates WHERE name IN (
    'welcome_email', 
    'execution_failed', 
    'execution_completed', 
    'password_reset'
);

-- Remove built-in nodes
DELETE FROM node.nodes WHERE is_builtin = true;

-- Remove node categories
DELETE FROM node.categories WHERE name IN (
    'triggers', 'actions', 'flow', 'data', 
    'integrations', 'utilities', 'ai', 'custom'
);

-- Remove default billing plans
DELETE FROM billing.plans WHERE name IN ('free', 'starter', 'professional', 'enterprise');

-- Remove default roles
DELETE FROM auth.roles WHERE is_system = true;

COMMIT;
