package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	docpkg "github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

func TestUpdateReviewStateWithSearchOutbox(t *testing.T) {
	db := setupDocumentSearchOutboxTestDB(t)
	require.NoError(t, db.Create(&models.DocumentType{Name: "RFC", LongName: "RFC"}).Error)
	require.NoError(t, db.Create(&models.Product{Name: "Hermes", Abbreviation: "H"}).Error)
	require.NoError(t, db.Create(&models.User{EmailAddress: "owner@example.com"}).Error)
	require.NoError(t, db.Create(&models.User{EmailAddress: "approver@example.com"}).Error)

	model := &models.Document{
		GoogleFileID:       "doc-review-1",
		Title:              "Review Document",
		DocumentNumber:     2,
		DocumentCreatedAt:  time.Now().UTC(),
		DocumentModifiedAt: time.Now().UTC(),
		Status:             models.InReviewDocumentStatus,
		Imported:           true,
		DocumentType:       models.DocumentType{Name: "RFC"},
		Product:            models.Product{Name: "Hermes", Abbreviation: "H"},
		Owner:              &models.User{EmailAddress: "owner@example.com"},
		Approvers:          []*models.User{{EmailAddress: "approver@example.com"}},
		ApproverGroups:     []*models.Group{},
		Contributors:       []*models.User{},
		CustomFields:       []*models.DocumentCustomField{},
		RelatedResources:   []*models.DocumentRelatedResource{},
		FileRevisions:      []models.DocumentFileRevision{},
	}
	require.NoError(t, model.Upsert(db))

	doc := &docpkg.Document{
		ObjectID:      "doc-review-1",
		Title:         "Review Document",
		DocNumber:     "H-002",
		DocType:       "RFC",
		Product:       "Hermes",
		Status:        "In-Review",
		Owners:        []string{"owner@example.com"},
		Approvers:     []string{"approver@example.com"},
		ApprovedBy:    []string{"approver@example.com"},
		Contributors:  []string{},
		FileRevisions: map[string]string{"rev-1": "Approved by approver@example.com"},
		ModifiedTime:  time.Now().Unix(),
		CreatedTime:   time.Now().Unix(),
	}
	fr := &models.DocumentFileRevision{
		Document: models.Document{
			GoogleFileID: "doc-review-1",
		},
		GoogleDriveFileRevisionID: "rev-1",
		Name:                      "Approved by approver@example.com",
	}

	require.NoError(t, updateReviewStateWithSearchOutbox(db, fr, doc))

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, models.SearchEventReviewStateChanged, events[0].EventType)
	assert.Equal(t, models.SearchAggregateDocument, events[0].AggregateType)
	assert.Equal(t, models.SearchIndexDocuments, events[0].IndexName)
	assert.Equal(t, models.SearchOutboxOperationUpsert, events[0].Operation)
	assert.Equal(t, int64(1), events[0].Sequence)
	assert.Equal(t, "Review Document", events[0].Payload["title"])

	var review models.DocumentReview
	require.NoError(t, db.First(&review).Error)
	assert.Equal(t, models.ApprovedDocumentReviewStatus, review.Status)

	var revision models.DocumentFileRevision
	require.NoError(t, db.First(&revision).Error)
	assert.Equal(t, "rev-1", revision.GoogleDriveFileRevisionID)
}
