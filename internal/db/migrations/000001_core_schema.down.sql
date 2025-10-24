-- Rollback core schema migration
-- Drop tables in reverse order of creation to respect foreign key constraints

DROP TABLE IF EXISTS recently_viewed_projects;
DROP TABLE IF EXISTS recently_viewed_docs;
DROP TABLE IF EXISTS document_contributors;
DROP TABLE IF EXISTS document_group_reviews;
DROP TABLE IF EXISTS document_reviews;

DROP TABLE IF EXISTS product_latest_document_numbers;
DROP TABLE IF EXISTS indexer_metadata;
DROP TABLE IF EXISTS indexer_folders;

DROP TABLE IF EXISTS project_related_resource_hermes_documents;
DROP TABLE IF EXISTS project_related_resource_external_links;
DROP TABLE IF EXISTS project_related_resources;
DROP TABLE IF EXISTS projects;

DROP TABLE IF EXISTS document_related_resource_hermes_documents;
DROP TABLE IF EXISTS document_related_resource_external_links;
DROP TABLE IF EXISTS document_related_resources;

DROP TABLE IF EXISTS document_revisions;
DROP TABLE IF EXISTS document_file_revisions;
DROP TABLE IF EXISTS document_custom_fields;
DROP TABLE IF EXISTS documents;

DROP TABLE IF EXISTS workspace_projects;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS document_type_custom_fields;
DROP TABLE IF EXISTS document_types;
DROP TABLE IF EXISTS hermes_instances;
