package relay

import (
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

// createTestOutboxEntry creates a test outbox entry.
func createTestOutboxEntry(t *testing.T, db *gorm.DB) *models.DocumentRevisionOutbox {
	docUUID := uuid.New()

	// Create revision first
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc",
		ProviderType: "google",
		Title:        "Test",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Create outbox entry
	entry := &models.DocumentRevisionOutbox{
		RevisionID:    revision.ID,
		DocumentUUID:  docUUID,
		DocumentID:    "test-doc",
		IdempotentKey: models.GenerateIdempotentKey(docUUID, "hash123"),
		ContentHash:   "hash123",
		EventType:     models.RevisionEventCreated,
		ProviderType:  "google",
		Payload: map[string]interface{}{
			"test": "data",
		},
		Status: models.OutboxStatusPending,
	}
	require.NoError(t, db.Create(entry).Error)

	return entry
}

func TestRelay_GetStats(t *testing.T) {
	db := setupTestDB(t)

	// Note: In a real test, we'd create a Relay with a mocked Kafka client
	// For now, we'll test the stats directly on the database

	// Create some test entries
	createTestOutboxEntry(t, db)
	createTestOutboxEntry(t, db)

	// Mark one as published
	var entry models.DocumentRevisionOutbox
	require.NoError(t, db.First(&entry).Error)
	require.NoError(t, entry.MarkAsPublished(db))

	// Get stats
	pending, err := models.CountOutboxByStatus(db, models.OutboxStatusPending)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pending)

	published, err := models.CountOutboxByStatus(db, models.OutboxStatusPublished)
	require.NoError(t, err)
	assert.Equal(t, int64(1), published)

	failed, err := models.CountOutboxByStatus(db, models.OutboxStatusFailed)
	require.NoError(t, err)
	assert.Equal(t, int64(0), failed)
}

func TestOutboxEntry_MarkAsPublished(t *testing.T) {
	db := setupTestDB(t)
	entry := createTestOutboxEntry(t, db)

	// Mark as published
	err := entry.MarkAsPublished(db)
	require.NoError(t, err)

	// Reload from database
	var reloaded models.DocumentRevisionOutbox
	err = db.First(&reloaded, entry.ID).Error
	require.NoError(t, err)

	assert.Equal(t, models.OutboxStatusPublished, reloaded.Status)
	assert.NotNil(t, reloaded.PublishedAt)
}

func TestOutboxEntry_MarkAsFailed(t *testing.T) {
	db := setupTestDB(t)
	entry := createTestOutboxEntry(t, db)

	// Mark as failed
	testErr := assert.AnError
	err := entry.MarkAsFailed(db, testErr)
	require.NoError(t, err)

	// Reload from database
	var reloaded models.DocumentRevisionOutbox
	err = db.First(&reloaded, entry.ID).Error
	require.NoError(t, err)

	assert.Equal(t, models.OutboxStatusFailed, reloaded.Status)
	assert.Equal(t, 1, reloaded.PublishAttempts)
	assert.Contains(t, reloaded.LastError, testErr.Error())
}

func TestOutboxEntry_Retry(t *testing.T) {
	db := setupTestDB(t)
	entry := createTestOutboxEntry(t, db)

	// Mark as failed first
	err := entry.MarkAsFailed(db, assert.AnError)
	require.NoError(t, err)

	// Reload
	var reloaded models.DocumentRevisionOutbox
	err = db.First(&reloaded, entry.ID).Error
	require.NoError(t, err)
	assert.Equal(t, models.OutboxStatusFailed, reloaded.Status)

	// Retry
	err = reloaded.Retry(db)
	require.NoError(t, err)

	// Reload again
	err = db.First(&reloaded, entry.ID).Error
	require.NoError(t, err)

	assert.Equal(t, models.OutboxStatusPending, reloaded.Status)
	assert.Empty(t, reloaded.LastError)
}

func TestFindPendingOutboxEntries(t *testing.T) {
	db := setupTestDB(t)

	// Create multiple entries
	createTestOutboxEntry(t, db)
	createTestOutboxEntry(t, db)
	createTestOutboxEntry(t, db)

	// Mark one as published
	var entry models.DocumentRevisionOutbox
	require.NoError(t, db.First(&entry).Error)
	require.NoError(t, entry.MarkAsPublished(db))

	// Find pending entries
	pending, err := models.FindPendingOutboxEntries(db, 10)
	require.NoError(t, err)

	// Should have 2 pending entries
	assert.Len(t, pending, 2)

	for _, e := range pending {
		assert.Equal(t, models.OutboxStatusPending, e.Status)
	}
}

func TestFindPendingOutboxEntries_Limit(t *testing.T) {
	db := setupTestDB(t)

	// Create 5 entries
	for i := 0; i < 5; i++ {
		createTestOutboxEntry(t, db)
	}

	// Find with limit of 3
	pending, err := models.FindPendingOutboxEntries(db, 3)
	require.NoError(t, err)

	// Should respect limit
	assert.Len(t, pending, 3)
}

func TestDeleteOldPublishedEntries(t *testing.T) {
	db := setupTestDB(t)

	// Create an old entry
	entry := createTestOutboxEntry(t, db)
	err := entry.MarkAsPublished(db)
	require.NoError(t, err)

	// Manually set published_at to old time
	oldTime := time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	err = db.Model(&entry).Update("published_at", oldTime).Error
	require.NoError(t, err)

	// Create a recent entry
	recentEntry := createTestOutboxEntry(t, db)
	err = recentEntry.MarkAsPublished(db)
	require.NoError(t, err)

	// Delete entries older than 7 days
	deleted, err := models.DeleteOldPublishedEntries(db, 7*24*time.Hour)
	require.NoError(t, err)

	// Should have deleted 1 entry
	assert.Equal(t, int64(1), deleted)

	// Verify old entry is gone
	var count int64
	err = db.Model(&models.DocumentRevisionOutbox{}).Where("id = ?", entry.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Verify recent entry still exists
	err = db.Model(&models.DocumentRevisionOutbox{}).Where("id = ?", recentEntry.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestGetOutboxByIdempotentKey(t *testing.T) {
	db := setupTestDB(t)
	entry := createTestOutboxEntry(t, db)

	// Find by idempotent key
	found, err := models.GetOutboxByIdempotentKey(db, entry.IdempotentKey)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, entry.ID, found.ID)
	assert.Equal(t, entry.DocumentUUID, found.DocumentUUID)

	// Try to find non-existent key
	_, err = models.GetOutboxByIdempotentKey(db, "non-existent-key")
	assert.Error(t, err)
}

func TestGetFailedOutboxEntries(t *testing.T) {
	db := setupTestDB(t)

	// Create entries and mark some as failed
	entry1 := createTestOutboxEntry(t, db)
	entry2 := createTestOutboxEntry(t, db)
	createTestOutboxEntry(t, db) // Leave as pending

	require.NoError(t, entry1.MarkAsFailed(db, assert.AnError))
	require.NoError(t, entry2.MarkAsFailed(db, assert.AnError))

	// Get failed entries
	failed, err := models.GetFailedOutboxEntries(db, 10)
	require.NoError(t, err)

	assert.Len(t, failed, 2)
	for _, e := range failed {
		assert.Equal(t, models.OutboxStatusFailed, e.Status)
	}
}

// TestRelay_CleanupOldEntries tests the cleanup logic
func TestRelay_CleanupOldEntries(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()

	// Note: We can't easily test the full Relay without a real Kafka broker
	// But we can test the cleanup logic directly

	// Create old published entry
	entry := createTestOutboxEntry(t, db)
	err := entry.MarkAsPublished(db)
	require.NoError(t, err)

	// Set to 8 days ago
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	err = db.Model(&entry).Update("published_at", oldTime).Error
	require.NoError(t, err)

	// Run cleanup
	deleted, err := models.DeleteOldPublishedEntries(db, 7*24*time.Hour)
	require.NoError(t, err)

	logger.Info("cleanup completed", "deleted", deleted)
	assert.Equal(t, int64(1), deleted)
}
