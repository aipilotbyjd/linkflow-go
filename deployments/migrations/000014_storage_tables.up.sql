-- ============================================================================
-- Migration: 000014_storage_tables
-- Description: Create file storage tables
-- Schema: storage
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Buckets table - Storage buckets/containers
-- ---------------------------------------------------------------------------
CREATE TABLE storage.buckets (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(100) NOT NULL,
    
    -- Configuration
    storage_type    VARCHAR(20) DEFAULT 'local' CHECK (storage_type IN ('local', 's3', 'gcs', 'azure')),
    config          JSONB DEFAULT '{}',
    
    -- Limits
    max_file_size_bytes BIGINT DEFAULT 104857600, -- 100MB
    allowed_mime_types  TEXT[] DEFAULT '{}',
    
    -- Access
    is_public       BOOLEAN DEFAULT FALSE,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT buckets_name_unique UNIQUE (name)
);

-- ---------------------------------------------------------------------------
-- Files table - File metadata
-- ---------------------------------------------------------------------------
CREATE TABLE storage.files (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bucket_id       UUID NOT NULL REFERENCES storage.buckets(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES auth.teams(id) ON DELETE SET NULL,
    
    -- File info
    name            VARCHAR(255) NOT NULL,
    original_name   VARCHAR(255) NOT NULL,
    path            VARCHAR(500) NOT NULL,
    
    -- Type and size
    mime_type       VARCHAR(100) NOT NULL,
    size_bytes      BIGINT NOT NULL,
    
    -- Integrity
    checksum_md5    VARCHAR(32),
    checksum_sha256 VARCHAR(64),
    
    -- Metadata
    metadata        JSONB DEFAULT '{}',
    
    -- Access
    is_public       BOOLEAN DEFAULT FALSE,
    
    -- Expiration
    expires_at      TIMESTAMP,
    
    -- Timestamps
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMP
);

CREATE INDEX idx_files_bucket_id ON storage.files(bucket_id);
CREATE INDEX idx_files_user_id ON storage.files(user_id);
CREATE INDEX idx_files_team_id ON storage.files(team_id);
CREATE INDEX idx_files_path ON storage.files(path);
CREATE INDEX idx_files_mime_type ON storage.files(mime_type);
CREATE INDEX idx_files_deleted_at ON storage.files(deleted_at) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- File Shares table - File sharing
-- ---------------------------------------------------------------------------
CREATE TABLE storage.file_shares (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    file_id         UUID NOT NULL REFERENCES storage.files(id) ON DELETE CASCADE,
    
    -- Share target
    shared_with_user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    shared_with_team_id UUID REFERENCES auth.teams(id) ON DELETE CASCADE,
    
    -- Public share
    share_token     VARCHAR(100),
    
    -- Permissions
    permission      VARCHAR(20) DEFAULT 'view' CHECK (permission IN ('view', 'download', 'edit')),
    
    -- Limits
    download_limit  INTEGER,
    download_count  INTEGER DEFAULT 0,
    
    expires_at      TIMESTAMP,
    
    shared_by       UUID NOT NULL REFERENCES auth.users(id),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT file_shares_token_unique UNIQUE (share_token)
);

CREATE INDEX idx_file_shares_file_id ON storage.file_shares(file_id);
CREATE INDEX idx_file_shares_token ON storage.file_shares(share_token) WHERE share_token IS NOT NULL;

-- ---------------------------------------------------------------------------
-- File Access Logs table - Access audit
-- ---------------------------------------------------------------------------
CREATE TABLE storage.file_access_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    file_id         UUID NOT NULL REFERENCES storage.files(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    
    action          VARCHAR(20) NOT NULL CHECK (action IN ('view', 'download', 'upload', 'delete', 'share')),
    
    ip_address      VARCHAR(45),
    user_agent      TEXT,
    
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_file_access_logs_file_id ON storage.file_access_logs(file_id);
CREATE INDEX idx_file_access_logs_created_at ON storage.file_access_logs(created_at DESC);

-- ---------------------------------------------------------------------------
-- Insert default buckets
-- ---------------------------------------------------------------------------
INSERT INTO storage.buckets (name, storage_type, max_file_size_bytes, allowed_mime_types) VALUES
    ('workflows', 'local', 10485760, ARRAY['application/json']),
    ('attachments', 'local', 104857600, ARRAY['image/*', 'application/pdf', 'text/*']),
    ('exports', 'local', 524288000, ARRAY['application/zip', 'application/json'])
ON CONFLICT (name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_buckets_updated_at 
    BEFORE UPDATE ON storage.buckets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_files_updated_at 
    BEFORE UPDATE ON storage.files
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
