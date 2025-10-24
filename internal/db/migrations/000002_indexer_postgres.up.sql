-- PostgreSQL-specific indexer enhancements
-- Requires: 000002_indexer_core.up.sql

-- Convert TEXT UUID columns to proper UUID type
ALTER TABLE indexers ALTER COLUMN id TYPE UUID USING id::uuid;
ALTER TABLE indexer_tokens ALTER COLUMN id TYPE UUID USING id::uuid;
ALTER TABLE indexer_tokens ALTER COLUMN indexer_id TYPE UUID USING indexer_id::uuid;

-- Convert INTEGER boolean to BOOLEAN type
ALTER TABLE indexer_tokens ALTER COLUMN revoked TYPE BOOLEAN USING revoked::boolean;
