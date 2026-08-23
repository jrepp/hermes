-- Add indexer_folders.share_point_folder_id.
--
-- IndexerFolder declares the field and IndexerFolder.Get queries it, but no
-- migration ever created the column. GORM names every column in its SELECT
-- list, so on a migrated database *every* query against indexer_folders failed
-- with "column share_point_folder_id does not exist" -- not only the SharePoint
-- ones.
--
-- It stayed hidden because the model was commented out of ToAutoMigrate with a
-- note about a GORM constraint-rename bug, so the table was absent from test
-- schemas entirely and its tests failed for that reason instead. The bug no
-- longer reproduces; the model is back in the list, and this is the column it
-- always needed.
--
-- Nullable and uniquely indexed only where present: a Google-backed folder has
-- no SharePoint ID, and several of those must coexist.
ALTER TABLE indexer_folders
  ADD COLUMN IF NOT EXISTS share_point_folder_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_indexer_folders_share_point_folder_id
  ON indexer_folders (share_point_folder_id)
  WHERE share_point_folder_id IS NOT NULL;
