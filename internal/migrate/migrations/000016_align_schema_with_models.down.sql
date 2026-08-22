-- Rollback: drop the columns added to align the schema with the models.
DROP INDEX IF EXISTS idx_projects_creator_id;
DROP INDEX IF EXISTS idx_document_revisions_project_id;
DROP INDEX IF EXISTS idx_document_revisions_document_id;

ALTER TABLE projects
  DROP COLUMN IF EXISTS creator_id,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS project_created_at,
  DROP COLUMN IF EXISTS project_modified_at;

ALTER TABLE product_latest_document_numbers
  DROP COLUMN IF EXISTS document_type_id,
  DROP COLUMN IF EXISTS latest_document_number;

ALTER TABLE documents
  DROP COLUMN IF EXISTS archived;

ALTER TABLE document_revisions
  DROP COLUMN IF EXISTS document_id,
  DROP COLUMN IF EXISTS migrated_at,
  DROP COLUMN IF EXISTS migrated_from,
  DROP COLUMN IF EXISTS modified_time,
  DROP COLUMN IF EXISTS project_id,
  DROP COLUMN IF EXISTS provider_folder_id,
  DROP COLUMN IF EXISTS title;

ALTER TABLE project_related_resources
  DROP COLUMN IF EXISTS related_resource_id,
  DROP COLUMN IF EXISTS related_resource_type;

ALTER TABLE document_related_resources
  DROP COLUMN IF EXISTS related_resource_id,
  DROP COLUMN IF EXISTS related_resource_type;
