-- Rollback PostgreSQL-specific enhancements

-- Revert UUID columns to TEXT
ALTER TABLE document_revisions ALTER COLUMN project_uuid TYPE TEXT;
ALTER TABLE document_revisions ALTER COLUMN document_uuid TYPE TEXT;
ALTER TABLE documents ALTER COLUMN project_uuid TYPE TEXT;
ALTER TABLE documents ALTER COLUMN document_uuid TYPE TEXT;
ALTER TABLE workspace_projects ALTER COLUMN project_uuid TYPE TEXT;
ALTER TABLE hermes_instances ALTER COLUMN instance_uuid TYPE TEXT;

-- Revert CITEXT columns to TEXT
ALTER TABLE groups ALTER COLUMN email_address TYPE TEXT;
ALTER TABLE users ALTER COLUMN email_address TYPE TEXT;

-- Extensions will remain (safe to keep)
-- DROP EXTENSION IF EXISTS citext;
-- DROP EXTENSION IF EXISTS "uuid-ossp";
