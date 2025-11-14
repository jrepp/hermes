package notifications

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig holds retry configuration
// RFC-087-ADDENDUM Section 1: Retry Logic and Error Handling
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (default: 5)
	MaxRetries int

	// InitialBackoff is the initial backoff duration (default: 1 minute)
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration (default: 2 hours)
	MaxBackoff time.Duration

	// BackoffMultiplier is the backoff multiplier for exponential backoff (default: 2)
	BackoffMultiplier float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    1 * time.Minute,
		MaxBackoff:        2 * time.Hour,
		BackoffMultiplier: 2.0,
	}
}

// RetryHandler handles retry logic for failed notifications
type RetryHandler struct {
	config       RetryConfig
	publisher    *Publisher
	dlqPublisher *DLQPublisher
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(config RetryConfig, publisher *Publisher, dlqPublisher *DLQPublisher) *RetryHandler {
	return &RetryHandler{
		config:       config,
		publisher:    publisher,
		dlqPublisher: dlqPublisher,
	}
}

// CalculateNextRetry calculates the next retry time using exponential backoff
// Formula: min(initialBackoff * multiplier^retryCount, maxBackoff)
func (h *RetryHandler) CalculateNextRetry(retryCount int) time.Duration {
	// Calculate exponential backoff: initialBackoff * (multiplier ^ retryCount)
	backoff := float64(h.config.InitialBackoff)
	for i := 0; i < retryCount; i++ {
		backoff *= h.config.BackoffMultiplier
	}

	duration := time.Duration(backoff)

	// Cap at max backoff
	if duration > h.config.MaxBackoff {
		duration = h.config.MaxBackoff
	}

	return duration
}

// ShouldRetry determines if a message should be retried
func (h *RetryHandler) ShouldRetry(msg *NotificationMessage) bool {
	return msg.RetryCount < h.config.MaxRetries
}

// PrepareRetry prepares a message for retry by updating retry metadata
func (h *RetryHandler) PrepareRetry(msg *NotificationMessage, err error, failedBackends []string) *NotificationMessage {
	now := time.Now()
	retryCount := msg.RetryCount + 1
	backoff := h.CalculateNextRetry(retryCount)
	nextRetryAt := now.Add(backoff)

	// Create a copy of the message with updated retry metadata
	retryMsg := *msg
	retryMsg.RetryCount = retryCount
	retryMsg.LastError = err.Error()
	retryMsg.LastRetryAt = now
	retryMsg.NextRetryAt = nextRetryAt
	retryMsg.FailedBackends = failedBackends

	return &retryMsg
}

// ScheduleRetry publishes a message to the retry topic with delay
// The message will be reprocessed after the backoff period
func (h *RetryHandler) ScheduleRetry(ctx context.Context, msg *NotificationMessage) error {
	// In a production system with Kafka/Redpanda, we would:
	// 1. Publish to a retry topic with a timestamp header
	// 2. Use a separate consumer that polls the retry topic
	// 3. Check the NextRetryAt timestamp and requeue to main topic when ready
	//
	// For now, we'll publish back to the main topic
	// TODO: Implement proper retry topic with timestamp-based reprocessing

	return h.publisher.PublishMessage(ctx, msg)
}

// PublishToDLQ publishes a message to the Dead Letter Queue
// This is called when max retries are exhausted
func (h *RetryHandler) PublishToDLQ(ctx context.Context, msg *NotificationMessage) error {
	if h.dlqPublisher == nil {
		// DLQ not configured, just return error
		return fmt.Errorf("message exceeded max retries (%d): %s", h.config.MaxRetries, msg.ID)
	}

	failureReason := fmt.Sprintf("Exceeded max retries (%d). Last error: %s", msg.RetryCount, msg.LastError)
	return h.dlqPublisher.PublishToDLQ(ctx, msg, failureReason)
}

// HandleFailure handles a failed notification
// It either schedules a retry or sends to DLQ
func (h *RetryHandler) HandleFailure(ctx context.Context, msg *NotificationMessage, err error, failedBackends []string) error {
	if h.ShouldRetry(msg) {
		// Prepare message for retry
		retryMsg := h.PrepareRetry(msg, err, failedBackends)

		// Schedule retry
		if err := h.ScheduleRetry(ctx, retryMsg); err != nil {
			return fmt.Errorf("failed to schedule retry: %w", err)
		}

		return nil
	}

	// Max retries exhausted, send to DLQ
	return h.PublishToDLQ(ctx, msg)
}
