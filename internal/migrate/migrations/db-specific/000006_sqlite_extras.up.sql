-- SQLite-specific schema enhancements
-- Applied AFTER core schema migration
-- Requires: 000001_core_schema.up.sql

-- SQLite doesn't need extensions or type conversions
-- The core schema is already SQLite-compatible

-- Enable foreign key constraints (disabled by default in SQLite)
PRAGMA foreign_keys = ON;

-- Enable WAL mode for better concurrency
PRAGMA journal_mode = WAL;

-- Optimize for local development performance
PRAGMA synchronous = NORMAL;
PRAGMA temp_store = MEMORY;
PRAGMA mmap_size = 30000000000;
