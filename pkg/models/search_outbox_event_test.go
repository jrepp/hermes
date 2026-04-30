package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSearchOutboxEventValidation(t *testing.T) {
	t.Run("enqueue requires transaction", func(t *testing.T) {
		err := EnqueueSearchOutboxEvent(nil, &SearchOutboxEvent{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transaction handle is required")
	})

	t.Run("enqueue requires event", func(t *testing.T) {
		err := EnqueueSearchOutboxEvent(&gorm.DB{}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "search outbox event is required")
	})

	t.Run("generates default idempotency key", func(t *testing.T) {
		key := GenerateSearchOutboxIdempotencyKey(
			SearchIndexDocuments,
			"doc-1",
			SearchEventDocumentUpdated,
			7,
		)
		assert.Equal(t, "documents:doc-1:document.updated:7", key)
	})

	t.Run("before create validates required fields", func(t *testing.T) {
		event := &SearchOutboxEvent{}
		err := event.BeforeCreate(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "event_type is required")
	})

	t.Run("before create sets defaults", func(t *testing.T) {
		event := &SearchOutboxEvent{
			EventType:     SearchEventDocumentUpdated,
			AggregateID:   "doc-1",
			AggregateType: SearchAggregateDocument,
			IndexName:     SearchIndexDocuments,
			Operation:     SearchOutboxOperationUpsert,
			Sequence:      1,
		}

		require.NoError(t, event.BeforeCreate(nil))
		assert.Equal(t, SearchOutboxStatusPending, event.Status)
		assert.False(t, event.AvailableAt.IsZero())
		assert.Equal(t, "documents:doc-1:document.updated:1", event.IdempotencyKey)
	})

	t.Run("sequence enqueue rejects preset sequence", func(t *testing.T) {
		event := &SearchOutboxEvent{
			AggregateType: SearchAggregateDocument,
			AggregateID:   "doc-1",
			Sequence:      1,
		}
		err := validateSearchOutboxEventForSequencedEnqueue(event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sequence must be allocated by enqueue helper")
	})
}

func TestSearchOutboxEventPersistence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SearchOutboxSequence{}, &SearchOutboxEvent{}))

	t.Run("enqueue validates required fields", func(t *testing.T) {
		tx := db.Begin()
		require.NoError(t, tx.Error)
		defer tx.Rollback()

		err := EnqueueSearchOutboxEvent(tx, &SearchOutboxEvent{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "event_type is required")
	})

	t.Run("enqueue sets defaults", func(t *testing.T) {
		tx := db.Begin()
		require.NoError(t, tx.Error)
		defer tx.Rollback()

		event := &SearchOutboxEvent{
			EventType:     SearchEventDocumentUpdated,
			AggregateID:   "doc-1",
			AggregateType: SearchAggregateDocument,
			IndexName:     SearchIndexDocuments,
			Operation:     SearchOutboxOperationUpsert,
			Sequence:      1,
			Payload: map[string]any{
				"objectID": "doc-1",
			},
		}

		require.NoError(t, EnqueueSearchOutboxEvent(tx, event))
		assert.NotZero(t, event.ID)
		assert.Equal(t, SearchOutboxStatusPending, event.Status)
		assert.False(t, event.AvailableAt.IsZero())
		assert.Equal(t, "documents:doc-1:document.updated:1", event.IdempotencyKey)
	})

	t.Run("duplicate idempotency key fails", func(t *testing.T) {
		tx := db.Begin()
		require.NoError(t, tx.Error)
		defer tx.Rollback()

		event := func(aggregateID string) *SearchOutboxEvent {
			return &SearchOutboxEvent{
				EventType:      SearchEventDraftDeleted,
				AggregateID:    aggregateID,
				AggregateType:  SearchAggregateDraft,
				IndexName:      SearchIndexDrafts,
				Operation:      SearchOutboxOperationDelete,
				Sequence:       1,
				IdempotencyKey: "draft-delete-key",
			}
		}

		require.NoError(t, EnqueueSearchOutboxEvent(tx, event("draft-1")))
		err := EnqueueSearchOutboxEvent(tx, event("draft-2"))
		require.Error(t, err)
	})

	t.Run("allocates per aggregate sequence", func(t *testing.T) {
		tx := db.Begin()
		require.NoError(t, tx.Error)
		defer tx.Rollback()

		first, err := AllocateSearchOutboxSequence(tx, SearchAggregateDocument, "doc-seq")
		require.NoError(t, err)
		second, err := AllocateSearchOutboxSequence(tx, SearchAggregateDocument, "doc-seq")
		require.NoError(t, err)
		other, err := AllocateSearchOutboxSequence(tx, SearchAggregateProject, "project-seq")
		require.NoError(t, err)

		assert.Equal(t, int64(1), first)
		assert.Equal(t, int64(2), second)
		assert.Equal(t, int64(1), other)
	})

	t.Run("enqueue with sequence allocates idempotency key", func(t *testing.T) {
		tx := db.Begin()
		require.NoError(t, tx.Error)
		defer tx.Rollback()

		first := &SearchOutboxEvent{
			EventType:     SearchEventProjectCreated,
			AggregateID:   "project-1",
			AggregateType: SearchAggregateProject,
			IndexName:     SearchIndexProjects,
			Operation:     SearchOutboxOperationUpsert,
		}
		second := &SearchOutboxEvent{
			EventType:     SearchEventProjectUpdated,
			AggregateID:   "project-1",
			AggregateType: SearchAggregateProject,
			IndexName:     SearchIndexProjects,
			Operation:     SearchOutboxOperationUpsert,
		}

		require.NoError(t, EnqueueSearchOutboxEventWithSequence(tx, first))
		require.NoError(t, EnqueueSearchOutboxEventWithSequence(tx, second))

		assert.Equal(t, int64(1), first.Sequence)
		assert.Equal(t, int64(2), second.Sequence)
		assert.Equal(t, "projects:project-1:project.created:1", first.IdempotencyKey)
		assert.Equal(t, "projects:project-1:project.updated:2", second.IdempotencyKey)
	})
}
