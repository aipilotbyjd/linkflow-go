-- ============================================================================
-- Migration: 000015_billing_tables
-- Description: Create billing and subscription tables
-- Schema: billing
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Plans table - Subscription plans
-- ---------------------------------------------------------------------------
CREATE TABLE billing.plans (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(50) NOT NULL,
    slug            VARCHAR(50) NOT NULL,
    description     TEXT,
    
    -- Pricing
    price_monthly   DECIMAL(10, 2) NOT NULL DEFAULT 0,
    price_yearly    DECIMAL(10, 2) NOT NULL DEFAULT 0,
    currency        VARCHAR(3) DEFAULT 'USD',
    
    -- Limits
    max_workflows           INTEGER DEFAULT 5,
    max_executions_month    INTEGER DEFAULT 1000,
    max_team_members        INTEGER DEFAULT 1,
    max_storage_bytes       BIGINT DEFAULT 104857600, -- 100MB
    
    -- Features (JSON flags)
    features        JSONB DEFAULT '{}',
    
    -- Status
    is_active       BOOLEAN DEFAULT TRUE,
    is_public       BOOLEAN DEFAULT TRUE,
    
    -- Ordering
    sort_order      INTEGER DEFAULT 0,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT plans_slug_unique UNIQUE (slug)
);

-- ---------------------------------------------------------------------------
-- Subscriptions table - User/team subscriptions
-- ---------------------------------------------------------------------------
CREATE TABLE billing.subscriptions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES auth.teams(id) ON DELETE CASCADE,
    plan_id         UUID NOT NULL REFERENCES billing.plans(id),
    
    -- Status
    status          VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'canceled', 'past_due', 'trialing', 'paused')),
    
    -- Billing cycle
    billing_cycle   VARCHAR(10) DEFAULT 'monthly' CHECK (billing_cycle IN ('monthly', 'yearly')),
    
    -- Dates
    trial_ends_at       TIMESTAMP,
    current_period_start TIMESTAMP NOT NULL,
    current_period_end  TIMESTAMP NOT NULL,
    canceled_at         TIMESTAMP,
    
    -- External provider
    provider            VARCHAR(20) CHECK (provider IN ('stripe', 'paddle', 'manual')),
    provider_subscription_id VARCHAR(100),
    provider_customer_id VARCHAR(100),
    
    -- Metadata
    metadata        JSONB DEFAULT '{}',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_subscriptions_user_id ON billing.subscriptions(user_id);
CREATE INDEX idx_subscriptions_team_id ON billing.subscriptions(team_id);
CREATE INDEX idx_subscriptions_status ON billing.subscriptions(status);
CREATE INDEX idx_subscriptions_provider_id ON billing.subscriptions(provider_subscription_id);

-- ---------------------------------------------------------------------------
-- Invoices table - Billing invoices
-- ---------------------------------------------------------------------------
CREATE TABLE billing.invoices (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subscription_id UUID NOT NULL REFERENCES billing.subscriptions(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id),
    
    -- Invoice details
    invoice_number  VARCHAR(50) NOT NULL,
    
    -- Amounts
    subtotal        DECIMAL(10, 2) NOT NULL,
    tax             DECIMAL(10, 2) DEFAULT 0,
    discount        DECIMAL(10, 2) DEFAULT 0,
    total           DECIMAL(10, 2) NOT NULL,
    currency        VARCHAR(3) DEFAULT 'USD',
    
    -- Status
    status          VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'open', 'paid', 'void', 'uncollectible')),
    
    -- Dates
    due_date        TIMESTAMP,
    paid_at         TIMESTAMP,
    
    -- External provider
    provider_invoice_id VARCHAR(100),
    
    -- PDF
    pdf_url         TEXT,
    
    -- Line items stored as JSON
    line_items      JSONB DEFAULT '[]',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT invoices_number_unique UNIQUE (invoice_number)
);

CREATE INDEX idx_invoices_subscription_id ON billing.invoices(subscription_id);
CREATE INDEX idx_invoices_user_id ON billing.invoices(user_id);
CREATE INDEX idx_invoices_status ON billing.invoices(status);
CREATE INDEX idx_invoices_created_at ON billing.invoices(created_at DESC);

-- ---------------------------------------------------------------------------
-- Payment Methods table
-- ---------------------------------------------------------------------------
CREATE TABLE billing.payment_methods (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    
    -- Type
    type            VARCHAR(20) NOT NULL CHECK (type IN ('card', 'bank_account', 'paypal')),
    
    -- Card details (masked)
    card_brand      VARCHAR(20),
    card_last4      VARCHAR(4),
    card_exp_month  INTEGER,
    card_exp_year   INTEGER,
    
    -- Status
    is_default      BOOLEAN DEFAULT FALSE,
    
    -- External provider
    provider            VARCHAR(20) CHECK (provider IN ('stripe', 'paddle')),
    provider_method_id  VARCHAR(100),
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_payment_methods_user_id ON billing.payment_methods(user_id);

-- ---------------------------------------------------------------------------
-- Usage Records table - Track usage for metered billing
-- ---------------------------------------------------------------------------
CREATE TABLE billing.usage_records (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subscription_id UUID NOT NULL REFERENCES billing.subscriptions(id) ON DELETE CASCADE,
    
    -- Usage type
    metric          VARCHAR(50) NOT NULL,
    quantity        BIGINT NOT NULL DEFAULT 0,
    
    -- Period
    period_start    TIMESTAMP NOT NULL,
    period_end      TIMESTAMP NOT NULL,
    
    -- Metadata
    metadata        JSONB DEFAULT '{}',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_usage_records_subscription_id ON billing.usage_records(subscription_id);
CREATE INDEX idx_usage_records_metric ON billing.usage_records(metric);
CREATE INDEX idx_usage_records_period ON billing.usage_records(period_start, period_end);

-- ---------------------------------------------------------------------------
-- Coupons table
-- ---------------------------------------------------------------------------
CREATE TABLE billing.coupons (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(50) NOT NULL,
    name            VARCHAR(100),
    
    -- Discount
    discount_type   VARCHAR(10) NOT NULL CHECK (discount_type IN ('percent', 'fixed')),
    discount_value  DECIMAL(10, 2) NOT NULL,
    currency        VARCHAR(3) DEFAULT 'USD',
    
    -- Limits
    max_redemptions     INTEGER,
    redemption_count    INTEGER DEFAULT 0,
    
    -- Validity
    valid_from      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    valid_until     TIMESTAMP,
    
    -- Restrictions
    applies_to_plans    UUID[] DEFAULT '{}',
    min_amount          DECIMAL(10, 2),
    
    is_active       BOOLEAN DEFAULT TRUE,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT coupons_code_unique UNIQUE (code)
);

CREATE INDEX idx_coupons_code ON billing.coupons(code);

-- ---------------------------------------------------------------------------
-- Insert default plans
-- ---------------------------------------------------------------------------
INSERT INTO billing.plans (name, slug, price_monthly, price_yearly, max_workflows, max_executions_month, max_team_members, max_storage_bytes, features, sort_order) VALUES
    ('Free', 'free', 0, 0, 5, 1000, 1, 104857600, '{"support": "community", "api_access": false}', 1),
    ('Starter', 'starter', 19.00, 190.00, 20, 10000, 3, 1073741824, '{"support": "email", "api_access": true}', 2),
    ('Pro', 'pro', 49.00, 490.00, 100, 100000, 10, 10737418240, '{"support": "priority", "api_access": true, "custom_nodes": true}', 3),
    ('Enterprise', 'enterprise', 199.00, 1990.00, -1, -1, -1, -1, '{"support": "dedicated", "api_access": true, "custom_nodes": true, "sso": true, "audit_logs": true}', 4)
ON CONFLICT (slug) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_plans_updated_at 
    BEFORE UPDATE ON billing.plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_subscriptions_updated_at 
    BEFORE UPDATE ON billing.subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_invoices_updated_at 
    BEFORE UPDATE ON billing.invoices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_payment_methods_updated_at 
    BEFORE UPDATE ON billing.payment_methods
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
