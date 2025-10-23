package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// TrackRevisionCommand creates or updates a document revision record.
// This enables tracking documents across multiple providers and detecting
// when documents change or conflict during migration.
type TrackRevisionCommand struct {
	DB           *gorm.DB
	ProviderType string // "google", "local", etc.
	ProjectID    *uint  // Optional project association
	Logger       hclog.Logger
}

// Name returns the command name.
func (c *TrackRevisionCommand) Name() string {
	return "track-revision"
}

// Execute creates or updates a revision record for the document.
func (c *TrackRevisionCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	if c.DB == nil {
		return fmt.Errorf("database connection is required")
	}

	// Ensure UUID is assigned
	if doc.DocumentUUID == uuid.Nil {
		return fmt.Errorf("document must have a UUID before tracking revision")
	}

	// Ensure content hash is calculated
	if doc.ContentHash == "" {
		return fmt.Errorf("document must have a content hash before tracking revision")
	}

	// Look for existing revision for this document and provider
	var existing models.DocumentRevision
	err := c.DB.Where("document_uuid = ? AND provider_type = ? AND document_id = ?",
		doc.DocumentUUID, c.ProviderType, doc.Document.ID).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new revision
		revision := &models.DocumentRevision{
			DocumentUUID:     doc.DocumentUUID,
			DocumentID:       doc.Document.ID,
			ProviderType:     c.ProviderType,
			ProviderFolderID: doc.Document.ParentFolderID,
			Title:            doc.Document.Name,
			ContentHash:      doc.ContentHash,
			ModifiedTime:     doc.Document.ModifiedTime,
			Status:           "active",
			ProjectID:        c.ProjectID,
		}

		if err := c.DB.Create(revision).Error; err != nil {
			return fmt.Errorf("failed to create revision: %w", err)
		}

		doc.Revision = revision

		c.Logger.Info("created new revision",
			"document_id", doc.Document.ID,
			"uuid", doc.DocumentUUID.String(),
			"provider", c.ProviderType,
			"hash", doc.ContentHash,
		)

		return nil
	} else if err != nil {
		return fmt.Errorf("failed to query existing revision: %w", err)
	}

	// Update existing revision if content changed
	if existing.ContentHash != doc.ContentHash || existing.ModifiedTime != doc.Document.ModifiedTime {
		existing.ContentHash = doc.ContentHash
		existing.ModifiedTime = doc.Document.ModifiedTime
		existing.Title = doc.Document.Name

		if err := c.DB.Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to update revision: %w", err)
		}

		c.Logger.Info("updated revision",
			"document_id", doc.Document.ID,
			"uuid", doc.DocumentUUID.String(),
			"provider", c.ProviderType,
			"old_hash", existing.ContentHash,
			"new_hash", doc.ContentHash,
		)
	} else {
		c.Logger.Debug("revision unchanged",
			"document_id", doc.Document.ID,
			"uuid", doc.DocumentUUID.String(),
			"provider", c.ProviderType,
		)
	}

	doc.Revision = &existing
	return nil
}

// ExecuteBatch implements BatchCommand for batch revision tracking.
// Uses a transaction for efficiency.
func (c *TrackRevisionCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	return c.DB.Transaction(func(tx *gorm.DB) error {
		// Create a temporary command with the transaction DB
		txCmd := &TrackRevisionCommand{
			DB:           tx,
			ProviderType: c.ProviderType,
			ProjectID:    c.ProjectID,
			Logger:       c.Logger,
		}

		// Process each document
		for _, doc := range docs {
			if err := txCmd.Execute(ctx, doc); err != nil {
				doc.AddError(err)
				// Continue processing other documents
			}
		}

		return nil
	})
}
