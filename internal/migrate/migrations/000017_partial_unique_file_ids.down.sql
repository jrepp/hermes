-- Rollback: restore the plain unique constraint on google_file_id.
--
-- This fails if the table holds more than one document with an empty
-- google_file_id, which is the state the up migration exists to allow.
DROP INDEX IF EXISTS idx_documents_file_id_unique;
DROP INDEX IF EXISTS idx_documents_google_file_id_unique;

ALTER TABLE documents
  ADD CONSTRAINT documents_google_file_id_key UNIQUE (google_file_id);
