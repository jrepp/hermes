# Indexer API Implementation Checklist

**Status**: 📋 Ready for Development  
**Estimated Time**: 8-12 hours  
**Priority**: High

## Phase 1: API Implementation (4-5 hours)

### Task 1.1: Create API Handler File
**File**: `internal/api/v2/indexer.go`
**Time**: 2 hours

- [ ] Create `IndexerDocumentsHandler` function
  - [ ] Handle POST for create/upsert documents
  - [ ] Handle GET for document lookup by UUID
  - [ ] Parse workspace provider metadata
  - [ ] Validate request schema
  - [ ] Return proper status codes (201, 200, 400, 404, 409)

- [ ] Create `IndexerRevisionsHandler` function
  - [ ] Handle POST for creating revisions
  - [ ] Validate content hash format
  - [ ] Check for duplicate revisions (by content hash)
  - [ ] Link revision to document by UUID
  - [ ] Return revision ID and duplicate status

- [ ] Create `IndexerSummaryHandler` function
  - [ ] Handle PUT for updating AI summaries
  - [ ] Validate revision exists
  - [ ] Store summary with model metadata
  - [ ] Handle content hash mismatch (409 Conflict)

- [ ] Create `IndexerEmbeddingsHandler` function
  - [ ] Handle PUT for storing vector embeddings
  - [ ] Validate dimensions match declaration
  - [ ] Support chunked embeddings (multiple per document)
  - [ ] Link to revision by ID

### Task 1.2: Request/Response Types
**File**: `internal/api/v2/indexer.go`
**Time**: 1 hour

```go
type CreateDocumentRequest struct {
    UUID               string                      `json:"uuid"`
    ProjectID          string                      `json:"project_id"`           // References project
    ProviderDocumentID string                      `json:"provider_document_id"` // Provider-specific ID
    Title              string                      `json:"title"`
    DocType            string                      `json:"doc_type"`
    DocNumber          string                      `json:"doc_number,omitempty"`
    Product            string                      `json:"product"`
    Status             string                      `json:"status"`
    Summary            string                      `json:"summary,omitempty"`
    Owners             []string                    `json:"owners"`
    Contributors       []string                    `json:"contributors,omitempty"`
    Approvers          []string                    `json:"approvers,omitempty"`
    Tags               []string                    `json:"tags,omitempty"`
    CustomFields       []document.CustomField      `json:"custom_fields,omitempty"`
    Metadata           map[string]interface{}      `json:"metadata,omitempty"`
}

type RegisterProjectRequest struct {
    ProjectID      string                 `json:"project_id"`      // Globally unique
    ShortName      string                 `json:"short_name"`
    Description    string                 `json:"description"`
    Status         string                 `json:"status"`          // active, archived
    ProviderType   string                 `json:"provider_type"`   // local, github, google, hermes
    ProviderConfig map[string]interface{} `json:"provider_config"` // Provider-specific settings
}

type CreateRevisionRequest struct {
    ProjectID         string                 `json:"project_id"`              // Which project owns this revision
    ContentHash       string                 `json:"content_hash"`
    RevisionReference string                 `json:"revision_reference,omitempty"`
    CommitSHA         string                 `json:"commit_sha,omitempty"`
    ContentLength     int64                  `json:"content_length,omitempty"`
    ContentType       string                 `json:"content_type,omitempty"`
    Summary           string                 `json:"summary,omitempty"`
    ModifiedBy        string                 `json:"modified_by,omitempty"`
    ModifiedAt        time.Time              `json:"modified_at,omitempty"`
    Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type UpdateSummaryRequest struct {
    Summary      string                 `json:"summary"`
    RevisionID   int                    `json:"revision_id,omitempty"`
    ContentHash  string                 `json:"content_hash,omitempty"`
    Model        string                 `json:"model"`
    ModelVersion string                 `json:"model_version,omitempty"`
    GeneratedAt  time.Time              `json:"generated_at"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ... (more types)
```

### Task 1.3: Database Schema Migration
**File**: `internal/db/migrations/XXX_add_indexer_fields.sql`
**Time**: 45 minutes

- [ ] Create `projects` table
  - [ ] `project_id VARCHAR(255) UNIQUE` - Globally unique ID
  - [ ] `short_name`, `description`, `status`
  - [ ] `provider_type VARCHAR(50)` - local, github, google, hermes
  - [ ] `provider_config JSONB` - Provider-specific settings
  - [ ] `config_hash VARCHAR(64)` - Detect config changes
  - [ ] Indexes on project_id, status

- [ ] Add fields to `documents` table
  - [ ] `project_id VARCHAR(255) NOT NULL` - References projects
  - [ ] `provider_document_id VARCHAR(255) NOT NULL` - Provider-specific ID
  - [ ] `indexed_at TIMESTAMP`
  - [ ] `indexer_version VARCHAR(50)`
  - [ ] Index on project_id, provider_document_id
  - [ ] Unique index on (project_id, provider_document_id, uuid)

- [ ] Create `document_revisions` table
  - [ ] Primary key, foreign key to documents
  - [ ] `project_id VARCHAR(255) NOT NULL` - Track project ownership
  - [ ] content_hash, revision_reference, commit_sha
  - [ ] summary, modified_by, modified_at
  - [ ] metadata JSONB
  - [ ] UNIQUE constraint on (document_id, project_id, content_hash)
  - [ ] Indexes on document_id, project_id, content_hash, modified_at

- [ ] Create `document_embeddings` table
  - [ ] Primary key, foreign keys to documents and revisions
  - [ ] model, model_version, dimensions
  - [ ] embeddings vector(768) -- requires pgvector
  - [ ] chunk_metadata JSONB
  - [ ] Indexes

### Task 1.4: Register Routes
**File**: `internal/server/server.go`
**Time**: 30 minutes

```go
// Add to router setup
router.Handle("/api/v2/indexer/documents", 
    middleware.Auth(api.IndexerDocumentsHandler(srv)))

router.Handle("/api/v2/indexer/documents/{uuid}/revisions",
    middleware.Auth(api.IndexerRevisionsHandler(srv)))

router.Handle("/api/v2/indexer/documents/{uuid}/summary",
    middleware.Auth(api.IndexerSummaryHandler(srv)))

router.Handle("/api/v2/indexer/documents/{uuid}/embeddings",
    middleware.Auth(api.IndexerEmbeddingsHandler(srv)))
```

### Task 1.5: Update Models
**File**: `pkg/models/document.go`, `pkg/models/project.go` (new)
**Time**: 1.5 hours

- [ ] Create Project model (new file: `pkg/models/project.go`)
```go
type Project struct {
    gorm.Model
    ProjectID      string         `gorm:"uniqueIndex;not null;size:255"`
    ShortName      string         `gorm:"not null;size:50"`
    Description    string         `gorm:"type:text"`
    Status         string         `gorm:"not null;default:active;size:50"`
    ProviderType   string         `gorm:"not null;size:50"`
    ProviderConfig datatypes.JSON `gorm:"type:jsonb;not null"`
    ConfigHash     string         `gorm:"size:64"`
}
```

- [ ] Add fields to Document model
```go
type Document struct {
    // ... existing fields
    ProjectID          string         `gorm:"not null;size:255;index"`
    ProviderDocumentID string         `gorm:"not null;size:255;index"`
    IndexedAt          *time.Time     `gorm:"column:indexed_at"`
    IndexerVersion     string         `gorm:"column:indexer_version;size:50"`
}
```

- [ ] Create DocumentRevision model
```go
type DocumentRevision struct {
    gorm.Model
    DocumentID        uint           `gorm:"not null"`
    Document          Document       `gorm:"foreignKey:DocumentID"`
    ProjectID         string         `gorm:"not null;size:255;index"` // NEW
    ContentHash       string         `gorm:"not null;size:255"`
    RevisionReference string         `gorm:"size:255"`
    CommitSHA         string         `gorm:"size:255"`
    ContentLength     int64
    ContentType       string         `gorm:"size:100"`
    Summary           string         `gorm:"type:text"`
    ModifiedBy        string         `gorm:"size:255"`
    ModifiedAt        *time.Time
    Metadata          datatypes.JSON `gorm:"type:jsonb"`
}
```

- [ ] Create DocumentEmbedding model

## Phase 2: API Client Implementation (2-3 hours)

### Task 2.1: Create API Client
**File**: `tests/integration/indexer/api_client.go`
**Time**: 2 hours

```go
type IndexerAPIClient struct {
    BaseURL    string
    HTTPClient *http.Client
    AuthToken  string
}

func NewIndexerAPIClient(baseURL, authToken string) *IndexerAPIClient {
    return &IndexerAPIClient{
        BaseURL:    baseURL,
        HTTPClient: &http.Client{Timeout: 30 * time.Second},
        AuthToken:  authToken,
    }
}

func (c *IndexerAPIClient) CreateDocument(ctx context.Context, req *CreateDocumentRequest) (*CreateDocumentResponse, error) {
    // POST /api/v2/indexer/documents
}

func (c *IndexerAPIClient) GetDocument(ctx context.Context, uuid string) (*GetDocumentResponse, error) {
    // GET /api/v2/indexer/documents/:uuid
}

func (c *IndexerAPIClient) CreateRevision(ctx context.Context, uuid string, req *CreateRevisionRequest) (*CreateRevisionResponse, error) {
    // POST /api/v2/indexer/documents/:uuid/revisions
}

func (c *IndexerAPIClient) UpdateSummary(ctx context.Context, uuid string, req *UpdateSummaryRequest) (*UpdateSummaryResponse, error) {
    // PUT /api/v2/indexer/documents/:uuid/summary
}

func (c *IndexerAPIClient) StoreEmbeddings(ctx context.Context, uuid string, req *StoreEmbeddingsRequest) (*StoreEmbeddingsResponse, error) {
    // PUT /api/v2/indexer/documents/:uuid/embeddings
}
```

### Task 2.2: Helper Functions
**Time**: 30 minutes

- [ ] `makeRequest(method, path string, body, result interface{})` helper
- [ ] Error handling for HTTP status codes
- [ ] Request/response logging (optional, for debugging)

## Phase 3: Command Refactoring (2-3 hours)

### Task 3.1: Update TrackCommand
**File**: `tests/integration/indexer/full_pipeline_test.go`
**Time**: 1 hour

**Current**:
```go
type TrackCommand struct {
    DB *gorm.DB
}

func (c *TrackCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    model := models.Document{...}
    return model.Create(c.DB)
}
```

**New**:
```go
type TrackCommand struct {
    APIClient *IndexerAPIClient
}

func (c *TrackCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    req := &CreateDocumentRequest{
        UUID:  doc.DocumentUUID.String(),
        Title: doc.Document.Name,
        // ... map fields
        WorkspaceProvider: WorkspaceProviderMetadata{
            Type: "local",
            Path: doc.Document.Path,
        },
    }
    
    resp, err := c.APIClient.CreateDocument(ctx, req)
    if err != nil {
        return fmt.Errorf("failed to create document via API: %w", err)
    }
    
    doc.DatabaseID = resp.ID
    return nil
}
```

### Task 3.2: Update TrackRevisionCommand
**File**: `tests/integration/indexer/full_pipeline_test.go`
**Time**: 45 minutes

**Current**: Uses direct DB insert
**New**: Calls `APIClient.CreateRevision()`

### Task 3.3: Update SummarizeCommand
**File**: `tests/integration/indexer/full_pipeline_test.go`
**Time**: 45 minutes

**Current**:
```go
type SummarizeCommand struct {
    AIProvider ai.Provider
    DB         *gorm.DB
}

func (c *SummarizeCommand) Execute(...) {
    summary := c.AIProvider.Summarize(content)
    model.Summary = &summary
    model.Update(c.DB)
}
```

**New**:
```go
type SummarizeCommand struct {
    AIProvider ai.Provider
    APIClient  *IndexerAPIClient
}

func (c *SummarizeCommand) Execute(...) {
    summary := c.AIProvider.Summarize(content)
    
    req := &UpdateSummaryRequest{
        Summary:     summary,
        RevisionID:  doc.RevisionID,
        Model:       "llama3.2",
        GeneratedAt: time.Now(),
    }
    
    _, err := c.APIClient.UpdateSummary(ctx, doc.DocumentUUID.String(), req)
    return err
}
```

## Phase 4: Project Config Integration (1-2 hours)

### Task 4.1: Load Project Config
**File**: `tests/integration/indexer/full_pipeline_test.go`
**Time**: 1 hour

**Current**:
```go
docsPath := filepath.Join(repoRoot, "docs-internal")
discoverCmd := &LocalFilesystemDiscoverCommand{
    basePath: docsPath,
}
```

**New**:
```go
// Load project config
cfg, err := projectconfig.LoadConfig(
    filepath.Join(repoRoot, "testing/projects.hcl"))
require.NoError(t, err)

// Get project
project := cfg.GetProject("docs-internal")
require.NotNil(t, project)

// Use project workspace
discoverCmd := &ProjectWorkspaceDiscoverCommand{
    Project: project,
    Folders: []string{project.Workspace.Folders.Docs},
}
```

### Task 4.2: Update Discovery Command
**Time**: 1 hour

- [ ] Create `ProjectWorkspaceDiscoverCommand`
- [ ] Uses project config to resolve workspace provider
- [ ] Supports multiple workspace types (local, github, etc.)
- [ ] Returns documents with workspace provider metadata

## Phase 5: Integration Test Updates (1-2 hours)

### Task 5.1: Update Test Setup
**File**: `tests/integration/indexer/full_pipeline_test.go`
**Time**: 30 minutes

```go
func TestFullPipelineWithDocsInternal(t *testing.T) {
    // Get auth token from Dex (or use test token)
    authToken, err := getTestAuthToken()
    require.NoError(t, err)
    
    // Create API client
    apiClient := NewIndexerAPIClient(
        "http://localhost:8001",
        authToken,
    )
    
    // Load project config
    cfg, err := projectconfig.LoadConfig(...)
    require.NoError(t, err)
    
    project := cfg.GetProject("docs-internal")
    require.NotNil(t, project)
    
    // ... rest of test
}
```

### Task 5.2: Update Pipeline Commands
**Time**: 30 minutes

```go
pipeline := &indexer.Pipeline{
    Commands: []indexer.Command{
        &DiscoverCommand{Project: project},
        &AssignUUIDCommand{},
        &ExtractContentCommand{},
        &CalculateHashCommand{},
        &TrackCommand{APIClient: apiClient},          // Changed
        &TrackRevisionCommand{APIClient: apiClient},   // Changed
        &SummarizeCommand{
            AIProvider: aiProvider,
            APIClient:  apiClient,                     // Changed
        },
        &GenerateEmbeddingCommand{AIProvider: aiProvider},
        &SimpleTransformCommand{},
        &IndexCommand{SearchProvider: searchProvider},
    },
}
```

### Task 5.3: Add Verification Steps
**Time**: 30 minutes

```go
// Verify via API
for _, doc := range processedDocs {
    resp, err := apiClient.GetDocument(ctx, doc.UUID)
    require.NoError(t, err)
    
    assert.Equal(t, doc.Title, resp.Title)
    assert.NotNil(t, resp.LatestRevision)
    assert.NotEmpty(t, resp.LatestRevision.Summary)
}

// Also verify in database (for integration test)
var dbDoc models.Document
err = testDB.Where("uuid = ?", doc.UUID).First(&dbDoc).Error
require.NoError(t, err)
```

## Phase 6: Testing & Validation (2-3 hours)

### Task 6.1: Unit Tests for API Handlers
**File**: `internal/api/v2/indexer_test.go`
**Time**: 1.5 hours

- [ ] Test CreateDocument happy path
- [ ] Test CreateDocument duplicate UUID (409)
- [ ] Test CreateRevision with valid data
- [ ] Test CreateRevision duplicate content hash (returns existing)
- [ ] Test UpdateSummary with valid revision
- [ ] Test UpdateSummary with content hash mismatch (409)
- [ ] Test authentication failures (401)

### Task 6.2: Integration Test Execution
**Time**: 1 hour

```bash
# Start testing environment
cd testing
make up

# Wait for services
sleep 10

# Run integration test
cd ..
HERMES_REPO_ROOT=$(pwd) go test -tags=integration -v -timeout=30m \
    ./tests/integration/indexer -run TestFullPipelineWithDocsInternal
```

**Expected Results**:
- [ ] All documents discovered from project workspace
- [ ] Documents created via API (check logs)
- [ ] Revisions created with content hashes
- [ ] Summaries stored and tied to revisions
- [ ] Meilisearch index updated
- [ ] No direct DB calls in indexer commands

### Task 6.3: End-to-End Verification
**Time**: 30 minutes

**Database**:
```sql
-- Check documents created
SELECT uuid, title, workspace_provider_type, indexed_at 
FROM documents 
WHERE workspace_provider_type = 'local'
LIMIT 10;

-- Check revisions
SELECT dr.id, d.uuid, dr.content_hash, dr.summary IS NOT NULL as has_summary
FROM document_revisions dr
JOIN documents d ON d.id = dr.document_id
LIMIT 10;
```

**API**:
```bash
# Get auth token
TOKEN=$(./scripts/get-test-token.sh)

# Query document via API
curl -H "Authorization: Bearer $TOKEN" \
    http://localhost:8001/api/v2/indexer/documents/<uuid>
```

**Meilisearch**:
```bash
# Check indexed documents
curl http://localhost:7701/indexes/docs_test/documents?limit=5 \
    -H "Authorization: Bearer masterKey123"
```

## Success Criteria

- [ ] All API endpoints return correct status codes
- [ ] Documents created with workspace provider metadata
- [ ] Revisions linked to documents by UUID
- [ ] Summaries tied to specific revisions
- [ ] Duplicate content hash detection works
- [ ] Project config resolves workspace correctly
- [ ] Integration test passes end-to-end
- [ ] No direct database access in indexer commands
- [ ] API authentication enforced
- [ ] Database migrations applied successfully

## Time Estimate Summary

| Phase | Task | Time |
|-------|------|------|
| 1 | API Implementation | 4-5 hours |
| 2 | API Client | 2-3 hours |
| 3 | Command Refactoring | 2-3 hours |
| 4 | Project Config Integration | 1-2 hours |
| 5 | Test Updates | 1-2 hours |
| 6 | Testing & Validation | 2-3 hours |
| **Total** | | **12-18 hours** |

## Next Steps

1. Start with Phase 1: Implement API handlers
2. Create database migrations
3. Add models for document_revisions
4. Build API client in test code
5. Refactor commands one at a time
6. Test continuously

**Ready to begin?** Start with `internal/api/v2/indexer.go`
