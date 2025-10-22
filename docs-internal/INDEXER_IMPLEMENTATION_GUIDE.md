# Indexer Refactor: Quick Implementation Guide

This is a companion to `INDEXER_REFACTOR_PLAN.md` with concrete code examples and implementation steps.

## Quick Start: Testing Locally Today

Even before the refactor, you can test indexer concepts locally:

```bash
# 1. Setup local workspace with test data
mkdir -p testing/workspace_data/{docs,drafts}

# 2. Create test documents
cat > testing/workspace_data/docs/RFC-001.md <<EOF
# RFC-001: Test Document

**Status**: In Review
**Owner**: test@example.com

This is a test RFC document for local indexer testing.
EOF

# 3. Run indexer against local Meilisearch
cd testing
docker compose up -d postgres meilisearch
cd ..

# 4. Configure for local testing (future)
./hermes indexer -config=testing/config.hcl -provider=local
```

## Command Pattern Examples

### Basic Command Interface

```go
// pkg/indexer/command.go
package indexer

import "context"

// Command represents a single operation in the indexer pipeline.
type Command interface {
    // Execute performs the command operation on a document context.
    Execute(ctx context.Context, doc *DocumentContext) error
    
    // Name returns the command name for logging and debugging.
    Name() string
}

// BatchCommand processes multiple documents at once.
// Commands can optionally implement this interface for better performance
// when processing multiple documents.
type BatchCommand interface {
    Command
    ExecuteBatch(ctx context.Context, docs []*DocumentContext) error
}
```

### Discover Command

```go
// pkg/indexer/commands/discover.go
package commands

import (
    "context"
    "time"
    
    "github.com/hashicorp-forge/hermes/pkg/indexer"
    "github.com/hashicorp-forge/hermes/pkg/workspace"
)

// DiscoverCommand finds documents that need processing.
type DiscoverCommand struct {
    Provider  workspace.DocumentStorage
    FolderID  string
    Since     time.Time
    Until     *time.Time
    Filter    indexer.DocumentFilter
}

func (c *DiscoverCommand) Name() string {
    return "discover"
}

func (c *DiscoverCommand) Execute(ctx context.Context, _ *indexer.DocumentContext) error {
    // This command doesn't operate on a single document
    return nil
}

// Discover returns documents that match the criteria.
func (c *DiscoverCommand) Discover(ctx context.Context) ([]*indexer.DocumentContext, error) {
    opts := &workspace.ListOptions{
        ModifiedAfter: &c.Since,
    }
    
    docs, err := c.Provider.ListDocuments(ctx, c.FolderID, opts)
    if err != nil {
        return nil, err
    }
    
    // Convert to DocumentContext and apply filter
    contexts := make([]*indexer.DocumentContext, 0, len(docs))
    for _, doc := range docs {
        // Skip if until time specified and doc is after it
        if c.Until != nil && doc.ModifiedTime.After(*c.Until) {
            continue
        }
        
        ctx := &indexer.DocumentContext{
            Document:       doc,
            SourceProvider: c.Provider,
            StartTime:      time.Now(),
        }
        
        // Apply filter if specified
        if c.Filter != nil && !c.Filter(ctx) {
            continue
        }
        
        contexts = append(contexts, ctx)
    }
    
    return contexts, nil
}
```

### Extract Content Command

```go
// pkg/indexer/commands/extract.go
package commands

import (
    "context"
    "fmt"
    
    "github.com/hashicorp-forge/hermes/pkg/indexer"
    "github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ExtractContentCommand retrieves document content from the provider.
type ExtractContentCommand struct {
    Provider workspace.DocumentStorage
    MaxSize  int // Maximum content size in bytes
}

func (c *ExtractContentCommand) Name() string {
    return "extract-content"
}

func (c *ExtractContentCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
    content, err := c.Provider.GetDocumentContent(ctx, doc.Document.ID)
    if err != nil {
        return fmt.Errorf("failed to get document content: %w", err)
    }
    
    // Trim if exceeds max size
    if c.MaxSize > 0 && len(content) > c.MaxSize {
        content = content[:c.MaxSize]
    }
    
    doc.Content = content
    return nil
}

// ExecuteBatch implements BatchCommand for parallel processing.
func (c *ExtractContentCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
    // Use worker pool for parallel extraction
    return indexer.ParallelProcess(ctx, docs, c.Execute, 5)
}
```

### Transform Command

```go
// pkg/indexer/commands/transform.go
package commands

import (
    "context"
    "fmt"
    
    "github.com/hashicorp-forge/hermes/internal/config"
    "github.com/hashicorp-forge/hermes/pkg/document"
    "github.com/hashicorp-forge/hermes/pkg/indexer"
    "gorm.io/gorm"
)

// TransformCommand converts workspace document to search document.
type TransformCommand struct {
    DB            *gorm.DB
    DocumentTypes []*config.DocumentType
}

func (c *TransformCommand) Name() string {
    return "transform"
}

func (c *TransformCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
    // Load metadata from database if not already loaded
    if doc.Metadata == nil {
        if err := doc.LoadMetadata(c.DB); err != nil {
            return fmt.Errorf("failed to load metadata: %w", err)
        }
    }
    
    // Convert to search document
    searchDoc, err := document.NewFromDatabaseModel(
        *doc.Metadata,
        doc.Reviews,
        doc.GroupReviews,
    )
    if err != nil {
        return fmt.Errorf("failed to create search document: %w", err)
    }
    
    // Add content
    searchDoc.Content = doc.Content
    searchDoc.ModifiedTime = doc.Document.ModifiedTime.Unix()
    
    doc.Transformed = searchDoc
    return nil
}
```

### Index Command

```go
// pkg/indexer/commands/index.go
package commands

import (
    "context"
    "fmt"
    
    "github.com/hashicorp-forge/hermes/pkg/indexer"
    "github.com/hashicorp-forge/hermes/pkg/search"
)

// IndexType specifies which index to use.
type IndexType string

const (
    IndexTypePublished IndexType = "published"
    IndexTypeDrafts    IndexType = "drafts"
)

// IndexCommand indexes a document in the search provider.
type IndexCommand struct {
    SearchProvider search.Provider
    IndexType      IndexType
}

func (c *IndexCommand) Name() string {
    return fmt.Sprintf("index-%s", c.IndexType)
}

func (c *IndexCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
    if doc.Transformed == nil {
        return fmt.Errorf("document not transformed, run transform command first")
    }
    
    // Convert to search document format
    searchDoc, err := doc.Transformed.ToSearchDocument()
    if err != nil {
        return fmt.Errorf("failed to convert to search document: %w", err)
    }
    
    // Index in appropriate index
    var idx search.DocumentIndex
    switch c.IndexType {
    case IndexTypePublished:
        idx = c.SearchProvider.DocumentIndex()
    case IndexTypeDrafts:
        idx = c.SearchProvider.DraftIndex()
    default:
        return fmt.Errorf("unknown index type: %s", c.IndexType)
    }
    
    if err := idx.Index(ctx, searchDoc); err != nil {
        return fmt.Errorf("failed to index document: %w", err)
    }
    
    return nil
}

// ExecuteBatch implements BatchCommand for batch indexing.
func (c *IndexCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
    searchDocs := make([]*search.Document, 0, len(docs))
    for _, doc := range docs {
        if doc.Transformed == nil {
            continue // Skip non-transformed
        }
        searchDoc, err := doc.Transformed.ToSearchDocument()
        if err != nil {
            return err
        }
        searchDocs = append(searchDocs, searchDoc)
    }
    
    var idx search.DocumentIndex
    switch c.IndexType {
    case IndexTypePublished:
        idx = c.SearchProvider.DocumentIndex()
    case IndexTypeDrafts:
        idx = c.SearchProvider.DraftIndex()
    default:
        return fmt.Errorf("unknown index type: %s", c.IndexType)
    }
    
    return idx.IndexBatch(ctx, searchDocs)
}
```

### Migration Command

```go
// pkg/indexer/commands/migrate.go
package commands

import (
    "context"
    "fmt"
    
    "github.com/hashicorp-forge/hermes/pkg/indexer"
    "github.com/hashicorp-forge/hermes/pkg/workspace"
    "github.com/hashicorp/go-hclog"
)

// MigrateCommand moves documents from one provider to another.
type MigrateCommand struct {
    Source           workspace.DocumentStorage
    Target           workspace.DocumentStorage
    TargetFolderID   string
    SkipExisting     bool
    DryRun           bool
    Logger           hclog.Logger
}

func (c *MigrateCommand) Name() string {
    return "migrate"
}

func (c *MigrateCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
    // Check if exists in target
    if c.SkipExisting {
        existing, err := c.Target.GetDocument(ctx, doc.Document.ID)
        if err == nil && existing != nil {
            c.Logger.Info("document already exists in target, skipping",
                "id", doc.Document.ID,
                "name", doc.Document.Name,
            )
            return nil
        }
    }
    
    if c.DryRun {
        c.Logger.Info("would migrate document",
            "id", doc.Document.ID,
            "name", doc.Document.Name,
            "target_folder", c.TargetFolderID,
        )
        return nil
    }
    
    // Ensure content is loaded
    if doc.Content == "" {
        content, err := c.Source.GetDocumentContent(ctx, doc.Document.ID)
        if err != nil {
            return fmt.Errorf("failed to get document content: %w", err)
        }
        doc.Content = content
    }
    
    // Create in target
    createOpts := &workspace.DocumentCreate{
        Name:           doc.Document.Name,
        ParentFolderID: c.TargetFolderID,
        Content:        doc.Content,
        Owner:          doc.Document.Owner,
        Metadata:       doc.Document.Metadata,
    }
    
    targetDoc, err := c.Target.CreateDocument(ctx, createOpts)
    if err != nil {
        return fmt.Errorf("failed to create document in target: %w", err)
    }
    
    doc.TargetDocument = targetDoc
    c.Logger.Info("migrated document",
        "source_id", doc.Document.ID,
        "target_id", targetDoc.ID,
        "name", targetDoc.Name,
    )
    
    return nil
}
```

## Pipeline Implementation

```go
// pkg/indexer/pipeline.go
package indexer

import (
    "context"
    "fmt"
    "sync"
    
    "github.com/hashicorp/go-hclog"
)

// Pipeline executes a sequence of commands on documents.
type Pipeline struct {
    Name     string
    Commands []Command
    Filter   DocumentFilter
    Logger   hclog.Logger
    
    // Configuration
    MaxParallel int
}

// Execute runs the pipeline on a set of documents.
func (p *Pipeline) Execute(ctx context.Context, docs []*DocumentContext) error {
    p.Logger.Info("starting pipeline",
        "name", p.Name,
        "documents", len(docs),
    )
    
    // Apply filter
    filtered := docs
    if p.Filter != nil {
        filtered = make([]*DocumentContext, 0, len(docs))
        for _, doc := range docs {
            if p.Filter(doc) {
                filtered = append(filtered, doc)
            }
        }
        p.Logger.Info("filtered documents",
            "before", len(docs),
            "after", len(filtered),
        )
    }
    
    // Execute commands in sequence
    for _, cmd := range p.Commands {
        p.Logger.Debug("executing command",
            "name", cmd.Name(),
            "documents", len(filtered),
        )
        
        // Check if command supports batch processing
        if batchCmd, ok := cmd.(BatchCommand); ok {
            if err := batchCmd.ExecuteBatch(ctx, filtered); err != nil {
                return fmt.Errorf("batch command %s failed: %w", cmd.Name(), err)
            }
        } else {
            // Process documents in parallel
            if err := p.executeParallel(ctx, cmd, filtered); err != nil {
                return fmt.Errorf("command %s failed: %w", cmd.Name(), err)
            }
        }
    }
    
    p.Logger.Info("pipeline completed",
        "name", p.Name,
        "documents", len(filtered),
    )
    
    return nil
}

// executeParallel runs a command on multiple documents in parallel.
func (p *Pipeline) executeParallel(ctx context.Context, cmd Command, docs []*DocumentContext) error {
    maxParallel := p.MaxParallel
    if maxParallel <= 0 {
        maxParallel = 5
    }
    
    return ParallelProcess(ctx, docs, cmd.Execute, maxParallel)
}

// ParallelProcess processes items in parallel with a worker pool.
func ParallelProcess[T any](ctx context.Context, items []T, fn func(context.Context, T) error, maxWorkers int) error {
    if len(items) == 0 {
        return nil
    }
    
    // Create worker pool
    workers := maxWorkers
    if len(items) < workers {
        workers = len(items)
    }
    
    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error
    
    ch := make(chan T, len(items))
    
    // Start workers
    wg.Add(workers)
    for i := 0; i < workers; i++ {
        go func() {
            defer wg.Done()
            for item := range ch {
                if err := fn(ctx, item); err != nil {
                    mu.Lock()
                    errs = append(errs, err)
                    mu.Unlock()
                }
            }
        }()
    }
    
    // Send items to workers
    for _, item := range items {
        ch <- item
    }
    close(ch)
    
    // Wait for completion
    wg.Wait()
    
    if len(errs) > 0 {
        return fmt.Errorf("parallel processing had %d errors: %v", len(errs), errs[0])
    }
    
    return nil
}
```

## DocumentContext Implementation

```go
// pkg/indexer/context.go
package indexer

import (
    "context"
    "time"
    
    "github.com/hashicorp-forge/hermes/pkg/document"
    "github.com/hashicorp-forge/hermes/pkg/models"
    "github.com/hashicorp-forge/hermes/pkg/workspace"
    "gorm.io/gorm"
)

// DocumentContext holds all information about a document being processed.
type DocumentContext struct {
    // Source document from workspace provider
    Document *workspace.Document
    
    // Database metadata
    Metadata     *models.Document
    Reviews      models.DocumentReviews
    GroupReviews models.DocumentGroupReviews
    
    // Processing state
    Content     string
    Transformed *document.Document
    
    // Provider references
    SourceProvider workspace.DocumentStorage
    TargetProvider workspace.DocumentStorage
    TargetFolderID string
    TargetDocument *workspace.Document
    
    // Tracking
    StartTime time.Time
    Errors    []error
}

// LoadMetadata loads database metadata for the document.
func (dc *DocumentContext) LoadMetadata(db *gorm.DB) error {
    if dc.Metadata != nil {
        return nil // Already loaded
    }
    
    // Get document from database
    dbDoc := models.Document{
        GoogleFileID: dc.Document.ID,
    }
    if err := dbDoc.Get(db); err != nil {
        return err
    }
    dc.Metadata = &dbDoc
    
    // Get reviews
    if err := dc.Reviews.Find(db, models.DocumentReview{
        Document: models.Document{
            GoogleFileID: dc.Document.ID,
        },
    }); err != nil {
        return err
    }
    
    // Get group reviews
    if err := dc.GroupReviews.Find(db, models.DocumentGroupReview{
        Document: models.Document{
            GoogleFileID: dc.Document.ID,
        },
    }); err != nil {
        return err
    }
    
    return nil
}

// AddError adds an error to the context.
func (dc *DocumentContext) AddError(err error) {
    dc.Errors = append(dc.Errors, err)
}

// HasErrors returns true if any errors occurred.
func (dc *DocumentContext) HasErrors() bool {
    return len(dc.Errors) > 0
}

// DocumentFilter filters documents based on criteria.
type DocumentFilter func(*DocumentContext) bool

// RecentlyModifiedFilter skips documents modified within the duration.
func RecentlyModifiedFilter(within time.Duration) DocumentFilter {
    return func(doc *DocumentContext) bool {
        return time.Since(doc.Document.ModifiedTime) > within
    }
}

// DocumentTypeFilter only processes specific document types.
func DocumentTypeFilter(docTypes ...string) DocumentFilter {
    typeMap := make(map[string]bool)
    for _, t := range docTypes {
        typeMap[t] = true
    }
    
    return func(doc *DocumentContext) bool {
        if doc.Metadata == nil {
            return false
        }
        return typeMap[doc.Metadata.DocType]
    }
}
```

## Testing Example

```go
// pkg/indexer/commands/extract_test.go
package commands

import (
    "context"
    "testing"
    "time"
    
    "github.com/hashicorp-forge/hermes/pkg/indexer"
    "github.com/hashicorp-forge/hermes/pkg/workspace"
    "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/mock"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExtractContentCommand(t *testing.T) {
    t.Run("extracts content successfully", func(t *testing.T) {
        // Setup mock provider
        mockProvider := &mock.DocumentStorage{}
        mockProvider.On("GetDocumentContent", mock.Anything, "doc-123").
            Return("Document content here", nil)
        
        // Create command
        cmd := &ExtractContentCommand{
            Provider: mockProvider,
            MaxSize:  0, // No limit
        }
        
        // Create document context
        doc := &indexer.DocumentContext{
            Document: &workspace.Document{
                ID:   "doc-123",
                Name: "Test Doc",
            },
        }
        
        // Execute
        err := cmd.Execute(context.Background(), doc)
        require.NoError(t, err)
        assert.Equal(t, "Document content here", doc.Content)
    })
    
    t.Run("trims content when exceeds max size", func(t *testing.T) {
        mockProvider := &mock.DocumentStorage{}
        mockProvider.On("GetDocumentContent", mock.Anything, "doc-123").
            Return("This is a very long document content", nil)
        
        cmd := &ExtractContentCommand{
            Provider: mockProvider,
            MaxSize:  10, // Trim to 10 bytes
        }
        
        doc := &indexer.DocumentContext{
            Document: &workspace.Document{ID: "doc-123"},
        }
        
        err := cmd.Execute(context.Background(), doc)
        require.NoError(t, err)
        assert.Equal(t, "This is a ", doc.Content)
        assert.Len(t, doc.Content, 10)
    })
}
```

## Pipeline Configuration (Indexer Plans)

Instead of hardcoding pipelines, define them as configuration that can be loaded and executed.

### Pipeline Plan Structure

```go
// pkg/indexer/plan.go
package indexer

import (
    "time"
)

// Plan defines a complete indexer execution plan.
type Plan struct {
    Name        string
    Description string
    
    // Provider configuration
    WorkspaceProvider string            // "google", "local", "mock"
    SearchProvider    string            // "algolia", "meilisearch"
    
    // Workspace folders
    Folders []FolderConfig
    
    // Pipelines to execute
    Pipelines []PipelineConfig
    
    // Execution settings
    RunInterval     time.Duration
    MaxParallelDocs int
    DryRun          bool
}

// FolderConfig defines a folder to monitor.
type FolderConfig struct {
    ID       string // Provider-specific folder ID
    Type     string // "docs", "drafts", "custom"
    Pipeline string // Which pipeline to use for this folder
}

// PipelineConfig defines a pipeline execution.
type PipelineConfig struct {
    Name        string
    Description string
    Commands    []CommandConfig
    Filter      FilterConfig
}

// CommandConfig defines a command in the pipeline.
type CommandConfig struct {
    Type   string         // "discover", "extract", "transform", "index", etc.
    Config map[string]any // Command-specific configuration
}

// FilterConfig defines document filtering.
type FilterConfig struct {
    SkipRecentlyModified string   // Duration like "30m"
    DocumentTypes        []string // Only process these doc types
    MinModifiedTime      string   // RFC3339 timestamp
}
```

### Example: Local Integration Testing Plan

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
    description: Index published documents from local workspace
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
    
    filter:
      skip_recently_modified: 0s  # Don't skip any for testing
  
  - name: index-drafts
    description: Index draft documents from local workspace
    commands:
      - type: discover
        config:
          folder_id: drafts
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
          index_type: drafts
          batch_size: 10

execution:
  run_interval: 10s      # Fast for testing
  max_parallel_docs: 3   # Moderate for testing
  dry_run: false
```

### Example: Production Google Workspace Plan

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
  
  - id: "{{ .GoogleWorkspace.DraftsFolder }}"
    type: drafts
    pipeline: index-and-refresh-drafts

pipelines:
  - name: index-and-refresh-published
    description: Index and refresh headers for published documents
    commands:
      - type: discover
        config:
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
      
      - type: update-header
        config:
          enabled: "{{ .Indexer.UpdateDocHeaders }}"
      
      - type: index
        config:
          index_type: published
          batch_size: 20
      
      - type: track
        config:
          update_folder_timestamp: true
    
    filter:
      skip_recently_modified: 30m  # Avoid disrupting active editing
  
  - name: index-and-refresh-drafts
    description: Index and refresh headers for draft documents
    commands:
      - type: discover
        config:
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
      
      - type: update-header
        config:
          enabled: "{{ .Indexer.UpdateDraftHeaders }}"
      
      - type: index
        config:
          index_type: drafts
          batch_size: 20

execution:
  run_interval: 60s
  max_parallel_docs: 5
  dry_run: false
```

### Example: Migration Plan

```yaml
# config/indexer/plans/migrate-google-to-local.yaml
name: migrate-google-to-local
description: Migrate documents from Google Workspace to local filesystem

workspace_provider: google  # Source
target_provider: local      # Target

folders:
  - id: "{{ .GoogleWorkspace.DocsFolder }}"
    type: published
    pipeline: migrate-docs
    target_folder: ./testing/workspace_data/docs

pipelines:
  - name: migrate-docs
    description: Migrate documents from Google to local
    commands:
      - type: discover
        config:
          since_last_run: false  # Process all documents
      
      - type: extract
        config:
          max_size: 0  # No limit for migration
      
      - type: load-metadata
        config:
          source: database
      
      - type: migrate
        config:
          target_provider: local
          target_folder: ./testing/workspace_data/docs
          skip_existing: true
          preserve_metadata: true
      
      - type: transform
        config:
          format: search-document
      
      - type: index
        config:
          index_type: published
          target_search_provider: meilisearch
      
      - type: track
        config:
          update_migration_status: true

execution:
  run_interval: 0s  # Run once
  max_parallel_docs: 2  # Conservative for migration
  dry_run: false
```

### Loading and Executing Plans

```go
// pkg/indexer/plan_loader.go
package indexer

import (
    "fmt"
    "os"
    "path/filepath"
    
    "gopkg.in/yaml.v3"
)

// LoadPlan loads an indexer plan from a YAML file.
func LoadPlan(path string) (*Plan, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read plan file: %w", err)
    }
    
    var plan Plan
    if err := yaml.Unmarshal(data, &plan); err != nil {
        return nil, fmt.Errorf("failed to parse plan: %w", err)
    }
    
    return &plan, nil
}

// ListPlans returns all plans in a directory.
func ListPlans(dir string) ([]string, error) {
    matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
    if err != nil {
        return nil, err
    }
    return matches, nil
}

// BuildOrchestrator creates an orchestrator from a plan.
func BuildOrchestrator(plan *Plan, cfg *Config) (*Orchestrator, error) {
    // Create providers based on plan configuration
    workspaceProvider, err := createWorkspaceProvider(plan.WorkspaceProvider, cfg)
    if err != nil {
        return nil, err
    }
    
    searchProvider, err := createSearchProvider(plan.SearchProvider, cfg)
    if err != nil {
        return nil, err
    }
    
    // Build pipelines from configuration
    pipelines := make(map[string]*Pipeline)
    for _, pipelineCfg := range plan.Pipelines {
        pipeline, err := buildPipeline(pipelineCfg, workspaceProvider, searchProvider, cfg)
        if err != nil {
            return nil, fmt.Errorf("failed to build pipeline %s: %w", pipelineCfg.Name, err)
        }
        pipelines[pipelineCfg.Name] = pipeline
    }
    
    // Create orchestrator
    return &Orchestrator{
        db:             cfg.Database,
        logger:         cfg.Logger,
        workspaceProvider: workspaceProvider,
        searchProvider: searchProvider,
        pipelines:      pipelines,
        plan:           plan,
    }, nil
}

// buildPipeline creates a pipeline from configuration.
func buildPipeline(cfg PipelineConfig, workspace WorkspaceProvider, search SearchProvider, appCfg *Config) (*Pipeline, error) {
    commands := make([]Command, 0, len(cfg.Commands))
    
    for _, cmdCfg := range cfg.Commands {
        cmd, err := buildCommand(cmdCfg, workspace, search, appCfg)
        if err != nil {
            return nil, fmt.Errorf("failed to build command %s: %w", cmdCfg.Type, err)
        }
        commands = append(commands, cmd)
    }
    
    // Build filter
    var filter DocumentFilter
    if cfg.Filter.SkipRecentlyModified != "" {
        duration, err := time.ParseDuration(cfg.Filter.SkipRecentlyModified)
        if err != nil {
            return nil, fmt.Errorf("invalid skip_recently_modified: %w", err)
        }
        filter = RecentlyModifiedFilter(duration)
    }
    
    return &Pipeline{
        Name:     cfg.Name,
        Commands: commands,
        Filter:   filter,
        Logger:   appCfg.Logger,
    }, nil
}

// buildCommand creates a command from configuration.
func buildCommand(cfg CommandConfig, workspace WorkspaceProvider, search SearchProvider, appCfg *Config) (Command, error) {
    switch cfg.Type {
    case "discover":
        return &commands.DiscoverCommand{
            Provider: workspace.DocumentStorage(),
            FolderID: cfg.Config["folder_id"].(string),
            // Parse other config...
        }, nil
    
    case "extract":
        maxSize := 85000
        if ms, ok := cfg.Config["max_size"].(int); ok {
            maxSize = ms
        }
        return &commands.ExtractContentCommand{
            Provider: workspace.DocumentStorage(),
            MaxSize:  maxSize,
        }, nil
    
    case "transform":
        return &commands.TransformCommand{
            DB:            appCfg.Database,
            DocumentTypes: appCfg.DocumentTypes,
        }, nil
    
    case "index":
        indexType := commands.IndexTypePublished
        if it, ok := cfg.Config["index_type"].(string); ok && it == "drafts" {
            indexType = commands.IndexTypeDrafts
        }
        return &commands.IndexCommand{
            SearchProvider: search,
            IndexType:      indexType,
        }, nil
    
    case "migrate":
        // Build migration command...
        return nil, fmt.Errorf("migrate command not yet implemented")
    
    default:
        return nil, fmt.Errorf("unknown command type: %s", cfg.Type)
    }
}
```

### CLI Integration

```go
// internal/cmd/commands/indexer/indexer.go
func (c *Command) Run(args []string) int {
    // ... flag parsing ...
    
    // Load indexer plan
    var plan *indexer.Plan
    if c.flagPlan != "" {
        var err error
        plan, err = indexer.LoadPlan(c.flagPlan)
        if err != nil {
            ui.Error(fmt.Sprintf("error loading plan: %v", err))
            return 1
        }
    } else {
        // Use default plan from config
        plan = buildDefaultPlan(cfg)
    }
    
    // Build orchestrator from plan
    orchestrator, err := indexer.BuildOrchestrator(plan, cfg)
    if err != nil {
        ui.Error(fmt.Sprintf("error building orchestrator: %v", err))
        return 1
    }
    
    // Run orchestrator
    ui.Info(fmt.Sprintf("starting indexer with plan: %s", plan.Name))
    return c.runOrchestrator(orchestrator)
}
```

### Usage Examples

```bash
# Run with specific plan
./hermes indexer -config=config.hcl -plan=testing/indexer/plans/local-integration-test.yaml

# Run production plan
./hermes indexer -config=config.hcl -plan=config/indexer/plans/production.yaml

# Run migration plan
./hermes indexer -config=config.hcl -plan=config/indexer/plans/migrate-google-to-local.yaml

# List available plans
./hermes indexer -config=config.hcl -list-plans

# Validate a plan without running
./hermes indexer -config=config.hcl -plan=testing/indexer/plans/local-integration-test.yaml -validate
```

### Makefile Targets (Simplified)

```makefile
# Add to root Makefile

.PHONY: indexer/test/unit
indexer/test/unit: ## Run indexer unit tests
	go test -v ./pkg/indexer/... -race -coverprofile=coverage.out

.PHONY: indexer/test/integration
indexer/test/integration: bin ## Run local integration test with plan
	./build/bin/hermes indexer \
		-config=testing/config.hcl \
		-plan=testing/indexer/plans/local-integration-test.yaml

.PHONY: indexer/plans/list
indexer/plans/list: ## List available indexer plans
	@find testing/indexer/plans config/indexer/plans -name "*.yaml" 2>/dev/null | sort

.PHONY: indexer/plans/validate
indexer/plans/validate: bin ## Validate all indexer plans
	@for plan in testing/indexer/plans/*.yaml config/indexer/plans/*.yaml; do \
		echo "Validating $$plan..."; \
		./build/bin/hermes indexer -config=config.hcl -plan=$$plan -validate || exit 1; \
	done
```

## Next Steps

1. **Start with Phase 1**: Implement command interfaces
2. **Add workspace provider methods**: Ensure `workspace.DocumentStorage` has all needed methods
3. **Create basic commands**: Start with Discover, Extract, Transform, Index
4. **Build pipeline executor**: Implement pipeline execution logic
5. **Test with mocks**: Unit test each command in isolation
6. **Add local integration test**: Create full test in `testing/indexer/`

See `INDEXER_REFACTOR_PLAN.md` for the complete 6-phase implementation plan.
