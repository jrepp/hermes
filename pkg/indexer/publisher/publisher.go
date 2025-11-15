package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// Publisher publishes document revision events to the outbox.
// This should be called within the same transaction as document/revision updates
// to ensure transactional consistency.
type Publisher struct {
	db     *gorm.DB
	logger hclog.Logger
}

// New creates a new Publisher.
func New(db *gorm.DB, logger hclog.Logger) *Publisher {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	return &Publisher{
		db:     db,
		logger: logger.Named("outbox-publisher"),
	}
}

// PublishRevisionCreated publishes a revision.created event to the outbox.
// Should be called in the same transaction as the revision insert.
func (p *Publisher) PublishRevisionCreated(ctx context.Context, tx *gorm.DB, revision *models.DocumentRevision, metadata map[string]interface{}) error {
	return p.publishRevisionEvent(ctx, tx, revision, models.RevisionEventCreated, metadata)
}

// PublishRevisionUpdated publishes a revision.updated event to the outbox.
// Should be called in the same transaction as the revision update.
func (p *Publisher) PublishRevisionUpdated(ctx context.Context, tx *gorm.DB, revision *models.DocumentRevision, metadata map[string]interface{}) error {
	return p.publishRevisionEvent(ctx, tx, revision, models.RevisionEventUpdated, metadata)
}

// PublishRevisionDeleted publishes a revision.deleted event to the outbox.
// Should be called in the same transaction as the revision deletion.
func (p *Publisher) PublishRevisionDeleted(ctx context.Context, tx *gorm.DB, revision *models.DocumentRevision, metadata map[string]interface{}) error {
	return p.publishRevisionEvent(ctx, tx, revision, models.RevisionEventDeleted, metadata)
}

// publishRevisionEvent is the internal method that creates and saves the outbox entry.
func (p *Publisher) publishRevisionEvent(ctx context.Context, tx *gorm.DB, revision *models.DocumentRevision, eventType string, metadata map[string]interface{}) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}

	// Build the event payload
	payload, err := p.buildPayload(revision, metadata)
	if err != nil {
		return fmt.Errorf("failed to build payload: %w", err)
	}

	// Create outbox entry
	outboxEntry, err := models.NewRevisionOutboxEntry(revision, eventType, payload)
	if err != nil {
		return fmt.Errorf("failed to create outbox entry: %w", err)
	}

	// Check for duplicate (idempotency)
	existing, err := models.GetOutboxByIdempotentKey(tx, outboxEntry.IdempotentKey)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check for existing outbox entry: %w", err)
	}

	if existing != nil {
		// Event already published, skip
		p.logger.Debug("skipping duplicate outbox entry",
			"idempotent_key", outboxEntry.IdempotentKey,
			"existing_id", existing.ID,
		)
		return nil
	}

	// Save outbox entry in the same transaction
	if err := tx.Create(outboxEntry).Error; err != nil {
		return fmt.Errorf("failed to create outbox entry: %w", err)
	}

	p.logger.Debug("published revision event to outbox",
		"event_type", eventType,
		"document_uuid", revision.DocumentUUID,
		"revision_id", revision.ID,
		"outbox_id", outboxEntry.ID,
		"idempotent_key", outboxEntry.IdempotentKey,
	)

	return nil
}

// buildPayload constructs the event payload from revision and metadata.
func (p *Publisher) buildPayload(revision *models.DocumentRevision, metadata map[string]interface{}) (map[string]interface{}, error) {
	payload := make(map[string]interface{})

	// Serialize revision to JSON
	revisionData, err := json.Marshal(revision)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal revision: %w", err)
	}

	var revisionMap map[string]interface{}
	if err := json.Unmarshal(revisionData, &revisionMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal revision: %w", err)
	}

	payload["revision"] = revisionMap

	// Add metadata if provided
	if metadata != nil {
		payload["metadata"] = metadata
	}

	return payload, nil
}

// PublishBatch publishes multiple revision events in a single transaction.
// Useful for bulk operations.
func (p *Publisher) PublishBatch(ctx context.Context, tx *gorm.DB, events []RevisionEvent) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}

	for _, event := range events {
		if err := p.publishRevisionEvent(ctx, tx, event.Revision, event.EventType, event.Metadata); err != nil {
			return fmt.Errorf("failed to publish event for revision %d: %w", event.Revision.ID, err)
		}
	}

	return nil
}

// RevisionEvent represents a revision event to be published.
type RevisionEvent struct {
	Revision  *models.DocumentRevision
	EventType string
	Metadata  map[string]interface{}
}

// WithTransaction wraps a function in a database transaction and publishes events.
// This is a convenience method for handlers that need to update a revision and publish an event.
func (p *Publisher) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) (*models.DocumentRevision, string, map[string]interface{}, error)) error {
	return p.db.Transaction(func(tx *gorm.DB) error {
		// Execute the user function
		revision, eventType, metadata, err := fn(tx)
		if err != nil {
			return err
		}

		// Publish the event
		return p.publishRevisionEvent(ctx, tx, revision, eventType, metadata)
	})
}

// Example usage in an API handler:
//
// publisher := publisher.New(db, logger)
//
// err := publisher.WithTransaction(ctx, func(tx *gorm.DB) (*models.DocumentRevision, string, map[string]interface{}, error) {
//     // Create or update revision
//     revision := &models.DocumentRevision{
//         DocumentUUID: docUUID,
//         DocumentID:   docID,
//         ProviderType: "google",
//         Title:        "RFC-088",
//         ContentHash:  computedHash,
//         ModifiedTime: time.Now(),
//         Status:       "active",
//     }
//
//     if err := tx.Create(revision).Error; err != nil {
//         return nil, "", nil, err
//     }
//
//     // Prepare metadata
//     metadata := map[string]interface{}{
//         "document_type": "RFC",
//         "product":       "Hermes",
//         "status":        "In-Review",
//     }
//
//     return revision, models.RevisionEventCreated, metadata, nil
// })

// PublishFromDocument is a helper that creates a revision from a document and publishes it.
// This is useful when you have a Document model and want to create a revision event.
func (p *Publisher) PublishFromDocument(ctx context.Context, tx *gorm.DB, documentUUID uuid.UUID, documentID, providerType, title, contentHash string, metadata map[string]interface{}) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}

	// Create or find existing revision
	revision := &models.DocumentRevision{
		DocumentUUID: documentUUID,
		DocumentID:   documentID,
		ProviderType: providerType,
		Title:        title,
		ContentHash:  contentHash,
		Status:       "active",
	}

	// Check if revision already exists for this content hash
	var existing models.DocumentRevision
	err := tx.Where("document_uuid = ? AND content_hash = ?", documentUUID, contentHash).
		First(&existing).Error

	if err == nil {
		// Revision with same content hash already exists, use it
		revision = &existing
		p.logger.Debug("using existing revision",
			"document_uuid", documentUUID,
			"content_hash", contentHash,
			"revision_id", existing.ID,
		)
	} else if err == gorm.ErrRecordNotFound {
		// Create new revision
		if err := tx.Create(revision).Error; err != nil {
			return fmt.Errorf("failed to create revision: %w", err)
		}
		p.logger.Debug("created new revision",
			"document_uuid", documentUUID,
			"content_hash", contentHash,
			"revision_id", revision.ID,
		)
	} else {
		return fmt.Errorf("failed to check for existing revision: %w", err)
	}

	// Publish the event
	return p.publishRevisionEvent(ctx, tx, revision, models.RevisionEventCreated, metadata)
}
