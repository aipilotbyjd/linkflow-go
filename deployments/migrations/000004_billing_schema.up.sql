-- Billing schema
CREATE SCHEMA IF NOT EXISTS billing;

-- Customers table
CREATE TABLE IF NOT EXISTS billing.customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    stripe_id VARCHAR(255) UNIQUE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    default_payment_id VARCHAR(255),
    balance BIGINT DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'usd',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Plans table
CREATE TABLE IF NOT EXISTS billing.plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stripe_id VARCHAR(255) UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    interval VARCHAR(20) NOT NULL,
    price BIGINT NOT NULL,
    currency VARCHAR(3) DEFAULT 'usd',
    features JSONB DEFAULT '[]',
    limits JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT TRUE,
    trial_days INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Subscriptions table
CREATE TABLE IF NOT EXISTS billing.subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stripe_id VARCHAR(255) UNIQUE,
    customer_id UUID NOT NULL REFERENCES billing.customers(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES billing.plans(id),
    status VARCHAR(50) NOT NULL,
    current_period_start TIMESTAMP NOT NULL,
    current_period_end TIMESTAMP NOT NULL,
    trial_start TIMESTAMP,
    trial_end TIMESTAMP,
    cancelled_at TIMESTAMP,
    cancel_at_period_end BOOLEAN DEFAULT FALSE,
    quantity INTEGER DEFAULT 1,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Invoices table
CREATE TABLE IF NOT EXISTS billing.invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stripe_id VARCHAR(255) UNIQUE,
    customer_id UUID NOT NULL REFERENCES billing.customers(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES billing.subscriptions(id),
    number VARCHAR(100),
    status VARCHAR(50) NOT NULL,
    currency VARCHAR(3) DEFAULT 'usd',
    subtotal BIGINT DEFAULT 0,
    tax BIGINT DEFAULT 0,
    total BIGINT DEFAULT 0,
    amount_paid BIGINT DEFAULT 0,
    amount_due BIGINT DEFAULT 0,
    lines JSONB DEFAULT '[]',
    period_start TIMESTAMP,
    period_end TIMESTAMP,
    due_date TIMESTAMP,
    paid_at TIMESTAMP,
    hosted_url TEXT,
    pdf_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Payment methods table
CREATE TABLE IF NOT EXISTS billing.payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stripe_id VARCHAR(255) UNIQUE,
    customer_id UUID NOT NULL REFERENCES billing.customers(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    card JSONB,
    billing_details JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Usage tracking table
CREATE TABLE IF NOT EXISTS billing.usage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES billing.customers(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES billing.subscriptions(id),
    metric VARCHAR(100) NOT NULL,
    quantity BIGINT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Credits table
CREATE TABLE IF NOT EXISTS billing.credits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES billing.customers(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    description TEXT,
    expires_at TIMESTAMP,
    used_amount BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Promo codes table
CREATE TABLE IF NOT EXISTS billing.promo_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) UNIQUE NOT NULL,
    discount_type VARCHAR(20) NOT NULL,
    discount_amount BIGINT NOT NULL,
    max_redemptions INTEGER,
    times_redeemed INTEGER DEFAULT 0,
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    applicable_plans JSONB DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_customers_user_id ON billing.customers(user_id);
CREATE INDEX idx_customers_stripe_id ON billing.customers(stripe_id);
CREATE INDEX idx_subscriptions_customer_id ON billing.subscriptions(customer_id);
CREATE INDEX idx_subscriptions_status ON billing.subscriptions(status);
CREATE INDEX idx_invoices_customer_id ON billing.invoices(customer_id);
CREATE INDEX idx_invoices_status ON billing.invoices(status);
CREATE INDEX idx_usage_customer_id ON billing.usage(customer_id);
CREATE INDEX idx_usage_metric ON billing.usage(metric);
CREATE INDEX idx_usage_timestamp ON billing.usage(timestamp);

-- Insert default plans
INSERT INTO billing.plans (name, description, interval, price, features, limits, trial_days) VALUES
('Free', 'Free tier for individuals', 'monthly', 0, 
 '["5 workflows", "100 executions/month", "Community support"]',
 '{"workflows": 5, "executions": 100, "teamMembers": 1, "storageGb": 1}',
 0),
('Starter', 'For small teams getting started', 'monthly', 2900,
 '["25 workflows", "2,500 executions/month", "Email support", "5 team members"]',
 '{"workflows": 25, "executions": 2500, "teamMembers": 5, "storageGb": 10}',
 14),
('Pro', 'For growing businesses', 'monthly', 9900,
 '["Unlimited workflows", "25,000 executions/month", "Priority support", "25 team members", "Advanced analytics"]',
 '{"workflows": -1, "executions": 25000, "teamMembers": 25, "storageGb": 100}',
 14),
('Enterprise', 'For large organizations', 'monthly', 29900,
 '["Unlimited everything", "Dedicated support", "SLA", "SSO", "Audit logs", "Custom integrations"]',
 '{"workflows": -1, "executions": -1, "teamMembers": -1, "storageGb": -1}',
 30)
ON CONFLICT DO NOTHING;

-- Triggers
CREATE TRIGGER update_customers_updated_at BEFORE UPDATE ON billing.customers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_subscriptions_updated_at BEFORE UPDATE ON billing.subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_invoices_updated_at BEFORE UPDATE ON billing.invoices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
