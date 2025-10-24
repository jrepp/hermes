-- Revert column type fixes for document_type_custom_fields

-- Revert type column (INTEGER -> TEXT)
ALTER TABLE document_type_custom_fields
  RENAME COLUMN type TO type_new;

ALTER TABLE document_type_custom_fields
  ADD COLUMN type TEXT;

UPDATE document_type_custom_fields
  SET type = CASE type_new
    WHEN 1 THEN 'string'
    WHEN 2 THEN 'person'
    WHEN 3 THEN 'people'
    ELSE ''
  END;

ALTER TABLE document_type_custom_fields
  DROP COLUMN type_new;

-- Revert read_only column (BOOLEAN -> INTEGER)
ALTER TABLE document_type_custom_fields
  ALTER COLUMN read_only DROP DEFAULT;

ALTER TABLE document_type_custom_fields
  ALTER COLUMN read_only TYPE INTEGER USING (CASE WHEN read_only THEN 1 ELSE 0 END);

ALTER TABLE document_type_custom_fields
  ALTER COLUMN read_only SET DEFAULT 0;
