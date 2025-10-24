-- Fix column type mismatches in document_type_custom_fields table
-- 1. read_only: Model uses bool, but migration created INTEGER
-- 2. type: Model uses int (DocumentTypeCustomFieldType), but migration created TEXT

-- Fix read_only column
ALTER TABLE document_type_custom_fields
  ALTER COLUMN read_only DROP DEFAULT;

ALTER TABLE document_type_custom_fields
  ALTER COLUMN read_only TYPE BOOLEAN USING (read_only != 0);

ALTER TABLE document_type_custom_fields
  ALTER COLUMN read_only SET DEFAULT false;

-- Fix type column (rename existing column to avoid conflicts)
ALTER TABLE document_type_custom_fields
  RENAME COLUMN type TO type_old;

ALTER TABLE document_type_custom_fields
  ADD COLUMN type INTEGER DEFAULT 0;

-- Migrate existing data (if any)
-- type_old values would have been empty or text, so default to 0 (Unspecified)
UPDATE document_type_custom_fields
  SET type = CASE
    WHEN type_old = 'string' THEN 1
    WHEN type_old = 'person' THEN 2
    WHEN type_old = 'people' THEN 3
    ELSE 0
  END;

ALTER TABLE document_type_custom_fields
  DROP COLUMN type_old;
