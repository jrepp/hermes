# Instance Identity Implementation Plan

**Date**: October 23, 2025  
**Status**: ✅ Approved - Composite Instance + Project Identity  
**Approach**: Instance UUID + Project ID = Global Project Identity

## Implementation Overview

We'll implement **Solution 1** from `DISTRIBUTED_PROJECT_IDENTITY.md`: composite identity using instance UUID + project ID.

### Architecture

```
Instance Identity (per deployment)
  ↓
  instance_uuid: 8c7d3f2e-4a5b-4c6d-8e7f-9a0b1c2d3e4f
  instance_id: "hermes.internal.example.com"
  ↓
Projects (per instance)
  ↓
  instance_uuid: 8c7d3f2e-4a5b-4c6d-8e7f-9a0b1c2d3e4f
  project_id: "docs-internal"
  project_uuid: auto-generated
  global_project_id: "8c7d3f2e.../docs-internal" (composite)
  ↓
Documents (per project)
  ↓
  project_uuid: reference to projects.project_uuid
  uuid: document UUID
```

## Phase 1: Database Schema (Priority 1)

### 1.1 Create Instance Table

**File**: `internal/db/migrations/001_create_hermes_instances.sql`

```sql
-- Hermes instance identity (singleton per database)
CREATE TABLE hermes_instances (
  id SERIAL PRIMARY KEY,
  
  -- Instance identifiers
  instance_uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  instance_id VARCHAR(255) NOT NULL UNIQUE,
  
  -- Instance metadata
  instance_name VARCHAR(255) NOT NULL,
  base_url VARCHAR(255),
  deployment_env VARCHAR(50) NOT NULL DEFAULT 'development',
  
  -- Tracking
  initialized_at TIMESTAMP NOT NULL DEFAULT NOW(),
  last_heartbeat TIMESTAMP NOT NULL DEFAULT NOW(),
  
  -- Metadata
  metadata JSONB,
  
  CONSTRAINT chk_deployment_env CHECK (deployment_env IN ('production', 'staging', 'development', 'test'))
);

-- Only one active instance per database (singleton constraint)
CREATE UNIQUE INDEX idx_hermes_instances_singleton ON hermes_instances ((1));

-- Indexes
CREATE INDEX idx_hermes_instances_instance_id ON hermes_instances(instance_id);
CREATE INDEX idx_hermes_instances_uuid ON hermes_instances(instance_uuid);

-- Insert initial instance (will be updated by application)
INSERT INTO hermes_instances (instance_id, instance_name, deployment_env, metadata)
VALUES 
  ('default', 'Hermes Instance', 'development', '{}')
ON CONFLICT (instance_uuid) DO NOTHING;

COMMENT ON TABLE hermes_instances IS 'Singleton table storing this Hermes instance identity';
COMMENT ON COLUMN hermes_instances.instance_uuid IS 'Globally unique instance identifier (auto-generated)';
COMMENT ON COLUMN hermes_instances.instance_id IS 'Human-readable instance identifier (e.g., hermes.internal.example.com)';
```

### 1.2 Update Projects Table

**File**: `internal/db/migrations/002_update_projects_with_instance.sql`

```sql
-- Add instance relationship to projects
ALTER TABLE projects ADD COLUMN instance_uuid UUID;
ALTER TABLE projects ADD COLUMN project_uuid UUID DEFAULT gen_random_uuid();
ALTER TABLE projects ADD COLUMN config_hash VARCHAR(64);

-- Set instance_uuid for existing projects (if any)
UPDATE projects 
SET instance_uuid = (SELECT instance_uuid FROM hermes_instances LIMIT 1)
WHERE instance_uuid IS NULL;

-- Make instance_uuid required
ALTER TABLE projects ALTER COLUMN instance_uuid SET NOT NULL;
ALTER TABLE projects ALTER COLUMN project_uuid SET NOT NULL;

-- Add foreign key to hermes_instances
ALTER TABLE projects 
  ADD CONSTRAINT fk_projects_instance 
  FOREIGN KEY (instance_uuid) 
  REFERENCES hermes_instances(instance_uuid) 
  ON DELETE CASCADE;

-- Add generated column for composite global_project_id
ALTER TABLE projects ADD COLUMN global_project_id VARCHAR(512) 
  GENERATED ALWAYS AS (instance_uuid::text || '/' || project_id) STORED;

-- Update constraints
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_project_id_key;
CREATE UNIQUE INDEX idx_projects_instance_project ON projects(instance_uuid, project_id);
CREATE UNIQUE INDEX idx_projects_project_uuid ON projects(project_uuid);
CREATE INDEX idx_projects_global_id ON projects(global_project_id);
CREATE INDEX idx_projects_config_hash ON projects(config_hash);

COMMENT ON COLUMN projects.instance_uuid IS 'Reference to hermes_instances.instance_uuid';
COMMENT ON COLUMN projects.project_uuid IS 'Auto-generated globally unique project identifier';
COMMENT ON COLUMN projects.global_project_id IS 'Composite global identifier: instance_uuid/project_id';
COMMENT ON COLUMN projects.config_hash IS 'SHA-256 hash of project configuration (detects drift)';
```

### 1.3 Update Documents Table

**File**: `internal/db/migrations/003_update_documents_with_project_uuid.sql`

```sql
-- Add project_uuid to documents (replaces project_id as FK)
ALTER TABLE documents ADD COLUMN project_uuid UUID;
ALTER TABLE documents ADD COLUMN provider_document_id VARCHAR(255);
ALTER TABLE documents ADD COLUMN indexed_at TIMESTAMP;
ALTER TABLE documents ADD COLUMN indexer_version VARCHAR(50);

-- Migrate existing documents to first project (if any exist)
UPDATE documents 
SET project_uuid = (SELECT project_uuid FROM projects LIMIT 1)
WHERE project_uuid IS NULL;

-- Make project_uuid required
ALTER TABLE documents ALTER COLUMN project_uuid SET NOT NULL;

-- Add foreign key
ALTER TABLE documents 
  ADD CONSTRAINT fk_documents_project 
  FOREIGN KEY (project_uuid) 
  REFERENCES projects(project_uuid) 
  ON DELETE CASCADE;

-- Indexes
CREATE INDEX idx_documents_project_uuid ON documents(project_uuid);
CREATE INDEX idx_documents_provider_document_id ON documents(provider_document_id);
CREATE INDEX idx_documents_indexed_at ON documents(indexed_at);

-- Unique constraint: one document UUID per project
CREATE UNIQUE INDEX idx_documents_project_uuid_doc_uuid 
  ON documents(project_uuid, uuid);

COMMENT ON COLUMN documents.project_uuid IS 'Reference to projects.project_uuid';
COMMENT ON COLUMN documents.provider_document_id IS 'Provider-specific document identifier (file path, Google file ID, etc)';
```

### 1.4 Update Document Revisions Table

**File**: `internal/db/migrations/004_update_revisions_with_project_uuid.sql`

```sql
-- Add project_uuid to document_revisions (for migration tracking)
ALTER TABLE document_revisions ADD COLUMN project_uuid UUID;

-- Populate from parent documents
UPDATE document_revisions dr
SET project_uuid = d.project_uuid
FROM documents d
WHERE dr.document_id = d.id AND dr.project_uuid IS NULL;

-- Make project_uuid required
ALTER TABLE document_revisions ALTER COLUMN project_uuid SET NOT NULL;

-- Add foreign key
ALTER TABLE document_revisions 
  ADD CONSTRAINT fk_revisions_project 
  FOREIGN KEY (project_uuid) 
  REFERENCES projects(project_uuid) 
  ON DELETE CASCADE;

-- Update unique constraint to include project_uuid (supports migration)
ALTER TABLE document_revisions DROP CONSTRAINT IF EXISTS document_revisions_document_id_content_hash_key;
CREATE UNIQUE INDEX idx_revisions_doc_project_hash 
  ON document_revisions(document_id, project_uuid, content_hash);

-- Indexes
CREATE INDEX idx_revisions_project_uuid ON document_revisions(project_uuid);

COMMENT ON COLUMN document_revisions.project_uuid IS 'Tracks which project owns this revision (supports migration tracking)';
```

## Phase 2: Models (Priority 1)

### 2.1 Create HermesInstance Model

**File**: `pkg/models/instance.go` (NEW)

```go
package models

import (
	"time"
	
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// HermesInstance represents this Hermes deployment's identity.
// This is a singleton table - only one instance per database.
type HermesInstance struct {
	ID uint `gorm:"primaryKey"`
	
	// Instance identifiers
	InstanceUUID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	InstanceID   string    `gorm:"uniqueIndex;not null;size:255"`
	
	// Instance metadata
	InstanceName  string `gorm:"not null;size:255"`
	BaseURL       string `gorm:"size:255"`
	DeploymentEnv string `gorm:"not null;default:development;size:50"`
	
	// Tracking
	InitializedAt time.Time  `gorm:"not null"`
	LastHeartbeat time.Time  `gorm:"not null"`
	
	// Metadata
	Metadata datatypes.JSON `gorm:"type:jsonb"`
	
	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName returns the table name for GORM
func (HermesInstance) TableName() string {
	return "hermes_instances"
}

// GetInstance retrieves the singleton instance record.
// Creates one if it doesn't exist.
func GetInstance(db *gorm.DB) (*HermesInstance, error) {
	var instance HermesInstance
	
	// Try to get existing instance
	if err := db.First(&instance).Error; err == nil {
		return &instance, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	
	// No instance exists - this shouldn't happen if migrations ran
	return nil, gorm.ErrRecordNotFound
}

// UpdateHeartbeat updates the last_heartbeat timestamp
func (i *HermesInstance) UpdateHeartbeat(db *gorm.DB) error {
	return db.Model(i).Update("last_heartbeat", time.Now()).Error
}
```

### 2.2 Create Project Model

**File**: `pkg/models/project.go` (NEW)

```go
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
	
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Project represents a document project with its workspace configuration.
type Project struct {
	ID uint `gorm:"primaryKey"`
	
	// Instance relationship
	InstanceUUID uuid.UUID `gorm:"type:uuid;not null;index"`
	Instance     *HermesInstance `gorm:"foreignKey:InstanceUUID;references:InstanceUUID"`
	
	// Project identifiers
	ProjectUUID      uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	ProjectID        string    `gorm:"not null;size:255"`
	GlobalProjectID  string    `gorm:"size:512;index"` // Generated: instance_uuid/project_id
	
	// Project metadata
	ShortName   string `gorm:"not null;size:50"`
	Description string `gorm:"type:text"`
	Status      string `gorm:"not null;default:active;size:50"`
	
	// Provider configuration
	ProviderType   string         `gorm:"not null;size:50"`
	ProviderConfig datatypes.JSON `gorm:"type:jsonb;not null"`
	ConfigHash     string         `gorm:"size:64;index"` // SHA-256 of config
	
	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for GORM
func (Project) TableName() string {
	return "projects"
}

// BeforeCreate hook to generate project_uuid if not set
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ProjectUUID == uuid.Nil {
		p.ProjectUUID = uuid.New()
	}
	return nil
}

// CalculateConfigHash computes SHA-256 hash of provider configuration
func (p *Project) CalculateConfigHash() string {
	// Marshal config to canonical JSON
	configBytes, _ := json.Marshal(p.ProviderConfig)
	hash := sha256.Sum256(configBytes)
	return hex.EncodeToString(hash[:])
}

// FindByProjectID finds a project by instance_uuid + project_id
func FindProjectByID(db *gorm.DB, instanceUUID uuid.UUID, projectID string) (*Project, error) {
	var project Project
	err := db.Where("instance_uuid = ? AND project_id = ?", instanceUUID, projectID).
		First(&project).Error
	return &project, err
}

// FindByProjectUUID finds a project by its UUID
func FindProjectByUUID(db *gorm.DB, projectUUID uuid.UUID) (*Project, error) {
	var project Project
	err := db.Where("project_uuid = ?", projectUUID).First(&project).Error
	return &project, err
}
```

### 2.3 Update Document Model

**File**: `pkg/models/document.go` (MODIFY)

```go
// Add to existing Document struct:

type Document struct {
	gorm.Model
	
	// ... existing fields (GoogleFileID, etc.)
	
	// Project relationship (NEW)
	ProjectUUID uuid.UUID `gorm:"type:uuid;not null;index"`
	Project     *Project  `gorm:"foreignKey:ProjectUUID;references:ProjectUUID"`
	
	// Provider-specific identifier (NEW)
	ProviderDocumentID string `gorm:"size:255;index"`
	
	// Indexer metadata (NEW)
	IndexedAt      *time.Time `gorm:"index"`
	IndexerVersion string     `gorm:"size:50"`
	
	// ... rest of existing fields
}

// Add unique index check in migration or via GORM tag:
// UNIQUE INDEX on (project_uuid, uuid)
```

### 2.4 Update DocumentRevision Model

**File**: `pkg/models/document_revision.go` (NEW or MODIFY)

```go
package models

import (
	"time"
	
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DocumentRevision tracks versions of documents across projects and providers
type DocumentRevision struct {
	gorm.Model
	
	// Document relationship
	DocumentID uint     `gorm:"not null;index"`
	Document   Document `gorm:"foreignKey:DocumentID"`
	
	// Project relationship (for migration tracking)
	ProjectUUID uuid.UUID `gorm:"type:uuid;not null;index"`
	Project     *Project  `gorm:"foreignKey:ProjectUUID;references:ProjectUUID"`
	
	// Revision metadata
	ContentHash       string `gorm:"not null;size:255;index"`
	RevisionReference string `gorm:"size:255"` // Git commit, version number, etc.
	CommitSHA         string `gorm:"size:255"`
	
	// Content metadata
	ContentLength int64
	ContentType   string `gorm:"size:100"`
	Summary       string `gorm:"type:text"`
	
	// Modification tracking
	ModifiedBy string     `gorm:"size:255"`
	ModifiedAt *time.Time
	
	// Additional metadata
	Metadata datatypes.JSON `gorm:"type:jsonb"`
}

// TableName returns the table name for GORM
func (DocumentRevision) TableName() string {
	return "document_revisions"
}

// FindByContentHash finds revisions by content hash (detects duplicates)
func FindRevisionByContentHash(db *gorm.DB, documentID uint, projectUUID uuid.UUID, contentHash string) (*DocumentRevision, error) {
	var revision DocumentRevision
	err := db.Where("document_id = ? AND project_uuid = ? AND content_hash = ?",
		documentID, projectUUID, contentHash).First(&revision).Error
	return &revision, err
}
```

## Phase 3: Instance Initialization (Priority 1)

### 3.1 Instance Configuration

**File**: `internal/config/config.go` (ADD)

```go
// Add to Config struct:

type Config struct {
	// ... existing fields
	
	// Instance configuration (NEW)
	Instance InstanceConfig `hcl:"instance,block"`
	
	// ... rest of fields
}

type InstanceConfig struct {
	ID          string `hcl:"id"`
	Name        string `hcl:"name"`
	Environment string `hcl:"environment,optional"`
	BaseURL     string `hcl:"base_url,optional"`
}
```

**Example Config**:
```hcl
# config.hcl
instance {
  id          = "hermes.local.dev"
  name        = "Local Development Instance"
  environment = "development"
  base_url    = "http://localhost:8000"
}

# ... rest of config
```

### 3.2 Instance Initialization Service

**File**: `internal/instance/instance.go` (NEW)

```go
package instance

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

var (
	currentInstance   *models.HermesInstance
	currentInstanceMu sync.RWMutex
)

// Initialize sets up the Hermes instance identity.
// This should be called once at startup.
func Initialize(ctx context.Context, db *gorm.DB, cfg *config.Config, logger hclog.Logger) error {
	currentInstanceMu.Lock()
	defer currentInstanceMu.Unlock()
	
	// Check if instance already exists
	var instance models.HermesInstance
	err := db.First(&instance).Error
	
	if err == nil {
		// Instance exists - update if config changed
		needsUpdate := false
		
		if instance.InstanceID != cfg.Instance.ID {
			logger.Warn("Instance ID changed in config",
				"old", instance.InstanceID,
				"new", cfg.Instance.ID)
			instance.InstanceID = cfg.Instance.ID
			needsUpdate = true
		}
		
		if instance.InstanceName != cfg.Instance.Name {
			instance.InstanceName = cfg.Instance.Name
			needsUpdate = true
		}
		
		if instance.BaseURL != cfg.Instance.BaseURL {
			instance.BaseURL = cfg.Instance.BaseURL
			needsUpdate = true
		}
		
		if instance.DeploymentEnv != cfg.Instance.Environment {
			instance.DeploymentEnv = cfg.Instance.Environment
			needsUpdate = true
		}
		
		if needsUpdate {
			if err := db.Save(&instance).Error; err != nil {
				return fmt.Errorf("failed to update instance: %w", err)
			}
			logger.Info("Instance configuration updated")
		}
		
		// Update heartbeat
		instance.LastHeartbeat = time.Now()
		if err := db.Model(&instance).Update("last_heartbeat", instance.LastHeartbeat).Error; err != nil {
			logger.Warn("Failed to update heartbeat", "error", err)
		}
		
		currentInstance = &instance
		logger.Info("Instance initialized",
			"instance_id", instance.InstanceID,
			"instance_uuid", instance.InstanceUUID,
			"environment", instance.DeploymentEnv)
		
		return nil
	}
	
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to query instance: %w", err)
	}
	
	// No instance exists - create new
	instance = models.HermesInstance{
		InstanceUUID:  uuid.New(),
		InstanceID:    cfg.Instance.ID,
		InstanceName:  cfg.Instance.Name,
		BaseURL:       cfg.Instance.BaseURL,
		DeploymentEnv: cfg.Instance.Environment,
		InitializedAt: time.Now(),
		LastHeartbeat: time.Now(),
	}
	
	if err := db.Create(&instance).Error; err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}
	
	currentInstance = &instance
	
	logger.Info("Hermes instance initialized",
		"instance_id", instance.InstanceID,
		"instance_uuid", instance.InstanceUUID,
		"environment", instance.DeploymentEnv)
	
	return nil
}

// GetCurrentInstance returns the current instance (must call Initialize first)
func GetCurrentInstance() *models.HermesInstance {
	currentInstanceMu.RLock()
	defer currentInstanceMu.RUnlock()
	return currentInstance
}

// StartHeartbeat starts a background goroutine that updates last_heartbeat
func StartHeartbeat(ctx context.Context, db *gorm.DB, interval time.Duration, logger hclog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			instance := GetCurrentInstance()
			if instance == nil {
				continue
			}
			
			if err := instance.UpdateHeartbeat(db); err != nil {
				logger.Warn("Failed to update instance heartbeat", "error", err)
			}
		}
	}
}
```

### 3.3 Update Server Initialization

**File**: `internal/server/server.go` (MODIFY)

```go
import (
	"github.com/hashicorp-forge/hermes/internal/instance"
	// ... other imports
)

func NewServer(cfg *config.Config, logger hclog.Logger) (*Server, error) {
	// ... existing setup (DB, etc.)
	
	// Initialize instance identity (NEW)
	if err := instance.Initialize(context.Background(), db, cfg, logger); err != nil {
		return nil, fmt.Errorf("failed to initialize instance: %w", err)
	}
	
	// Start heartbeat (NEW)
	ctx, cancel := context.WithCancel(context.Background())
	go instance.StartHeartbeat(ctx, db, 1*time.Minute, logger)
	
	// ... rest of server setup
	
	return srv, nil
}
```

## Phase 4: Project Registration (Priority 2)

### 4.1 Project Registration Service

**File**: `internal/projects/registration.go` (NEW)

```go
package projects

import (
	"context"
	"fmt"
	
	"github.com/hashicorp-forge/hermes/internal/instance"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// RegisterProject registers a project from config with the current instance
func RegisterProject(ctx context.Context, db *gorm.DB, cfg *projectconfig.Project, logger hclog.Logger) (*models.Project, error) {
	inst := instance.GetCurrentInstance()
	if inst == nil {
		return nil, fmt.Errorf("instance not initialized")
	}
	
	// Check if project already registered
	project, err := models.FindProjectByID(db, inst.InstanceUUID, cfg.ID)
	if err == nil {
		// Project exists - check for config changes
		newHash := calculateConfigHash(cfg)
		if project.ConfigHash != newHash {
			logger.Warn("Project configuration changed",
				"project_id", cfg.ID,
				"old_hash", project.ConfigHash,
				"new_hash", newHash)
			
			// Update project config
			project.ShortName = cfg.ShortName
			project.Description = cfg.Description
			project.Status = cfg.Status
			project.ProviderType = cfg.Workspace.Type
			project.ProviderConfig = cfg.Workspace.ToJSON()
			project.ConfigHash = newHash
			
			if err := db.Save(project).Error; err != nil {
				return nil, fmt.Errorf("failed to update project: %w", err)
			}
			
			logger.Info("Project configuration updated", "project_id", cfg.ID)
		}
		
		return project, nil
	}
	
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to query project: %w", err)
	}
	
	// Create new project
	project = &models.Project{
		InstanceUUID:   inst.InstanceUUID,
		ProjectID:      cfg.ID,
		ShortName:      cfg.ShortName,
		Description:    cfg.Description,
		Status:         cfg.Status,
		ProviderType:   cfg.Workspace.Type,
		ProviderConfig: cfg.Workspace.ToJSON(),
		ConfigHash:     calculateConfigHash(cfg),
	}
	
	if err := db.Create(project).Error; err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	
	logger.Info("Project registered",
		"project_id", project.ProjectID,
		"project_uuid", project.ProjectUUID,
		"global_project_id", project.GlobalProjectID,
		"provider_type", project.ProviderType)
	
	return project, nil
}

// RegisterAllProjects registers all projects from config
func RegisterAllProjects(ctx context.Context, db *gorm.DB, cfg *projectconfig.Config, logger hclog.Logger) error {
	for _, project := range cfg.Projects {
		if _, err := RegisterProject(ctx, db, project, logger); err != nil {
			logger.Error("Failed to register project",
				"project_id", project.ID,
				"error", err)
			continue
		}
	}
	return nil
}

func calculateConfigHash(cfg *projectconfig.Project) string {
	// Implementation similar to models.Project.CalculateConfigHash()
	// ...
}
```

## Phase 5: Updated Integration Test (Priority 2)

**File**: `tests/integration/indexer/full_pipeline_test.go` (MODIFY)

```go
func TestFullPipelineWithProjectComposite(t *testing.T) {
	// Setup
	ctx := context.Background()
	testDB := setupDatabase(t)
	
	// 1. Initialize instance identity
	instanceCfg := &config.Config{
		Instance: config.InstanceConfig{
			ID:          "test-instance-local",
			Name:        "Test Instance",
			Environment: "test",
		},
	}
	
	err := instance.Initialize(ctx, testDB, instanceCfg, logger)
	require.NoError(t, err)
	
	inst := instance.GetCurrentInstance()
	require.NotNil(t, inst)
	logger.Info("Instance initialized",
		"instance_id", inst.InstanceID,
		"instance_uuid", inst.InstanceUUID)
	
	// 2. Load and register project
	projectCfg, err := projectconfig.LoadConfig("testing/projects.hcl")
	require.NoError(t, err)
	
	project, err := projects.RegisterProject(ctx, testDB, projectCfg.GetProject("docs-internal"), logger)
	require.NoError(t, err)
	require.NotNil(t, project)
	
	logger.Info("Project registered",
		"project_uuid", project.ProjectUUID,
		"global_project_id", project.GlobalProjectID)
	
	// 3. Create API client
	apiClient := NewIndexerAPIClient("http://localhost:8001", testAuthToken)
	
	// 4. Discover documents
	provider, err := workspace.NewProvider(projectCfg.GetProject("docs-internal").Workspace)
	require.NoError(t, err)
	
	docs, err := provider.ListDocuments(ctx, ".", nil)
	require.NoError(t, err)
	
	// 5. Process documents through pipeline
	for _, doc := range docs {
		// Create document via API (references project_uuid)
		docResp, err := apiClient.CreateDocument(ctx, &CreateDocumentRequest{
			UUID:               doc.UUID,
			ProjectUUID:        project.ProjectUUID, // Use project UUID
			ProviderDocumentID: doc.Path,
			Title:              doc.Name,
			// ... other fields
		})
		require.NoError(t, err)
		
		// Create revision
		revResp, err := apiClient.CreateRevision(ctx, doc.UUID, &CreateRevisionRequest{
			ProjectUUID: project.ProjectUUID, // Include project UUID
			ContentHash: doc.ContentHash,
			// ... other fields
		})
		require.NoError(t, err)
		
		// ... rest of pipeline
	}
	
	// 6. Verify via global_project_id
	var dbProject models.Project
	err = testDB.Where("global_project_id = ?", project.GlobalProjectID).First(&dbProject).Error
	require.NoError(t, err)
	
	// 7. Verify documents belong to correct project
	var docCount int64
	err = testDB.Model(&models.Document{}).
		Where("project_uuid = ?", project.ProjectUUID).
		Count(&docCount).Error
	require.NoError(t, err)
	assert.Greater(t, docCount, int64(0))
}
```

## Summary

This implementation provides:

✅ **Globally unique instance identity** (instance_uuid)  
✅ **Composite project identity** (instance_uuid + project_id)  
✅ **Auto-generated project UUIDs** (for database FK)  
✅ **Computed global_project_id** (for human-readable queries)  
✅ **Migration support** (document_revisions tracks project_uuid)  
✅ **Config drift detection** (config_hash)  
✅ **Heartbeat monitoring** (instance health)  
✅ **Federation-ready** (instance table can be federated)

**Next Step**: Start with Phase 1 (database migrations) to create the schema foundation.
