-- PostgreSQL-specific indexer enhancements
-- Requires: 000002_indexer_core.up.sql

-- Drop foreign key constraint before type conversion (check both old and new table names)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'indexer_tokens') THEN
        ALTER TABLE indexer_tokens DROP CONSTRAINT IF EXISTS indexer_tokens_indexer_id_fkey;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'service_tokens') THEN
        ALTER TABLE service_tokens DROP CONSTRAINT IF EXISTS indexer_tokens_indexer_id_fkey;
        ALTER TABLE service_tokens DROP CONSTRAINT IF EXISTS service_tokens_indexer_id_fkey;
    END IF;
END $$;

-- Convert TEXT UUID columns to proper UUID type
ALTER TABLE indexers ALTER COLUMN id TYPE UUID USING id::uuid;

-- Support both indexer_tokens (old name) and service_tokens (new name after migration 000009)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'indexer_tokens') THEN
        ALTER TABLE indexer_tokens ALTER COLUMN id TYPE UUID USING id::uuid;
        ALTER TABLE indexer_tokens ALTER COLUMN indexer_id TYPE UUID USING indexer_id::uuid;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'service_tokens') THEN
        ALTER TABLE service_tokens ALTER COLUMN id TYPE UUID USING id::uuid;
        ALTER TABLE service_tokens ALTER COLUMN indexer_id TYPE UUID USING indexer_id::uuid;
    END IF;
END $$;

-- Re-add foreign key constraint with UUID types
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'indexer_tokens') THEN
        ALTER TABLE indexer_tokens
            ADD CONSTRAINT indexer_tokens_indexer_id_fkey
            FOREIGN KEY (indexer_id) REFERENCES indexers(id) ON DELETE CASCADE;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'service_tokens') THEN
        ALTER TABLE service_tokens
            ADD CONSTRAINT service_tokens_indexer_id_fkey
            FOREIGN KEY (indexer_id) REFERENCES indexers(id) ON DELETE CASCADE;
    END IF;
END $$;

-- Convert INTEGER boolean to BOOLEAN type
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'indexer_tokens') THEN
        ALTER TABLE indexer_tokens ALTER COLUMN revoked DROP DEFAULT;
        ALTER TABLE indexer_tokens ALTER COLUMN revoked TYPE BOOLEAN USING revoked::boolean;
        ALTER TABLE indexer_tokens ALTER COLUMN revoked SET DEFAULT false;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'service_tokens') THEN
        ALTER TABLE service_tokens ALTER COLUMN revoked DROP DEFAULT;
        ALTER TABLE service_tokens ALTER COLUMN revoked TYPE BOOLEAN USING revoked::boolean;
        ALTER TABLE service_tokens ALTER COLUMN revoked SET DEFAULT false;
    END IF;
END $$;
