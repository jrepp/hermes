package commands

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// TrackCommand updates database tracking information after processing documents.
// It updates the last indexed timestamp for folders and individual documents.
type TrackCommand struct {
	DB                 *gorm.DB
	FolderID           string
	UpdateDocumentTime bool // Update document's DocumentModifiedAt
}

// Name returns the command name.
func (c *TrackCommand) Name() string {
	return "track"
}

// Execute updates tracking for a single document.
func (c *TrackCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.UpdateDocumentTime && doc.Metadata != nil {
		// Update document modified time
		doc.Metadata.DocumentModifiedAt = doc.Document.ModifiedTime
		if err := doc.Metadata.Upsert(c.DB); err != nil {
			return fmt.Errorf("failed to update document modified time: %w", err)
		}
	}

	return nil
}

// ExecuteBatch implements BatchCommand for batch tracking updates.
func (c *TrackCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// Process individual document updates in parallel
	if c.UpdateDocumentTime {
		if err := indexer.ParallelProcess(ctx, docs, c.Execute, 10); err != nil {
			return err
		}
	}

	// Update folder timestamp to the latest document modified time
	if c.FolderID != "" {
		latestTime := time.Time{}
		for _, doc := range docs {
			if doc.Document.ModifiedTime.After(latestTime) {
				latestTime = doc.Document.ModifiedTime
			}
		}

		if !latestTime.IsZero() {
			folder := models.IndexerFolder{
				GoogleDriveID: c.FolderID,
			}

			// Get existing folder data
			if err := folder.Get(c.DB); err != nil && err != gorm.ErrRecordNotFound {
				return fmt.Errorf("failed to get folder tracking: %w", err)
			}

			// Update last indexed time
			folder.LastIndexedAt = latestTime
			if err := folder.Upsert(c.DB); err != nil {
				return fmt.Errorf("failed to update folder tracking: %w", err)
			}
		}
	}

	return nil
}
