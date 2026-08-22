-- Add the columns the GORM models declare but no migration ever created.
--
-- The two schemas are maintained by hand in different places, and nothing
-- checked that they agreed. The drift is invisible to the test suite because
-- the API tests build their tables with AutoMigrate, which derives columns
-- from the models and therefore cannot disagree with them.
--
-- It shows up in two ways at runtime, neither obvious:
--
--   * A query where GORM enumerates columns -- anything with a join or a
--     preload -- fails outright with "column does not exist". That is what
--     made /api/v2/drafts return 500.
--   * A `SELECT *` query succeeds and silently leaves the missing fields at
--     their zero values, so a Project always read back with no description and
--     no creator. Writes fail, because INSERT names its columns.
--
-- Types match what AutoMigrate produces for the same models. Every column is
-- added nullable: several are tagged `not null` in the model, but that is an
-- AutoMigrate directive, and existing rows have no value to put there. Adding
-- them NOT NULL would fail on any non-empty table.

-- DocumentRelatedResource: polymorphic association.
ALTER TABLE document_related_resources
  ADD COLUMN IF NOT EXISTS related_resource_id BIGINT,
  ADD COLUMN IF NOT EXISTS related_resource_type TEXT;

-- ProjectRelatedResource: the same association on the project side.
ALTER TABLE project_related_resources
  ADD COLUMN IF NOT EXISTS related_resource_id BIGINT,
  ADD COLUMN IF NOT EXISTS related_resource_type TEXT;

-- DocumentRevision (RFC-017).
ALTER TABLE document_revisions
  ADD COLUMN IF NOT EXISTS document_id VARCHAR(255),
  ADD COLUMN IF NOT EXISTS migrated_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS migrated_from BIGINT,
  ADD COLUMN IF NOT EXISTS modified_time TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS project_id BIGINT,
  ADD COLUMN IF NOT EXISTS provider_folder_id VARCHAR(255),
  ADD COLUMN IF NOT EXISTS title VARCHAR(255);

-- Document.
ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT FALSE;

-- ProductLatestDocumentNumber.
ALTER TABLE product_latest_document_numbers
  ADD COLUMN IF NOT EXISTS document_type_id BIGINT,
  ADD COLUMN IF NOT EXISTS latest_document_number BIGINT;

-- Project.
ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS creator_id BIGINT,
  ADD COLUMN IF NOT EXISTS description TEXT,
  ADD COLUMN IF NOT EXISTS project_created_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS project_modified_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_document_revisions_document_id
  ON document_revisions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_revisions_project_id
  ON document_revisions(project_id);
CREATE INDEX IF NOT EXISTS idx_projects_creator_id
  ON projects(creator_id);
