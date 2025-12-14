-- ============================================================================
-- Migration: 000010_notification_tables
-- Description: Create notification tables
-- Schema: notification
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Notification Channels table - User notification channels
-- ---------------------------------------------------------------------------
CREATE TABLE notification.channels (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    
    type            VARCHAR(30) NOT NULL CHECK (type IN ('email', 'slack', 'webhook', 'sms', 'push')),
    name            VARCHAR(100) NOT NULL,
    
    -- Channel configuration (encrypted for sensitive data)
    config          JSONB NOT NULL DEFAULT '{}',
    
    -- Verification
    is_verified     BOOLEAN DEFAULT FALSE,
    verified_at     TIMESTAMP,
    verification_token VARCHAR(100),
    
    is_active       BOOLEAN DEFAULT TRUE,
    is_default      BOOLEAN DEFAULT FALSE,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notification_channels_user_id ON notification.channels(user_id);
CREATE INDEX idx_notification_channels_type ON notification.channels(type);

-- ---------------------------------------------------------------------------
-- Notification Preferences table - User preferences
-- ---------------------------------------------------------------------------
CREATE TABLE notification.preferences (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    
    -- Event preferences
    execution_success   BOOLEAN DEFAULT FALSE,
    execution_failure   BOOLEAN DEFAULT TRUE,
    execution_timeout   BOOLEAN DEFAULT TRUE,
    workflow_shared     BOOLEAN DEFAULT TRUE,
    team_invite         BOOLEAN DEFAULT TRUE,
    billing_alerts      BOOLEAN DEFAULT TRUE,
    security_alerts     BOOLEAN DEFAULT TRUE,
    
    -- Digest preferences
    daily_digest        BOOLEAN DEFAULT FALSE,
    weekly_digest       BOOLEAN DEFAULT TRUE,
    
    -- Quiet hours
    quiet_hours_enabled BOOLEAN DEFAULT FALSE,
    quiet_hours_start   TIME,
    quiet_hours_end     TIME,
    quiet_hours_timezone VARCHAR(50) DEFAULT 'UTC',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT notification_preferences_user_unique UNIQUE (user_id)
);

-- ---------------------------------------------------------------------------
-- Notifications table - Notification records
-- ---------------------------------------------------------------------------
CREATE TABLE notification.notifications (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    
    -- Notification content
    type            VARCHAR(50) NOT NULL,
    title           VARCHAR(255) NOT NULL,
    message         TEXT,
    data            JSONB DEFAULT '{}',
    
    -- Related entities
    workflow_id     UUID REFERENCES workflow.workflows(id) ON DELETE SET NULL,
    execution_id    UUID REFERENCES execution.workflow_executions(id) ON DELETE SET NULL,
    
    -- Status
    is_read         BOOLEAN DEFAULT FALSE,
    read_at         TIMESTAMP,
    
    -- Priority
    priority        VARCHAR(10) DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    
    -- Action
    action_url      VARCHAR(500),
    action_label    VARCHAR(100),
    
    expires_at      TIMESTAMP,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notifications_user_id ON notification.notifications(user_id);
CREATE INDEX idx_notifications_is_read ON notification.notifications(user_id, is_read) WHERE is_read = FALSE;
CREATE INDEX idx_notifications_created_at ON notification.notifications(created_at DESC);
CREATE INDEX idx_notifications_type ON notification.notifications(type);

-- ---------------------------------------------------------------------------
-- Notification Queue table - Outbound notification queue
-- ---------------------------------------------------------------------------
CREATE TABLE notification.queue (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    notification_id UUID REFERENCES notification.notifications(id) ON DELETE CASCADE,
    channel_id      UUID REFERENCES notification.channels(id) ON DELETE CASCADE,
    
    -- Delivery info
    channel_type    VARCHAR(30) NOT NULL,
    recipient       VARCHAR(255) NOT NULL,
    
    -- Content
    subject         VARCHAR(255),
    body            TEXT NOT NULL,
    template_id     VARCHAR(100),
    template_data   JSONB DEFAULT '{}',
    
    -- Status
    status          VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'cancelled')),
    
    -- Retry
    attempts        INTEGER DEFAULT 0,
    max_attempts    INTEGER DEFAULT 3,
    last_error      TEXT,
    next_retry_at   TIMESTAMP,
    
    -- Timing
    scheduled_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    sent_at         TIMESTAMP,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notification_queue_status ON notification.queue(status, scheduled_at) WHERE status = 'pending';
CREATE INDEX idx_notification_queue_next_retry ON notification.queue(next_retry_at) WHERE status = 'pending';

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_notification_channels_updated_at 
    BEFORE UPDATE ON notification.channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_notification_preferences_updated_at 
    BEFORE UPDATE ON notification.preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
