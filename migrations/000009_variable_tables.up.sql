-- ============================================================================
-- Migration: 000009_variable_tables
-- Description: Create variable storage tables
-- Schema: variable
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Variables table - Global and scoped variables
-- ---------------------------------------------------------------------------
CREATE TABLE variable.variables (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Ownership
    user_id         UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES auth.teams(id) ON DELETE CASCADE,
    workflow_id     UUID REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    
    -- Variable definition
    key             VARCHAR(100) NOT NULL,
    value           TEXT NOT NULL,
    type            VARCHAR(20) DEFAULT 'string' CHECK (type IN ('string', 'number', 'boolean', 'json', 'secret')),
    
    -- Metadata
    description     TEXT,
    is_secret       BOOLEAN DEFAULT FALSE,
    
    -- Scope
    scope           VARCHAR(20) DEFAULT 'user' CHECK (scope IN ('global', 'user', 'team', 'workflow')),
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Unique constraint based on scope
    CONSTRAINT variables_scope_key_unique UNIQUE (user_id, team_id, workflow_id, key)
);

CREATE INDEX idx_variables_user_id ON variable.variables(user_id);
CREATE INDEX idx_variables_team_id ON variable.variables(team_id);
CREATE INDEX idx_variables_workflow_id ON variable.variables(workflow_id);
CREATE INDEX idx_variables_key ON variable.variables(key);
CREATE INDEX idx_variables_scope ON variable.variables(scope);

-- ---------------------------------------------------------------------------
-- Variable History table - Change tracking
-- ---------------------------------------------------------------------------
CREATE TABLE variable.variable_history (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    variable_id     UUID NOT NULL REFERENCES variable.variables(id) ON DELETE CASCADE,
    
    old_value       TEXT,
    new_value       TEXT NOT NULL,
    
    changed_by      UUID REFERENCES auth.users(id),
    change_reason   TEXT,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_variable_history_variable_id ON variable.variable_history(variable_id);
CREATE INDEX idx_variable_history_created_at ON variable.variable_history(created_at DESC);

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_variables_updated_at 
    BEFORE UPDATE ON variable.variables
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to track variable changes
CREATE OR REPLACE FUNCTION variable.track_variable_changes()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.value IS DISTINCT FROM NEW.value THEN
        INSERT INTO variable.variable_history (variable_id, old_value, new_value)
        VALUES (NEW.id, OLD.value, NEW.value);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_variables_track_changes
    AFTER UPDATE ON variable.variables
    FOR EACH ROW EXECUTE FUNCTION variable.track_variable_changes();

COMMIT;
