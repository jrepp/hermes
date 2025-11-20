package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// MigrateCommand migrates a document from one workspace provider to another.
// It handles copying the document, preserving metadata, and tracking the migration
// in the document context for conflict detection.
type MigrateCommand struct {
	Source         workspace.DocumentStorage
	Target         workspace.DocumentStorage
	TargetFolderID string
	DryRun         bool
	Logger         hclog.Logger
}

// Name returns the command name.
func (c *MigrateCommand) Name() string {
	return "migrate"
}

// Execute migrates a single document from source to target provider.
func (c *MigrateCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	c.Logger.Info("migrating document",
		"document_id", doc.Document.ID,
		"name", doc.Document.Name,
		"dry_run", c.DryRun,
	)

	// Verify source provider matches
	if doc.SourceProvider != c.Source {
		return fmt.Errorf("document source provider does not match command source provider")
	}

	// Check if document already exists in target
	existingDocs, err := c.Target.ListDocuments(ctx, c.TargetFolderID, &workspace.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list target documents: %w", err)
	}

	// Look for existing document with same name
	for _, existing := range existingDocs {
		if existing.Name == doc.Document.Name {
			c.Logger.Warn("document already exists in target",
				"document_id", doc.Document.ID,
				"name", doc.Document.Name,
				"target_id", existing.ID,
			)

			// Store target document for conflict detection
			doc.TargetProvider = c.Target
			doc.TargetDocument = existing
			doc.SetCustom("migration_status", "conflict")
			doc.SetCustom("conflict_reason", "document_exists")

			return nil // Don't fail, let conflict detection handle it
		}
	}

	if c.DryRun {
		c.Logger.Info("dry run: would migrate document",
			"document_id", doc.Document.ID,
			"name", doc.Document.Name,
		)
		doc.SetCustom("migration_status", "dry_run")
		return nil
	}

	// Extract content if not already done
	if doc.Content == "" {
		content, err := c.Source.GetDocumentContent(ctx, doc.Document.ID)
		if err != nil {
			return fmt.Errorf("failed to get source document content: %w", err)
		}
		doc.Content = content
	}

	// Create document in target provider
	targetDoc, err := c.Target.CreateDocument(ctx, &workspace.DocumentCreate{
		Name:           doc.Document.Name,
		ParentFolderID: c.TargetFolderID,
		Content:        doc.Content,
	})
	if err != nil {
		return fmt.Errorf("failed to create target document: %w", err)
	}

	c.Logger.Info("document migrated successfully",
		"source_id", doc.Document.ID,
		"target_id", targetDoc.ID,
		"name", doc.Document.Name,
	)

	// Update context with migration info
	doc.TargetProvider = c.Target
	doc.TargetDocument = targetDoc
	doc.TargetFolderID = c.TargetFolderID
	doc.SetCustom("migration_status", "success")
	doc.SetCustom("migration_time", time.Now())

	return nil
}

// ExecuteBatch implements BatchCommand for parallel migration.
func (c *MigrateCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// Migration can be done in parallel with a reasonable worker pool
	return indexer.ParallelProcess(ctx, docs, c.Execute, 3)
}
