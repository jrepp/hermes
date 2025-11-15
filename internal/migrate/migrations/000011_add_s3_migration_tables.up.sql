-- RFC-089: S3-Compatible Storage Backend and Document Migration System
-- Migration 000011: Add provider storage registry and migration orchestration tables

-- Provider Storage Registry
-- Tracks all configured storage providers (Google, S3, Local, Azure, Office365, etc.)
CREATE TABLE IF NOT EXISTS provider_storage (
    id BIGSERIAL PRIMARY KEY,

    -- Provider identification
    provider_name VARCHAR(100) NOT NULL UNIQUE,  -- "google-prod", "s3-archive", "local-edge-01"
    provider_type VARCHAR(50) NOT NULL,          -- "google", "s3", "local", "azure", "office365"

    -- Configuration (encrypted credentials stored as JSONB)
    config JSONB NOT NULL,

    -- Capabilities (what features this provider supports)
    capabilities JSONB,  -- {"versioning": true, "permissions": true, "search": false}

    -- Status and flags
    status VARCHAR(20) NOT NULL DEFAULT 'active',  -- 'active', 'readonly', 'disabled', 'migrating'
    is_primary BOOLEAN NOT NULL DEFAULT false,
    is_writable BOOLEAN NOT NULL DEFAULT true,

    -- Statistics
    document_count INTEGER DEFAULT 0,
    total_size_bytes BIGINT DEFAULT 0,
    last_health_check TIMESTAMP WITH TIME ZONE,
    health_status VARCHAR(20),  -- 'healthy', 'degraded', 'unhealthy'

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by TEXT,
    metadata JSONB
);

CREATE INDEX idx_provider_storage_type ON provider_storage(provider_type);
CREATE INDEX idx_provider_storage_status ON provider_storage(status);
CREATE INDEX idx_provider_storage_is_writable ON provider_storage(is_writable) WHERE is_writable = true;

-- Migration Jobs
-- Tracks migration operations between storage providers
CREATE TABLE IF NOT EXISTS migration_jobs (
    id BIGSERIAL PRIMARY KEY,

    -- Job identification
    job_uuid UUID NOT NULL UNIQUE,
    job_name VARCHAR(200) NOT NULL,

    -- Source and destination providers
    source_provider_id INTEGER NOT NULL REFERENCES provider_storage(id),
    dest_provider_id INTEGER NOT NULL REFERENCES provider_storage(id),

    -- Scope (what documents to migrate)
    filter_criteria JSONB,  -- {"document_type": "RFC", "status": "Published", "project_uuid": "..."}

    -- Migration strategy
    strategy VARCHAR(50) NOT NULL DEFAULT 'copy',  -- 'copy', 'move', 'mirror'
    transform_rules JSONB,  -- Content transformation rules

    -- State
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'running', 'paused', 'completed', 'failed', 'cancelled'

    -- Progress tracking
    total_documents INTEGER DEFAULT 0,
    migrated_documents INTEGER DEFAULT 0,
    failed_documents INTEGER DEFAULT 0,
    skipped_documents INTEGER DEFAULT 0,

    -- Timing
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    estimated_completion_at TIMESTAMP WITH TIME ZONE,

    -- Scheduling
    schedule_type VARCHAR(20) DEFAULT 'manual',  -- 'manual', 'scheduled', 'recurring'
    scheduled_at TIMESTAMP WITH TIME ZONE,       -- When to start (for scheduled)
    cron_expression VARCHAR(100),                -- Cron schedule (for recurring)
    next_run_at TIMESTAMP WITH TIME ZONE,        -- Next execution time (for recurring)
    recurrence_enabled BOOLEAN DEFAULT false,

    -- Configuration
    concurrency INTEGER DEFAULT 5,
    batch_size INTEGER DEFAULT 100,
    dry_run BOOLEAN DEFAULT false,

    -- Validation
    validate_after_migration BOOLEAN DEFAULT true,
    validation_status VARCHAR(20),  -- 'pending', 'passed', 'failed'
    validation_errors JSONB,

    -- Rollback support
    rollback_enabled BOOLEAN DEFAULT true,
    rollback_data JSONB,  -- Data needed to rollback

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL,
    metadata JSONB
);

CREATE INDEX idx_migration_jobs_status ON migration_jobs(status);
CREATE INDEX idx_migration_jobs_source ON migration_jobs(source_provider_id);
CREATE INDEX idx_migration_jobs_dest ON migration_jobs(dest_provider_id);
CREATE INDEX idx_migration_jobs_created ON migration_jobs(created_at);
CREATE INDEX idx_migration_jobs_scheduled ON migration_jobs(scheduled_at) WHERE status = 'pending' AND schedule_type = 'scheduled';
CREATE INDEX idx_migration_jobs_next_run ON migration_jobs(next_run_at) WHERE recurrence_enabled = true;
CREATE INDEX idx_migration_jobs_uuid ON migration_jobs(job_uuid);

-- Migration Items
-- Tracks individual document migration status within a job
CREATE TABLE IF NOT EXISTS migration_items (
    id BIGSERIAL PRIMARY KEY,

    -- Links to migration job
    migration_job_id BIGINT NOT NULL REFERENCES migration_jobs(id) ON DELETE CASCADE,

    -- Document identification
    document_uuid UUID NOT NULL,
    source_provider_id VARCHAR(500) NOT NULL,
    dest_provider_id VARCHAR(500),

    -- State
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'in_progress', 'completed', 'failed', 'skipped'

    -- Progress and retry logic
    attempt_count INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,

    -- Validation results
    source_content_hash VARCHAR(64),
    dest_content_hash VARCHAR(64),
    content_match BOOLEAN,

    -- Timing
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER,

    -- Error handling
    error_message TEXT,
    error_details JSONB,
    is_retryable BOOLEAN DEFAULT true,

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX idx_migration_items_job ON migration_items(migration_job_id);
CREATE INDEX idx_migration_items_status ON migration_items(status);
CREATE INDEX idx_migration_items_uuid ON migration_items(document_uuid);
CREATE INDEX idx_migration_items_job_status ON migration_items(migration_job_id, status);
CREATE INDEX idx_migration_items_pending ON migration_items(migration_job_id) WHERE status = 'pending';

-- Migration Outbox
-- Transactional outbox pattern for reliable event publishing to Kafka/Redpanda
-- Ensures atomicity between database writes and event publishing
CREATE TABLE IF NOT EXISTS migration_outbox (
    id BIGSERIAL PRIMARY KEY,

    -- Links to migration job and item
    migration_job_id BIGINT NOT NULL REFERENCES migration_jobs(id) ON DELETE CASCADE,
    migration_item_id BIGINT NOT NULL REFERENCES migration_items(id) ON DELETE CASCADE,

    -- Document identification (for Kafka partitioning by document UUID)
    document_uuid UUID NOT NULL,
    document_id VARCHAR(500) NOT NULL,

    -- Idempotency key (prevents duplicate processing)
    idempotent_key VARCHAR(128) NOT NULL UNIQUE,  -- Format: {job_id}:{document_uuid}

    -- Event metadata
    event_type VARCHAR(50) NOT NULL,  -- 'migration.task.created', 'migration.task.retry'
    provider_source VARCHAR(100) NOT NULL,
    provider_dest VARCHAR(100) NOT NULL,

    -- Payload (all data needed for migration task execution)
    payload JSONB NOT NULL,  -- {job_config, item_details, transform_rules}

    -- Outbox state (RFC-080 pattern)
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'published', 'failed'
    published_at TIMESTAMP WITH TIME ZONE,
    publish_attempts INTEGER DEFAULT 0,
    last_error TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_migration_outbox_status ON migration_outbox(status, created_at);
CREATE INDEX idx_migration_outbox_job ON migration_outbox(migration_job_id);
CREATE INDEX idx_migration_outbox_document ON migration_outbox(document_uuid);
CREATE INDEX idx_migration_outbox_idempotent ON migration_outbox(idempotent_key);
CREATE INDEX idx_migration_outbox_pending ON migration_outbox(created_at) WHERE status = 'pending';

-- Add comment to tables for documentation
COMMENT ON TABLE provider_storage IS 'RFC-089: Registry of all storage provider configurations (Google, S3, Local, Azure, Office365)';
COMMENT ON TABLE migration_jobs IS 'RFC-089: Migration job tracking with scheduling and progress monitoring';
COMMENT ON TABLE migration_items IS 'RFC-089: Per-document migration status within a job';
COMMENT ON TABLE migration_outbox IS 'RFC-089: Transactional outbox for reliable migration task publishing to Kafka';

-- Add trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_modified_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_provider_storage_modtime
    BEFORE UPDATE ON provider_storage
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();

CREATE TRIGGER update_migration_jobs_modtime
    BEFORE UPDATE ON migration_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();

CREATE TRIGGER update_migration_items_modtime
    BEFORE UPDATE ON migration_items
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();

CREATE TRIGGER update_migration_outbox_modtime
    BEFORE UPDATE ON migration_outbox
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();
