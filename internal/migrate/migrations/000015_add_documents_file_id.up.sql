-- Add documents.file_id, the provider-agnostic document identifier.
--
-- models.Document declares this column (`gorm:"column:file_id;index"`) but no
-- migration ever created it. GORM names every column in its SELECT list, so on
-- a migrated database *every* query touching documents failed with
-- `column documents.file_id does not exist` -- /api/v2/drafts, document reads,
-- the lot.
--
-- It went unnoticed because the API test suite builds its schema with
-- AutoMigrate, which derives columns from the model and therefore cannot
-- disagree with it. Only a real migration run has the two schemas differ.
--
-- google_file_id stays as it is: it remains the Google Workspace identifier
-- and is still NOT NULL UNIQUE. file_id is the SharePoint-and-later
-- equivalent, nullable because Google-backed rows have none.
ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS file_id TEXT;

CREATE INDEX IF NOT EXISTS idx_documents_file_id ON documents(file_id);
