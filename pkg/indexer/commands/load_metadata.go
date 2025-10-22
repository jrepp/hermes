package commands

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"gorm.io/gorm"
)

// LoadMetadataCommand loads document metadata from the database.
// This populates the DocumentContext with Document, Reviews, and GroupReviews.
type LoadMetadataCommand struct {
	DB *gorm.DB
}

// Name returns the command name.
func (c *LoadMetadataCommand) Name() string {
	return "load-metadata"
}

// Execute loads metadata for the document.
func (c *LoadMetadataCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if err := doc.LoadMetadata(c.DB); err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}
	return nil
}

// ExecuteBatch implements BatchCommand for batch metadata loading.
func (c *LoadMetadataCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// Could be optimized with bulk queries, but for now use parallel processing
	return indexer.ParallelProcess(ctx, docs, c.Execute, 10)
}
