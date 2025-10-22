package indexer_test

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/indexer/commands"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

// TestOrchestratorExample demonstrates how to use the indexer orchestrator.
func TestOrchestratorExample(t *testing.T) {
	t.Skip("Example test - requires full setup")

	ctx := context.Background()

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "indexer-test",
		Level:  hclog.Debug,
		Output: t.Logf,
	})

	// Setup would include:
	// - Database connection
	// - Workspace provider (local, google, etc.)
	// - Search provider (meilisearch, algolia, etc.)

	// Create orchestrator
	orchestrator, err := indexer.NewOrchestrator(
		// indexer.WithDatabase(db),
		// indexer.WithWorkspaceProvider(workspaceProvider),
		// indexer.WithSearchProvider(searchProvider),
		indexer.WithLogger(logger),
		indexer.WithMaxParallelDocs(3),
	)
	require.NoError(t, err)

	// Build and register a pipeline for indexing published documents
	indexPublishedPipeline := &indexer.Pipeline{
		Name:        "index-published",
		Description: "Index published documents",
		Commands: []indexer.Command{
			// Discover documents in the folder
			&commands.DiscoverCommand{
				// Provider:  workspaceProvider.DocumentStorage(),
				FolderID: "docs",
				// Since would be loaded from database tracking
			},
			// Extract content from documents
			&commands.ExtractContentCommand{
				// Provider: workspaceProvider.DocumentStorage(),
				MaxSize: 85000,
			},
			// Load metadata from database
			&commands.LoadMetadataCommand{
				// DB: db,
			},
			// Transform to search format
			&commands.TransformCommand{
				// DocumentTypes: cfg.DocumentTypes,
			},
			// Index in search provider
			&commands.IndexCommand{
				// SearchProvider: searchProvider,
				IndexType: commands.IndexTypePublished,
			},
			// Update tracking
			&commands.TrackCommand{
				// DB:                 db,
				FolderID:           "docs",
				UpdateDocumentTime: true,
			},
		},
		// Don't process documents edited in last 30 minutes
		Filter:      indexer.RecentlyModifiedFilter(30 * time.Minute),
		MaxParallel: 3,
	}

	orchestrator.RegisterPipeline("index-published", indexPublishedPipeline)

	// Execute once
	err = orchestrator.RunOnce(ctx)
	require.NoError(t, err)

	// Or run continuously
	// orchestrator.Run(ctx, 60*time.Second)
}
