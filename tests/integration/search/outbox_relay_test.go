//go:build integration
// +build integration

package search

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
	hermessearch "github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
	searchoutbox "github.com/hashicorp-forge/hermes/pkg/search/outbox"
	"github.com/hashicorp-forge/hermes/tests/integration"
)

func TestSearchOutboxRelayConvergesWithMeilisearch(t *testing.T) {
	integration.WithTimeout(t, 45*time.Second, 10*time.Second, func(ctx context.Context, progress func(string)) {
		host, apiKey := integration.GetMeilisearchConfig()
		suffix := time.Now().UnixNano()
		provider, err := meilisearch.NewAdapter(&meilisearch.Config{
			Host:              host,
			APIKey:            apiKey,
			DocsIndexName:     fmt.Sprintf("outbox-docs-%d", suffix),
			DraftsIndexName:   fmt.Sprintf("outbox-drafts-%d", suffix),
			ProjectsIndexName: fmt.Sprintf("outbox-projects-%d", suffix),
			LinksIndexName:    fmt.Sprintf("outbox-links-%d", suffix),
		})
		require.NoError(t, err)
		progress("created Meilisearch adapter")

		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SearchOutboxSequence{}, &models.SearchOutboxEvent{}))

		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			events := []*models.SearchOutboxEvent{
				{
					EventType:     models.SearchEventDraftCreated,
					AggregateID:   "outbox-doc-1",
					AggregateType: models.SearchAggregateDraft,
					IndexName:     models.SearchIndexDrafts,
					Operation:     models.SearchOutboxOperationUpsert,
					Payload: map[string]any{
						"objectID": "outbox-doc-1",
						"docID":    "outbox-doc-1",
						"status":   "WIP",
						"title":    "Outbox Draft Document",
					},
				},
				{
					EventType:     models.SearchEventDocumentPublished,
					AggregateID:   "outbox-doc-1",
					AggregateType: models.SearchAggregateDocument,
					IndexName:     models.SearchIndexDocuments,
					Operation:     models.SearchOutboxOperationUpsert,
					Payload: map[string]any{
						"objectID": "outbox-doc-1",
						"docID":    "outbox-doc-1",
						"status":   "In-Review",
						"title":    "Outbox Published Document",
					},
				},
				{
					EventType:     models.SearchEventDraftPublished,
					AggregateID:   "outbox-doc-1",
					AggregateType: models.SearchAggregateDraft,
					IndexName:     models.SearchIndexDrafts,
					Operation:     models.SearchOutboxOperationDelete,
				},
				{
					EventType:     models.SearchEventReviewStateChanged,
					AggregateID:   "outbox-doc-1",
					AggregateType: models.SearchAggregateDocument,
					IndexName:     models.SearchIndexDocuments,
					Operation:     models.SearchOutboxOperationUpsert,
					Payload: map[string]any{
						"approvedBy": []string{"approver@example.com"},
						"objectID":   "outbox-doc-1",
						"docID":      "outbox-doc-1",
						"status":     "Approved",
						"title":      "Outbox Approved Document",
					},
				},
				{
					EventType:     models.SearchEventLinkCreated,
					AggregateID:   "/rfc/outbox-001",
					AggregateType: models.SearchAggregateLink,
					IndexName:     models.SearchIndexLinks,
					Operation:     models.SearchOutboxOperationUpsert,
					Payload: map[string]any{
						"documentID": "outbox-doc-1",
						"objectID":   "/rfc/outbox-001",
					},
				},
			}
			for _, event := range events {
				if err := models.EnqueueSearchOutboxEventWithSequence(tx, event); err != nil {
					return err
				}
			}
			return nil
		}))

		relay, err := searchoutbox.New(searchoutbox.Config{
			DB:       db,
			Provider: provider,
			Logger:   hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		var completed int64
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.NoError(c, relay.ProcessBatch(ctx))
			assert.NoError(c, db.Model(&models.SearchOutboxEvent{}).
				Where("status = ?", models.SearchOutboxStatusCompleted).
				Count(&completed).Error)
			assert.Equal(c, int64(5), completed)
		}, 20*time.Second, 250*time.Millisecond)
		progress("processed outbox batches")

		var indexed *hermessearch.Document
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			doc, err := provider.DocumentIndex().GetObject(ctx, "outbox-doc-1")
			assert.NoError(c, err)
			if err == nil {
				indexed = doc
				assert.Equal(c, "Outbox Approved Document", doc.Title)
				assert.Equal(c, "Approved", doc.Status)
			}
		}, 10*time.Second, 250*time.Millisecond)
		require.NotNil(t, indexed)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			_, err := provider.DraftIndex().GetObject(ctx, "outbox-doc-1")
			assert.Error(c, err)
		}, 10*time.Second, 250*time.Millisecond)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			link, err := provider.LinksIndex().GetLink(ctx, "/rfc/outbox-001")
			assert.NoError(c, err)
			assert.Equal(c, "outbox-doc-1", link["documentID"])
		}, 10*time.Second, 250*time.Millisecond)
	})
}
