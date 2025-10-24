-- Core indexer registration tables (compatible with both PostgreSQL and SQLite)
-- Requires: 000001_core_schema.up.sql

-- Indexers table (tracks registered indexer instances)
CREATE TABLE IF NOT EXISTS indexers (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Indexer identity
    indexer_type TEXT NOT NULL,
    workspace_path TEXT,
    
    -- Connection info
    hostname TEXT,
    version TEXT,
    
    -- Status tracking
    status TEXT DEFAULT 'active',
    last_heartbeat_at TIMESTAMP,
    document_count INTEGER DEFAULT 0,
    
    -- Metadata
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_indexers_deleted_at ON indexers(deleted_at);
CREATE INDEX IF NOT EXISTS idx_indexers_status ON indexers(status);
CREATE INDEX IF NOT EXISTS idx_indexers_type ON indexers(indexer_type);

-- IndexerTokens table (manages authentication tokens for indexers)
CREATE TABLE IF NOT EXISTS indexer_tokens (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Token info
    token_hash TEXT NOT NULL UNIQUE,
    token_type TEXT DEFAULT 'api',
    
    -- Lifecycle
    expires_at TIMESTAMP,
    revoked INTEGER NOT NULL DEFAULT 0,
    revoked_at TIMESTAMP,
    revoked_reason TEXT,
    
    -- Association
    indexer_id TEXT REFERENCES indexers(id) ON DELETE CASCADE,
    
    -- Metadata
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_indexer_tokens_deleted_at ON indexer_tokens(deleted_at);
CREATE INDEX IF NOT EXISTS idx_indexer_tokens_hash ON indexer_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_indexer_tokens_indexer_id ON indexer_tokens(indexer_id);
CREATE INDEX IF NOT EXISTS idx_indexer_tokens_expires_at ON indexer_tokens(expires_at);

-- IndexerFolders table (tracks indexed folders for workspace scanning)
CREATE TABLE IF NOT EXISTS indexer_folders (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Folder identification
    google_drive_id TEXT NOT NULL UNIQUE,
    
    -- Tracking
    last_indexed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_indexer_folders_deleted_at ON indexer_folders(deleted_at);
CREATE INDEX IF NOT EXISTS idx_indexer_folders_google_drive_id ON indexer_folders(google_drive_id);
