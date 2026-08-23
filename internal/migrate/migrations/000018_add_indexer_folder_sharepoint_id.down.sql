-- Rollback: drop indexer_folders.share_point_folder_id.
DROP INDEX IF EXISTS idx_indexer_folders_share_point_folder_id;

ALTER TABLE indexer_folders
  DROP COLUMN IF EXISTS share_point_folder_id;
