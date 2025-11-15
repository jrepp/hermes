-- RFC-088: AI-Enhanced Document Indexing
--
-- This migration adds tables for storing AI-generated summaries and embeddings
-- to enable semantic search and intelligent document analysis.
--
-- Tables:
--   - document_summaries: AI summaries (executive summary, key points, topics, tags)
--   - document_embeddings: Vector embeddings for semantic search

-- Document Summaries: Store AI-generated summaries with metadata
--
-- Use case: Generate summaries once, cache for reuse. Idempotent by content_hash.
CREATE TABLE IF NOT EXISTS document_summaries (
    -- Primary key
    id BIGSERIAL PRIMARY KEY,

    -- Document identification
    document_id VARCHAR(500) NOT NULL,
    document_uuid UUID,

    -- Summary content
    executive_summary TEXT NOT NULL,
    key_points JSONB,        -- ["point 1", "point 2", ...]
    topics JSONB,             -- ["topic1", "topic2", ...]
    tags JSONB,               -- ["tag1", "tag2", ...]

    -- AI analysis
    suggested_status VARCHAR(50),
    confidence DOUBLE PRECISION,

    -- Metadata (for cost tracking and debugging)
    model VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    tokens_used INTEGER,
    generation_time_ms INTEGER,

    -- Document context at generation time
    document_title VARCHAR(500),
    document_type VARCHAR(50),
    content_hash VARCHAR(64),
    content_length INTEGER,

    -- Timestamps
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for document_summaries
CREATE INDEX IF NOT EXISTS idx_doc_summaries_doc_id
    ON document_summaries(document_id);

CREATE INDEX IF NOT EXISTS idx_doc_summaries_uuid
    ON document_summaries(document_uuid)
    WHERE document_uuid IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_doc_summaries_model
    ON document_summaries(model);

CREATE INDEX IF NOT EXISTS idx_doc_summaries_doc_type
    ON document_summaries(document_type)
    WHERE document_type IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_doc_summaries_content_hash
    ON document_summaries(content_hash)
    WHERE content_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_doc_summaries_generated
    ON document_summaries(generated_at DESC);

-- Document Embeddings: Store vector embeddings for semantic search
--
-- Use case: Generate embeddings for documents, store in vector DB or PostgreSQL
-- with pgvector extension for similarity search.
CREATE TABLE IF NOT EXISTS document_embeddings (
    -- Primary key
    id BIGSERIAL PRIMARY KEY,

    -- Document identification
    document_id VARCHAR(500) NOT NULL,
    document_uuid UUID,
    revision_id INTEGER,  -- Optional reference to specific revision

    -- Embedding vector (stored as JSONB for compatibility, can migrate to pgvector later)
    -- For pgvector: embedding vector(1536) NOT NULL
    embedding JSONB NOT NULL,
    dimensions INTEGER NOT NULL,  -- e.g., 1536, 3072

    -- Metadata
    model VARCHAR(100) NOT NULL,          -- e.g., "text-embedding-3-small"
    provider VARCHAR(50) NOT NULL,        -- "openai", "bedrock", etc.
    tokens_used INTEGER,
    generation_time_ms INTEGER,

    -- Document context at generation time
    content_hash VARCHAR(64),
    content_length INTEGER,
    chunk_index INTEGER,  -- For document chunking (0 = first chunk, 1 = second, etc.)
    chunk_text TEXT,      -- Optional: store the chunk text for debugging

    -- Timestamps
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for document_embeddings
CREATE INDEX IF NOT EXISTS idx_doc_embeddings_doc_id
    ON document_embeddings(document_id);

CREATE INDEX IF NOT EXISTS idx_doc_embeddings_uuid
    ON document_embeddings(document_uuid)
    WHERE document_uuid IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_doc_embeddings_revision
    ON document_embeddings(revision_id)
    WHERE revision_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_doc_embeddings_content_hash
    ON document_embeddings(content_hash)
    WHERE content_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_doc_embeddings_model
    ON document_embeddings(model);

CREATE INDEX IF NOT EXISTS idx_doc_embeddings_generated
    ON document_embeddings(generated_at DESC);

-- Unique constraint: one embedding per document/model/chunk combination
CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_embeddings_unique
    ON document_embeddings(document_id, model, COALESCE(chunk_index, 0));

-- Comments for documentation
COMMENT ON TABLE document_summaries IS
    'RFC-088: Stores AI-generated document summaries with metadata and cost tracking';

COMMENT ON COLUMN document_summaries.content_hash IS
    'SHA-256 hash of document content at generation time (for idempotency)';

COMMENT ON COLUMN document_summaries.tokens_used IS
    'Number of tokens consumed by the LLM (for cost tracking)';

COMMENT ON TABLE document_embeddings IS
    'RFC-088: Stores vector embeddings for semantic search';

COMMENT ON COLUMN document_embeddings.embedding IS
    'Vector embedding stored as JSONB array (can migrate to pgvector later)';

COMMENT ON COLUMN document_embeddings.chunk_index IS
    'For large documents split into chunks, 0-indexed position';
