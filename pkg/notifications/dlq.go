package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// DLQMessage represents a message in the Dead Letter Queue
// RFC-087-ADDENDUM Section 2: Dead Letter Queue (DLQ)
type DLQMessage struct {
	// Original message that failed
	OriginalMessage *NotificationMessage `json:"original_message"`

	// Failure metadata
	FailureReason  string    `json:"failure_reason"`   // Last error message
	FailedBackends []string  `json:"failed_backends"`  // Which backends failed
	RetryCount     int       `json:"retry_count"`      // How many times we retried
	FirstFailureAt time.Time `json:"first_failure_at"` // When it first failed
	LastFailureAt  time.Time `json:"last_failure_at"`  // When it finally gave up
	DLQTimestamp   time.Time `json:"dlq_timestamp"`    // When added to DLQ

	// Original message metadata for tracking
	MessageID        string           `json:"message_id"`        // Original message ID
	NotificationType NotificationType `json:"notification_type"` // Original notification type
	DocumentUUID     string           `json:"document_uuid,omitempty"`
	ProjectID        string           `json:"project_id,omitempty"`
	UserID           string           `json:"user_id,omitempty"`
}

// DLQPublisher publishes messages to the Dead Letter Queue
type DLQPublisher struct {
	client *kgo.Client
	topic  string
}

// DLQPublisherConfig holds DLQ publisher configuration
type DLQPublisherConfig struct {
	Brokers []string
	Topic   string // DLQ topic name (e.g., "hermes.notifications.dlq")
}

// NewDLQPublisher creates a new DLQ publisher
func NewDLQPublisher(cfg DLQPublisherConfig) (*DLQPublisher, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}
	if cfg.Topic == "" {
		cfg.Topic = "hermes.notifications.dlq" // Default DLQ topic
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		// DLQ messages should never be lost
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.GzipCompression()),
		kgo.RequestRetries(10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DLQ kafka client: %w", err)
	}

	return &DLQPublisher{
		client: client,
		topic:  cfg.Topic,
	}, nil
}

// PublishToDLQ publishes a failed notification to the DLQ
func (p *DLQPublisher) PublishToDLQ(ctx context.Context, msg *NotificationMessage, failureReason string) error {
	now := time.Now()

	// Determine first failure time
	firstFailureAt := msg.LastRetryAt
	if firstFailureAt.IsZero() {
		firstFailureAt = msg.Timestamp
	}

	// Create DLQ message
	dlqMsg := DLQMessage{
		OriginalMessage:  msg,
		FailureReason:    failureReason,
		FailedBackends:   msg.FailedBackends,
		RetryCount:       msg.RetryCount,
		FirstFailureAt:   firstFailureAt,
		LastFailureAt:    now,
		DLQTimestamp:     now,
		MessageID:        msg.ID,
		NotificationType: msg.Type,
		DocumentUUID:     msg.DocumentUUID,
		ProjectID:        msg.ProjectID,
		UserID:           msg.UserID,
	}

	// Marshal to JSON
	dlqJSON, err := json.Marshal(dlqMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	// Publish to DLQ topic
	// Use message ID as key for consistent partitioning
	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(msg.ID),
		Value: dlqJSON,
	}

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	return nil
}

// Close closes the DLQ publisher
func (p *DLQPublisher) Close() {
	p.client.Close()
}

// DLQMonitor provides methods for monitoring and replaying DLQ messages
type DLQMonitor struct {
	client *kgo.Client
	topic  string
}

// NewDLQMonitor creates a new DLQ monitor
func NewDLQMonitor(brokers []string, topic string) (*DLQMonitor, error) {
	if topic == "" {
		topic = "hermes.notifications.dlq"
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DLQ monitor client: %w", err)
	}

	return &DLQMonitor{
		client: client,
		topic:  topic,
	}, nil
}

// GetDLQMessages retrieves messages from the DLQ
func (m *DLQMonitor) GetDLQMessages(ctx context.Context, limit int) ([]*DLQMessage, error) {
	messages := make([]*DLQMessage, 0, limit)

	fetches := m.client.PollFetches(ctx)
	if errs := fetches.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("error fetching from DLQ: %v", errs[0])
	}

	fetches.EachRecord(func(record *kgo.Record) {
		if len(messages) >= limit {
			return
		}

		var dlqMsg DLQMessage
		if err := json.Unmarshal(record.Value, &dlqMsg); err != nil {
			// Log error but continue
			return
		}

		messages = append(messages, &dlqMsg)
	})

	return messages, nil
}

// Close closes the DLQ monitor
func (m *DLQMonitor) Close() {
	m.client.Close()
}
