-- Rollback: Remove fields added to document_types

ALTER TABLE document_types 
  DROP COLUMN IF EXISTS flight_icon,
  DROP COLUMN IF EXISTS more_info_link_text,
  DROP COLUMN IF EXISTS more_info_link_url,
  DROP COLUMN IF EXISTS checks;
