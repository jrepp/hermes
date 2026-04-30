package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	docpkg "github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

func TestCreateDraftWithSearchOutbox(t *testing.T) {
	db := setupDocumentSearchOutboxTestDB(t)
	require.NoError(t, db.Create(&models.DocumentType{Name: "RFC", LongName: "RFC"}).Error)
	require.NoError(t, db.Create(&models.Product{Name: "Hermes", Abbreviation: "H"}).Error)

	model := testDraftModel("draft-1", "Draft One")
	doc := testDraftDocument("draft-1", "Draft One")

	require.NoError(t, createDraftWithSearchOutbox(db, model, doc))

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, models.SearchEventDraftCreated, events[0].EventType)
	assert.Equal(t, models.SearchAggregateDraft, events[0].AggregateType)
	assert.Equal(t, models.SearchIndexDrafts, events[0].IndexName)
	assert.Equal(t, models.SearchOutboxOperationUpsert, events[0].Operation)
	assert.Equal(t, int64(1), events[0].Sequence)
	assert.Equal(t, "Draft One", events[0].Payload["title"])

	var stored models.Document
	require.NoError(t, db.Where("google_file_id = ?", "draft-1").First(&stored).Error)
	assert.Equal(t, "Draft One", stored.Title)
}

func TestUpdateDraftWithSearchOutbox(t *testing.T) {
	db := setupDocumentSearchOutboxTestDB(t)
	require.NoError(t, db.Create(&models.DocumentType{Name: "RFC", LongName: "RFC"}).Error)
	require.NoError(t, db.Create(&models.Product{Name: "Hermes", Abbreviation: "H"}).Error)
	require.NoError(t, createDraftWithSearchOutbox(db, testDraftModel("draft-2", "Draft Two"), testDraftDocument("draft-2", "Draft Two")))

	model := testDraftModel("draft-2", "Draft Two Updated")
	doc := testDraftDocument("draft-2", "Draft Two Updated")

	require.NoError(t, updateDraftWithSearchOutbox(db, model, doc))

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Order("sequence ASC").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, models.SearchEventDraftCreated, events[0].EventType)
	assert.Equal(t, int64(1), events[0].Sequence)
	assert.Equal(t, models.SearchEventDraftUpdated, events[1].EventType)
	assert.Equal(t, int64(2), events[1].Sequence)
	assert.Equal(t, "Draft Two Updated", events[1].Payload["title"])
}

func TestDeleteDraftWithSearchOutbox(t *testing.T) {
	db := setupDocumentSearchOutboxTestDB(t)
	require.NoError(t, db.Create(&models.DocumentType{Name: "RFC", LongName: "RFC"}).Error)
	require.NoError(t, db.Create(&models.Product{Name: "Hermes", Abbreviation: "H"}).Error)
	require.NoError(t, createDraftWithSearchOutbox(db, testDraftModel("draft-3", "Draft Three"), testDraftDocument("draft-3", "Draft Three")))

	draft := &models.Document{GoogleFileID: "draft-3"}
	require.NoError(t, deleteDraftWithSearchOutbox(db, draft, "draft-3"))

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Order("sequence ASC").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, models.SearchEventDraftCreated, events[0].EventType)
	assert.Equal(t, models.SearchEventDraftDeleted, events[1].EventType)
	assert.Equal(t, models.SearchOutboxOperationDelete, events[1].Operation)
	assert.Equal(t, int64(2), events[1].Sequence)
	assert.Empty(t, events[1].Payload)
}

func testDraftModel(id, title string) *models.Document {
	now := time.Now().UTC()
	return &models.Document{
		GoogleFileID:       id,
		Title:              title,
		DocumentCreatedAt:  now,
		DocumentModifiedAt: now,
		Status:             models.WIPDocumentStatus,
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
}

func testDraftDocument(id, title string) *docpkg.Document {
	now := time.Now().Unix()
	return &docpkg.Document{
		ObjectID:     id,
		Title:        title,
		DocNumber:    "H-???",
		DocType:      "RFC",
		Product:      "Hermes",
		Status:       "WIP",
		Owners:       []string{"owner@example.com"},
		Approvers:    []string{},
		Contributors: []string{},
		CreatedTime:  now,
		ModifiedTime: now,
	}
}
