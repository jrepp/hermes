package publisher

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate tables
	err = db.AutoMigrate(
		&models.DocumentRevision{},
		&models.DocumentRevisionOutbox{},
	)
	require.NoError(t, err)

	return db
}

func TestPublisher_PublishRevisionCreated(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	pub := New(db, logger)

	ctx := context.Background()
	docUUID := uuid.New()

	// Create a revision
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc-123",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "abc123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}

	// Create revision in a transaction and publish event
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(revision).Error; err != nil {
			return err
		}

		metadata := map[string]interface{}{
			"document_type": "RFC",
			"product":       "Hermes",
		}

		return pub.PublishRevisionCreated(ctx, tx, revision, metadata)
	})

	require.NoError(t, err)

	// Verify outbox entry was created
	var outboxEntry models.DocumentRevisionOutbox
	err = db.First(&outboxEntry, "document_uuid = ?", docUUID).Error
	require.NoError(t, err)

	assert.Equal(t, revision.ID, outboxEntry.RevisionID)
	assert.Equal(t, docUUID, outboxEntry.DocumentUUID)
	assert.Equal(t, "test-doc-123", outboxEntry.DocumentID)
	assert.Equal(t, "abc123", outboxEntry.ContentHash)
	assert.Equal(t, models.RevisionEventCreated, outboxEntry.EventType)
	assert.Equal(t, "google", outboxEntry.ProviderType)
	assert.Equal(t, models.OutboxStatusPending, outboxEntry.Status)

	// Verify idempotent key
	expectedKey := models.GenerateIdempotentKey(docUUID, "abc123")
	assert.Equal(t, expectedKey, outboxEntry.IdempotentKey)

	// Verify payload contains revision and metadata
	assert.NotNil(t, outboxEntry.Payload)
	assert.Contains(t, outboxEntry.Payload, "revision")
	assert.Contains(t, outboxEntry.Payload, "metadata")
}

func TestPublisher_Idempotency(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	pub := New(db, logger)

	ctx := context.Background()
	docUUID := uuid.New()

	// Create a revision
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc-456",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "def456",
		Status:       "active",
		ModifiedTime: time.Now(),
	}

	metadata := map[string]interface{}{
		"document_type": "RFC",
	}

	// Publish the same event twice
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(revision).Error; err != nil {
			return err
		}
		return pub.PublishRevisionCreated(ctx, tx, revision, metadata)
	})
	require.NoError(t, err)

	// Try to publish again (should be idempotent)
	err = db.Transaction(func(tx *gorm.DB) error {
		return pub.PublishRevisionCreated(ctx, tx, revision, metadata)
	})
	require.NoError(t, err)

	// Verify only one outbox entry exists
	var count int64
	err = db.Model(&models.DocumentRevisionOutbox{}).
		Where("document_uuid = ?", docUUID).
		Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestPublisher_MultipleEvents(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	pub := New(db, logger)

	ctx := context.Background()
	docUUID := uuid.New()

	// Create initial revision
	revision1 := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc-789",
		ProviderType: "google",
		Title:        "Test Document V1",
		ContentHash:  "hash1",
		Status:       "active",
		ModifiedTime: time.Now(),
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(revision1).Error; err != nil {
			return err
		}
		return pub.PublishRevisionCreated(ctx, tx, revision1, nil)
	})
	require.NoError(t, err)

	// Create updated revision with different content hash
	revision2 := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc-789",
		ProviderType: "google",
		Title:        "Test Document V2",
		ContentHash:  "hash2", // Different hash
		Status:       "active",
		ModifiedTime: time.Now(),
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(revision2).Error; err != nil {
			return err
		}
		return pub.PublishRevisionUpdated(ctx, tx, revision2, nil)
	})
	require.NoError(t, err)

	// Verify two outbox entries exist (different content hashes)
	var count int64
	err = db.Model(&models.DocumentRevisionOutbox{}).
		Where("document_uuid = ?", docUUID).
		Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Verify event types
	var entries []models.DocumentRevisionOutbox
	err = db.Where("document_uuid = ?", docUUID).
		Order("created_at ASC").
		Find(&entries).Error
	require.NoError(t, err)

	assert.Equal(t, models.RevisionEventCreated, entries[0].EventType)
	assert.Equal(t, models.RevisionEventUpdated, entries[1].EventType)
}

func TestPublisher_PublishFromDocument(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	pub := New(db, logger)

	ctx := context.Background()
	docUUID := uuid.New()

	metadata := map[string]interface{}{
		"document_type": "PRD",
		"status":        "draft",
	}

	// Publish from document (should create revision + outbox entry)
	err := db.Transaction(func(tx *gorm.DB) error {
		return pub.PublishFromDocument(
			ctx,
			tx,
			docUUID,
			"doc-abc",
			"local",
			"My PRD",
			"contentHash123",
			metadata,
		)
	})
	require.NoError(t, err)

	// Verify revision was created
	var revision models.DocumentRevision
	err = db.First(&revision, "document_uuid = ?", docUUID).Error
	require.NoError(t, err)

	assert.Equal(t, "doc-abc", revision.DocumentID)
	assert.Equal(t, "local", revision.ProviderType)
	assert.Equal(t, "My PRD", revision.Title)
	assert.Equal(t, "contentHash123", revision.ContentHash)

	// Verify outbox entry was created
	var outboxEntry models.DocumentRevisionOutbox
	err = db.First(&outboxEntry, "document_uuid = ?", docUUID).Error
	require.NoError(t, err)

	assert.Equal(t, revision.ID, outboxEntry.RevisionID)
	assert.Equal(t, models.RevisionEventCreated, outboxEntry.EventType)
}

func TestPublisher_PublishFromDocument_Idempotency(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	pub := New(db, logger)

	ctx := context.Background()
	docUUID := uuid.New()

	// Publish same document twice with same content hash
	err := db.Transaction(func(tx *gorm.DB) error {
		return pub.PublishFromDocument(ctx, tx, docUUID, "doc-xyz", "local", "Title", "hash1", nil)
	})
	require.NoError(t, err)

	err = db.Transaction(func(tx *gorm.DB) error {
		return pub.PublishFromDocument(ctx, tx, docUUID, "doc-xyz", "local", "Title", "hash1", nil)
	})
	require.NoError(t, err)

	// Should only have one revision (reused)
	var revisionCount int64
	err = db.Model(&models.DocumentRevision{}).
		Where("document_uuid = ?", docUUID).
		Count(&revisionCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), revisionCount)

	// Should only have one outbox entry (idempotent)
	var outboxCount int64
	err = db.Model(&models.DocumentRevisionOutbox{}).
		Where("document_uuid = ?", docUUID).
		Count(&outboxCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), outboxCount)
}
