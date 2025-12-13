-- Variables schema
CREATE SCHEMA IF NOT EXISTS variable;

-- Global variables table
CREATE TABLE IF NOT EXISTS variable.variables (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key VARCHAR(50) UNIQUE NOT NULL,
    value TEXT NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'string',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Storage schema
CREATE SCHEMA IF NOT EXISTS storage;

-- Files table
CREATE TABLE IF NOT EXISTS storage.files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id UUID REFERENCES auth.teams(id),
    name VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    path VARCHAR(500) NOT NULL,
    bucket VARCHAR(100) NOT NULL,
    storage_type VARCHAR(50) DEFAULT 'local',
    checksum VARCHAR(64),
    metadata JSONB DEFAULT '{}',
    is_public BOOLEAN DEFAULT FALSE,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- File access logs
CREATE TABLE IF NOT EXISTS storage.file_access_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    file_id UUID NOT NULL REFERENCES storage.files(id) ON DELETE CASCADE,
    user_id UUID REFERENCES auth.users(id),
    action VARCHAR(50) NOT NULL,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_variables_key ON variable.variables(key);
CREATE INDEX idx_files_user_id ON storage.files(user_id);
CREATE INDEX idx_files_team_id ON storage.files(team_id);
CREATE INDEX idx_files_bucket ON storage.files(bucket);
CREATE INDEX idx_file_access_logs_file_id ON storage.file_access_logs(file_id);

-- Triggers
CREATE TRIGGER update_variables_updated_at BEFORE UPDATE ON variable.variables
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_files_updated_at BEFORE UPDATE ON storage.files
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
