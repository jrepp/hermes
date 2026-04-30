package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	docpkg "github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

func setupDocumentSearchOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
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
		&models.SearchOutboxSequence{},
		&models.SearchOutboxEvent{},
	))

	return db
}

func TestUpsertDocumentWithSearchOutbox(t *testing.T) {
	db := setupDocumentSearchOutboxTestDB(t)
	require.NoError(t, db.Create(&models.DocumentType{Name: "RFC", LongName: "RFC"}).Error)
	require.NoError(t, db.Create(&models.Product{Name: "Hermes", Abbreviation: "H"}).Error)

	model := &models.Document{
		GoogleFileID:       "doc-1",
		Title:              "Document One",
		DocumentNumber:     1,
		DocumentCreatedAt:  time.Now().UTC(),
		DocumentModifiedAt: time.Now().UTC(),
		Status:             models.InReviewDocumentStatus,
		Imported:           true,
		DocumentType:       models.DocumentType{Name: "RFC"},
		Product:            models.Product{Name: "Hermes", Abbreviation: "H"},
		Owner:              &models.User{EmailAddress: "owner@example.com"},
		Approvers:          []*models.User{{EmailAddress: "approver@example.com"}},
		Contributors:       []*models.User{{EmailAddress: "contributor@example.com"}},
		ApproverGroups:     []*models.Group{},
		CustomFields:       []*models.DocumentCustomField{},
		RelatedResources:   []*models.DocumentRelatedResource{},
		FileRevisions:      []models.DocumentFileRevision{},
	}
	doc := &docpkg.Document{
		ObjectID:     "doc-1",
		Title:        "Document One",
		DocNumber:    "H-001",
		DocType:      "RFC",
		Product:      "Hermes",
		Status:       "In-Review",
		Owners:       []string{"owner@example.com"},
		Approvers:    []string{"approver@example.com"},
		Contributors: []string{"contributor@example.com"},
		ModifiedTime: time.Now().Unix(),
	}

	require.NoError(t, upsertDocumentWithSearchOutbox(db, model, doc))

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, models.SearchEventDocumentUpdated, events[0].EventType)
	assert.Equal(t, models.SearchAggregateDocument, events[0].AggregateType)
	assert.Equal(t, models.SearchIndexDocuments, events[0].IndexName)
	assert.Equal(t, models.SearchOutboxOperationUpsert, events[0].Operation)
	assert.Equal(t, int64(1), events[0].Sequence)
	assert.Equal(t, "Document One", events[0].Payload["title"])
	assert.Equal(t, "doc-1", events[0].Payload["objectID"])
}
