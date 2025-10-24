# Indexer API Design Update: Project-Based Normalization

**Date**: October 23, 2025  
**Status**: ✅ Design Updated - Project Normalization  
**Supersedes**: Previous workspace_provider_metadata inline approach  
**Related**: See `DISTRIBUTED_PROJECT_IDENTITY.md` for distributed identity challenges

## Key Change: Normalize via Projects

Instead of storing workspace provider metadata **in each document**, we normalize by referencing the **project** that owns the document. The project configuration already contains all provider information.

### Why This is Better

❌ **Old Approach**: Duplicate workspace provider data in every document
```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "title": "RFC-001",
  "workspace_provider": {
    "type": "github",
    "repository": "hashicorp/hermes",
    "branch": "main",
    "path": "docs-internal/RFC-001.md",
    "commit_sha": "abc123def456"
  }
}
```
**Problems**:
- Duplicates provider config in every document
- Hard to update provider settings (e.g., change repository URL)
- Inconsistent data if provider config changes
- Larger database storage

✅ **New Approach**: Reference the project, store only document-specific data
```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "title": "RFC-001",
  "project_id": "docs-internal",
  "provider_document_id": "docs-internal/RFC-001.md"
}
```
**Benefits**:
- ✅ Single source of truth (project config)
- ✅ Easy to update provider settings (change project config)
- ✅ Consistent data across all documents in project
- ✅ Smaller database footprint
- ✅ Natural for migration (change project provider, re-index)

## Updated Database Schema

### Documents Table

**Changed Fields**:
```sql
ALTER TABLE documents ADD COLUMN project_id VARCHAR(255) NOT NULL;
ALTER TABLE documents ADD COLUMN provider_document_id VARCHAR(255) NOT NULL;
ALTER TABLE documents ADD COLUMN indexed_at TIMESTAMP;
ALTER TABLE documents ADD COLUMN indexer_version VARCHAR(50);

-- Remove old denormalized fields (if they exist)
-- ALTER TABLE documents DROP COLUMN IF EXISTS workspace_provider_type;
-- ALTER TABLE documents DROP COLUMN IF EXISTS workspace_provider_metadata;

-- Add index for project-based queries
CREATE INDEX idx_documents_project_id ON documents(project_id);
CREATE INDEX idx_documents_provider_document_id ON documents(provider_document_id);

-- Unique constraint: one UUID per project-provider combination
CREATE UNIQUE INDEX idx_documents_project_provider_uuid 
  ON documents(project_id, provider_document_id, uuid);
```

**Rationale**:
- `project_id`: Links to project configuration (globally unique, e.g., "docs-internal", "hashicorp/hermes/docs")
- `provider_document_id`: Provider-specific identifier (file path, Google file ID, remote UUID)
- No `workspace_provider_type` or `workspace_provider_metadata` - get from project config instead

### Projects Table (New)

We need a **projects table** to store project metadata loaded from HCL config:

```sql
CREATE TABLE projects (
  id SERIAL PRIMARY KEY,
  project_id VARCHAR(255) NOT NULL UNIQUE,  -- Globally unique: "docs-internal"
  short_name VARCHAR(50) NOT NULL,           -- "DOCS"
  description TEXT,
  status VARCHAR(50) NOT NULL DEFAULT 'active',  -- active, archived, migrating
  
  -- Provider configuration (JSONB for flexibility)
  provider_type VARCHAR(50) NOT NULL,        -- local, github, google, hermes
  provider_config JSONB NOT NULL,            -- Provider-specific settings
  
  -- Metadata
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  config_hash VARCHAR(64),                   -- SHA-256 of project config (detect changes)
  
  CONSTRAINT chk_provider_type CHECK (provider_type IN ('local', 'github', 'google', 'hermes'))
);

CREATE INDEX idx_projects_project_id ON projects(project_id);
CREATE INDEX idx_projects_status ON projects(status);
```

**Example Records**:
```sql
-- Local project
INSERT INTO projects (project_id, short_name, description, status, provider_type, provider_config) VALUES
('docs-internal', 'DOCS', 'Internal documentation', 'active', 'local', 
 '{"root": "./docs-internal", "folders": {"docs": ".", "drafts": ".drafts"}}');

-- GitHub project
INSERT INTO projects (project_id, short_name, description, status, provider_type, provider_config) VALUES
('hashicorp/hermes/docs', 'HER-DOCS', 'Hermes documentation', 'active', 'github',
 '{"repository": "hashicorp/hermes", "branch": "main", "path": "docs", "auth": "token"}');

-- Remote Hermes project
INSERT INTO projects (project_id, short_name, description, status, provider_type, provider_config) VALUES
('hermes.example.com/engineering', 'ENG-REMOTE', 'Remote engineering docs', 'active', 'hermes',
 '{"endpoint": "https://hermes.example.com", "workspace_id": "engineering", "auth": "oidc"}');
```

### Document Revisions Table

**Add project_id** for tracking revisions across projects during migration:

```sql
CREATE TABLE IF NOT EXISTS document_revisions (
  id SERIAL PRIMARY KEY,
  document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  project_id VARCHAR(255) NOT NULL,  -- NEW: Which project owns this revision
  content_hash VARCHAR(255) NOT NULL,
  revision_reference VARCHAR(255),    -- Git commit, version number, etc.
  commit_sha VARCHAR(255),            -- For git-based providers
  content_length BIGINT,
  content_type VARCHAR(100),
  summary TEXT,
  modified_by VARCHAR(255),
  modified_at TIMESTAMP,
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  
  -- Allow multiple revisions per document (migration scenario)
  UNIQUE(document_id, project_id, content_hash)
);

CREATE INDEX idx_document_revisions_document_id ON document_revisions(document_id);
CREATE INDEX idx_document_revisions_project_id ON document_revisions(project_id);
CREATE INDEX idx_document_revisions_content_hash ON document_revisions(content_hash);
```

## Updated API Endpoints

### 1. Create/Update Document

**POST /api/v2/indexer/documents**

**Request**:
```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "project_id": "docs-internal",
  "provider_document_id": "docs-internal/RFC-001.md",
  "title": "RFC-001: Local Workspace Provider",
  "doc_type": "RFC",
  "doc_number": "RFC-001",
  "product": "Engineering",
  "status": "In Review",
  "summary": "Design document for local filesystem workspace support",
  "owners": ["user@example.com"],
  "contributors": ["contributor@example.com"],
  "approvers": ["approver@example.com"],
  "tags": ["indexer", "workspace"],
  "custom_fields": [
    {"name": "priority", "type": "string", "value": "high"}
  ],
  "metadata": {
    "source": "indexer",
    "indexed_at": "2025-10-23T10:00:00Z"
  }
}
```

**Response**:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "project_id": "docs-internal",
  "provider_document_id": "docs-internal/RFC-001.md",
  "created": true,
  "updated_fields": [],
  "created_at": "2025-10-23T10:00:00Z",
  "updated_at": "2025-10-23T10:00:00Z"
}
```

### 2. Create Revision

**POST /api/v2/indexer/documents/:uuid/revisions**

**Request**:
```json
{
  "project_id": "docs-internal",
  "content_hash": "sha256:abc123def456789...",
  "revision_reference": "v1.2.3",
  "commit_sha": "abc123def456",
  "content_length": 15847,
  "content_type": "text/markdown",
  "summary": "Added implementation details",
  "modified_by": "user@example.com",
  "modified_at": "2025-10-23T10:00:00Z",
  "metadata": {
    "indexer_version": "2.0.0",
    "processing_time_ms": 1234
  }
}
```

**Response**:
```json
{
  "id": 42,
  "document_uuid": "550e8400-e29b-41d4-a716-446655440000",
  "project_id": "docs-internal",
  "content_hash": "sha256:abc123def456789...",
  "revision_reference": "v1.2.3",
  "is_duplicate": false,
  "created_at": "2025-10-23T10:00:00Z"
}
```

### 3. Project Management (New Endpoints)

**POST /api/v2/indexer/projects** - Register project from config

**Request**:
```json
{
  "project_id": "docs-internal",
  "short_name": "DOCS",
  "description": "Internal documentation",
  "status": "active",
  "provider_type": "local",
  "provider_config": {
    "root": "./docs-internal",
    "folders": {
      "docs": ".",
      "drafts": ".drafts"
    }
  }
}
```

**GET /api/v2/indexer/projects/:project_id** - Get project info

**Response**:
```json
{
  "project_id": "docs-internal",
  "short_name": "DOCS",
  "description": "Internal documentation",
  "status": "active",
  "provider_type": "local",
  "provider_config": {
    "root": "./docs-internal",
    "folders": {"docs": ".", "drafts": ".drafts"}
  },
  "document_count": 102,
  "created_at": "2025-10-22T08:00:00Z",
  "updated_at": "2025-10-23T10:00:00Z"
}
```

## Globally Unique Project IDs

### Format Convention

**Project ID Format**: `[owner/]project-name[/subpath]`

**Examples**:
- `docs-internal` - Local project (testing)
- `hashicorp/hermes/docs` - GitHub project with subpath
- `hermes.example.com/engineering` - Remote Hermes instance
- `google-workspace/rfc-archive` - Google Workspace project

**Rules**:
- Must be globally unique within Hermes instance
- Case-insensitive for lookups
- No spaces (use hyphens or underscores)
- Max 255 characters
- Can include slashes for hierarchical organization

### Project ID Resolution

```go
// Load project config
cfg, err := projectconfig.LoadConfig("testing/projects.hcl")

// Register projects with API (one-time setup)
for _, project := range cfg.Projects {
    req := &RegisterProjectRequest{
        ProjectID:      project.ID,          // "docs-internal"
        ShortName:      project.ShortName,   // "DOCS"
        Description:    project.Description,
        Status:         project.Status,
        ProviderType:   project.Workspace.Type,
        ProviderConfig: project.Workspace.ToJSON(),
    }
    apiClient.RegisterProject(ctx, req)
}

// Index documents (use registered project ID)
for _, doc := range documents {
    req := &CreateDocumentRequest{
        UUID:               doc.UUID,
        ProjectID:          "docs-internal",  // References project
        ProviderDocumentID: doc.Path,          // Provider-specific ID
        Title:              doc.Title,
        // ... other fields
    }
    apiClient.CreateDocument(ctx, req)
}
```

## Migration Support

During migration from one provider to another, documents can have **multiple revisions** with different `project_id` values:

```sql
-- Document has 2 active revisions (migration in progress)
SELECT 
  dr.id,
  dr.project_id,
  p.provider_type,
  dr.content_hash,
  dr.modified_at
FROM document_revisions dr
JOIN projects p ON p.project_id = dr.project_id
WHERE dr.document_id = (
  SELECT id FROM documents WHERE uuid = '550e8400-...'
);

-- Results:
-- id | project_id      | provider_type | content_hash | modified_at
-- 41 | google-rfc-old  | google        | sha256:abc.. | 2025-10-01
-- 42 | local-rfc-new   | local         | sha256:abc.. | 2025-10-01  (same hash - migrated)
```

**Conflict Detection**:
```sql
-- Find documents with conflicting revisions (different hashes in same time window)
SELECT 
  d.uuid,
  d.title,
  COUNT(DISTINCT dr.content_hash) as hash_count,
  MAX(dr.modified_at) as last_modified
FROM documents d
JOIN document_revisions dr ON dr.document_id = d.id
WHERE dr.modified_at > NOW() - INTERVAL '7 days'
GROUP BY d.uuid, d.title
HAVING COUNT(DISTINCT dr.content_hash) > 1;
```

## Updated Integration Test Flow

```go
func TestFullPipelineWithProject(t *testing.T) {
    // 1. Load project config
    cfg, err := projectconfig.LoadConfig("testing/projects.hcl")
    require.NoError(t, err)
    
    project := cfg.GetProject("docs-internal")
    require.NotNil(t, project)
    
    // 2. Register project with API (one-time setup)
    apiClient := NewIndexerAPIClient("http://localhost:8001", authToken)
    
    projectResp, err := apiClient.RegisterProject(ctx, &RegisterProjectRequest{
        ProjectID:      project.ID,
        ShortName:      project.ShortName,
        ProviderType:   project.Workspace.Type,
        ProviderConfig: project.Workspace.ToJSON(),
    })
    require.NoError(t, err)
    
    // 3. Discover documents from project workspace
    provider, err := workspace.NewProvider(project.Workspace)
    require.NoError(t, err)
    
    docs, err := provider.ListDocuments(ctx, project.Workspace.Folders.Docs, nil)
    require.NoError(t, err)
    
    // 4. Process each document
    for _, doc := range docs {
        // Create document via API
        docResp, err := apiClient.CreateDocument(ctx, &CreateDocumentRequest{
            UUID:               doc.UUID,
            ProjectID:          project.ID,          // Reference project
            ProviderDocumentID: doc.Path,            // Provider-specific ID
            Title:              doc.Name,
            // ... other fields
        })
        require.NoError(t, err)
        
        // Create revision
        revResp, err := apiClient.CreateRevision(ctx, doc.UUID, &CreateRevisionRequest{
            ProjectID:   project.ID,  // Include project context
            ContentHash: doc.ContentHash,
            CommitSHA:   getGitCommitSHA(doc.Path),  // If git-based
            // ... other fields
        })
        require.NoError(t, err)
        
        // Generate and save summary
        summary := generateSummary(doc.Content)
        _, err = apiClient.UpdateSummary(ctx, doc.UUID, &UpdateSummaryRequest{
            Summary:     summary,
            RevisionID:  revResp.ID,
            Model:       "llama3.2",
            GeneratedAt: time.Now(),
        })
        require.NoError(t, err)
    }
}
```

## Benefits of Project-Based Normalization

### 1. Configuration Management
✅ **Single source of truth**: Provider config lives in project HCL files  
✅ **Easy updates**: Change provider settings in one place (project config)  
✅ **Version control**: Project configs tracked in git  
✅ **Testing**: Easy to switch between test/prod configs

### 2. Migration Support
✅ **Track migrations**: Multiple revisions per document with different project IDs  
✅ **Conflict detection**: Compare content hashes across projects  
✅ **Gradual migration**: Documents can exist in multiple projects simultaneously  
✅ **Rollback**: Keep old project config, re-index if needed

### 3. Data Consistency
✅ **No duplication**: Provider data stored once in projects table  
✅ **Atomic updates**: Update project config, all documents use new settings  
✅ **Smaller database**: No JSON blobs in every document record  
✅ **Referential integrity**: Foreign key from documents to projects

### 4. Query Performance
✅ **Fast project lookups**: Index on project_id  
✅ **Provider resolution**: Join documents → projects → get provider config  
✅ **Batch operations**: Index all documents in project with single config read  
✅ **Aggregations**: Count documents per project, provider type, etc.

## Summary of Changes

### Database Schema
- **Added**: `projects` table with provider configuration
- **Changed**: `documents.project_id` + `documents.provider_document_id` instead of denormalized workspace metadata
- **Changed**: `document_revisions.project_id` for migration tracking
- **Removed**: `documents.workspace_provider_type`, `documents.workspace_provider_metadata` (no longer needed)

### API Endpoints
- **Changed**: Document creation requires `project_id` + `provider_document_id`
- **Changed**: Revision creation includes `project_id`
- **Added**: Project registration endpoints (POST/GET /api/v2/indexer/projects)
- **Simplified**: No more inline workspace provider metadata

### Integration Test
- **Added**: Project registration step (one-time setup)
- **Changed**: Discovery uses project workspace configuration
- **Changed**: Document creation includes project context
- **Simplified**: No need to build workspace provider metadata per document

## Next Steps

1. Update `INDEXER_API_IMPLEMENTATION_CHECKLIST.md` with project-based approach
2. Revise database migrations to use projects table
3. Update API handler implementations
4. Modify integration test to register project first
5. Test project-based indexing flow

**This design is cleaner, more scalable, and aligns with the Distributed Projects Architecture.**
