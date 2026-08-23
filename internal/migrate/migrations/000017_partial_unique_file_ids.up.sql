-- Let more than one document have no Google file ID.
--
-- documents.google_file_id was NOT NULL UNIQUE, from a time when Google Drive
-- was the only provider. A document held by another provider has no Google
-- file ID, and Document.GoogleFileID is a plain string, so such a document is
-- written with google_file_id = ''. A plain unique constraint permits exactly
-- one of those: creating a second SharePoint-backed document failed with a
-- duplicate key error.
--
-- The fix is a partial unique index that ignores empty values. Real Google
-- file IDs stay unique; any number of documents may have none. The same index
-- is added for file_id, which had no uniqueness at all.
--
-- Making the column nullable and writing NULL instead would be tidier, but
-- GoogleFileID would have to become a *string to hold NULL, and it appears in
-- several hundred places.

ALTER TABLE documents
  DROP CONSTRAINT IF EXISTS documents_google_file_id_key;

-- AutoMigrate names the same constraint differently; drop that too so a
-- database built either way ends up in the same state.
ALTER TABLE documents
  DROP CONSTRAINT IF EXISTS uni_documents_google_file_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_google_file_id_unique
  ON documents (google_file_id)
  WHERE google_file_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_file_id_unique
  ON documents (file_id)
  WHERE file_id IS NOT NULL AND file_id <> '';
