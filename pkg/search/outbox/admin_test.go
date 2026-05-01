package outbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

func setupAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SearchOutboxSequence{}, &models.SearchOutboxEvent{}))
	return db
}

func createAdminTestEvent(t *testing.T, db *gorm.DB, status string) models.SearchOutboxEvent {
	t.Helper()

	event := models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Sequence:      time.Now().UnixNano(),
		Status:        status,
		ErrorMessage:  "boom",
		Payload: map[string]any{
			"objectID": "doc-1",
		},
	}
	require.NoError(t, db.Create(&event).Error)
	return event
}

func TestListFailedEvents(t *testing.T) {
	db := setupAdminTestDB(t)
	failed := createAdminTestEvent(t, db, models.SearchOutboxStatusFailed)
	dlq := createAdminTestEvent(t, db, models.SearchOutboxStatusDLQ)
	createAdminTestEvent(t, db, models.SearchOutboxStatusPending)

	events, err := ListFailedEvents(db, 10)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.ElementsMatch(t, []uint{failed.ID, dlq.ID}, []uint{events[0].ID, events[1].ID})
}

func TestGetStats(t *testing.T) {
	db := setupAdminTestDB(t)
	now := time.Now().UTC()
	pending := createAdminTestEvent(t, db, models.SearchOutboxStatusPending)
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).Where("id = ?", pending.ID).Update("available_at", now.Add(-2*time.Minute)).Error)
	failed := createAdminTestEvent(t, db, models.SearchOutboxStatusFailed)
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).Where("id = ?", failed.ID).Update("available_at", now.Add(-time.Minute)).Error)
	createAdminTestEvent(t, db, models.SearchOutboxStatusDLQ)
	createAdminTestEvent(t, db, models.SearchOutboxStatusSkipped)

	stats, err := GetStats(db, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Pending)
	assert.Equal(t, int64(1), stats.Failed)
	assert.Equal(t, int64(1), stats.DLQ)
	assert.Equal(t, int64(1), stats.Skipped)
	assert.Equal(t, int64(120), stats.OldestPendingAgeSeconds)
}

func TestRetryEvent(t *testing.T) {
	db := setupAdminTestDB(t)
	event := createAdminTestEvent(t, db, models.SearchOutboxStatusDLQ)
	now := time.Now().UTC()

	require.NoError(t, RetryEvent(db, event.ID, now))

	var reloaded models.SearchOutboxEvent
	require.NoError(t, db.First(&reloaded, event.ID).Error)
	assert.Equal(t, models.SearchOutboxStatusPending, reloaded.Status)
	assert.Equal(t, "", reloaded.ErrorMessage)
	assert.WithinDuration(t, now, reloaded.AvailableAt, time.Second)
}

func TestSkipEvent(t *testing.T) {
	db := setupAdminTestDB(t)
	event := createAdminTestEvent(t, db, models.SearchOutboxStatusFailed)

	require.NoError(t, SkipEvent(db, event.ID, "bad payload superseded", time.Now().UTC()))

	var reloaded models.SearchOutboxEvent
	require.NoError(t, db.First(&reloaded, event.ID).Error)
	assert.Equal(t, models.SearchOutboxStatusSkipped, reloaded.Status)
	assert.Contains(t, reloaded.ErrorMessage, "bad payload superseded")
}

func TestSkipEventRequiresNote(t *testing.T) {
	db := setupAdminTestDB(t)
	event := createAdminTestEvent(t, db, models.SearchOutboxStatusDLQ)

	err := SkipEvent(db, event.ID, "", time.Now().UTC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator note is required")
}

func TestRebuildCurrentDocument(t *testing.T) {
	db := setupAdminTestDB(t)
	autoMigrateDocumentProjectionTables(t, db)
	require.NoError(t, db.Create(&models.DocumentType{Name: "RFC", LongName: "RFC"}).Error)
	require.NoError(t, db.Create(&models.Product{Name: "Hermes", Abbreviation: "H"}).Error)
	doc := &models.Document{
		GoogleFileID:       "doc-rebuild-1",
		Title:              "Rebuild Me",
		DocumentNumber:     4,
		DocumentCreatedAt:  time.Now().UTC(),
		DocumentModifiedAt: time.Now().UTC(),
		Status:             models.InReviewDocumentStatus,
		DocumentType:       models.DocumentType{Name: "RFC"},
		Product:            models.Product{Name: "Hermes", Abbreviation: "H"},
		Owner:              &models.User{EmailAddress: "owner@example.com"},
		Contributors:       []*models.User{},
		Approvers:          []*models.User{},
		ApproverGroups:     []*models.Group{},
		CustomFields:       []*models.DocumentCustomField{},
		RelatedResources:   []*models.DocumentRelatedResource{},
		FileRevisions:      []models.DocumentFileRevision{},
	}
	require.NoError(t, doc.Upsert(db))
	original := createAdminTestEvent(t, db, models.SearchOutboxStatusDLQ)
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).Where("id = ?", original.ID).Updates(map[string]any{
		"aggregate_id":   "doc-rebuild-1",
		"aggregate_type": models.SearchAggregateDocument,
		"index_name":     models.SearchIndexDocuments,
	}).Error)

	rebuilt, err := RebuildCurrent(db, original.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SearchEventBackfill, rebuilt.EventType)
	assert.Equal(t, models.SearchOutboxOperationUpsert, rebuilt.Operation)
	assert.Equal(t, "Rebuild Me", rebuilt.Payload["title"])
}

func TestRebuildCurrentMissingAggregateEnqueuesDelete(t *testing.T) {
	db := setupAdminTestDB(t)
	autoMigrateDocumentProjectionTables(t, db)
	original := createAdminTestEvent(t, db, models.SearchOutboxStatusDLQ)

	rebuilt, err := RebuildCurrent(db, original.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SearchEventBackfill, rebuilt.EventType)
	assert.Equal(t, models.SearchOutboxOperationDelete, rebuilt.Operation)
}

func autoMigrateDocumentProjectionTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&models.DocumentType{},
		&models.Document{},
		&models.DocumentCustomField{},
		&models.DocumentFileRevision{},
		&models.DocumentGroupReview{},
		&models.DocumentRelatedResource{},
		&models.DocumentRelatedResourceExternalLink{},
		&models.DocumentRelatedResourceHermesDocument{},
		&models.DocumentReview{},
		&models.DocumentTypeCustomField{},
		&models.Group{},
		&models.Product{},
		&models.User{},
	))
}
