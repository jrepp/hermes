-- PostgreSQL-specific indexer enhancements
-- Requires: 000002_indexer_core.up.sql

-- Drop foreign key constraint before type conversion
ALTER TABLE indexer_tokens DROP CONSTRAINT IF EXISTS indexer_tokens_indexer_id_fkey;

-- Convert TEXT UUID columns to proper UUID type
ALTER TABLE indexers ALTER COLUMN id TYPE UUID USING id::uuid;
ALTER TABLE indexer_tokens ALTER COLUMN id TYPE UUID USING id::uuid;
ALTER TABLE indexer_tokens ALTER COLUMN indexer_id TYPE UUID USING indexer_id::uuid;

-- Re-add foreign key constraint with UUID types
ALTER TABLE indexer_tokens 
    ADD CONSTRAINT indexer_tokens_indexer_id_fkey 
    FOREIGN KEY (indexer_id) REFERENCES indexers(id) ON DELETE CASCADE;

-- Convert INTEGER boolean to BOOLEAN type
ALTER TABLE indexer_tokens ALTER COLUMN revoked DROP DEFAULT;
ALTER TABLE indexer_tokens ALTER COLUMN revoked TYPE BOOLEAN USING revoked::boolean;
ALTER TABLE indexer_tokens ALTER COLUMN revoked SET DEFAULT false;
