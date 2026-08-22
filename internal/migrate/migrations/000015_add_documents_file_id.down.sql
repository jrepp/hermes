-- Rollback: drop documents.file_id.
DROP INDEX IF EXISTS idx_documents_file_id;

ALTER TABLE documents
  DROP COLUMN IF EXISTS file_id;
