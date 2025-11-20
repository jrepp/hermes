package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/twmb/franz-go/pkg/kgo"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// Relay polls the document_revision_outbox table and publishes events to Redpanda.
// Implements the outbox pattern relay component.
type Relay struct {
	db           *gorm.DB
	kafkaClient  *kgo.Client
	topic        string
	logger       hclog.Logger
	pollInterval time.Duration
	batchSize    int
	stopCh       chan struct{}
}

// Config holds configuration for the relay service.
type Config struct {
	// Database connection
	DB *gorm.DB

	// Kafka/Redpanda configuration
	Brokers []string
	Topic   string

	// Polling configuration
	PollInterval time.Duration // How often to poll the outbox (default: 1s)
	BatchSize    int           // How many outbox entries to process per batch (default: 100)

	// Logger
	Logger hclog.Logger
}

// New creates a new outbox relay service.
func New(cfg Config) (*Relay, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("database is required")
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}

	// Set defaults
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}

	// Create Kafka client
	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),

		// Producer durability settings
		kgo.RequiredAcks(kgo.AllISRAcks()), // Wait for all in-sync replicas
		kgo.ProducerBatchCompression(kgo.GzipCompression()),

		// Retry configuration
		kgo.RetryBackoffFn(func(tries int) time.Duration {
			backoff := time.Duration(tries) * 100 * time.Millisecond
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			return backoff
		}),
		kgo.RequestRetries(10),

		// Batching for better throughput
		kgo.ProducerLinger(10*time.Millisecond),
		kgo.ProducerBatchMaxBytes(1<<20), // 1MB
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	return &Relay{
		db:           cfg.DB,
		kafkaClient:  kafkaClient,
		topic:        cfg.Topic,
		logger:       cfg.Logger.Named("outbox-relay"),
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		stopCh:       make(chan struct{}),
	}, nil
}

// Start starts the relay service polling loop.
// Blocks until Stop() is called or context is cancelled.
func (r *Relay) Start(ctx context.Context) error {
	r.logger.Info("starting outbox relay service",
		"poll_interval", r.pollInterval,
		"batch_size", r.batchSize,
		"topic", r.topic,
	)

	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("outbox relay service stopped by context")
			return ctx.Err()

		case <-r.stopCh:
			r.logger.Info("outbox relay service stopped")
			return nil

		case <-ticker.C:
			if err := r.processBatch(ctx); err != nil {
				r.logger.Error("failed to process outbox batch", "error", err)
				// Continue polling even on errors
			}
		}
	}
}

// Stop gracefully stops the relay service.
func (r *Relay) Stop() {
	close(r.stopCh)
	r.kafkaClient.Close()
}

// processBatch fetches pending outbox entries and publishes them to Redpanda.
func (r *Relay) processBatch(ctx context.Context) error {
	// Fetch pending entries
	entries, err := models.FindPendingOutboxEntries(r.db, r.batchSize)
	if err != nil {
		return fmt.Errorf("failed to find pending outbox entries: %w", err)
	}

	if len(entries) == 0 {
		// No pending entries, nothing to do
		return nil
	}

	r.logger.Debug("processing outbox batch", "count", len(entries))

	// Process each entry
	successCount := 0
	failCount := 0

	for _, entry := range entries {
		if err := r.publishEntry(ctx, &entry); err != nil {
			r.logger.Error("failed to publish outbox entry",
				"outbox_id", entry.ID,
				"document_uuid", entry.DocumentUUID,
				"error", err,
			)

			// Mark as failed
			if markErr := entry.MarkAsFailed(r.db, err); markErr != nil {
				r.logger.Error("failed to mark outbox entry as failed",
					"outbox_id", entry.ID,
					"error", markErr,
				)
			}

			failCount++
			continue
		}

		// Mark as published
		if err := entry.MarkAsPublished(r.db); err != nil {
			r.logger.Error("failed to mark outbox entry as published",
				"outbox_id", entry.ID,
				"error", err,
			)
			failCount++
			continue
		}

		successCount++
	}

	r.logger.Info("processed outbox batch",
		"total", len(entries),
		"success", successCount,
		"failed", failCount,
	)

	return nil
}

// publishEntry publishes a single outbox entry to Redpanda.
func (r *Relay) publishEntry(ctx context.Context, entry *models.DocumentRevisionOutbox) error {
	// Build the event message
	event := DocumentRevisionEvent{
		ID:           entry.ID,
		DocumentUUID: entry.DocumentUUID.String(),
		DocumentID:   entry.DocumentID,
		EventType:    entry.EventType,
		ProviderType: entry.ProviderType,
		ContentHash:  entry.ContentHash,
		Payload:      entry.Payload,
		Timestamp:    entry.CreatedAt,
	}

	// Marshal to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create Kafka record
	// Key: document UUID (ensures ordering of events for the same document)
	record := &kgo.Record{
		Topic: r.topic,
		Key:   []byte(entry.DocumentUUID.String()),
		Value: eventJSON,
		Headers: []kgo.RecordHeader{
			{Key: "event_type", Value: []byte(entry.EventType)},
			{Key: "provider_type", Value: []byte(entry.ProviderType)},
			{Key: "idempotent_key", Value: []byte(entry.IdempotentKey)},
		},
	}

	// Publish synchronously (wait for ack)
	if err := r.kafkaClient.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("failed to publish to kafka: %w", err)
	}

	r.logger.Debug("published event to kafka",
		"outbox_id", entry.ID,
		"document_uuid", entry.DocumentUUID,
		"event_type", entry.EventType,
		"partition_key", entry.DocumentUUID.String(),
	)

	return nil
}

// CleanupOldEntries removes published outbox entries older than the specified duration.
// Should be called periodically (e.g., daily) to prevent unbounded table growth.
func (r *Relay) CleanupOldEntries(olderThan time.Duration) error {
	deleted, err := models.DeleteOldPublishedEntries(r.db, olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old outbox entries: %w", err)
	}

	r.logger.Info("cleaned up old outbox entries",
		"deleted", deleted,
		"older_than", olderThan,
	)

	return nil
}

// RetryFailed retries all failed outbox entries.
// Useful for manual intervention when publishing failures occur.
func (r *Relay) RetryFailed(ctx context.Context, limit int) error {
	failed, err := models.GetFailedOutboxEntries(r.db, limit)
	if err != nil {
		return fmt.Errorf("failed to get failed outbox entries: %w", err)
	}

	if len(failed) == 0 {
		r.logger.Info("no failed outbox entries to retry")
		return nil
	}

	r.logger.Info("retrying failed outbox entries", "count", len(failed))

	successCount := 0
	for _, entry := range failed {
		// Reset to pending
		if err := entry.Retry(r.db); err != nil {
			r.logger.Error("failed to reset outbox entry to pending",
				"outbox_id", entry.ID,
				"error", err,
			)
			continue
		}

		// Try to publish
		if err := r.publishEntry(ctx, &entry); err != nil {
			r.logger.Error("failed to republish entry",
				"outbox_id", entry.ID,
				"error", err,
			)

			// Mark as failed again
			if markErr := entry.MarkAsFailed(r.db, err); markErr != nil {
				r.logger.Warn("failed to mark entry as failed", "outbox_id", entry.ID, "error", markErr)
			}
			continue
		}

		// Mark as published
		if err := entry.MarkAsPublished(r.db); err != nil {
			r.logger.Error("failed to mark entry as published",
				"outbox_id", entry.ID,
				"error", err,
			)
			continue
		}

		successCount++
	}

	r.logger.Info("retry completed",
		"attempted", len(failed),
		"success", successCount,
		"failed", len(failed)-successCount,
	)

	return nil
}

// GetStats returns statistics about the outbox state.
func (r *Relay) GetStats() (OutboxStats, error) {
	var stats OutboxStats

	pending, err := models.CountOutboxByStatus(r.db, models.OutboxStatusPending)
	if err != nil {
		return stats, err
	}
	stats.Pending = pending

	published, err := models.CountOutboxByStatus(r.db, models.OutboxStatusPublished)
	if err != nil {
		return stats, err
	}
	stats.Published = published

	failed, err := models.CountOutboxByStatus(r.db, models.OutboxStatusFailed)
	if err != nil {
		return stats, err
	}
	stats.Failed = failed

	return stats, nil
}

// DocumentRevisionEvent represents a document revision event published to Kafka.
type DocumentRevisionEvent struct {
	ID           uint                   `json:"id"`
	DocumentUUID string                 `json:"documentUuid"`
	DocumentID   string                 `json:"documentId"`
	EventType    string                 `json:"eventType"`
	ProviderType string                 `json:"providerType"`
	ContentHash  string                 `json:"contentHash"`
	Payload      map[string]interface{} `json:"payload"`
	Timestamp    time.Time              `json:"timestamp"`
}

// OutboxStats contains statistics about the outbox state.
type OutboxStats struct {
	Pending   int64 `json:"pending"`
	Published int64 `json:"published"`
	Failed    int64 `json:"failed"`
}
