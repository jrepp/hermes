package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// AssignUUIDCommand ensures each document has a stable UUID across providers.
// It checks if a document already has a UUID in its metadata; if not, it generates
// one and writes it back to the document. This UUID is used for revision tracking
// and enables documents to maintain their identity when migrating between providers.
type AssignUUIDCommand struct {
	Provider workspace.DocumentStorage
	Logger   hclog.Logger
}

// Name returns the command name.
func (c *AssignUUIDCommand) Name() string {
	return "assign-uuid"
}

// Execute assigns a UUID to a document if it doesn't have one.
func (c *AssignUUIDCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if c.Logger == nil {
		c.Logger = hclog.NewNullLogger()
	}

	// Check if document already has a UUID in metadata
	if doc.Document.Metadata != nil {
		if uuidVal, ok := doc.Document.Metadata["hermes_uuid"]; ok {
			if uuidStr, ok := uuidVal.(string); ok && uuidStr != "" {
				// Parse and validate existing UUID
				parsedUUID, err := uuid.Parse(uuidStr)
				if err != nil {
					c.Logger.Warn("invalid uuid in document metadata, generating new one",
						"document_id", doc.Document.ID,
						"name", doc.Document.Name,
						"invalid_uuid", uuidStr,
					)
				} else {
					// Valid UUID exists
					doc.DocumentUUID = parsedUUID
					c.Logger.Debug("document already has uuid",
						"document_id", doc.Document.ID,
						"uuid", parsedUUID.String(),
					)
					return nil
				}
			}
		}
	}

	// Generate new UUID
	newUUID := uuid.New()
	doc.DocumentUUID = newUUID

	c.Logger.Info("assigning new uuid to document",
		"document_id", doc.Document.ID,
		"name", doc.Document.Name,
		"uuid", newUUID.String(),
	)

	// Write UUID back to document metadata using UpdateDocument
	updates := &workspace.DocumentUpdate{
		Metadata: map[string]any{
			"hermes_uuid": newUUID.String(),
		},
	}

	_, err := c.Provider.UpdateDocument(ctx, doc.Document.ID, updates)
	if err != nil {
		return fmt.Errorf("failed to update document metadata with uuid: %w", err)
	}

	// Update local document metadata
	if doc.Document.Metadata == nil {
		doc.Document.Metadata = make(map[string]any)
	}
	doc.Document.Metadata["hermes_uuid"] = newUUID.String()

	doc.SetCustom("uuid_assigned", true)
	return nil
}

// ExecuteBatch implements BatchCommand for parallel UUID assignment.
func (c *AssignUUIDCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	// UUID assignment can be done in parallel
	return indexer.ParallelProcess(ctx, docs, c.Execute, 5)
}
