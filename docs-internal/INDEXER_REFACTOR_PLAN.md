# Indexer Refactor Plan: Command Pattern & Provider Architecture

## Executive Summary

Refactor the Hermes indexer from a Google Workspace-specific service into a provider-agnostic document processing pipeline using the Command pattern and visitor pattern. This will enable:

1. **Multi-provider support**: Work with Google Workspace, Local filesystem, and future providers
2. **Document migration**: Move documents between providers (e.g., Google → Local for testing)
3. **Local integration testing**: Run full indexer validation in `./testing` infrastructure
4. **Composable operations**: Chain document processing commands (index, migrate, transform, validate)

## Current Architecture (Problems)

### Hard Dependencies on Google Workspace

```go
// internal/indexer/indexer.go
type Indexer struct {
    GoogleWorkspaceService *gw.Service  // ❌ Hardcoded to Google
    AlgoliaClient *algolia.Client       // ❌ Hardcoded to Algolia
    DocumentsFolderID string             // ❌ Google Drive specific
    DraftsFolderID string                // ❌ Google Drive specific
}

func (idx *Indexer) Run() error {
    // ❌ Direct calls to Google Drive API
    docFiles, err := gwSvc.GetUpdatedDocsBetween(
        idx.DocumentsFolderID, lastIndexedAtStr, currentTimeStr)
    
    // ❌ Direct export from Google Docs
    exp, err := gwSvc.Drive.Files.Export(file.Id, "text/plain").Download()
}
```

### Monolithic Processing Loop

- Single 60-second loop that does everything
- No separation of concerns (discovery, transformation, indexing)
- No way to test individual operations
- No way to chain or compose operations

### Inability to Test Locally

- Requires live Google Workspace connection
- Can't validate indexer behavior in `./testing` environment
- No way to create reproducible test scenarios

## New Architecture: Command Pipeline

### Core Abstraction Layers

```
┌─────────────────────────────────────────────────────────────┐
│                     Indexer Orchestrator                     │
│  (Schedules runs, manages state, executes pipelines)        │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                  Document Pipeline                           │
│  (Chain of commands that process documents)                  │
│                                                              │
│  [Discover] → [Transform] → [Index] → [Notify]              │
└───────────────────────┬─────────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│   Workspace  │ │    Search    │ │   Database   │
│   Provider   │ │   Provider   │ │   (GORM)     │
│              │ │              │ │              │
│ • Google     │ │ • Algolia    │ │ • Tracking   │
│ • Local      │ │ • Meilisearch│ │ • Metadata   │
│ • Mock       │ │ • Mock       │ │              │
└──────────────┘ └──────────────┘ └──────────────┘
```

## Design Patterns

### 1. Command Pattern

Each indexer operation becomes a command that can be executed, undone, and chained:

```go
// Command interface for document operations
type Command interface {
    Execute(ctx context.Context, doc *DocumentContext) error
    Name() string
}

// Example commands
type DiscoverDocumentsCommand struct {
    provider workspace.DocumentStorage
    folderID string
    since    time.Time
}

type ExtractContentCommand struct {
    provider workspace.DocumentStorage
    maxSize  int
}

type IndexDocumentCommand struct {
    searchProvider search.Provider
    indexType      IndexType // docs, drafts
}

type UpdateHeaderCommand struct {
    provider     workspace.DocumentStorage
    documentTypes []*config.DocumentType
}

type MigrateDocumentCommand struct {
    source workspace.DocumentStorage
    target workspace.DocumentStorage
}
```

### 2. Pipeline Pattern

Commands are composed into pipelines for different operations:

```go
type Pipeline struct {
    name     string
    commands []Command
    filter   DocumentFilter // Skip certain documents
}

// Standard pipelines
var (
    IndexPublishedPipeline = Pipeline{
        name: "index-published",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &ExtractContentCommand{},
            &TransformToSearchDocCommand{},
            &IndexDocumentCommand{},
            &UpdateTrackingCommand{},
        },
    }
    
    RefreshHeadersPipeline = Pipeline{
        name: "refresh-headers",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &LoadMetadataCommand{},
            &UpdateHeaderCommand{},
            &UpdateTrackingCommand{},
        },
        filter: RecentlyModifiedFilter(30 * time.Minute),
    }
    
    MigrationPipeline = Pipeline{
        name: "migrate",
        commands: []Command{
            &DiscoverDocumentsCommand{},
            &ExtractContentCommand{},
            &LoadMetadataCommand{},
            &MigrateDocumentCommand{},
            &IndexDocumentCommand{}, // Index in target
            &UpdateTrackingCommand{},
        },
    }
)
```

### 3. Document Visitor Pattern

Process documents without knowing their concrete provider:

```go
type DocumentVisitor interface {
    VisitDocument(ctx context.Context, doc *workspace.Document) error
}

type DocumentContext struct {
    // Source document from provider
    Document *workspace.Document
    
    // Hermes-specific metadata
    Metadata *models.Document
    Reviews  models.DocumentReviews
    GroupReviews models.DocumentGroupReviews
    
    // Processing state
    Content string
    Transformed *document.Document // For indexing
    
    // Provider information
    SourceProvider workspace.DocumentStorage
    TargetProvider workspace.DocumentStorage
    
    // Tracking
    StartTime time.Time
    Errors    []error
}
```

## New Package Structure

```
pkg/
  indexer/                    # New provider-agnostic indexer
    command.go                # Command interface
    pipeline.go               # Pipeline composition
    context.go                # DocumentContext
    orchestrator.go           # Main orchestrator
    
    commands/                 # Individual commands
      discover.go             # Discover documents in a provider
      extract.go              # Extract content
      transform.go            # Transform to search format
      index.go                # Index in search provider
      header.go               # Update headers
      migrate.go              # Migrate between providers
      tracking.go             # Update database tracking
      
    filters/                  # Document filters
      time.go                 # Time-based filtering
      type.go                 # Document type filtering
      status.go               # Status filtering
      
    config.go                 # Configuration structures
    
internal/
  indexer/                    # Legacy - will be deprecated
    [current files]
    
  cmd/
    commands/
      indexer/
        indexer.go            # Updated CLI using new pkg/indexer
        
testing/
  indexer/                    # Local testing infrastructure
    test-data/                # Sample documents
      docs/
        RFC-001.md
        PRD-002.md
      drafts/
        DRAFT-003.md
    fixtures.go               # Test data generation
    integration_test.go       # Full integration tests
```

## Implementation Phases

### Phase 1: Core Abstractions (Week 1)

**Goal**: Create command interfaces and basic pipeline

```go
// pkg/indexer/command.go
type Command interface {
    Execute(ctx context.Context, doc *DocumentContext) error
    Name() string
}

// Optional: Commands that can process multiple documents efficiently
type BatchCommand interface {
    Command
    ExecuteBatch(ctx context.Context, docs []*DocumentContext) error
}

// pkg/indexer/pipeline.go
type Pipeline struct {
    name     string
    commands []Command
}

func (p *Pipeline) Execute(ctx context.Context, docs []*DocumentContext) error

// pkg/indexer/context.go
type DocumentContext struct {
    Document *workspace.Document
    Metadata *models.Document
    Content  string
    // ...
}
```

**Deliverables**:
- [ ] `pkg/indexer/command.go` - Command interface (no rollback needed)
- [ ] `pkg/indexer/pipeline.go` - Pipeline execution
- [ ] `pkg/indexer/context.go` - Document context
- [ ] Unit tests for pipeline execution

### Phase 2: Basic Commands (Week 2)

**Goal**: Implement essential commands

```go
// pkg/indexer/commands/discover.go
type DiscoverCommand struct {
    Provider  workspace.DocumentStorage
    FolderID  string
    Since     time.Time
    Until     time.Time
}

func (c *DiscoverCommand) Execute(ctx context.Context, _ *DocumentContext) ([]*DocumentContext, error) {
    opts := &workspace.ListOptions{
        ModifiedAfter: &c.Since,
    }
    docs, err := c.Provider.ListDocuments(ctx, c.FolderID, opts)
    // Convert to DocumentContext slice
}

// pkg/indexer/commands/extract.go
type ExtractContentCommand struct {
    Provider workspace.DocumentStorage
    MaxSize  int
}

func (c *ExtractContentCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    content, err := c.Provider.GetDocumentContent(ctx, doc.Document.ID)
    if len(content) > c.MaxSize {
        content = content[:c.MaxSize]
    }
    doc.Content = content
    return nil
}
```

**Deliverables**:
- [ ] `pkg/indexer/commands/discover.go` - Document discovery
- [ ] `pkg/indexer/commands/extract.go` - Content extraction
- [ ] `pkg/indexer/commands/transform.go` - Transform to search format
- [ ] `pkg/indexer/commands/index.go` - Index in search provider
- [ ] Integration tests with mock providers

### Phase 3: Orchestrator (Week 3)

**Goal**: Create the main orchestrator that replaces current `Indexer.Run()`

```go
// pkg/indexer/orchestrator.go
type Orchestrator struct {
    db             *gorm.DB
    logger         hclog.Logger
    
    // Providers
    workspaceProvider workspace.StorageProvider
    searchProvider    search.Provider
    
    // Configuration
    config *Config
    
    // Pipelines
    pipelines map[string]*Pipeline
}

func (o *Orchestrator) Run(ctx context.Context) error {
    ticker := time.NewTicker(o.config.RunInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := o.runCycle(ctx); err != nil {
                o.logger.Error("indexer cycle failed", "error", err)
            }
        }
    }
}

func (o *Orchestrator) runCycle(ctx context.Context) error {
    // Get last run metadata
    md := models.IndexerMetadata{}
    if err := md.Get(o.db); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
        return err
    }
    
    // Execute configured pipelines
    for _, pipelineName := range o.config.EnabledPipelines {
        pipeline := o.pipelines[pipelineName]
        if err := o.executePipeline(ctx, pipeline); err != nil {
            return fmt.Errorf("pipeline %s failed: %w", pipelineName, err)
        }
    }
    
    return nil
}
```

**Deliverables**:
- [ ] `pkg/indexer/orchestrator.go` - Main orchestration logic
- [ ] `pkg/indexer/config.go` - Configuration structures
- [ ] Pipeline registration and discovery
- [ ] Integration with existing database tracking

### Phase 4: Migration Commands (Week 4)

**Goal**: Enable document migration between providers

```go
// pkg/indexer/commands/migrate.go
type MigrateCommand struct {
    Source workspace.DocumentStorage
    Target workspace.DocumentStorage
    
    // Options
    SkipExisting bool
    DryRun       bool
}

func (c *MigrateCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Check if document exists in target
    if c.SkipExisting {
        existing, err := c.Target.GetDocument(ctx, doc.Document.ID)
        if err == nil && existing != nil {
            return nil // Skip
        }
    }
    
    if c.DryRun {
        log.Info("would migrate", "id", doc.Document.ID, "name", doc.Document.Name)
        return nil
    }
    
    // Create document in target
    createOpts := &workspace.DocumentCreate{
        Name:           doc.Document.Name,
        ParentFolderID: doc.TargetFolderID,
        Content:        doc.Content,
        Owner:          doc.Document.Owner,
        Metadata:       doc.Document.Metadata,
    }
    
    targetDoc, err := c.Target.CreateDocument(ctx, createOpts)
    if err != nil {
        return fmt.Errorf("failed to create document in target: %w", err)
    }
    
    doc.TargetDocument = targetDoc
    return nil
}
```

**Deliverables**:
- [ ] `pkg/indexer/commands/migrate.go` - Document migration
- [ ] Support for dry-run mode
- [ ] Conflict resolution strategies
- [ ] Migration progress tracking

### Phase 5: Local Testing Infrastructure (Week 5)

**Goal**: Enable full integration testing with local workspace

```go
// testing/indexer/integration_test.go
func TestLocalIndexerIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Setup local workspace
    localProvider := setupLocalWorkspace(t)
    
    // Create test documents
    docs := createTestDocuments(t, localProvider)
    
    // Setup search provider (Meilisearch in docker)
    searchProvider := setupSearchProvider(t)
    
    // Setup database
    db := setupDatabase(t)
    
    // Create orchestrator
    orchestrator := indexer.NewOrchestrator(
        indexer.WithDatabase(db),
        indexer.WithWorkspaceProvider(localProvider),
        indexer.WithSearchProvider(searchProvider),
        indexer.WithLogger(testLogger),
        indexer.WithConfig(&indexer.Config{
            RunInterval: 5 * time.Second,
            EnabledPipelines: []string{"index-published"},
            MaxParallelDocs: 2,
        }),
    )
    
    // Run one cycle
    err := orchestrator.ExecutePipeline(ctx, "index-published")
    require.NoError(t, err)
    
    // Verify documents are indexed
    for _, doc := range docs {
        result, err := searchProvider.DocumentIndex().GetObject(ctx, doc.ID)
        require.NoError(t, err)
        assert.Equal(t, doc.Name, result.Title)
    }
}

// testing/indexer/fixtures.go
func setupLocalWorkspace(t *testing.T) workspace.StorageProvider {
    basePath := t.TempDir()
    cfg := &local.Config{
        BasePath:   basePath,
        DocsPath:   filepath.Join(basePath, "docs"),
        DraftsPath: filepath.Join(basePath, "drafts"),
    }
    
    adapter, err := local.NewAdapter(cfg)
    require.NoError(t, err)
    return adapter
}

func createTestDocuments(t *testing.T, provider workspace.StorageProvider) []*workspace.Document {
    ctx := context.Background()
    docs := []*workspace.Document{}
    
    // Create RFC
    rfc, err := provider.DocumentStorage().CreateDocument(ctx, &workspace.DocumentCreate{
        Name:           "RFC-001: Test Document",
        ParentFolderID: "docs",
        Content:        "# RFC-001\n\nThis is a test RFC document.",
        Metadata: map[string]any{
            "docType":   "RFC",
            "docNumber": "RFC-001",
            "status":    "In Review",
        },
    })
    require.NoError(t, err)
    docs = append(docs, rfc)
    
    // Create PRD
    prd, err := provider.DocumentStorage().CreateDocument(ctx, &workspace.DocumentCreate{
        Name:           "PRD-002: Test Product",
        ParentFolderID: "docs",
        Content:        "# PRD-002\n\nThis is a test PRD document.",
        Metadata: map[string]any{
            "docType":   "PRD",
            "docNumber": "PRD-002",
            "status":    "Approved",
        },
    })
    require.NoError(t, err)
    docs = append(docs, prd)
    
    return docs
}
```

**Test Scenarios**:
1. **Basic indexing**: Create documents, run indexer, verify in search
2. **Incremental updates**: Modify documents, run indexer, verify changes
3. **Header refresh**: Update metadata, run refresh, verify document updated
4. **Migration**: Migrate from one local folder to another
5. **Error handling**: Simulate failures, verify recovery
6. **Concurrent processing**: Process multiple documents in parallel

**Deliverables**:
- [ ] `testing/indexer/integration_test.go` - Integration tests
- [ ] `testing/indexer/fixtures.go` - Test data generation
- [ ] `testing/indexer/test-data/` - Sample documents
- [ ] Makefile targets for running local indexer tests
- [ ] Documentation for local testing setup

### Phase 6: CLI Integration (Week 6)

**Goal**: Update CLI to support new architecture

```go
// internal/cmd/commands/indexer/indexer.go
func (c *Command) Run(args []string) int {
    // ... flag parsing ...
    
    cfg, err := config.NewConfig(c.flagConfig, "")
    if err != nil {
        ui.Error(fmt.Sprintf("error parsing configuration: %v", err))
        return 1
    }
    
    // Initialize workspace provider (based on config)
    var workspaceProvider workspace.StorageProvider
    if cfg.WorkspaceProvider == "local" {
        localCfg := &local.Config{
            BasePath:   cfg.LocalWorkspace.BasePath,
            DocsPath:   cfg.LocalWorkspace.DocsPath,
            DraftsPath: cfg.LocalWorkspace.DraftsPath,
        }
        workspaceProvider, err = local.NewAdapter(localCfg)
    } else {
        // Default to Google Workspace
        workspaceProvider = google.NewFromConfig(cfg.GoogleWorkspace.Auth)
    }
    
    // Initialize search provider
    searchProvider, err := createSearchProvider(cfg)
    if err != nil {
        ui.Error(fmt.Sprintf("error initializing search: %v", err))
        return 1
    }
    
    // Create orchestrator
    orchestrator, err := indexer.NewOrchestrator(
        indexer.WithDatabase(db),
        indexer.WithWorkspaceProvider(workspaceProvider),
        indexer.WithSearchProvider(searchProvider),
        indexer.WithLogger(log),
        indexer.WithConfig(cfg.Indexer),
    )
    
    // Run based on mode
    if c.flagMigrate {
        return c.runMigration(orchestrator)
    } else {
        return c.runContinuous(orchestrator)
    }
}
```

**New CLI Flags**:
```bash
# Run with specific plan
./hermes indexer -config=config.hcl -plan=testing/indexer/plans/local-integration-test.yaml

# Run production plan
./hermes indexer -config=config.hcl -plan=config/indexer/plans/production.yaml

# Migrate from Google to Local (using migration plan)
./hermes indexer -config=config.hcl -plan=config/indexer/plans/migrate-google-to-local.yaml

# List available plans
./hermes indexer -config=config.hcl -list-plans

# Validate plan without running
./hermes indexer -config=config.hcl -plan=testing/indexer/plans/local.yaml -validate

# Dry run mode (override plan setting)
./hermes indexer -config=config.hcl -plan=production.yaml -dry-run
```

**Deliverables**:
- [ ] Update CLI command implementation
- [ ] Add plan loading and validation
- [ ] Add plan listing functionality
- [ ] Build orchestrator from plan configuration
- [ ] Update help text and examples

## Configuration Changes

### Indexer Plans (Declarative Configuration)

Instead of hardcoded pipelines, use declarative YAML plans that define the complete indexer execution:

```yaml
# testing/indexer/plans/local-integration-test.yaml
name: local-integration-test
description: Full integration test using local workspace provider

workspace_provider: local
search_provider: meilisearch

folders:
  - id: docs
    type: published
    pipeline: index-published
  
  - id: drafts
    type: drafts
    pipeline: index-drafts

pipelines:
  - name: index-published
    description: Index published documents
    commands:
      - type: discover
        config:
          folder_id: docs
          since_last_run: true
      - type: extract
        config:
          max_size: 85000
      - type: load-metadata
        config:
          source: database
      - type: transform
        config:
          format: search-document
      - type: index
        config:
          index_type: published
          batch_size: 10
      - type: track
        config:
          update_folder_timestamp: true

execution:
  run_interval: 10s
  max_parallel_docs: 3
  dry_run: false
```

### Production Plan Example

```yaml
# config/indexer/plans/production.yaml
name: production
description: Production indexer for Google Workspace

workspace_provider: google
search_provider: algolia

folders:
  - id: "{{ .GoogleWorkspace.DocsFolder }}"
    type: published
    pipeline: index-and-refresh-published

pipelines:
  - name: index-and-refresh-published
    commands:
      - type: discover
      - type: extract
      - type: load-metadata
      - type: transform
      - type: update-header
        config:
          enabled: "{{ .Indexer.UpdateDocHeaders }}"
      - type: index
      - type: track
    filter:
      skip_recently_modified: 30m

execution:
  run_interval: 60s
  max_parallel_docs: 5
```

### Updated `config.hcl` Structure

```hcl
// Workspace provider selection
workspace_provider = "google" // or "local"

// Local workspace configuration (when provider = "local")
local_workspace {
  base_path   = "/tmp/hermes-workspace"
  docs_path   = "/tmp/hermes-workspace/docs"
  drafts_path = "/tmp/hermes-workspace/drafts"
}

// Indexer configuration
indexer {
  // Plan to use (optional, can be specified via CLI flag)
  plan = "config/indexer/plans/production.yaml"
  
  // Legacy options (for backward compatibility)
  max_parallel_docs            = 5
  update_doc_headers           = true
  update_draft_headers         = true
  use_database_for_document_data = false
}
```

## Testing Strategy

### Unit Tests

Each command is independently testable:

```go
func TestExtractContentCommand(t *testing.T) {
    mockProvider := &mock.DocumentStorage{}
    mockProvider.On("GetDocumentContent", mock.Anything, "doc-123").
        Return("Document content here", nil)
    
    cmd := &commands.ExtractContentCommand{
        Provider: mockProvider,
        MaxSize:  100,
    }
    
    doc := &indexer.DocumentContext{
        Document: &workspace.Document{ID: "doc-123"},
    }
    
    err := cmd.Execute(context.Background(), doc)
    require.NoError(t, err)
    assert.Equal(t, "Document content here", doc.Content)
}
```

### Integration Tests (in `./testing`)

```bash
# Run local integration tests
cd testing
make indexer/test

# Or via root Makefile
make test/indexer/integration
```

**Test Scenarios**:
1. Index documents from local filesystem
2. Migrate documents between providers
3. Refresh headers on local documents
4. Concurrent document processing
5. Error recovery and retry logic
6. Database tracking verification

### E2E Tests (with Docker)

```yaml
# testing/docker-compose.yml (add to existing)
services:
  indexer-test:
    build:
      context: ..
      dockerfile: testing/Dockerfile.hermes
    command: >
      indexer
      -config=/config/config.hcl
      -provider=local
    volumes:
      - ./workspace_data:/workspace
      - ./config.hcl:/config/config.hcl
    depends_on:
      - postgres
      - meilisearch
    environment:
      HERMES_INDEXER_RUN_ONCE: "true"
```

## Migration Path

### Backward Compatibility

Phase 1-3: Old indexer continues to work alongside new implementation

```go
// internal/cmd/commands/indexer/indexer.go
func (c *Command) Run(args []string) int {
    if c.flagUseLegacy {
        return c.runLegacyIndexer()
    }
    return c.runNewIndexer()
}
```

### Deprecation Timeline

- **Weeks 1-6**: Implement new architecture in `pkg/indexer`
- **Week 7**: Feature flag to enable new indexer
- **Week 8**: Run both in parallel, compare results
- **Week 9**: Default to new indexer, keep legacy as fallback
- **Week 10**: Remove legacy indexer code from `internal/indexer`

## Benefits

### 1. Provider Agnostic

```go
// Same code works with any provider
orchestrator := indexer.NewOrchestrator(
    indexer.WithWorkspaceProvider(googleProvider), // or localProvider
    // ...
)
```

### 2. Composable Operations

```go
// Create custom pipelines
customPipeline := &indexer.Pipeline{
    name: "custom",
    commands: []indexer.Command{
        &commands.DiscoverCommand{},
        &commands.ValidateCommand{},
        &commands.TransformCommand{},
        &myCustomCommand{},
    },
}
```

### 3. Testable Locally

```bash
# Full integration test without Google Workspace
make test/indexer/integration

# Test specific pipeline
./hermes indexer -config=testing/config.hcl \
  -provider=local \
  -pipeline=index-published \
  -dry-run
```

### 4. Migration Support

```bash
# Migrate from Google to Local for testing
./hermes indexer -config=config.hcl -migrate \
  -source=google -target=local \
  -source-folder=1abc... -target-folder=/tmp/docs

# Migrate between local folders
./hermes indexer -config=config.hcl -migrate \
  -source=local -target=local \
  -source-folder=./old -target-folder=./new
```

### 5. Extensibility

Easy to add new commands:

```go
// Add custom validation command
type ValidateLinksCommand struct {
    httpClient *http.Client
}

func (c *ValidateLinksCommand) Execute(ctx context.Context, doc *DocumentContext) error {
    // Extract links from doc.Content
    // Validate each link
    // Add results to doc.Errors
}

// Use in pipeline
pipeline.AddCommand(&ValidateLinksCommand{})
```

## Success Criteria

### Phase 1-3 (Foundation)
- [ ] All existing indexer functionality works with new architecture
- [ ] Unit tests for all commands (>80% coverage)
- [ ] Integration tests pass with mock providers

### Phase 4-5 (Local Testing)
- [ ] Can index documents from local filesystem
- [ ] Can migrate documents between providers
- [ ] Full integration tests run in `./testing` without external dependencies
- [ ] Documentation for local testing workflow

### Phase 6 (Production Ready)
- [ ] CLI supports new architecture
- [ ] Backward compatibility maintained
- [ ] Performance equals or exceeds legacy indexer
- [ ] Deployed and running in production

## Risks & Mitigation

### Risk 1: Workspace Provider Interface Gaps

**Risk**: `workspace.DocumentStorage` interface may not have all operations needed

**Mitigation**: 
- Audit current indexer for all workspace operations
- Extend interface as needed before Phase 2
- Add `GetUpdatedDocsBetween` equivalent to interface

### Risk 2: Performance Regression

**Risk**: Abstraction layers may slow down indexing

**Mitigation**:
- Benchmark each phase against legacy
- Optimize hot paths (content extraction, API calls)
- Maintain parallel processing capability

### Risk 3: Breaking Changes

**Risk**: Refactor may break existing deployments

**Mitigation**:
- Feature flag for new vs legacy
- Run both in parallel initially
- Comprehensive integration tests

## Next Steps

1. **Review & Approve**: Get team feedback on this plan
2. **Extend Workspace Interface**: Add missing operations (`GetUpdatedDocsBetween`, etc.)
3. **Create Feature Branch**: `feat/indexer-refactor`
4. **Phase 1 Implementation**: Start with command interfaces
5. **Weekly Reviews**: Check progress and adjust plan

## Related Documentation

- [Indexer README](./README-indexer.md)
- [Workspace Provider Architecture](../pkg/workspace/README.md)
- [Search Provider Architecture](../pkg/search/README.md)
- [Testing Infrastructure](../testing/README.md)

---

**Author**: GitHub Copilot
**Date**: October 22, 2025
**Status**: Proposed
