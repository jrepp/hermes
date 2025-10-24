# Distributed Project Identity & Document Resolution

**Date**: October 23, 2025  
**Status**: 🚧 Design Discussion  
**Problem**: How to uniquely identify projects and locate documents across distributed Hermes instances

## The Problem

### Scenario: Multiple Hermes Instances

```
Instance A (hermes.internal.example.com)
  └─ Project "docs-internal" from ./docs-internal/
      └─ RFC-001.md (uuid: 550e8400...)

Instance B (hermes.dev.example.com)  
  └─ Project "docs-internal" from ./docs-internal/  (SAME CONFIG!)
      └─ RFC-001.md (uuid: 550e8400...)  (SAME DOCUMENT!)

Instance C (hermes.prod.example.com)
  └─ Project "docs-internal" from ./docs-internal/  (SAME CONFIG!)
      └─ RFC-001.md (uuid: 550e8400...)  (SAME DOCUMENT!)
```

**Questions**:
1. Are these the **same project** or **different projects**?
2. How do we route requests to the correct instance?
3. What happens if the document content diverges?
4. How do we federate/aggregate across instances?

## Identity Layers: Project vs Instance vs Document

### Layer 1: Document Identity (SOLVED ✅)
**UUID**: Stable, globally unique identifier for the document
```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
```
✅ Persists across migrations, providers, instances  
✅ Can be declared in frontmatter or auto-assigned  
✅ Content hash tracks versions

### Layer 2: Project Identity (PROBLEM ❌)
**Current**: `project_id` as string (e.g., "docs-internal")
```hcl
project "docs-internal" {
  short_name = "DOCS"
  # ... config
}
```
❌ Not globally unique across instances  
❌ Two instances can have same project_id  
❌ No way to distinguish "docs-internal@instanceA" vs "docs-internal@instanceB"

### Layer 3: Instance Identity (MISSING ❌)
**Need**: Globally unique instance identifier
```
Instance ID: hermes.internal.example.com
Instance UUID: 8c7d3f2e-4a5b-4c6d-8e7f-9a0b1c2d3e4f
```
❌ No current concept of "which Hermes instance am I?"

## Proposed Solutions

### Solution 1: Composite Project Identity (Recommended)

Use a **composite key** combining instance identity + local project ID.

#### Schema Changes

**Add Instance Identifier** (one-time setup per Hermes deployment):
```sql
-- New table: hermes_instances (singleton or very small)
CREATE TABLE hermes_instances (
  id SERIAL PRIMARY KEY,
  instance_id VARCHAR(255) NOT NULL UNIQUE,  -- "hermes.internal.example.com"
  instance_uuid UUID NOT NULL UNIQUE,         -- 8c7d3f2e-4a5b-4c6d-8e7f-9a0b1c2d3e4f
  instance_name VARCHAR(255),                 -- "Internal Hermes Dev"
  base_url VARCHAR(255),                      -- "https://hermes.internal.example.com"
  deployment_env VARCHAR(50),                 -- "production", "staging", "dev"
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Only one active instance per database
CREATE UNIQUE INDEX idx_hermes_instances_active ON hermes_instances ((1));
```

**Update Projects Table**:
```sql
CREATE TABLE projects (
  id SERIAL PRIMARY KEY,
  
  -- Composite identity
  instance_uuid UUID NOT NULL REFERENCES hermes_instances(instance_uuid),
  project_id VARCHAR(255) NOT NULL,           -- Local ID from config: "docs-internal"
  
  -- Globally unique composite key
  global_project_id VARCHAR(512) GENERATED ALWAYS AS 
    (instance_uuid::text || '/' || project_id) STORED,
  
  -- Or use a true UUID for the project
  project_uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  
  short_name VARCHAR(50) NOT NULL,
  description TEXT,
  status VARCHAR(50) NOT NULL DEFAULT 'active',
  provider_type VARCHAR(50) NOT NULL,
  provider_config JSONB NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  
  -- Unique within instance
  UNIQUE(instance_uuid, project_id)
);

CREATE INDEX idx_projects_instance_project ON projects(instance_uuid, project_id);
CREATE INDEX idx_projects_global_id ON projects(global_project_id);
CREATE INDEX idx_projects_project_uuid ON projects(project_uuid);
```

**Update Documents Table**:
```sql
ALTER TABLE documents ADD COLUMN project_uuid UUID NOT NULL REFERENCES projects(project_uuid);
-- Keep project_id for backward compat / queries, but project_uuid is the FK
```

#### Project Registration Flow

```go
// 1. Instance initialization (once per deployment)
func InitializeInstance(cfg *config.Config) (*models.HermesInstance, error) {
    db := getDB()
    
    // Check if instance already initialized
    var instance models.HermesInstance
    if err := db.First(&instance).Error; err == nil {
        // Already initialized
        return &instance, nil
    }
    
    // Create new instance identity
    instance = models.HermesInstance{
        InstanceID:    cfg.Instance.ID,      // From config: "hermes.internal.example.com"
        InstanceUUID:  uuid.New(),
        InstanceName:  cfg.Instance.Name,
        BaseURL:       cfg.Server.BaseURL,
        DeploymentEnv: cfg.Instance.Environment,
    }
    
    if err := db.Create(&instance).Error; err != nil {
        return nil, err
    }
    
    log.Info("Hermes instance initialized", "instance_id", instance.InstanceID)
    return &instance, nil
}

// 2. Project registration (from config)
func RegisterProject(cfg *projectconfig.Project) (*models.Project, error) {
    db := getDB()
    instance := getCurrentInstance() // From context or global
    
    // Check if project already exists for this instance
    var project models.Project
    err := db.Where("instance_uuid = ? AND project_id = ?", 
        instance.InstanceUUID, cfg.ID).First(&project).Error
    
    if err == nil {
        // Project exists, update if config changed
        return updateProjectIfNeeded(&project, cfg)
    }
    
    // Create new project
    project = models.Project{
        InstanceUUID:   instance.InstanceUUID,
        ProjectID:      cfg.ID,                // "docs-internal"
        ProjectUUID:    uuid.New(),            // Auto-generated
        ShortName:      cfg.ShortName,
        Description:    cfg.Description,
        Status:         cfg.Status,
        ProviderType:   cfg.Workspace.Type,
        ProviderConfig: cfg.Workspace.ToJSON(),
    }
    
    if err := db.Create(&project).Error; err != nil {
        return nil, err
    }
    
    log.Info("Project registered",
        "instance_id", instance.InstanceID,
        "project_id", project.ProjectID,
        "project_uuid", project.ProjectUUID,
        "global_id", project.GlobalProjectID)
    
    return &project, nil
}
```

#### Document Resolution

**Local Query** (within instance):
```go
// Documents know their project_uuid
doc, err := db.Where("uuid = ? AND project_uuid = ?", docUUID, projectUUID).First(&doc).Error
```

**Federated Query** (across instances):
```go
// Query local instance
localDoc, err := queryLocal(docUUID)

// If not found and federation enabled, query remote instances
if err == gorm.ErrRecordNotFound && cfg.Federation.Enabled {
    remoteDoc, remoteInstance, err := queryFederatedInstances(docUUID)
    if err == nil {
        // Found in remote instance
        return &FederatedDocument{
            Document:       remoteDoc,
            SourceInstance: remoteInstance,
            LocalProxy:     false,
        }
    }
}
```

#### Advantages ✅
- ✅ Globally unique project identification (instance_uuid + project_id)
- ✅ Each instance maintains its own project registry
- ✅ Same project config can be deployed to multiple instances
- ✅ Documents reference project_uuid (stable FK)
- ✅ Federation-ready: can query remote instances by instance_uuid
- ✅ Clear ownership: documents belong to specific instance's project

#### Disadvantages ❌
- ❌ Requires instance initialization step
- ❌ More complex schema (3-level hierarchy: instance → project → document)
- ❌ Federation requires network calls to remote instances

---

### Solution 2: Content-Addressable Projects (Alternative)

Use **content hash** of project configuration as identity.

```sql
CREATE TABLE projects (
  id SERIAL PRIMARY KEY,
  project_id VARCHAR(255) NOT NULL,           -- "docs-internal"
  config_hash VARCHAR(64) NOT NULL,           -- SHA-256 of normalized config
  project_uuid UUID NOT NULL UNIQUE,          -- Auto-generated
  
  provider_type VARCHAR(50) NOT NULL,
  provider_config JSONB NOT NULL,
  
  -- Multiple instances can have same config_hash (intentional duplicates)
  UNIQUE(project_id, config_hash)
);
```

**Config Hash Calculation**:
```go
func calculateProjectConfigHash(project *projectconfig.Project) string {
    // Normalize config (sort keys, remove comments, etc.)
    normalized := normalizeProjectConfig(project)
    
    // SHA-256 hash
    hash := sha256.Sum256([]byte(normalized))
    return hex.EncodeToString(hash[:])
}

// Same config = same hash across instances
// Different config = different hash (allows evolution)
```

#### Advantages ✅
- ✅ No instance concept needed
- ✅ Automatically detects config changes (different hash = new project version)
- ✅ Content-addressable: same config always produces same hash
- ✅ Simpler schema

#### Disadvantages ❌
- ❌ Doesn't solve distributed identity problem (multiple instances with same hash)
- ❌ Config changes create new project records (versioning issue)
- ❌ No way to query "which instance owns this project?"

---

### Solution 3: Hybrid - Config UUID + Instance Registry (Best?)

Require **explicit UUID in project config**, but track **instance ownership**.

#### Project Config (Required UUID)

```hcl
# testing/projects/docs-internal.hcl
project "docs-internal" {
  # REQUIRED: Globally unique UUID (generate once, commit to git)
  uuid        = "a1b2c3d4-e5f6-4a5b-8c7d-9e0f1a2b3c4d"
  
  short_name  = "DOCS"
  description = "Internal documentation"
  status      = "active"
  
  workspace "local" {
    type = "local"
    root = "./docs-internal"
  }
}
```

#### Schema

```sql
-- Projects table
CREATE TABLE projects (
  id SERIAL PRIMARY KEY,
  
  -- From config (immutable, set once)
  project_uuid UUID NOT NULL UNIQUE,          -- From config file
  project_id VARCHAR(255) NOT NULL,           -- "docs-internal"
  
  -- Instance ownership
  instance_id VARCHAR(255) NOT NULL,          -- Which instance registered this
  registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
  
  -- Config
  short_name VARCHAR(50) NOT NULL,
  description TEXT,
  status VARCHAR(50) NOT NULL DEFAULT 'active',
  provider_type VARCHAR(50) NOT NULL,
  provider_config JSONB NOT NULL,
  config_hash VARCHAR(64) NOT NULL,           -- Detect config drift
  
  -- Allow same UUID to be registered by multiple instances (federation)
  UNIQUE(instance_id, project_uuid)
);

-- Track which instances know about which projects (federation registry)
CREATE TABLE project_instances (
  id SERIAL PRIMARY KEY,
  project_uuid UUID NOT NULL REFERENCES projects(project_uuid),
  instance_id VARCHAR(255) NOT NULL,
  instance_url VARCHAR(255),
  last_seen TIMESTAMP NOT NULL DEFAULT NOW(),
  is_authoritative BOOLEAN DEFAULT false,     -- Which instance is source of truth?
  
  UNIQUE(project_uuid, instance_id)
);
```

#### Registration Flow

```go
func RegisterProject(instanceID string, cfg *projectconfig.Project) error {
    db := getDB()
    
    // Validate config has UUID
    if cfg.UUID == nil {
        return fmt.Errorf("project %s missing required 'uuid' field", cfg.ID)
    }
    
    // Check if this instance already registered this project
    var project models.Project
    err := db.Where("instance_id = ? AND project_uuid = ?", 
        instanceID, cfg.UUID).First(&project).Error
    
    if err == nil {
        // Already registered by this instance - check for config drift
        newHash := calculateConfigHash(cfg)
        if project.ConfigHash != newHash {
            log.Warn("Project config changed",
                "project_id", cfg.ID,
                "old_hash", project.ConfigHash,
                "new_hash", newHash)
            return updateProjectConfig(&project, cfg, newHash)
        }
        return nil // No changes
    }
    
    // Register project for this instance
    configHash := calculateConfigHash(cfg)
    project = models.Project{
        ProjectUUID:    cfg.UUID,              // FROM CONFIG
        ProjectID:      cfg.ID,
        InstanceID:     instanceID,
        ShortName:      cfg.ShortName,
        ProviderType:   cfg.Workspace.Type,
        ProviderConfig: cfg.Workspace.ToJSON(),
        ConfigHash:     configHash,
    }
    
    if err := db.Create(&project).Error; err != nil {
        return err
    }
    
    // Register in federation table
    if cfg.Federation.Announce {
        registerProjectInstance(cfg.UUID, instanceID, cfg.Federation.URL)
    }
    
    return nil
}
```

#### Document Resolution (Federated)

**Hermes URI with Project Context**:
```
hermes://project/<project-uuid>/documents/<doc-uuid>
hermes://project/a1b2c3d4-e5f6-4a5b-8c7d-9e0f1a2b3c4d/documents/550e8400...

# Or shorter with document UUID only (query federation registry)
hermes://uuid/<doc-uuid>
hermes://uuid/550e8400...
```

**Resolution Algorithm**:
```go
func ResolveDocument(docUUID uuid.UUID, projectUUID *uuid.UUID) (*Document, error) {
    db := getDB()
    
    // 1. Try local lookup
    var doc models.Document
    query := db.Where("uuid = ?", docUUID)
    if projectUUID != nil {
        query = query.Where("project_uuid = ?", projectUUID)
    }
    
    if err := query.First(&doc).Error; err == nil {
        return &doc, nil // Found locally
    }
    
    // 2. Not found locally - check if we should query federation
    if !config.Federation.Enabled {
        return nil, ErrNotFound
    }
    
    // 3. Query project_instances table to find which instances might have it
    var instances []models.ProjectInstance
    query = db.Where("project_uuid = ?", projectUUID)
    if err := db.Find(&instances).Error; err != nil {
        return nil, ErrNotFound
    }
    
    // 4. Query remote instances in parallel
    results := make(chan *FederatedDocument, len(instances))
    for _, inst := range instances {
        go func(inst models.ProjectInstance) {
            doc, err := queryRemoteInstance(inst.InstanceURL, docUUID)
            if err == nil {
                results <- &FederatedDocument{
                    Document:       doc,
                    SourceInstance: inst.InstanceID,
                }
            }
        }(inst)
    }
    
    // 5. Return first successful result (with timeout)
    select {
    case doc := <-results:
        return doc, nil
    case <-time.After(2 * time.Second):
        return nil, ErrFederatedQueryTimeout
    }
}
```

#### Advantages ✅
- ✅ **Explicit UUID in config** - developer must think about identity
- ✅ **Git-tracked** - UUID committed with project config
- ✅ **Reproducible** - same config + same UUID across deployments
- ✅ **Federation-ready** - project_instances table tracks distribution
- ✅ **Authoritative instance** - can designate which instance is source of truth
- ✅ **Conflict detection** - config_hash detects drift between instances

#### Disadvantages ❌
- ❌ Manual UUID generation (developer responsibility)
- ❌ Risk of UUID collision if not generated properly (use `uuidgen` or similar)
- ❌ Config migration needed (add UUID to all existing projects)

---

## Recommended Approach: Hybrid (Solution 3)

### Implementation Plan

#### Phase 1: Add UUID to Project Configs

```bash
# Generate UUIDs for all projects
cd testing/projects
for file in *.hcl; do
  if ! grep -q "uuid" "$file"; then
    uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    # Insert UUID after project name
    sed -i '' "s/^project \"\(.*\)\" {/project \"\1\" {\n  uuid = \"$uuid\"/" "$file"
  fi
done
```

**Example**:
```hcl
# Before
project "docs-internal" {
  short_name = "DOCS"
}

# After
project "docs-internal" {
  uuid       = "a1b2c3d4-e5f6-4a5b-8c7d-9e0f1a2b3c4d"  # ADDED
  short_name = "DOCS"
}
```

#### Phase 2: Update Schema

```sql
-- Add instance_id to projects
ALTER TABLE projects ADD COLUMN instance_id VARCHAR(255) NOT NULL DEFAULT 'default';
ALTER TABLE projects ADD COLUMN project_uuid UUID;
ALTER TABLE projects ADD COLUMN config_hash VARCHAR(64);

-- Create federation table
CREATE TABLE project_instances (
  id SERIAL PRIMARY KEY,
  project_uuid UUID NOT NULL,
  instance_id VARCHAR(255) NOT NULL,
  instance_url VARCHAR(255),
  last_seen TIMESTAMP NOT NULL DEFAULT NOW(),
  is_authoritative BOOLEAN DEFAULT false,
  UNIQUE(project_uuid, instance_id)
);
```

#### Phase 3: Update Registration Logic

See code examples above.

#### Phase 4: Federation (Optional)

Enable cross-instance queries if needed.

---

## Configuration Example

### Hermes Instance Config

```hcl
# config.hcl
instance {
  id          = "hermes.internal.example.com"
  name        = "Internal Development Hermes"
  environment = "development"
}

server {
  addr     = "0.0.0.0:8000"
  base_url = "https://hermes.internal.example.com"
}

federation {
  enabled = true
  
  # Announce this instance's projects to federation registry
  announce = true
  
  # Known peer instances
  peers = [
    {
      instance_id = "hermes.prod.example.com"
      url         = "https://hermes.prod.example.com"
    }
  ]
}
```

### Project Config (with UUID)

```hcl
# testing/projects/docs-internal.hcl
project "docs-internal" {
  # Globally unique, committed to git
  uuid        = "a1b2c3d4-e5f6-4a5b-8c7d-9e0f1a2b3c4d"
  
  short_name  = "DOCS"
  description = "Internal documentation"
  status      = "active"
  
  workspace "local" {
    type = "local"
    root = "./docs-internal"
  }
  
  # Optional: federation settings
  federation {
    announce        = true  # Announce to peers
    authoritative   = true  # This instance is source of truth
  }
}
```

---

## Summary

| Aspect | Solution 1 (Instance+Project) | Solution 2 (Content Hash) | **Solution 3 (Config UUID)** ✅ |
|--------|------------------------------|---------------------------|--------------------------------|
| **Global Uniqueness** | ✅ instance_uuid + project_id | ⚠️ Hash collision risk | ✅ Explicit UUID in config |
| **Developer Experience** | ⚠️ Auto-assigned, opaque | ✅ Automatic | ✅ Explicit, transparent |
| **Git Tracking** | ❌ UUID in database only | ❌ Hash changes with config | ✅ UUID in git with config |
| **Federation** | ✅ Built-in | ❌ No instance tracking | ✅ Built-in with registry |
| **Config Changes** | ✅ Update, keep UUID | ❌ New hash = new project | ✅ Update, keep UUID |
| **Migration** | ⚠️ Instance init required | ✅ No migration | ⚠️ Add UUIDs to configs |

**Recommendation**: **Solution 3 (Hybrid)** - Explicit UUID in project config + instance registry for federation.

---

## Next Steps

1. Update project config schema to include `uuid` field
2. Generate UUIDs for existing projects
3. Update database schema (projects table + project_instances table)
4. Implement registration logic with UUID validation
5. Add federation support (optional, future)
6. Update API to use project_uuid instead of project_id for FK

**Ready to implement?** Start with Phase 1 (add UUIDs to project configs).
