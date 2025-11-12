-- Migration: Convert integer boolean fields to proper BOOLEAN type
-- This fixes type mismatch between Go bool and PostgreSQL INTEGER columns

-- Convert imported field from INTEGER to BOOLEAN
ALTER TABLE documents ALTER COLUMN imported DROP DEFAULT;
ALTER TABLE documents ALTER COLUMN imported TYPE BOOLEAN USING (imported != 0);
ALTER TABLE documents ALTER COLUMN imported SET DEFAULT false;

-- Convert locked field from INTEGER to BOOLEAN
ALTER TABLE documents ALTER COLUMN locked DROP DEFAULT;
ALTER TABLE documents ALTER COLUMN locked TYPE BOOLEAN USING (locked != 0);
ALTER TABLE documents ALTER COLUMN locked SET DEFAULT false;

-- Convert shareable_as_draft field from INTEGER to BOOLEAN
ALTER TABLE documents ALTER COLUMN shareable_as_draft DROP DEFAULT;
ALTER TABLE documents ALTER COLUMN shareable_as_draft TYPE BOOLEAN USING (shareable_as_draft != 0);
ALTER TABLE documents ALTER COLUMN shareable_as_draft SET DEFAULT false;
