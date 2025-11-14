package backends

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
)

// TestBackend is a mock backend for testing failure scenarios
// It allows tests to inject various types of failures to verify
// retry logic, DLQ processing, and error handling
type TestBackend struct {
	name     string
	mu       sync.RWMutex
	config   TestBackendConfig
	messages []TestBackendMessage
}

// TestBackendConfig configures the test backend behavior
type TestBackendConfig struct {
	// FailureMode determines how the backend should fail
	FailureMode FailureMode

	// FailureRate is the percentage of messages that should fail (0-100)
	// Only used when FailureMode is FailureModeIntermittent
	FailureRate int

	// FailureDelay adds artificial latency before processing
	FailureDelay time.Duration

	// FailureMessage is the error message to return
	FailureMessage string

	// RecordMessages enables recording of all processed messages for verification
	RecordMessages bool
}

// FailureMode defines how the test backend should behave
type FailureMode string

const (
	// FailureModeNone processes all messages successfully
	FailureModeNone FailureMode = "none"

	// FailureModeAlways always fails with a retryable error
	FailureModeAlways FailureMode = "always"

	// FailureModePermanent always fails with a permanent (non-retryable) error
	FailureModePermanent FailureMode = "permanent"

	// FailureModeIntermittent fails X% of messages (configured by FailureRate)
	FailureModeIntermittent FailureMode = "intermittent"

	// FailureModeTimeout simulates a timeout
	FailureModeTimeout FailureMode = "timeout"

	// FailureModeRateLimit simulates rate limiting (retryable with 429 status)
	FailureModeRateLimit FailureMode = "rate_limit"

	// FailureModeFirstNFail fails the first N messages, then succeeds
	// Useful for testing retry logic that eventually succeeds
	FailureModeFirstNFail FailureMode = "first_n_fail"

	// FailureModePartial simulates partial success (some recipients succeed, some fail)
	FailureModePartial FailureMode = "partial"
)

// TestBackendMessage records a processed message for verification
type TestBackendMessage struct {
	Message   *notifications.NotificationMessage
	Timestamp time.Time
	Success   bool
	Error     error
}

// NewTestBackend creates a new test backend
func NewTestBackend(config TestBackendConfig) *TestBackend {
	return &TestBackend{
		name:     "test",
		config:   config,
		messages: make([]TestBackendMessage, 0),
	}
}

// Name returns the backend name
func (b *TestBackend) Name() string {
	return b.name
}

// Handle processes a notification message according to the configured failure mode
func (b *TestBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Add artificial delay if configured
	if b.config.FailureDelay > 0 {
		time.Sleep(b.config.FailureDelay)
	}

	// Check for context cancellation (simulates timeout)
	select {
	case <-ctx.Done():
		err := NewBackendError("test", "send", true, ctx.Err())
		if b.config.RecordMessages {
			b.messages = append(b.messages, TestBackendMessage{
				Message:   msg,
				Timestamp: time.Now(),
				Success:   false,
				Error:     err,
			})
		}
		return err
	default:
	}

	var err error
	success := true

	switch b.config.FailureMode {
	case FailureModeNone:
		// Success - do nothing

	case FailureModeAlways:
		errMsg := b.config.FailureMessage
		if errMsg == "" {
			errMsg = "simulated retryable failure"
		}
		err = NewBackendError("test", "send", true, errors.New(errMsg))
		success = false

	case FailureModePermanent:
		errMsg := b.config.FailureMessage
		if errMsg == "" {
			errMsg = "simulated permanent failure"
		}
		err = NewBackendError("test", "send", false, errors.New(errMsg))
		success = false

	case FailureModeIntermittent:
		// Fail based on failure rate percentage
		messageCount := len(b.messages)
		shouldFail := (messageCount % 100) < b.config.FailureRate
		if shouldFail {
			err = NewBackendError("test", "send", true, errors.New("simulated intermittent failure"))
			success = false
		}

	case FailureModeTimeout:
		err = NewBackendError("test", "send", true, errors.New("simulated timeout"))
		success = false

	case FailureModeRateLimit:
		err = NewBackendError("test", "send", true, errors.New("simulated rate limit (429)"))
		success = false

	case FailureModeFirstNFail:
		// Fail the first N messages (N = FailureRate)
		messageCount := len(b.messages)
		if messageCount < b.config.FailureRate {
			err = NewBackendError("test", "send", true,
				fmt.Errorf("simulated failure %d/%d", messageCount+1, b.config.FailureRate))
			success = false
		}

	case FailureModePartial:
		// Simulate partial success - some recipients succeed, some fail
		if len(msg.Recipients) > 1 {
			err = NewBackendError("test", "send", true,
				fmt.Errorf("partial failure: %d/%d recipients failed", len(msg.Recipients)/2, len(msg.Recipients)))
			success = false
		}

	default:
		err = NewBackendError("test", "send", false,
			fmt.Errorf("unknown failure mode: %s", b.config.FailureMode))
		success = false
	}

	// Record message if configured
	if b.config.RecordMessages {
		b.messages = append(b.messages, TestBackendMessage{
			Message:   msg,
			Timestamp: time.Now(),
			Success:   success,
			Error:     err,
		})
	}

	return err
}

// SupportsBackend checks if this backend should process the message
func (b *TestBackend) SupportsBackend(backend string) bool {
	return backend == "test"
}

// GetMessages returns all recorded messages (for test verification)
func (b *TestBackend) GetMessages() []TestBackendMessage {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy to avoid race conditions
	messages := make([]TestBackendMessage, len(b.messages))
	copy(messages, b.messages)
	return messages
}

// GetMessageCount returns the number of processed messages
func (b *TestBackend) GetMessageCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.messages)
}

// GetSuccessCount returns the number of successfully processed messages
func (b *TestBackend) GetSuccessCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := 0
	for _, msg := range b.messages {
		if msg.Success {
			count++
		}
	}
	return count
}

// GetFailureCount returns the number of failed messages
func (b *TestBackend) GetFailureCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := 0
	for _, msg := range b.messages {
		if !msg.Success {
			count++
		}
	}
	return count
}

// Reset clears all recorded messages and resets counters
func (b *TestBackend) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = make([]TestBackendMessage, 0)
}

// SetFailureMode dynamically changes the failure mode
func (b *TestBackend) SetFailureMode(mode FailureMode) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.FailureMode = mode
}

// SetFailureRate dynamically changes the failure rate
func (b *TestBackend) SetFailureRate(rate int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if rate < 0 {
		rate = 0
	}
	if rate > 100 {
		rate = 100
	}
	b.config.FailureRate = rate
}
