//go:build integration && nfr
// +build integration,nfr

package nfr

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
	searchoutbox "github.com/hashicorp-forge/hermes/pkg/search/outbox"
	"github.com/hashicorp-forge/hermes/tests/integration"
)

func testSearchOutboxStress(t *testing.T) {
	config := applyDefaults(cfg)
	if config.Profile == "smoke" && cfg.Duration == 0 {
		config.Duration = 5 * time.Second
	}
	if config.Profile == "smoke" && cfg.Rate == "" {
		config.Rate = "12/min"
	}
	require.NoError(t, validateConfig(config))

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration+config.ConvergenceDeadline+30*time.Second)
	defer cancel()
	startedAt := time.Now().UTC()

	obs := observations{}
	passed := false
	defer func() {
		res := newResult("search-outbox-stress", config, startedAt, obs, passed)
		require.NoError(t, writeArtifacts(config.Output, res))
	}()

	host, apiKey := integration.GetMeilisearchConfig()
	suffix := time.Now().UnixNano()
	provider, err := meilisearch.NewAdapter(&meilisearch.Config{
		Host:              host,
		APIKey:            apiKey,
		DocsIndexName:     fmt.Sprintf("nfr-outbox-docs-%d", suffix),
		DraftsIndexName:   fmt.Sprintf("nfr-outbox-drafts-%d", suffix),
		ProjectsIndexName: fmt.Sprintf("nfr-outbox-projects-%d", suffix),
		LinksIndexName:    fmt.Sprintf("nfr-outbox-links-%d", suffix),
	})
	require.NoError(t, err)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SearchOutboxSequence{}, &models.SearchOutboxEvent{}))

	relay, err := searchoutbox.New(searchoutbox.Config{
		DB:       db,
		Provider: provider,
		Logger:   hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	interval, err := intervalForRate(config.Rate)
	require.NoError(t, err)
	endAt := time.Now().Add(config.Duration)
	nextRestartAt := time.Now().Add(config.RestartInterval)
	relayPaused := false

	for time.Now().Before(endAt) {
		docID := fmt.Sprintf("nfr-doc-%06d", obs.ItemsGenerated+1)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
				EventType:     models.SearchEventDocumentUpdated,
				AggregateID:   docID,
				AggregateType: models.SearchAggregateDocument,
				IndexName:     models.SearchIndexDocuments,
				Operation:     models.SearchOutboxOperationUpsert,
				Payload: map[string]any{
					"objectID": docID,
					"docID":    docID,
					"status":   "Approved",
					"title":    fmt.Sprintf("NFR Document %d", obs.ItemsGenerated+1),
				},
			})
		}))
		obs.ItemsGenerated++

		if config.RestartInterval > 0 && time.Now().After(nextRestartAt) {
			relayPaused = !relayPaused
			obs.WorkerRestarts++
			nextRestartAt = time.Now().Add(config.RestartInterval)
		}
		if !relayPaused {
			require.NoError(t, relay.ProcessBatch(ctx))
		}
		updateSearchOutboxObservations(t, db, &obs)
		time.Sleep(interval)
	}

	convergeStartedAt := time.Now()
	require.Eventually(t, func() bool {
		require.NoError(t, relay.ProcessBatch(ctx))
		updateSearchOutboxObservations(t, db, &obs)
		return obs.ItemsCompleted == obs.ItemsGenerated && obs.UnexpectedDLQ == 0
	}, config.ConvergenceDeadline, 250*time.Millisecond)
	obs.MaxConvergenceSeconds = int64(time.Since(convergeStartedAt).Seconds())

	for i := 1; i <= obs.ItemsGenerated; i++ {
		docID := fmt.Sprintf("nfr-doc-%06d", i)
		_, err := provider.DocumentIndex().GetObject(ctx, docID)
		require.NoError(t, err)
	}
	passed = true
}

func updateSearchOutboxObservations(t *testing.T, db *gorm.DB, obs *observations) {
	t.Helper()

	var completed int64
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).
		Where("status = ?", models.SearchOutboxStatusCompleted).
		Count(&completed).Error)
	obs.ItemsCompleted = int(completed)

	var pending int64
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).
		Where("status IN ?", []string{models.SearchOutboxStatusPending, models.SearchOutboxStatusProcessing, models.SearchOutboxStatusFailed}).
		Count(&pending).Error)
	if int(pending) > obs.MaxQueueDepth {
		obs.MaxQueueDepth = int(pending)
	}

	var dlq int64
	require.NoError(t, db.Model(&models.SearchOutboxEvent{}).
		Where("status = ?", models.SearchOutboxStatusDLQ).
		Count(&dlq).Error)
	obs.UnexpectedDLQ = int(dlq)
}
