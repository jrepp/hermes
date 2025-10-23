package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// DiscoverCommand finds documents that need processing.
// It queries the workspace provider for documents in a specific folder
// that have been modified within a time range.
type DiscoverCommand struct {
	Provider workspace.DocumentStorage
	FolderID string
	Since    *time.Time
	Until    *time.Time
	Filter   indexer.DocumentFilter
}

// Name returns the command name.
func (c *DiscoverCommand) Name() string {
	return "discover"
}

// Execute is not used for DiscoverCommand.
// Use Discover() instead.
func (c *DiscoverCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	return fmt.Errorf("DiscoverCommand should use Discover() method")
}

// Discover returns documents that match the criteria.
func (c *DiscoverCommand) Discover(ctx context.Context) ([]*indexer.DocumentContext, error) {
	opts := &workspace.ListOptions{}

	if c.Since != nil {
		opts.ModifiedAfter = c.Since
	}

	docs, err := c.Provider.ListDocuments(ctx, c.FolderID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	// Convert to DocumentContext and apply filter
	contexts := make([]*indexer.DocumentContext, 0, len(docs))
	for _, doc := range docs {
		// Skip if until time specified and doc is after it
		if c.Until != nil && doc.ModifiedTime.After(*c.Until) {
			continue
		}

		docCtx := &indexer.DocumentContext{
			Document:       doc,
			SourceProvider: c.Provider,
			StartTime:      time.Now(),
		}

		// Apply filter if specified
		if c.Filter != nil && !c.Filter(docCtx) {
			continue
		}

		contexts = append(contexts, docCtx)
	}

	return contexts, nil
}
