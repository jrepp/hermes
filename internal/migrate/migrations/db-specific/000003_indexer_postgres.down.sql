-- Rollback PostgreSQL-specific indexer enhancements

-- Revert BOOLEAN to INTEGER (support both table names)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'indexer_tokens') THEN
        ALTER TABLE indexer_tokens ALTER COLUMN revoked TYPE INTEGER USING revoked::integer;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'service_tokens') THEN
        ALTER TABLE service_tokens ALTER COLUMN revoked TYPE INTEGER USING revoked::integer;
    END IF;
END $$;

-- Revert UUID columns to TEXT (support both table names)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'indexer_tokens') THEN
        ALTER TABLE indexer_tokens ALTER COLUMN indexer_id TYPE TEXT;
        ALTER TABLE indexer_tokens ALTER COLUMN id TYPE TEXT;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'service_tokens') THEN
        ALTER TABLE service_tokens ALTER COLUMN indexer_id TYPE TEXT;
        ALTER TABLE service_tokens ALTER COLUMN id TYPE TEXT;
    END IF;
END $$;

ALTER TABLE indexers ALTER COLUMN id TYPE TEXT;
