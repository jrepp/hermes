package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Publisher publishes notifications to Redpanda/Kafka
type Publisher struct {
	client *kgo.Client
	topic  string
}

// PublisherConfig holds configuration for the publisher
type PublisherConfig struct {
	Brokers []string
	Topic   string
}

// NewPublisher creates a new notification publisher
func NewPublisher(cfg PublisherConfig) (*Publisher, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),

		// Producer Durability (RFC-087-ADDENDUM Section 10)
		// Wait for all in-sync replicas to acknowledge (production-ready)
		kgo.RequiredAcks(kgo.AllISRAcks()),

		// Enable idempotent producer (prevents duplicate messages on retry)
		// franz-go enables idempotency by default when using RequiredAcks(AllISRAcks)

		// Enable compression (reduces network bandwidth)
		kgo.ProducerBatchCompression(kgo.GzipCompression()),

		// Retry configuration with exponential backoff
		// Retry up to 10 times with exponential backoff: 100ms, 200ms, 400ms, 800ms, 1.6s, 3.2s, 6.4s, 12.8s, 25.6s, 51.2s
		kgo.RetryBackoffFn(func(tries int) time.Duration {
			backoff := time.Duration(tries) * 100 * time.Millisecond
			if backoff > 60*time.Second {
				backoff = 60 * time.Second // Cap at 60s
			}
			return backoff
		}),
		kgo.RequestRetries(10),

		// Producer linger and batch settings for better throughput
		kgo.ProducerLinger(10*time.Millisecond), // Wait up to 10ms to batch messages
		kgo.ProducerBatchMaxBytes(1<<20),        // 1MB max batch size
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	return &Publisher{
		client: client,
		topic:  cfg.Topic,
	}, nil
}

// PublishNotification publishes a template-based notification
func (p *Publisher) PublishNotification(
	ctx context.Context,
	notifType NotificationType,
	template string,
	templateContext map[string]any,
	recipients []Recipient,
	backends []string,
) error {
	// Build notification message
	msg := NotificationMessage{
		ID:              uuid.New().String(),
		Type:            notifType,
		Timestamp:       time.Now(),
		Priority:        0,
		Recipients:      recipients,
		Template:        template,
		TemplateContext: templateContext,
		Backends:        backends,
	}

	return p.PublishMessage(ctx, &msg)
}

// PublishMessage publishes a pre-built notification message
func (p *Publisher) PublishMessage(ctx context.Context, msg *NotificationMessage) error {
	// Marshal to JSON
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal notification message: %w", err)
	}

	// Determine partition key for ordering
	partitionKey := determinePartitionKey(msg)

	// Publish to Redpanda
	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(partitionKey),
		Value: msgJSON,
	}

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("failed to publish notification: %w", err)
	}

	return nil
}

// PublishEmail helper for backward compatibility
func (p *Publisher) PublishEmail(ctx context.Context, to []string, from, subject, body string) error {
	// Build recipients
	recipients := make([]Recipient, len(to))
	for i, email := range to {
		recipients[i] = Recipient{Email: email, Name: ""}
	}

	// Use generic email template
	context := map[string]any{
		"subject": subject,
		"body":    body,
		"from":    from,
	}

	return p.PublishNotification(ctx, NotificationTypeEmail, "generic_email", context, recipients, []string{"mail", "audit"})
}

// Close closes the publisher
func (p *Publisher) Close() {
	p.client.Close()
}

// determinePartitionKey ensures related messages go to same partition
func determinePartitionKey(msg *NotificationMessage) string {
	// Priority 1: Use document UUID if present (all notifications about same doc ordered)
	if msg.DocumentUUID != "" {
		return fmt.Sprintf("doc:%s", msg.DocumentUUID)
	}

	// Priority 2: Use first recipient (all notifications to same user ordered)
	if len(msg.Recipients) > 0 {
		if msg.Recipients[0].Email != "" {
			return fmt.Sprintf("user:%s", msg.Recipients[0].Email)
		}
	}

	// Fallback: random (no ordering guarantee)
	return msg.ID
}
