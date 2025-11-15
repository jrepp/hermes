-- RFC-089: Rollback migration for S3-Compatible Storage Backend and Document Migration System

-- Drop triggers
DROP TRIGGER IF EXISTS update_migration_outbox_modtime ON migration_outbox;
DROP TRIGGER IF EXISTS update_migration_items_modtime ON migration_items;
DROP TRIGGER IF EXISTS update_migration_jobs_modtime ON migration_jobs;
DROP TRIGGER IF EXISTS update_provider_storage_modtime ON provider_storage;

-- Drop tables in reverse order (respecting foreign key dependencies)
DROP TABLE IF EXISTS migration_outbox;
DROP TABLE IF EXISTS migration_items;
DROP TABLE IF EXISTS migration_jobs;
DROP TABLE IF EXISTS provider_storage;

-- Note: update_modified_column() function is kept as it may be used by other tables
-- If you need to remove it completely, add: DROP FUNCTION IF EXISTS update_modified_column();
