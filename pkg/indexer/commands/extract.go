package commands

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ExtractContentCommand retrieves document content from the workspace provider.
// It can optionally trim content to a maximum size to stay within search
// provider limits.
type ExtractContentCommand struct {
	Provider workspace.DocumentStorage
	MaxSize  int // Maximum content size in bytes (0 = no limit)
}

// Name returns the command name.
func (c *ExtractContentCommand) Name() string {
	return "extract-content"
}

// Execute extracts content from the document.
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

// ExecuteBatch implements BatchCommand for parallel extraction.
func (c *ExtractContentCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// Use parallel processing with a reasonable worker pool
	return indexer.ParallelProcess(ctx, docs, c.Execute, 5)
}
