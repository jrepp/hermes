package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
)

func setupRelayTest(t *testing.T, provider search.Provider) (*gorm.DB, *Relay, *time.Time) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SearchOutboxSequence{}, &models.SearchOutboxEvent{}))

	now := time.Now().Add(time.Hour)
	relay, err := New(Config{
		DB:                db,
		Provider:          provider,
		Now:               func() time.Time { return now },
		BatchSize:         10,
		MaxRetries:        2,
		VisibilityTimeout: time.Minute,
		MaxRetryBackoff:   10 * time.Second,
	})
	require.NoError(t, err)

	return db, relay, &now
}

func enqueueRelayTestEvent(t *testing.T, db *gorm.DB, event *models.SearchOutboxEvent) *models.SearchOutboxEvent {
	t.Helper()

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return models.EnqueueSearchOutboxEventWithSequence(tx, event)
	}))
	return event
}

func reloadSearchOutboxEvent(t *testing.T, db *gorm.DB, id uint) models.SearchOutboxEvent {
	t.Helper()

	var event models.SearchOutboxEvent
	require.NoError(t, db.First(&event, id).Error)
	return event
}

func TestRelayProcessesDocumentUpsert(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, _ := setupRelayTest(t, provider)

	event := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"objectID": "doc-1",
			"docID":    "doc-1",
			"title":    "Doc One",
		},
	})

	require.NoError(t, relay.ProcessBatch(context.Background()))

	reloaded := reloadSearchOutboxEvent(t, db, event.ID)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloaded.Status)
	assert.NotNil(t, reloaded.CompletedAt)
	assert.Equal(t, []string{"doc-1"}, provider.documents.indexedIDs)
}

func TestRelayProcessesDraftDelete(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, _ := setupRelayTest(t, provider)

	event := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDraftDeleted,
		AggregateID:   "draft-1",
		AggregateType: models.SearchAggregateDraft,
		IndexName:     models.SearchIndexDrafts,
		Operation:     models.SearchOutboxOperationDelete,
	})

	require.NoError(t, relay.ProcessBatch(context.Background()))

	reloaded := reloadSearchOutboxEvent(t, db, event.ID)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloaded.Status)
	assert.Equal(t, []string{"draft-1"}, provider.drafts.deletedIDs)
}

func TestRelayProcessesLinkUpsert(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, _ := setupRelayTest(t, provider)

	event := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventLinkCreated,
		AggregateID:   "/rfc/h-003",
		AggregateType: models.SearchAggregateLink,
		IndexName:     models.SearchIndexLinks,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"documentID": "doc-1",
			"objectID":   "/rfc/h-003",
		},
	})

	require.NoError(t, relay.ProcessBatch(context.Background()))

	reloaded := reloadSearchOutboxEvent(t, db, event.ID)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloaded.Status)
	assert.Equal(t, []string{"/rfc/h-003"}, provider.links.savedIDs)
}

func TestRelayProcessesLinkDelete(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, _ := setupRelayTest(t, provider)

	event := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventLinkDeleted,
		AggregateID:   "/rfc/h-003",
		AggregateType: models.SearchAggregateLink,
		IndexName:     models.SearchIndexLinks,
		Operation:     models.SearchOutboxOperationDelete,
	})

	require.NoError(t, relay.ProcessBatch(context.Background()))

	reloaded := reloadSearchOutboxEvent(t, db, event.ID)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloaded.Status)
	assert.Equal(t, []string{"/rfc/h-003"}, provider.links.deletedIDs)
}

func TestRelayMovesFailedEventToDLQAfterRetries(t *testing.T) {
	provider := newMockSearchProvider()
	provider.documents.indexErr = errors.New("search unavailable")
	db, relay, now := setupRelayTest(t, provider)

	event := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"objectID": "doc-1",
		},
	})

	require.NoError(t, relay.ProcessBatch(context.Background()))
	reloaded := reloadSearchOutboxEvent(t, db, event.ID)
	assert.Equal(t, models.SearchOutboxStatusFailed, reloaded.Status)
	assert.Equal(t, 1, reloaded.AttemptCount)
	assert.Contains(t, reloaded.ErrorMessage, "search unavailable")
	assert.True(t, reloaded.AvailableAt.After(*now))

	*now = reloaded.AvailableAt
	require.NoError(t, relay.ProcessBatch(context.Background()))
	reloaded = reloadSearchOutboxEvent(t, db, event.ID)
	assert.Equal(t, models.SearchOutboxStatusDLQ, reloaded.Status)
	assert.Equal(t, 2, reloaded.AttemptCount)
}

func TestRelayRecoversStaleProcessingEvent(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, now := setupRelayTest(t, provider)

	event := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventProjectUpdated,
		AggregateID:   "project-1",
		AggregateType: models.SearchAggregateProject,
		IndexName:     models.SearchIndexProjects,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"objectID": "project-1",
			"title":    "Project One",
		},
	})
	staleLock := now.Add(-2 * time.Minute)
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
		"status":    models.SearchOutboxStatusProcessing,
		"locked_at": staleLock,
		"locked_by": "crashed-worker",
	}).Error)

	require.NoError(t, relay.ProcessBatch(context.Background()))

	reloaded := reloadSearchOutboxEvent(t, db, event.ID)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloaded.Status)
	assert.Equal(t, []string{"project-1"}, provider.projects.indexedIDs)
}

func TestRelayBlocksLaterSameAggregateButProcessesUnrelated(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, _ := setupRelayTest(t, provider)

	first := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"objectID": "doc-1",
		},
	})
	laterSameAggregate := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"objectID": "doc-1",
			"title":    "second",
		},
	})
	unrelated := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-2",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"objectID": "doc-2",
		},
	})
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).Where("id = ?", first.ID).Update("status", models.SearchOutboxStatusDLQ).Error)

	require.NoError(t, relay.ProcessBatch(context.Background()))

	sameReloaded := reloadSearchOutboxEvent(t, db, laterSameAggregate.ID)
	unrelatedReloaded := reloadSearchOutboxEvent(t, db, unrelated.ID)
	assert.Equal(t, models.SearchOutboxStatusPending, sameReloaded.Status)
	assert.Equal(t, models.SearchOutboxStatusCompleted, unrelatedReloaded.Status)
	assert.Equal(t, []string{"doc-2"}, provider.documents.indexedIDs)
}

func TestRelayProcessesSameAggregateInSequenceOrder(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, now := setupRelayTest(t, provider)

	sequenceTwo := &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Sequence:      2,
		Payload: map[string]any{
			"objectID": "doc-1",
			"title":    "final",
		},
	}
	sequenceOne := &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Sequence:      1,
		Payload: map[string]any{
			"objectID": "doc-1",
			"title":    "initial",
		},
	}
	repeatedFinal := &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Sequence:      3,
		Payload: map[string]any{
			"objectID": "doc-1",
			"title":    "final",
		},
	}
	require.NoError(t, models.EnqueueSearchOutboxEvent(db, sequenceTwo))
	*now = now.Add(time.Millisecond)
	require.NoError(t, models.EnqueueSearchOutboxEvent(db, sequenceOne))
	*now = now.Add(time.Millisecond)
	require.NoError(t, models.EnqueueSearchOutboxEvent(db, repeatedFinal))

	require.NoError(t, relay.ProcessBatch(context.Background()))
	assert.Equal(t, []string{"initial"}, provider.documents.indexedTitles)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloadSearchOutboxEvent(t, db, sequenceOne.ID).Status)
	assert.Equal(t, models.SearchOutboxStatusPending, reloadSearchOutboxEvent(t, db, sequenceTwo.ID).Status)

	require.NoError(t, relay.ProcessBatch(context.Background()))
	require.NoError(t, relay.ProcessBatch(context.Background()))

	assert.Equal(t, []string{"doc-1", "doc-1", "doc-1"}, provider.documents.indexedIDs)
	assert.Equal(t, "final", provider.documents.docsByID["doc-1"].Title)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloadSearchOutboxEvent(t, db, sequenceTwo.ID).Status)
	assert.Equal(t, models.SearchOutboxStatusCompleted, reloadSearchOutboxEvent(t, db, repeatedFinal.ID).Status)
}

func TestRelayPoisonEventBlocksOnlySameAggregate(t *testing.T) {
	provider := newMockSearchProvider()
	db, relay, _ := setupRelayTest(t, provider)

	poison := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
	})
	laterSameAggregate := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentUpdated,
		AggregateID:   "doc-1",
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"objectID": "doc-1",
		},
	})
	unrelated := enqueueRelayTestEvent(t, db, &models.SearchOutboxEvent{
		EventType:     models.SearchEventLinkCreated,
		AggregateID:   "/rfc/poison-unrelated",
		AggregateType: models.SearchAggregateLink,
		IndexName:     models.SearchIndexLinks,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"documentID": "doc-2",
			"objectID":   "/rfc/poison-unrelated",
		},
	})

	require.NoError(t, relay.ProcessBatch(context.Background()))

	poisonReloaded := reloadSearchOutboxEvent(t, db, poison.ID)
	sameReloaded := reloadSearchOutboxEvent(t, db, laterSameAggregate.ID)
	unrelatedReloaded := reloadSearchOutboxEvent(t, db, unrelated.ID)
	assert.Equal(t, models.SearchOutboxStatusFailed, poisonReloaded.Status)
	assert.Contains(t, poisonReloaded.ErrorMessage, "payload is required")
	assert.Equal(t, models.SearchOutboxStatusPending, sameReloaded.Status)
	assert.Equal(t, models.SearchOutboxStatusCompleted, unrelatedReloaded.Status)
	assert.Equal(t, []string{"/rfc/poison-unrelated"}, provider.links.savedIDs)
}

type mockSearchProvider struct {
	documents *mockDocumentIndex
	drafts    *mockDocumentIndex
	links     *mockLinksIndex
	projects  *mockProjectIndex
}

func newMockSearchProvider() *mockSearchProvider {
	return &mockSearchProvider{
		documents: &mockDocumentIndex{},
		drafts:    &mockDocumentIndex{},
		links:     &mockLinksIndex{},
		projects:  &mockProjectIndex{},
	}
}

func (p *mockSearchProvider) DocumentIndex() search.DocumentIndex { return p.documents }
func (p *mockSearchProvider) DraftIndex() search.DraftIndex       { return p.drafts }
func (p *mockSearchProvider) ProjectIndex() search.ProjectIndex   { return p.projects }
func (p *mockSearchProvider) LinksIndex() search.LinksIndex       { return p.links }
func (p *mockSearchProvider) Name() string                        { return "mock" }
func (p *mockSearchProvider) Healthy(context.Context) error       { return nil }

type mockDocumentIndex struct {
	indexErr      error
	deleteErr     error
	docsByID      map[string]*search.Document
	indexedIDs    []string
	indexedTitles []string
	deletedIDs    []string
}

func (i *mockDocumentIndex) Index(_ context.Context, doc *search.Document) error {
	if i.indexErr != nil {
		return i.indexErr
	}
	if i.docsByID == nil {
		i.docsByID = map[string]*search.Document{}
	}
	docCopy := *doc
	i.docsByID[doc.ObjectID] = &docCopy
	i.indexedIDs = append(i.indexedIDs, doc.ObjectID)
	i.indexedTitles = append(i.indexedTitles, doc.Title)
	return nil
}

func (i *mockDocumentIndex) IndexBatch(ctx context.Context, docs []*search.Document) error {
	for _, doc := range docs {
		if err := i.Index(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

func (i *mockDocumentIndex) Delete(_ context.Context, docID string) error {
	if i.deleteErr != nil {
		return i.deleteErr
	}
	i.deletedIDs = append(i.deletedIDs, docID)
	return nil
}

func (i *mockDocumentIndex) DeleteBatch(ctx context.Context, docIDs []string) error {
	for _, docID := range docIDs {
		if err := i.Delete(ctx, docID); err != nil {
			return err
		}
	}
	return nil
}

func (i *mockDocumentIndex) Search(context.Context, *search.SearchQuery) (*search.SearchResult, error) {
	return nil, nil
}

func (i *mockDocumentIndex) GetObject(context.Context, string) (*search.Document, error) {
	return nil, nil
}

func (i *mockDocumentIndex) GetFacets(context.Context, []string) (*search.Facets, error) {
	return nil, nil
}

func (i *mockDocumentIndex) Clear(context.Context) error { return nil }

type mockProjectIndex struct {
	indexErr   error
	deleteErr  error
	indexedIDs []string
	deletedIDs []string
}

func (i *mockProjectIndex) Index(_ context.Context, project map[string]any) error {
	if i.indexErr != nil {
		return i.indexErr
	}
	objectID, _ := project["objectID"].(string)
	i.indexedIDs = append(i.indexedIDs, objectID)
	return nil
}

func (i *mockProjectIndex) Delete(_ context.Context, projectID string) error {
	if i.deleteErr != nil {
		return i.deleteErr
	}
	i.deletedIDs = append(i.deletedIDs, projectID)
	return nil
}

func (i *mockProjectIndex) Search(context.Context, *search.SearchQuery) (*search.SearchResult, error) {
	return nil, nil
}

func (i *mockProjectIndex) GetObject(context.Context, string) (map[string]any, error) {
	return nil, nil
}

func (i *mockProjectIndex) Clear(context.Context) error { return nil }

type mockLinksIndex struct {
	saveErr    error
	deleteErr  error
	savedIDs   []string
	deletedIDs []string
}

func (i *mockLinksIndex) SaveLink(_ context.Context, link map[string]string) error {
	if i.saveErr != nil {
		return i.saveErr
	}
	i.savedIDs = append(i.savedIDs, link["objectID"])
	return nil
}

func (i *mockLinksIndex) DeleteLink(_ context.Context, objectID string) error {
	if i.deleteErr != nil {
		return i.deleteErr
	}
	i.deletedIDs = append(i.deletedIDs, objectID)
	return nil
}
func (mockLinksIndex) GetLink(context.Context, string) (map[string]string, error) {
	return nil, nil
}
func (mockLinksIndex) Clear(context.Context) error { return nil }
