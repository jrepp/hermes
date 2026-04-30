package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	docpkg "github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

func TestEnqueueReviewCreatedSearchOutbox(t *testing.T) {
	db := setupDocumentSearchOutboxTestDB(t)
	doc := &docpkg.Document{
		ObjectID:     "doc-publish-1",
		Title:        "Published Document",
		DocNumber:    "H-003",
		DocType:      "RFC",
		Product:      "Hermes",
		Status:       "In-Review",
		Owners:       []string{"owner@example.com"},
		Approvers:    []string{"approver@example.com"},
		Contributors: []string{},
		ModifiedTime: time.Now().Unix(),
		CreatedTime:  time.Now().Unix(),
	}

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return enqueueReviewCreatedSearchOutbox(tx, doc)
	}))

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Order("id ASC").Find(&events).Error)
	require.Len(t, events, 3)

	assert.Equal(t, models.SearchEventDocumentPublished, events[0].EventType)
	assert.Equal(t, models.SearchAggregateDocument, events[0].AggregateType)
	assert.Equal(t, models.SearchIndexDocuments, events[0].IndexName)
	assert.Equal(t, models.SearchOutboxOperationUpsert, events[0].Operation)
	assert.Equal(t, int64(1), events[0].Sequence)
	assert.Equal(t, "Published Document", events[0].Payload["title"])

	assert.Equal(t, models.SearchEventDraftPublished, events[1].EventType)
	assert.Equal(t, models.SearchAggregateDraft, events[1].AggregateType)
	assert.Equal(t, models.SearchIndexDrafts, events[1].IndexName)
	assert.Equal(t, models.SearchOutboxOperationDelete, events[1].Operation)
	assert.Equal(t, int64(1), events[1].Sequence)
	assert.Empty(t, events[1].Payload)

	assert.Equal(t, models.SearchEventLinkCreated, events[2].EventType)
	assert.Equal(t, models.SearchAggregateLink, events[2].AggregateType)
	assert.Equal(t, models.SearchIndexLinks, events[2].IndexName)
	assert.Equal(t, models.SearchOutboxOperationUpsert, events[2].Operation)
	assert.Equal(t, int64(1), events[2].Sequence)
	assert.Equal(t, "/rfc/h-003", events[2].AggregateID)
	assert.Equal(t, "doc-publish-1", events[2].Payload["documentID"])
}
