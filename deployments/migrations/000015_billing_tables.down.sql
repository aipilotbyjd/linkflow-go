-- ============================================================================
-- Migration: 000015_billing_tables (ROLLBACK)
-- Description: Drop billing and subscription tables
-- Schema: billing
-- ============================================================================

BEGIN;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_payment_methods_updated_at ON billing.payment_methods;
DROP TRIGGER IF EXISTS trg_invoices_updated_at ON billing.invoices;
DROP TRIGGER IF EXISTS trg_subscriptions_updated_at ON billing.subscriptions;
DROP TRIGGER IF EXISTS trg_plans_updated_at ON billing.plans;

-- Drop tables in reverse order of creation
DROP TABLE IF EXISTS billing.coupons;
DROP TABLE IF EXISTS billing.usage_records;
DROP TABLE IF EXISTS billing.payment_methods;
DROP TABLE IF EXISTS billing.invoices;
DROP TABLE IF EXISTS billing.subscriptions;
DROP TABLE IF EXISTS billing.plans;

COMMIT;
