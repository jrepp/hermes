-- RFC-088: Event-Driven Document Indexer with Pipeline Rulesets
--
-- This migration adds tables for the outbox pattern to enable reliable,
-- event-driven document indexing with pipeline executions.
--
-- Architecture:
--   API handlers → document_revisions + document_revision_outbox (TRANSACTION)
--   Outbox relay → Redpanda topic
--   Indexer consumers → Pipeline execution based on rulesets
--
-- Key features:
--   - Transactional consistency via outbox pattern
--   - Idempotency via content_hash-based keys
--   - Pipeline execution tracking
--   - Scalable async processing

-- Document Revision Outbox: Transactional event queue for document changes
--
-- Use case: When a document is created/updated, write to outbox atomically
-- with the revision. Relay service publishes to Redpanda for async processing.
CREATE TABLE IF NOT EXISTS document_revision_outbox (
    -- Primary key
    id BIGSERIAL PRIMARY KEY,

    -- Document identification
    revision_id INTEGER NOT NULL REFERENCES document_revisions(id) ON DELETE CASCADE,
    document_uuid UUID NOT NULL,
    document_id VARCHAR(500) NOT NULL,

    -- Idempotency key: {document_uuid}:{content_hash}
    -- Prevents duplicate processing of the same document version
    idempotent_key VARCHAR(128) NOT NULL UNIQUE,
    content_hash VARCHAR(64) NOT NULL,

    -- Event metadata
    event_type VARCHAR(50) NOT NULL,  -- 'revision.created', 'revision.updated', 'revision.deleted'
    provider_type VARCHAR(50) NOT NULL,

    -- Event payload (full revision data + metadata for indexing)
    payload JSONB NOT NULL,

    -- Outbox state tracking
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'published', 'failed'
    published_at TIMESTAMPTZ,
    publish_attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for outbox relay service (finds pending events to publish)
CREATE INDEX IF NOT EXISTS idx_revision_outbox_pending
    ON document_revision_outbox(status, created_at)
    WHERE status = 'pending';

-- Index for finding outbox entries by document UUID
CREATE INDEX IF NOT EXISTS idx_revision_outbox_document_uuid
    ON document_revision_outbox(document_uuid);

-- Index for finding outbox entries by revision
CREATE INDEX IF NOT EXISTS idx_revision_outbox_revision_id
    ON document_revision_outbox(revision_id);

-- Index for cleanup queries (finding old published events)
CREATE INDEX IF NOT EXISTS idx_revision_outbox_cleanup
    ON document_revision_outbox(status, published_at)
    WHERE status = 'published';

-- Index for failed event queries
CREATE INDEX IF NOT EXISTS idx_revision_outbox_failed
    ON document_revision_outbox(status, publish_attempts)
    WHERE status = 'failed';

-- Document Revision Pipeline Executions: Tracks pipeline processing
--
-- Use case: Indexer consumer receives event, matches rulesets, executes pipeline
-- steps (search_index, embeddings, llm_summary). Track execution and results.
CREATE TABLE IF NOT EXISTS document_revision_pipeline_executions (
    -- Primary key
    id BIGSERIAL PRIMARY KEY,

    -- Links to revision and outbox
    revision_id INTEGER NOT NULL REFERENCES document_revisions(id) ON DELETE CASCADE,
    outbox_id BIGINT NOT NULL REFERENCES document_revision_outbox(id) ON DELETE CASCADE,

    -- Execution metadata
    ruleset_name VARCHAR(100) NOT NULL,
    pipeline_steps JSONB NOT NULL,  -- ['search_index', 'embeddings', 'llm_summary']

    -- Execution state
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'running', 'completed', 'failed', 'partial'
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Results per step (tracks success/failure for each pipeline step)
    -- Example: {"search_index": {"status": "success", "duration_ms": 234}, "embeddings": {"status": "failed", "error": "..."}}
    step_results JSONB,

    -- Error details (for debugging failed pipelines)
    error_details JSONB,

    -- Retry tracking
    attempt_number INTEGER NOT NULL DEFAULT 1,
    max_attempts INTEGER NOT NULL DEFAULT 3,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for finding executions by revision
CREATE INDEX IF NOT EXISTS idx_pipeline_exec_revision_id
    ON document_revision_pipeline_executions(revision_id);

-- Index for finding executions by outbox entry
CREATE INDEX IF NOT EXISTS idx_pipeline_exec_outbox_id
    ON document_revision_pipeline_executions(outbox_id);

-- Index for finding executions by status
CREATE INDEX IF NOT EXISTS idx_pipeline_exec_status
    ON document_revision_pipeline_executions(status, created_at);

-- Index for finding failed executions that need retry
CREATE INDEX IF NOT EXISTS idx_pipeline_exec_retry
    ON document_revision_pipeline_executions(status, attempt_number, max_attempts)
    WHERE status = 'failed' AND attempt_number < max_attempts;

-- Index for finding executions by ruleset (for monitoring specific pipelines)
CREATE INDEX IF NOT EXISTS idx_pipeline_exec_ruleset
    ON document_revision_pipeline_executions(ruleset_name, status);

-- Index for performance monitoring (find slow pipelines)
CREATE INDEX IF NOT EXISTS idx_pipeline_exec_duration
    ON document_revision_pipeline_executions(started_at, completed_at)
    WHERE completed_at IS NOT NULL;

-- Comments for documentation
COMMENT ON TABLE document_revision_outbox IS
    'RFC-088: Outbox pattern for reliable document revision event publishing';

COMMENT ON COLUMN document_revision_outbox.idempotent_key IS
    'Unique key: {document_uuid}:{content_hash} - prevents duplicate processing';

COMMENT ON COLUMN document_revision_outbox.payload IS
    'Full revision data + metadata serialized as JSON for indexer consumers';

COMMENT ON TABLE document_revision_pipeline_executions IS
    'RFC-088: Tracks indexer pipeline executions with per-step results';

COMMENT ON COLUMN document_revision_pipeline_executions.step_results IS
    'Per-step execution results: {"step_name": {"status": "success/failed", "duration_ms": N, "error": "..."}}';

COMMENT ON COLUMN document_revision_pipeline_executions.ruleset_name IS
    'Name of the ruleset that matched and triggered this pipeline execution';
