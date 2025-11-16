package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline"
	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"github.com/twmb/franz-go/pkg/kgo"
	"gorm.io/gorm"
)

// Consumer consumes document revision events from Redpanda and processes them.
type Consumer struct {
	kafkaClient *kgo.Client
	db          *gorm.DB
	matcher     *ruleset.Matcher
	executor    *pipeline.Executor
	logger      hclog.Logger
	stopCh      chan struct{}
}

// Config holds configuration for the consumer.
type Config struct {
	// Database connection
	DB *gorm.DB

	// Kafka/Redpanda configuration
	Brokers       []string
	Topic         string
	ConsumerGroup string

	// Consumer offset configuration (optional, defaults to AtEnd for new consumers)
	// Use AtStart for testing to ensure messages are consumed even if published before consumer joins
	ConsumeFromStart bool

	// Pipeline configuration
	Rulesets ruleset.Rulesets
	Executor *pipeline.Executor

	// Logger
	Logger hclog.Logger
}

// New creates a new indexer consumer.
func New(cfg Config) (*Consumer, error) {
	// Note: DB is optional. If not provided, idempotency checks and execution tracking are skipped.
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if cfg.ConsumerGroup == "" {
		cfg.ConsumerGroup = "hermes-indexer-workers"
	}
	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}
	if cfg.Executor == nil {
		return nil, fmt.Errorf("pipeline executor is required")
	}

	// Validate rulesets
	if err := cfg.Rulesets.ValidateAll(); err != nil {
		return nil, fmt.Errorf("invalid rulesets: %w", err)
	}

	// Determine offset strategy
	offset := kgo.NewOffset().AtEnd() // Start from latest for new consumers by default
	if cfg.ConsumeFromStart {
		offset = kgo.NewOffset().AtStart() // Start from beginning (useful for testing)
	}

	// Create Kafka consumer client
	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.ConsumerGroup),
		kgo.ConsumeTopics(cfg.Topic),

		// Consumer configuration
		kgo.ConsumeResetOffset(offset),
		kgo.SessionTimeout(10*time.Second),
		kgo.RebalanceTimeout(30*time.Second),

		// Enable auto-commit (commit after successful processing)
		kgo.DisableAutoCommit(), // We'll commit manually after successful processing

		// Fetch configuration
		kgo.FetchMaxWait(500*time.Millisecond),
		kgo.FetchMinBytes(1),
		kgo.FetchMaxBytes(5<<20), // 5MB
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	matcher := ruleset.NewMatcher(cfg.Rulesets)

	return &Consumer{
		kafkaClient: kafkaClient,
		db:          cfg.DB,
		matcher:     matcher,
		executor:    cfg.Executor,
		logger:      cfg.Logger.Named("indexer-consumer"),
		stopCh:      make(chan struct{}),
	}, nil
}

// Start starts the consumer polling loop.
func (c *Consumer) Start(ctx context.Context) error {
	group, _ := c.kafkaClient.GroupMetadata()
	c.logger.Info("starting indexer consumer",
		"consumer_group", group,
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("indexer consumer stopped by context")
			return ctx.Err()

		case <-c.stopCh:
			c.logger.Info("indexer consumer stopped")
			return nil

		default:
			// Poll for messages
			fetches := c.kafkaClient.PollFetches(ctx)

			// Handle errors
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					c.logger.Error("kafka fetch error", "error", err.Err)
				}
				continue
			}

			// Process records
			fetches.EachPartition(func(p kgo.FetchTopicPartition) {
				for _, record := range p.Records {
					if err := c.processRecord(ctx, record); err != nil {
						c.logger.Error("failed to process record",
							"partition", record.Partition,
							"offset", record.Offset,
							"error", err,
						)
						// Continue processing other records
						// TODO: Consider DLQ for permanently failed records
						continue
					}

					// Commit offset after successful processing
					if err := c.kafkaClient.CommitRecords(ctx, record); err != nil {
						c.logger.Warn("failed to commit Kafka offset",
							"partition", record.Partition,
							"offset", record.Offset,
							"error", err)
					}
				}
			})
		}
	}
}

// Stop gracefully stops the consumer.
func (c *Consumer) Stop() {
	select {
	case <-c.stopCh:
		// Already stopped
		return
	default:
		close(c.stopCh)
		c.kafkaClient.Close()
	}
}

// processRecord processes a single Kafka record.
func (c *Consumer) processRecord(ctx context.Context, record *kgo.Record) error {
	c.logger.Debug("processing record",
		"partition", record.Partition,
		"offset", record.Offset,
		"key", string(record.Key),
	)

	// Deserialize event
	var event DocumentRevisionEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Parse document UUID
	documentUUID, err := uuid.Parse(event.DocumentUUID)
	if err != nil {
		return fmt.Errorf("invalid document UUID: %w", err)
	}

	// Check for idempotency (only if database is available)
	if c.db != nil {
		executions, err := models.GetExecutionsByOutbox(c.db, uint(event.ID))
		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to check for existing executions: %w", err)
		}

		if len(executions) > 0 {
			c.logger.Debug("event already processed, skipping",
				"document_uuid", documentUUID,
				"outbox_id", event.ID,
				"executions", len(executions),
			)
			return nil
		}
	}

	// Reconstruct revision from payload (no database fetch needed)
	revision, err := reconstructRevisionFromPayload(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to reconstruct revision from payload: %w", err)
	}

	// Extract metadata from payload
	metadata, ok := event.Payload["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
	}

	// Match rulesets
	matched := c.matcher.Match(revision, metadata)

	if len(matched) == 0 {
		c.logger.Debug("no rulesets matched, skipping",
			"document_uuid", documentUUID,
			"revision_id", revision.ID,
		)
		return nil
	}

	c.logger.Info("matched rulesets for revision",
		"document_uuid", documentUUID,
		"revision_id", revision.ID,
		"rulesets", len(matched),
	)

	// Execute pipelines for each matched ruleset
	errs := c.executor.ExecuteMultiple(ctx, revision, uint(event.ID), matched)

	if len(errs) > 0 {
		// Log errors but don't fail the entire processing
		for _, err := range errs {
			c.logger.Error("pipeline execution failed", "error", err)
		}

		// Return the first error for retry logic
		return errs[0]
	}

	c.logger.Info("successfully processed revision",
		"document_uuid", documentUUID,
		"revision_id", revision.ID,
		"pipelines_executed", len(matched),
	)

	return nil
}

// DocumentRevisionEvent represents the event structure from Redpanda.
// This should match the structure published by the relay service.
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

// reconstructRevisionFromPayload reconstructs a DocumentRevision from the event payload.
// This allows the indexer to be database-independent.
func reconstructRevisionFromPayload(payload map[string]interface{}) (*models.DocumentRevision, error) {
	revisionData, ok := payload["revision"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("payload missing revision data")
	}

	// Extract required fields
	id, _ := revisionData["id"].(float64) // JSON numbers are float64
	documentUUID, _ := payload["document_uuid"].(string)
	documentID, _ := payload["document_id"].(string)
	providerType, _ := payload["provider_type"].(string)
	contentHash, _ := revisionData["content_hash"].(string)

	// Parse UUID
	parsedUUID, err := uuid.Parse(documentUUID)
	if err != nil {
		return nil, fmt.Errorf("invalid document UUID in payload: %w", err)
	}

	// Create revision struct
	revision := &models.DocumentRevision{
		ID:           uint(id),
		DocumentUUID: parsedUUID,
		DocumentID:   documentID,
		ProviderType: providerType,
		ContentHash:  contentHash,
	}

	// Add any additional fields from the payload as needed
	// This is a basic reconstruction - extend as needed for your use case

	return revision, nil
}
