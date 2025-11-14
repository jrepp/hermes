-- RFC-085: Edge-to-Central Document Tracking
--
-- This migration adds tables to track documents created on edge Hermes instances
-- and synchronized to central Hermes for global tracking and search.
--
-- Use case: Edge developer creates RFC-123 locally, metadata syncs to central
-- so colleagues can discover and track the document across the organization.

-- Edge Document Registry: Tracks documents from all edge instances
CREATE TABLE IF NOT EXISTS edge_document_registry (
    -- Document identification
    uuid UUID PRIMARY KEY,

    -- Document metadata (synced from edge)
    title TEXT NOT NULL,
    document_type TEXT NOT NULL,
    status TEXT,
    summary TEXT,

    -- Ownership
    owners TEXT[] NOT NULL DEFAULT '{}',
    contributors TEXT[] DEFAULT '{}',

    -- Edge instance tracking
    edge_instance TEXT NOT NULL,
    edge_provider_id TEXT, -- Backend-specific ID on edge (e.g., "local:path/to/doc")

    -- Organization
    product TEXT,
    tags TEXT[] DEFAULT '{}',
    parent_folders TEXT[] DEFAULT '{}',

    -- Extended metadata (document-type-specific fields)
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Sync tracking
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_sync_status TEXT DEFAULT 'synced', -- 'synced', 'pending', 'failed'
    sync_error TEXT,

    -- Content tracking
    content_hash TEXT -- SHA-256 for drift detection
);

-- Index for searching by edge instance
CREATE INDEX IF NOT EXISTS idx_edge_document_registry_edge_instance
    ON edge_document_registry(edge_instance);

-- Index for searching by document type
CREATE INDEX IF NOT EXISTS idx_edge_document_registry_document_type
    ON edge_document_registry(document_type);

-- Index for searching by owners (GIN for array search)
CREATE INDEX IF NOT EXISTS idx_edge_document_registry_owners
    ON edge_document_registry USING GIN(owners);

-- Index for searching by product
CREATE INDEX IF NOT EXISTS idx_edge_document_registry_product
    ON edge_document_registry(product)
    WHERE product IS NOT NULL;

-- Index for searching by status
CREATE INDEX IF NOT EXISTS idx_edge_document_registry_status
    ON edge_document_registry(status)
    WHERE status IS NOT NULL;

-- Index for sync status queries
CREATE INDEX IF NOT EXISTS idx_edge_document_registry_sync_status
    ON edge_document_registry(last_sync_status, synced_at);

-- Index for metadata JSONB searches
CREATE INDEX IF NOT EXISTS idx_edge_document_registry_metadata
    ON edge_document_registry USING GIN(metadata);

-- Document UUID Mappings: Track UUID merging for drift resolution
--
-- Use case: Same document created independently on multiple edge instances
-- (e.g., offline conflict) needs to be merged into single canonical UUID.
CREATE TABLE IF NOT EXISTS document_uuid_mappings (
    -- Mapping identification
    id SERIAL PRIMARY KEY,

    -- UUID mapping (edge UUID → central canonical UUID)
    edge_uuid UUID NOT NULL,
    central_uuid UUID, -- NULL if not yet merged
    edge_instance TEXT NOT NULL,

    -- Merge tracking
    merged_at TIMESTAMPTZ,
    merged_by TEXT, -- User email who performed the merge
    merge_strategy TEXT, -- 'keep-target', 'keep-source', 'merge-all'

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Unique constraint: one mapping per edge UUID per instance
    CONSTRAINT document_uuid_mappings_unique
        UNIQUE (edge_uuid, edge_instance)
);

-- Index for looking up central UUID by edge UUID
CREATE INDEX IF NOT EXISTS idx_document_uuid_mappings_edge_uuid
    ON document_uuid_mappings(edge_uuid);

-- Index for finding all mappings to a central UUID (for merge tracking)
CREATE INDEX IF NOT EXISTS idx_document_uuid_mappings_central_uuid
    ON document_uuid_mappings(central_uuid)
    WHERE central_uuid IS NOT NULL;

-- Index for edge instance lookups
CREATE INDEX IF NOT EXISTS idx_document_uuid_mappings_edge_instance
    ON document_uuid_mappings(edge_instance);

-- Edge Sync Queue: Tracks pending sync operations (for batch sync mode)
--
-- Use case: Edge creates 10 documents while offline, queues sync operations,
-- processes them in batch when connection is restored.
CREATE TABLE IF NOT EXISTS edge_sync_queue (
    -- Queue identification
    id SERIAL PRIMARY KEY,

    -- Sync operation details
    uuid UUID NOT NULL,
    operation_type TEXT NOT NULL, -- 'register', 'update', 'delete'
    edge_instance TEXT NOT NULL,

    -- Operation payload (JSONB for flexibility)
    payload JSONB NOT NULL,

    -- Retry tracking
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    last_attempt_at TIMESTAMPTZ,
    last_error TEXT,

    -- Status
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    -- Priority (higher = process first)
    priority INT NOT NULL DEFAULT 0
);

-- Index for processing queue (ordered by priority, then created_at)
CREATE INDEX IF NOT EXISTS idx_edge_sync_queue_processing
    ON edge_sync_queue(status, priority DESC, created_at)
    WHERE status IN ('pending', 'processing');

-- Index for finding operations by UUID
CREATE INDEX IF NOT EXISTS idx_edge_sync_queue_uuid
    ON edge_sync_queue(uuid);

-- Index for finding operations by edge instance
CREATE INDEX IF NOT EXISTS idx_edge_sync_queue_edge_instance
    ON edge_sync_queue(edge_instance);

-- Index for cleanup of old completed operations
CREATE INDEX IF NOT EXISTS idx_edge_sync_queue_cleanup
    ON edge_sync_queue(status, completed_at)
    WHERE status IN ('completed', 'failed');

-- Comments for documentation
COMMENT ON TABLE edge_document_registry IS
    'RFC-085: Tracks documents from edge Hermes instances for global discovery';

COMMENT ON COLUMN edge_document_registry.uuid IS
    'Global document UUID (RFC-082 DocID)';

COMMENT ON COLUMN edge_document_registry.edge_instance IS
    'Edge instance identifier (e.g., "edge-dev-1", "edge-chicago-office")';

COMMENT ON COLUMN edge_document_registry.content_hash IS
    'SHA-256 hash for drift detection between edge and central';

COMMENT ON TABLE document_uuid_mappings IS
    'RFC-085: Tracks UUID merging when same document created on multiple edges';

COMMENT ON TABLE edge_sync_queue IS
    'RFC-085: Queue for batch synchronization of edge documents to central';
