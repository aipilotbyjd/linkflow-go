-- ============================================================================
-- Migration: 000006_schedule_tables
-- Description: Create scheduling tables
-- Schema: schedule
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Schedules table - Workflow schedules
-- ---------------------------------------------------------------------------
CREATE TABLE schedule.schedules (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id     UUID NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES auth.teams(id) ON DELETE SET NULL,
    
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    
    -- Schedule configuration
    cron_expression VARCHAR(100) NOT NULL,
    timezone        VARCHAR(50) DEFAULT 'UTC',
    
    -- Input data for scheduled runs
    input_data      JSONB DEFAULT '{}',
    
    -- Status
    is_active       BOOLEAN DEFAULT TRUE,
    
    -- Date constraints
    start_date      TIMESTAMP,
    end_date        TIMESTAMP,
    
    -- Execution tracking
    last_run_at     TIMESTAMP,
    next_run_at     TIMESTAMP,
    run_count       BIGINT DEFAULT 0,
    
    -- Error handling
    misfire_policy  VARCHAR(20) DEFAULT 'skip' CHECK (misfire_policy IN ('skip', 'run_once', 'run_all')),
    max_retries     INTEGER DEFAULT 3,
    
    -- Metadata
    tags            TEXT[] DEFAULT '{}',
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_schedules_workflow_id ON schedule.schedules(workflow_id);
CREATE INDEX idx_schedules_user_id ON schedule.schedules(user_id);
CREATE INDEX idx_schedules_is_active ON schedule.schedules(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_schedules_next_run_at ON schedule.schedules(next_run_at) WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- Schedule Executions table - History of scheduled runs
-- ---------------------------------------------------------------------------
CREATE TABLE schedule.schedule_executions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    schedule_id     UUID NOT NULL REFERENCES schedule.schedules(id) ON DELETE CASCADE,
    execution_id    UUID REFERENCES execution.workflow_executions(id) ON DELETE SET NULL,
    
    -- Timing
    scheduled_at    TIMESTAMP NOT NULL,
    triggered_at    TIMESTAMP,
    
    -- Status
    status          VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'triggered', 'skipped', 'failed')),
    
    -- Error info
    error_message   TEXT,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_schedule_executions_schedule_id ON schedule.schedule_executions(schedule_id);
CREATE INDEX idx_schedule_executions_scheduled_at ON schedule.schedule_executions(scheduled_at DESC);

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_schedules_updated_at 
    BEFORE UPDATE ON schedule.schedules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
