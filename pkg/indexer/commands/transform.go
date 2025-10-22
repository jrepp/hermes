package commands

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/indexer"
)

// TransformCommand converts a workspace document to a search document.
// It uses metadata from the database to enrich the document with
// Hermes-specific information.
type TransformCommand struct {
	DocumentTypes []*config.DocumentType
}

// Name returns the command name.
func (c *TransformCommand) Name() string {
	return "transform"
}

// Execute transforms the document for search indexing.
func (c *TransformCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if doc.Metadata == nil {
		return fmt.Errorf("metadata not loaded, run load-metadata command first")
	}

	// Convert database model to search document
	searchDoc, err := document.NewFromDatabaseModel(
		*doc.Metadata,
		doc.Reviews,
		doc.GroupReviews,
	)
	if err != nil {
		return fmt.Errorf("failed to create search document: %w", err)
	}

	// Add content and modified time
	searchDoc.Content = doc.Content
	searchDoc.ModifiedTime = doc.Document.ModifiedTime.Unix()

	doc.Transformed = searchDoc
	return nil
}

// ExecuteBatch implements BatchCommand for batch transformation.
func (c *TransformCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	return indexer.ParallelProcess(ctx, docs, c.Execute, 10)
}
