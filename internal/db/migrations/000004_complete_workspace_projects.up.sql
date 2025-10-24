-- Add missing fields to workspace_projects table
-- Bringing migration in sync with WorkspaceProject model (pkg/models/workspace_project.go)

-- Drop obsolete columns that no longer exist in the model
ALTER TABLE workspace_projects 
  DROP COLUMN IF EXISTS project_id,
  DROP COLUMN IF EXISTS project_name;

-- Drop obsolete indexes
DROP INDEX IF EXISTS idx_workspace_projects_project_id;

-- Add instance relationship
ALTER TABLE workspace_projects 
  ADD COLUMN IF NOT EXISTS instance_uuid UUID;

-- Add computed identifiers
ALTER TABLE workspace_projects 
  ADD COLUMN IF NOT EXISTS global_project_id VARCHAR(512),
  ADD COLUMN IF NOT EXISTS config_hash VARCHAR(64);

-- Add core project fields (replacing legacy project_name with proper fields)
ALTER TABLE workspace_projects 
  ADD COLUMN IF NOT EXISTS name VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS friendly_name TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS short_name TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS description TEXT;

-- Update status column (already exists but ensure proper default)
-- Note: Can't add DEFAULT to existing column in PostgreSQL without rewriting
-- Instead, we update NULL values
UPDATE workspace_projects SET status = 'active' WHERE status IS NULL;

-- Add JSON storage columns (replacing legacy TEXT columns)
ALTER TABLE workspace_projects 
  ADD COLUMN IF NOT EXISTS providers_json JSONB,
  ADD COLUMN IF NOT EXISTS metadata_json JSONB;

-- Migrate legacy columns to JSON if they have data
UPDATE workspace_projects 
SET providers_json = to_jsonb(providers::text)
WHERE providers IS NOT NULL AND providers_json IS NULL;

UPDATE workspace_projects 
SET metadata_json = to_jsonb(metadata::text)
WHERE metadata IS NOT NULL AND metadata_json IS NULL;

-- Add source tracking fields
ALTER TABLE workspace_projects 
  ADD COLUMN IF NOT EXISTS source_type VARCHAR(50) NOT NULL DEFAULT 'hcl_file',
  ADD COLUMN IF NOT EXISTS source_identifier TEXT,
  ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMP,
  ADD COLUMN IF NOT EXISTS config_version VARCHAR(20) NOT NULL DEFAULT '1.0';

-- Create indexes for new columns
CREATE INDEX IF NOT EXISTS idx_workspace_projects_instance ON workspace_projects(instance_uuid);
CREATE INDEX IF NOT EXISTS idx_workspace_projects_global_id ON workspace_projects(global_project_id);
CREATE INDEX IF NOT EXISTS idx_workspace_projects_config_hash ON workspace_projects(config_hash);
CREATE INDEX IF NOT EXISTS idx_workspace_projects_instance_name ON workspace_projects(instance_uuid, name);

-- Note: project_uuid unique constraint already exists from original migration
-- Note: Legacy columns (project_id, project_name, providers, metadata) are kept for backward compatibility
-- They can be dropped in a future migration after data is fully migrated to new columns
