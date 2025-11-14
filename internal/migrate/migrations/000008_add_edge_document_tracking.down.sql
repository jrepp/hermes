-- Rollback RFC-085: Edge-to-Central Document Tracking

-- Drop sync queue table
DROP INDEX IF EXISTS idx_edge_sync_queue_cleanup;
DROP INDEX IF EXISTS idx_edge_sync_queue_edge_instance;
DROP INDEX IF EXISTS idx_edge_sync_queue_uuid;
DROP INDEX IF EXISTS idx_edge_sync_queue_processing;
DROP TABLE IF EXISTS edge_sync_queue;

-- Drop UUID mappings table
DROP INDEX IF EXISTS idx_document_uuid_mappings_edge_instance;
DROP INDEX IF EXISTS idx_document_uuid_mappings_central_uuid;
DROP INDEX IF EXISTS idx_document_uuid_mappings_edge_uuid;
DROP TABLE IF EXISTS document_uuid_mappings;

-- Drop edge document registry table
DROP INDEX IF EXISTS idx_edge_document_registry_metadata;
DROP INDEX IF EXISTS idx_edge_document_registry_sync_status;
DROP INDEX IF EXISTS idx_edge_document_registry_status;
DROP INDEX IF EXISTS idx_edge_document_registry_product;
DROP INDEX IF EXISTS idx_edge_document_registry_owners;
DROP INDEX IF EXISTS idx_edge_document_registry_document_type;
DROP INDEX IF EXISTS idx_edge_document_registry_edge_instance;
DROP TABLE IF EXISTS edge_document_registry;
