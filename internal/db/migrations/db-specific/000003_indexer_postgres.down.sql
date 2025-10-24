-- Rollback PostgreSQL-specific indexer enhancements

-- Revert BOOLEAN to INTEGER
ALTER TABLE indexer_tokens ALTER COLUMN revoked TYPE INTEGER USING revoked::integer;

-- Revert UUID columns to TEXT
ALTER TABLE indexer_tokens ALTER COLUMN indexer_id TYPE TEXT;
ALTER TABLE indexer_tokens ALTER COLUMN id TYPE TEXT;
ALTER TABLE indexers ALTER COLUMN id TYPE TEXT;
