// Package outbox relays search projection events from the database to search.Provider.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
)

// Relay drains search_outbox_events into the configured search provider.
type Relay struct {
	logger            hclog.Logger
	db                *gorm.DB
	provider          search.Provider
	now               func() time.Time
	pollInterval      time.Duration
	visibilityTimeout time.Duration
	maxRetryBackoff   time.Duration
	batchSize         int
	maxRetries        int
}

// Config configures a search outbox relay.
type Config struct {
	Logger            hclog.Logger
	DB                *gorm.DB
	Provider          search.Provider
	Now               func() time.Time
	PollInterval      time.Duration
	VisibilityTimeout time.Duration
	MaxRetryBackoff   time.Duration
	BatchSize         int
	MaxRetries        int
}

// New creates a search outbox relay.
func New(cfg Config) (*Relay, error) {
	if cfg.DB == nil {
		return nil, errors.New("database is required")
	}
	if cfg.Provider == nil {
		return nil, errors.New("search provider is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.VisibilityTimeout == 0 {
		cfg.VisibilityTimeout = 30 * time.Second
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = 60 * time.Second
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	return &Relay{
		db:                cfg.DB,
		provider:          cfg.Provider,
		logger:            cfg.Logger.Named("search-outbox-relay"),
		now:               cfg.Now,
		pollInterval:      cfg.PollInterval,
		visibilityTimeout: cfg.VisibilityTimeout,
		maxRetryBackoff:   cfg.MaxRetryBackoff,
		batchSize:         cfg.BatchSize,
		maxRetries:        cfg.MaxRetries,
	}, nil
}

// Start runs the relay loop until the context is canceled.
func (r *Relay) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.ProcessBatch(ctx); err != nil {
				r.logger.Error("search outbox batch failed", "error", err)
			}
		}
	}
}

// ProcessBatch claims and applies one batch of eligible search outbox events.
func (r *Relay) ProcessBatch(ctx context.Context) error {
	if err := r.recoverStaleProcessing(); err != nil {
		return err
	}

	events, err := r.claimBatch()
	if err != nil {
		return err
	}

	processed := make(map[uint]bool, len(events))
	if err := r.applyDocumentBatches(ctx, events, processed); err != nil {
		return err
	}

	for i := range events {
		if processed[events[i].ID] {
			continue
		}
		if err := r.applyEvent(ctx, &events[i]); err != nil {
			if markErr := r.markFailed(&events[i], err); markErr != nil {
				return markErr
			}
			continue
		}

		if err := r.markCompleted(&events[i]); err != nil {
			return err
		}
	}

	return nil
}

func (r *Relay) applyDocumentBatches(ctx context.Context, events []models.SearchOutboxEvent, processed map[uint]bool) error {
	types := []struct {
		indexName string
		idx       documentIndex
	}{
		{indexName: models.SearchIndexDocuments, idx: r.provider.DocumentIndex()},
		{indexName: models.SearchIndexDrafts, idx: r.provider.DraftIndex()},
	}

	for _, typ := range types {
		for _, operation := range []string{models.SearchOutboxOperationUpsert, models.SearchOutboxOperationDelete} {
			if err := r.applyDocumentBatch(ctx, events, processed, typ.indexName, operation, typ.idx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Relay) applyDocumentBatch(ctx context.Context, events []models.SearchOutboxEvent, processed map[uint]bool, indexName, operation string, idx documentIndex) error {
	var batchEvents []*models.SearchOutboxEvent
	var docs []*search.Document
	var docIDs []string

	for i := range events {
		if events[i].IndexName != indexName || events[i].Operation != operation {
			continue
		}

		processed[events[i].ID] = true
		if operation == models.SearchOutboxOperationDelete {
			batchEvents = append(batchEvents, &events[i])
			docIDs = append(docIDs, events[i].AggregateID)
			continue
		}

		doc, err := eventPayloadAsDocument(&events[i])
		if err != nil {
			if markErr := r.markFailed(&events[i], err); markErr != nil {
				return markErr
			}
			continue
		}

		batchEvents = append(batchEvents, &events[i])
		docs = append(docs, doc)
	}

	if len(docs) > 0 {
		if err := idx.IndexBatch(ctx, docs); err != nil {
			return r.markBatchFailed(batchEvents, err)
		}
		return r.markBatchCompleted(batchEvents)
	}
	if len(docIDs) > 0 {
		if err := idx.DeleteBatch(ctx, docIDs); err != nil {
			return r.markBatchFailed(batchEvents, err)
		}
		return r.markBatchCompleted(batchEvents)
	}

	return nil
}

func (r *Relay) recoverStaleProcessing() error {
	cutoff := r.now().Add(-r.visibilityTimeout)
	return r.db.Model(&models.SearchOutboxEvent{}).
		Where("status = ? AND locked_at < ?", models.SearchOutboxStatusProcessing, cutoff).
		Updates(map[string]any{
			"status":     models.SearchOutboxStatusPending,
			"locked_at":  nil,
			"locked_by":  "",
			"updated_at": r.now(),
		}).Error
}

func (r *Relay) claimBatch() ([]models.SearchOutboxEvent, error) {
	now := r.now()
	lockedBy := fmt.Sprintf("search-outbox-relay-%d", now.UnixNano())
	var claimed []models.SearchOutboxEvent

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var candidates []models.SearchOutboxEvent
		if err := tx.
			Where("status IN ? AND available_at <= ?", []string{models.SearchOutboxStatusPending, models.SearchOutboxStatusFailed}, now).
			Order("created_at ASC, id ASC").
			Limit(r.batchSize * 2).
			Find(&candidates).Error; err != nil {
			return err
		}

		for i := range candidates {
			if len(claimed) >= r.batchSize {
				break
			}

			blocked, err := hasEarlierBlockingEvent(tx, &candidates[i])
			if err != nil {
				return err
			}
			if blocked {
				continue
			}

			claimed = append(claimed, candidates[i])
		}

		if len(claimed) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(claimed))
		for i := range claimed {
			ids = append(ids, claimed[i].ID)
		}

		res := tx.Model(&models.SearchOutboxEvent{}).
			Where("id IN ? AND status IN ?", ids, []string{models.SearchOutboxStatusPending, models.SearchOutboxStatusFailed}).
			Updates(map[string]any{
				"status":          models.SearchOutboxStatusProcessing,
				"locked_at":       now,
				"locked_by":       lockedBy,
				"last_attempt_at": now,
				"updated_at":      now,
			})
		if res.Error != nil {
			return res.Error
		}

		claimed = claimed[:0]
		return tx.
			Where("id IN ? AND status = ? AND locked_by = ?", ids, models.SearchOutboxStatusProcessing, lockedBy).
			Order("created_at ASC, id ASC").
			Find(&claimed).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim search outbox batch: %w", err)
	}

	return claimed, nil
}

func hasEarlierBlockingEvent(tx *gorm.DB, event *models.SearchOutboxEvent) (bool, error) {
	if event.Sequence <= 1 {
		return false, nil
	}

	var count int64
	err := tx.Model(&models.SearchOutboxEvent{}).
		Where("aggregate_type = ? AND aggregate_id = ? AND sequence < ?", event.AggregateType, event.AggregateID, event.Sequence).
		Where("status IN ?", []string{
			models.SearchOutboxStatusPending,
			models.SearchOutboxStatusProcessing,
			models.SearchOutboxStatusFailed,
			models.SearchOutboxStatusDLQ,
		}).
		Count(&count).Error
	return count > 0, err
}

func (r *Relay) applyEvent(ctx context.Context, event *models.SearchOutboxEvent) error {
	switch event.IndexName {
	case models.SearchIndexDocuments:
		return applyDocumentEvent(ctx, r.provider.DocumentIndex(), event)
	case models.SearchIndexDrafts:
		return applyDraftEvent(ctx, r.provider.DraftIndex(), event)
	case models.SearchIndexLinks:
		return applyLinkEvent(ctx, r.provider.LinksIndex(), event)
	case models.SearchIndexProjects:
		return applyProjectEvent(ctx, r.provider.ProjectIndex(), event)
	default:
		return fmt.Errorf("unsupported search index %q", event.IndexName)
	}
}

type documentIndex interface {
	Index(context.Context, *search.Document) error
	IndexBatch(context.Context, []*search.Document) error
	Delete(context.Context, string) error
	DeleteBatch(context.Context, []string) error
}

func applyDocumentEvent(ctx context.Context, idx documentIndex, event *models.SearchOutboxEvent) error {
	if event.Operation == models.SearchOutboxOperationDelete {
		return idx.Delete(ctx, event.AggregateID)
	}

	doc, err := eventPayloadAsDocument(event)
	if err != nil {
		return err
	}
	return idx.Index(ctx, doc)
}

func applyDraftEvent(ctx context.Context, idx documentIndex, event *models.SearchOutboxEvent) error {
	return applyDocumentEvent(ctx, idx, event)
}

func applyProjectEvent(ctx context.Context, idx search.ProjectIndex, event *models.SearchOutboxEvent) error {
	if event.Operation == models.SearchOutboxOperationDelete {
		return idx.Delete(ctx, event.AggregateID)
	}
	return idx.Index(ctx, event.Payload)
}

func applyLinkEvent(ctx context.Context, idx search.LinksIndex, event *models.SearchOutboxEvent) error {
	if event.Operation == models.SearchOutboxOperationDelete {
		return idx.DeleteLink(ctx, event.AggregateID)
	}

	link, err := eventPayloadAsLink(event)
	if err != nil {
		return err
	}
	return idx.SaveLink(ctx, link)
}

func eventPayloadAsDocument(event *models.SearchOutboxEvent) (*search.Document, error) {
	if event.Payload == nil {
		return nil, errors.New("document search event payload is required")
	}

	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal search event payload: %w", err)
	}

	var doc search.Document
	if err := json.Unmarshal(payload, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal search document payload: %w", err)
	}
	if doc.ObjectID == "" {
		doc.ObjectID = event.AggregateID
	}
	if doc.DocID == "" {
		doc.DocID = event.AggregateID
	}

	return &doc, nil
}

func eventPayloadAsLink(event *models.SearchOutboxEvent) (map[string]string, error) {
	if event.Payload == nil {
		return nil, errors.New("link search event payload is required")
	}

	link := make(map[string]string, len(event.Payload))
	for key, value := range event.Payload {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("link payload field %q must be string", key)
		}
		link[key] = str
	}
	if link["objectID"] == "" {
		link["objectID"] = event.AggregateID
	}

	return link, nil
}

func (r *Relay) markCompleted(event *models.SearchOutboxEvent) error {
	now := r.now()
	return r.db.Model(&models.SearchOutboxEvent{}).
		Where("id = ?", event.ID).
		Updates(map[string]any{
			"status":       models.SearchOutboxStatusCompleted,
			"completed_at": now,
			"locked_at":    nil,
			"locked_by":    "",
			"updated_at":   now,
		}).Error
}

func (r *Relay) markBatchCompleted(events []*models.SearchOutboxEvent) error {
	if len(events) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}

	now := r.now()
	return r.db.Model(&models.SearchOutboxEvent{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"status":       models.SearchOutboxStatusCompleted,
			"completed_at": now,
			"locked_at":    nil,
			"locked_by":    "",
			"updated_at":   now,
		}).Error
}

func (r *Relay) markBatchFailed(events []*models.SearchOutboxEvent, applyErr error) error {
	for _, event := range events {
		if err := r.markFailed(event, applyErr); err != nil {
			return err
		}
	}
	return nil
}

func (r *Relay) markFailed(event *models.SearchOutboxEvent, applyErr error) error {
	now := r.now()
	attemptCount := event.AttemptCount + 1
	status := models.SearchOutboxStatusFailed
	availableAt := now.Add(r.backoff(attemptCount))
	if attemptCount >= r.maxRetries {
		status = models.SearchOutboxStatusDLQ
		availableAt = now
	}

	return r.db.Model(&models.SearchOutboxEvent{}).
		Where("id = ?", event.ID).
		Updates(map[string]any{
			"status":          status,
			"attempt_count":   attemptCount,
			"available_at":    availableAt,
			"locked_at":       nil,
			"locked_by":       "",
			"last_attempt_at": now,
			"error_message":   applyErr.Error(),
			"updated_at":      now,
		}).Error
}

func (r *Relay) backoff(attemptCount int) time.Duration {
	if attemptCount <= 0 {
		return 0
	}
	d := time.Second << (attemptCount - 1)
	if d > r.maxRetryBackoff {
		return r.maxRetryBackoff
	}
	return d
}
