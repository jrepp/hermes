-- RFC-085 Phase 3: Rename indexer_tokens to service_tokens
--
-- This migration renames the indexer_tokens table to service_tokens to reflect
-- its broader use for various service authentication tokens (indexer, edge sync, etc.)
--
-- Token types supported:
--   - 'registration' - Indexer registration tokens
--   - 'api' - General API tokens
--   - 'edge' - Edge-to-central sync tokens
--
-- Overlapping token rotation strategy:
--   Multiple active tokens per service allowed for zero-downtime rotation.

-- Rename the table
ALTER TABLE indexer_tokens RENAME TO service_tokens;

-- Rename indexes
ALTER INDEX indexer_tokens_pkey RENAME TO service_tokens_pkey;
ALTER INDEX indexer_tokens_token_hash_key RENAME TO service_tokens_token_hash_key;

-- Rename existing indexes if they exist
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_indexer_tokens_hash') THEN
        ALTER INDEX idx_indexer_tokens_hash RENAME TO idx_service_tokens_hash;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_indexer_tokens_type') THEN
        ALTER INDEX idx_indexer_tokens_type RENAME TO idx_service_tokens_type;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_indexer_tokens_expires') THEN
        ALTER INDEX idx_service_tokens_expires RENAME TO idx_service_tokens_expires;
    END IF;
END $$;

-- Create missing indexes if they don't exist
CREATE INDEX IF NOT EXISTS idx_service_tokens_type ON service_tokens(token_type);
CREATE INDEX IF NOT EXISTS idx_service_tokens_expires ON service_tokens(expires_at) WHERE expires_at IS NOT NULL;

-- Add comment explaining the table's purpose
COMMENT ON TABLE service_tokens IS 'Service authentication tokens for indexers, edge instances, and API access. Supports multiple active tokens per service for overlapping rotation strategy.';

-- Add helpful comments on key columns
COMMENT ON COLUMN service_tokens.token_type IS 'Token type: registration, api, or edge';
COMMENT ON COLUMN service_tokens.expires_at IS 'When token expires. NULL = never expires. Used for overlapping rotation strategy.';
COMMENT ON COLUMN service_tokens.revoked IS 'Whether token has been revoked. Used to invalidate tokens during rotation.';
COMMENT ON COLUMN service_tokens.indexer_id IS 'Optional foreign key to indexers table for indexer-specific tokens';
