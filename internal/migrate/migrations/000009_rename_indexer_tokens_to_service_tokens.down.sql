-- RFC-085 Phase 3: Rollback rename of service_tokens to indexer_tokens

-- Rename the table back
ALTER TABLE service_tokens RENAME TO indexer_tokens;

-- Rename indexes back
ALTER INDEX service_tokens_pkey RENAME TO indexer_tokens_pkey;
ALTER INDEX service_tokens_token_hash_key RENAME TO indexer_tokens_token_hash_key;

-- Rename custom indexes back if they exist
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_service_tokens_hash') THEN
        ALTER INDEX idx_service_tokens_hash RENAME TO idx_indexer_tokens_hash;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_service_tokens_type') THEN
        ALTER INDEX idx_service_tokens_type RENAME TO idx_indexer_tokens_type;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_service_tokens_expires') THEN
        ALTER INDEX idx_service_tokens_expires RENAME TO idx_indexer_tokens_expires;
    END IF;
END $$;
