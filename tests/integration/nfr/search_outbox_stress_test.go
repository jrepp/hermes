//go:build integration && nfr
// +build integration,nfr

package nfr

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
	searchoutbox "github.com/hashicorp-forge/hermes/pkg/search/outbox"
	"github.com/hashicorp-forge/hermes/tests/integration"
)

func testSearchOutboxStress(t *testing.T) {
	requireFixture(t)

	config := applyDefaults(cfg)
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

	db, err := gorm.Open(postgres.Open(integration.GetPostgresURL()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	schemaName := fmt.Sprintf("nfr_search_outbox_%d", suffix)
	require.NoError(t, db.Exec(fmt.Sprintf("CREATE SCHEMA %s", schemaName)).Error)
	t.Cleanup(func() {
		_ = db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName)).Error
	})
	require.NoError(t, db.Exec(fmt.Sprintf("SET search_path TO %s,public", schemaName)).Error)
	require.NoError(t, db.AutoMigrate(&models.SearchOutboxSequence{}, &models.SearchOutboxEvent{}))

	relay, err := searchoutbox.New(searchoutbox.Config{
		DB:        db,
		Provider:  provider,
		Logger:    hclog.NewNullLogger(),
		BatchSize: config.RelayBatchSize,
	})
	require.NoError(t, err)

	interval, err := intervalForRate(config.Rate)
	require.NoError(t, err)
	generated, relayRestarts, runErrs := runSearchOutboxLoad(ctx, db, relay, config, interval)
	for _, err := range runErrs {
		obs.Errors = append(obs.Errors, err.Error())
	}
	require.Empty(t, obs.Errors)
	obs.ItemsGenerated = int(generated.Load())
	obs.WorkerRestarts = int(relayRestarts.Load())
	updateSearchOutboxObservations(t, db, &obs)

	convergeStartedAt := time.Now()
	require.NoError(t, waitForSearchOutboxConvergence(ctx, db, relay, config.ConvergenceDeadline, &obs))
	obs.MaxConvergenceSeconds = int64(time.Since(convergeStartedAt).Seconds())

	for i := 1; i <= obs.ItemsGenerated; i++ {
		docID := fmt.Sprintf("nfr-doc-%06d", i)
		_, err := provider.DocumentIndex().GetObject(ctx, docID)
		require.NoError(t, err)
	}
	passed = true
}

func runSearchOutboxLoad(ctx context.Context, db *gorm.DB, relay *searchoutbox.Relay, config harnessConfig, interval time.Duration) (*atomic.Int64, *atomic.Int64, []error) {
	runCtx, cancel := context.WithTimeout(ctx, config.Duration)
	defer cancel()

	var generated atomic.Int64
	var relayRestarts atomic.Int64
	var relayPaused atomic.Bool
	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				next := generated.Load() + 1
				if config.MaxItems > 0 && next > config.MaxItems {
					cancel()
					return
				}
				docID := fmt.Sprintf("nfr-doc-%06d", next)
				if err := db.Transaction(func(tx *gorm.DB) error {
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
							"title":    fmt.Sprintf("NFR Document %d", next),
						},
					})
				}); err != nil {
					errCh <- err
					cancel()
					return
				}
				generated.Store(next)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		restartTicker := time.NewTicker(config.RestartInterval)
		defer restartTicker.Stop()

		for {
			select {
			case <-runCtx.Done():
				return
			case <-restartTicker.C:
				relayPaused.Store(!relayPaused.Load())
				relayRestarts.Add(1)
			case <-ticker.C:
				if relayPaused.Load() {
					continue
				}
				if err := relay.ProcessBatch(ctx); err != nil {
					errCh <- err
					cancel()
					return
				}
			}
		}
	}()

	wg.Wait()
	close(errCh)
	errs := make([]error, 0, len(errCh))
	for err := range errCh {
		errs = append(errs, err)
	}
	return &generated, &relayRestarts, errs
}

func waitForSearchOutboxConvergence(ctx context.Context, db *gorm.DB, relay *searchoutbox.Relay, deadline time.Duration, obs *observations) error {
	deadlineAt := time.Now().Add(deadline)
	var lastErr error
	for time.Now().Before(deadlineAt) {
		if err := relay.ProcessBatch(ctx); err != nil {
			lastErr = err
			obs.Errors = append(obs.Errors, err.Error())
		} else if err := updateSearchOutboxObservationsE(db, obs); err != nil {
			lastErr = err
			obs.Errors = append(obs.Errors, err.Error())
		} else if obs.ItemsCompleted == obs.ItemsGenerated && obs.UnexpectedDLQ == 0 {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("search outbox did not converge before deadline: %w", lastErr)
	}
	return fmt.Errorf("search outbox did not converge before deadline: completed=%d generated=%d dlq=%d", obs.ItemsCompleted, obs.ItemsGenerated, obs.UnexpectedDLQ)
}

func updateSearchOutboxObservations(t *testing.T, db *gorm.DB, obs *observations) {
	t.Helper()
	require.NoError(t, updateSearchOutboxObservationsE(db, obs))
}

func updateSearchOutboxObservationsE(db *gorm.DB, obs *observations) error {
	var completed int64
	if err := db.Model(&models.SearchOutboxEvent{}).
		Where("status = ?", models.SearchOutboxStatusCompleted).
		Count(&completed).Error; err != nil {
		return err
	}
	obs.ItemsCompleted = int(completed)

	var pending int64
	if err := db.Model(&models.SearchOutboxEvent{}).
		Where("status IN ?", []string{models.SearchOutboxStatusPending, models.SearchOutboxStatusProcessing, models.SearchOutboxStatusFailed}).
		Count(&pending).Error; err != nil {
		return err
	}
	if int(pending) > obs.MaxQueueDepth {
		obs.MaxQueueDepth = int(pending)
	}

	var dlq int64
	if err := db.Model(&models.SearchOutboxEvent{}).
		Where("status = ?", models.SearchOutboxStatusDLQ).
		Count(&dlq).Error; err != nil {
		return err
	}
	obs.UnexpectedDLQ = int(dlq)
	return nil
}
