-- Drop billing schema
DROP TRIGGER IF EXISTS update_invoices_updated_at ON billing.invoices;
DROP TRIGGER IF EXISTS update_subscriptions_updated_at ON billing.subscriptions;
DROP TRIGGER IF EXISTS update_customers_updated_at ON billing.customers;

DROP TABLE IF EXISTS billing.promo_codes;
DROP TABLE IF EXISTS billing.credits;
DROP TABLE IF EXISTS billing.usage;
DROP TABLE IF EXISTS billing.payment_methods;
DROP TABLE IF EXISTS billing.invoices;
DROP TABLE IF EXISTS billing.subscriptions;
DROP TABLE IF EXISTS billing.plans;
DROP TABLE IF EXISTS billing.customers;

DROP SCHEMA IF EXISTS billing;
