-- PostgreSQL-specific schema enhancements
-- Applied AFTER core schema migration
-- Requires: 000001_core_schema.up.sql

-- Enable required PostgreSQL extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS citext;

-- Convert TEXT columns to CITEXT for case-insensitive lookups
-- (email addresses should be case-insensitive)
ALTER TABLE users ALTER COLUMN email_address TYPE citext;
ALTER TABLE groups ALTER COLUMN email_address TYPE citext;

-- Convert generic TEXT UUID columns to proper UUID type
ALTER TABLE hermes_instances ALTER COLUMN instance_uuid TYPE UUID USING instance_uuid::uuid;
ALTER TABLE workspace_projects ALTER COLUMN project_uuid TYPE UUID USING project_uuid::uuid;
ALTER TABLE documents ALTER COLUMN document_uuid TYPE UUID USING document_uuid::uuid;
ALTER TABLE documents ALTER COLUMN project_uuid TYPE UUID USING project_uuid::uuid;
ALTER TABLE document_revisions ALTER COLUMN document_uuid TYPE UUID USING document_uuid::uuid;
ALTER TABLE document_revisions ALTER COLUMN project_uuid TYPE UUID USING project_uuid::uuid;

-- Convert AUTOINCREMENT columns to PostgreSQL SERIAL/BIGSERIAL
-- (No action needed - PostgreSQL handles AUTOINCREMENT as SERIAL automatically)

-- Add PostgreSQL-specific constraints/defaults that SQLite doesn't support
-- (None currently needed beyond what's in core schema)
