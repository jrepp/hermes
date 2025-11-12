-- Rollback: Convert BOOLEAN fields back to INTEGER
-- This reverses the migration from INTEGER to BOOLEAN

-- Convert imported field from BOOLEAN back to INTEGER
ALTER TABLE documents
ALTER COLUMN imported TYPE INTEGER USING (CASE WHEN imported THEN 1 ELSE 0 END);

-- Convert locked field from BOOLEAN back to INTEGER
ALTER TABLE documents
ALTER COLUMN locked TYPE INTEGER USING (CASE WHEN locked THEN 1 ELSE 0 END);

-- Convert shareable_as_draft field from BOOLEAN back to INTEGER
ALTER TABLE documents
ALTER COLUMN shareable_as_draft TYPE INTEGER USING (CASE WHEN shareable_as_draft THEN 1 ELSE 0 END);
