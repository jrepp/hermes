# Indexer Package

Provider-agnostic document indexing system using command pattern and pipeline architecture.

## Overview

The indexer package provides a flexible, composable system for processing documents through pipelines of commands. It's designed to work with any workspace provider (Google Workspace, local filesystem, etc.) and any search provider (Algolia, Meilisearch, etc.).

## Key Concepts

### Commands

Commands are individual operations that process documents. Each command implements the `Command` interface:

```go
type Command interface {
    Execute(ctx context.Context, doc *DocumentContext) error
    Name() string
}
```

Available commands:
- **DiscoverCommand**: Find documents in a folder
- **ExtractContentCommand**: Get document content
- **LoadMetadataCommand**: Load from database
- **TransformCommand**: Convert to search format
- **IndexCommand**: Index in search provider
- **TrackCommand**: Update database tracking

### Pipelines

Pipelines chain commands together to perform complex operations:

```go
pipeline := &indexer.Pipeline{
    Name: "index-published",
    Commands: []indexer.Command{
        &commands.DiscoverCommand{},
        &commands.ExtractContentCommand{},
        &commands.TransformCommand{},
        &commands.IndexCommand{},
        &commands.TrackCommand{},
    },
    Filter: indexer.RecentlyModifiedFilter(30 * time.Minute),
}
```

### Orchestrator

The orchestrator manages pipeline execution:

```go
orchestrator, err := indexer.NewOrchestrator(
    indexer.WithDatabase(db),
    indexer.WithWorkspaceProvider(workspaceProvider),
    indexer.WithSearchProvider(searchProvider),
    indexer.WithLogger(logger),
)

orchestrator.RegisterPipeline("index-published", pipeline)
orchestrator.Run(ctx, 60*time.Second) // Run every 60 seconds
```

## Usage Examples

### Basic Indexing Pipeline

```go
// Create pipeline
pipeline := &indexer.Pipeline{
    Name: "index-docs",
    Commands: []indexer.Command{
        &commands.DiscoverCommand{
            Provider: workspace.DocumentStorage(),
            FolderID: "docs-folder-id",
        },
        &commands.ExtractContentCommand{
            Provider: workspace.DocumentStorage(),
            MaxSize:  85000,
        },
        &commands.LoadMetadataCommand{
            DB: db,
        },
        &commands.TransformCommand{
            DocumentTypes: config.DocumentTypes,
        },
        &commands.IndexCommand{
            SearchProvider: searchProvider,
            IndexType:      commands.IndexTypePublished,
        },
        &commands.TrackCommand{
            DB:                 db,
            FolderID:           "docs-folder-id",
            UpdateDocumentTime: true,
        },
    },
}

// Execute
err := pipeline.Execute(ctx, nil)
```

### With Filtering

```go
// Only process RFC documents modified more than 30 minutes ago
pipeline.Filter = indexer.CombineFilters(
    indexer.RecentlyModifiedFilter(30 * time.Minute),
    indexer.DocumentTypeFilter("RFC"),
)
```

### Run Continuously

```go
orchestrator := indexer.NewOrchestrator(...)
orchestrator.RegisterPipeline("index-published", publishedPipeline)
orchestrator.RegisterPipeline("index-drafts", draftsPipeline)

// Run every 60 seconds
orchestrator.Run(ctx, 60*time.Second)
```

### Run Once

```go
// Execute all pipelines once
err := orchestrator.RunOnce(ctx)
```

## Testing

The package is designed to be easily testable with mock providers:

```go
func TestIndexing(t *testing.T) {
    mockWorkspace := &MockWorkspaceProvider{}
    mockSearch := &MockSearchProvider{}
    
    orchestrator, _ := indexer.NewOrchestrator(
        indexer.WithWorkspaceProvider(mockWorkspace),
        indexer.WithSearchProvider(mockSearch),
        indexer.WithDatabase(testDB),
    )
    
    // Test pipeline execution
    err := orchestrator.RunOnce(context.Background())
    assert.NoError(t, err)
}
```

## Custom Commands

You can create custom commands for specific needs:

```go
type ValidateLinksCommand struct {
    httpClient *http.Client
}

func (c *ValidateLinksCommand) Name() string {
    return "validate-links"
}

func (c *ValidateLinksCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
    // Extract and validate links from doc.Content
    // Add errors to doc.Errors if validation fails
    return nil
}

// Use in pipeline
pipeline.Commands = append(pipeline.Commands, &ValidateLinksCommand{})
```

## Performance

- **Parallel Processing**: Commands process multiple documents concurrently
- **Batch Operations**: Commands can implement `BatchCommand` for efficiency
- **Configurable Concurrency**: Set `MaxParallel` per pipeline

```go
pipeline.MaxParallel = 10 // Process up to 10 documents at once
```

## Error Handling

Errors are collected in `DocumentContext.Errors` without stopping the pipeline:

```go
for _, doc := range processedDocs {
    if doc.HasErrors() {
        log.Error("document had errors", "id", doc.Document.ID, "errors", doc.Errors)
    }
}
```

## Integration with Existing Code

The new indexer can run alongside the legacy indexer during migration:

```go
// Legacy indexer (internal/indexer)
legacyIndexer := &indexer.Indexer{...}

// New indexer (pkg/indexer)  
newOrchestrator := indexer.NewOrchestrator(...)

// Run both (feature flag controlled)
if useNewIndexer {
    newOrchestrator.Run(ctx, interval)
} else {
    legacyIndexer.Run()
}
```

## See Also

- [Implementation Guide](../../docs-internal/INDEXER_IMPLEMENTATION_GUIDE.md)
- [Refactor Plan](../../docs-internal/INDEXER_REFACTOR_PLAN.md)
- [Workspace Providers](../workspace/README.md)
- [Search Providers](../search/README.md)
