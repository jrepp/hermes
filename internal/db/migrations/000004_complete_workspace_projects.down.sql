-- Rollback: Remove fields added to workspace_projects

-- Drop indexes
DROP INDEX IF EXISTS idx_workspace_projects_instance_name;
DROP INDEX IF EXISTS idx_workspace_projects_config_hash;
DROP INDEX IF EXISTS idx_workspace_projects_global_id;
DROP INDEX IF EXISTS idx_workspace_projects_instance;

-- Drop columns (in reverse order of addition)
ALTER TABLE workspace_projects 
  DROP COLUMN IF EXISTS config_version,
  DROP COLUMN IF EXISTS last_synced_at,
  DROP COLUMN IF EXISTS source_identifier,
  DROP COLUMN IF EXISTS source_type,
  DROP COLUMN IF EXISTS metadata_json,
  DROP COLUMN IF EXISTS providers_json,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS short_name,
  DROP COLUMN IF EXISTS friendly_name,
  DROP COLUMN IF EXISTS title,
  DROP COLUMN IF EXISTS name,
  DROP COLUMN IF EXISTS config_hash,
  DROP COLUMN IF EXISTS global_project_id,
  DROP COLUMN IF EXISTS instance_uuid;

-- Note: Legacy columns (project_id, project_name, providers, metadata, status) are preserved
