-- Notifications schema
CREATE SCHEMA IF NOT EXISTS notification;

-- Notification channels table
CREATE TABLE IF NOT EXISTS notification.channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN DEFAULT TRUE,
    is_verified BOOLEAN DEFAULT FALSE,
    verified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Notification preferences
CREATE TABLE IF NOT EXISTS notification.preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    email_enabled BOOLEAN DEFAULT TRUE,
    push_enabled BOOLEAN DEFAULT TRUE,
    slack_enabled BOOLEAN DEFAULT FALSE,
    webhook_enabled BOOLEAN DEFAULT FALSE,
    execution_success BOOLEAN DEFAULT FALSE,
    execution_failure BOOLEAN DEFAULT TRUE,
    workflow_shared BOOLEAN DEFAULT TRUE,
    team_invite BOOLEAN DEFAULT TRUE,
    billing_alerts BOOLEAN DEFAULT TRUE,
    weekly_digest BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Notification queue
CREATE TABLE IF NOT EXISTS notification.queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    channel_id UUID REFERENCES notification.channels(id),
    type VARCHAR(50) NOT NULL,
    priority VARCHAR(20) DEFAULT 'normal',
    subject VARCHAR(255),
    body TEXT NOT NULL,
    data JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    scheduled_at TIMESTAMP,
    sent_at TIMESTAMP,
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Notification history
CREATE TABLE IF NOT EXISTS notification.history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    channel_type VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    subject VARCHAR(255),
    body TEXT,
    status VARCHAR(50) NOT NULL,
    sent_at TIMESTAMP,
    read_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Templates schema
CREATE SCHEMA IF NOT EXISTS template;

-- Workflow templates table
CREATE TABLE IF NOT EXISTS template.workflow_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    subcategory VARCHAR(100),
    icon VARCHAR(100),
    color VARCHAR(7),
    creator_id UUID REFERENCES auth.users(id),
    workflow_data JSONB NOT NULL,
    preview_image VARCHAR(500),
    is_official BOOLEAN DEFAULT FALSE,
    is_public BOOLEAN DEFAULT TRUE,
    is_featured BOOLEAN DEFAULT FALSE,
    version VARCHAR(20) DEFAULT '1.0.0',
    downloads INTEGER DEFAULT 0,
    rating DECIMAL(3,2) DEFAULT 0,
    ratings_count INTEGER DEFAULT 0,
    tags TEXT[],
    requirements JSONB DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Template categories
CREATE TABLE IF NOT EXISTS template.categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    icon VARCHAR(100),
    parent_id UUID REFERENCES template.categories(id),
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Template ratings
CREATE TABLE IF NOT EXISTS template.ratings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID NOT NULL REFERENCES template.workflow_templates(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    review TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(template_id, user_id)
);

-- Template downloads tracking
CREATE TABLE IF NOT EXISTS template.downloads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID NOT NULL REFERENCES template.workflow_templates(id) ON DELETE CASCADE,
    user_id UUID REFERENCES auth.users(id),
    ip_address VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_channels_user_id ON notification.channels(user_id);
CREATE INDEX idx_channels_type ON notification.channels(type);
CREATE INDEX idx_queue_user_id ON notification.queue(user_id);
CREATE INDEX idx_queue_status ON notification.queue(status);
CREATE INDEX idx_queue_scheduled_at ON notification.queue(scheduled_at);
CREATE INDEX idx_history_user_id ON notification.history(user_id);
CREATE INDEX idx_history_created_at ON notification.history(created_at);
CREATE INDEX idx_templates_category ON template.workflow_templates(category);
CREATE INDEX idx_templates_creator_id ON template.workflow_templates(creator_id);
CREATE INDEX idx_templates_is_public ON template.workflow_templates(is_public);
CREATE INDEX idx_templates_is_featured ON template.workflow_templates(is_featured);
CREATE INDEX idx_ratings_template_id ON template.ratings(template_id);
CREATE INDEX idx_downloads_template_id ON template.downloads(template_id);

-- Triggers
CREATE TRIGGER update_channels_updated_at BEFORE UPDATE ON notification.channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_preferences_updated_at BEFORE UPDATE ON notification.preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_templates_updated_at BEFORE UPDATE ON template.workflow_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert default categories
INSERT INTO template.categories (name, description, icon, sort_order) VALUES
('Marketing', 'Marketing automation workflows', 'megaphone', 1),
('Sales', 'Sales and CRM workflows', 'chart-line', 2),
('DevOps', 'Development and operations workflows', 'code', 3),
('Data', 'Data processing and ETL workflows', 'database', 4),
('Communication', 'Email, chat, and notification workflows', 'message-circle', 5),
('Productivity', 'Personal and team productivity workflows', 'zap', 6),
('Finance', 'Financial and accounting workflows', 'dollar-sign', 7),
('HR', 'Human resources workflows', 'users', 8),
('Support', 'Customer support workflows', 'headphones', 9),
('Social Media', 'Social media management workflows', 'share-2', 10)
ON CONFLICT (name) DO NOTHING;
