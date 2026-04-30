-- RFC-008: Search outbox events for transactional database-to-search convergence.
--
-- This table is separate from document_revision_outbox. Search projection events
-- are owned by the central server and are drained through search.Provider.

CREATE TABLE IF NOT EXISTS search_outbox_sequences (
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(255) NOT NULL,
    next_sequence BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (aggregate_type, aggregate_id),
    CONSTRAINT chk_search_outbox_sequences_next_positive
        CHECK (next_sequence > 0)
);

CREATE TABLE IF NOT EXISTS search_outbox_events (
    id BIGSERIAL PRIMARY KEY,

    event_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    index_name VARCHAR(50) NOT NULL,
    operation VARCHAR(20) NOT NULL,
    sequence BIGINT NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    payload JSONB,

    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    locked_by VARCHAR(100),
    last_attempt_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_message TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_search_outbox_operation
        CHECK (operation IN ('upsert', 'delete')),
    CONSTRAINT chk_search_outbox_status
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dlq', 'skipped')),
    CONSTRAINT chk_search_outbox_sequence_positive
        CHECK (sequence > 0),
    CONSTRAINT chk_search_outbox_attempt_count_nonnegative
        CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_search_outbox_status_created
    ON search_outbox_events(status, created_at);

CREATE INDEX IF NOT EXISTS idx_search_outbox_event_type
    ON search_outbox_events(event_type);

CREATE INDEX IF NOT EXISTS idx_search_outbox_aggregate_id
    ON search_outbox_events(aggregate_id);

CREATE INDEX IF NOT EXISTS idx_search_outbox_available
    ON search_outbox_events(status, available_at, created_at);

CREATE INDEX IF NOT EXISTS idx_search_outbox_ordering
    ON search_outbox_events(aggregate_type, aggregate_id, sequence);

CREATE UNIQUE INDEX IF NOT EXISTS idx_search_outbox_aggregate_sequence
    ON search_outbox_events(aggregate_type, aggregate_id, sequence);
