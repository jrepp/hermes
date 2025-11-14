package backends_test

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
	"github.com/hashicorp-forge/hermes/pkg/notifications/backends"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestBackend_FailureModeNone(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeNone,
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-001",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)

	require.NoError(t, err, "Expected no error in FailureModeNone")
	assert.Equal(t, 1, backend.GetMessageCount())
	assert.Equal(t, 1, backend.GetSuccessCount())
	assert.Equal(t, 0, backend.GetFailureCount())
}

func TestTestBackend_FailureModeAlways(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeAlways,
		FailureMessage: "test retryable failure",
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-002",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)

	require.Error(t, err, "Expected error in FailureModeAlways")
	assert.Contains(t, err.Error(), "test retryable failure")

	// Verify error is retryable
	var backendErr *backends.BackendError
	require.ErrorAs(t, err, &backendErr)
	assert.True(t, backendErr.IsRetryable(), "Error should be retryable")

	assert.Equal(t, 1, backend.GetMessageCount())
	assert.Equal(t, 0, backend.GetSuccessCount())
	assert.Equal(t, 1, backend.GetFailureCount())
}

func TestTestBackend_FailureModePermanent(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModePermanent,
		FailureMessage: "test permanent failure",
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-003",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)

	require.Error(t, err, "Expected error in FailureModePermanent")
	assert.Contains(t, err.Error(), "test permanent failure")

	// Verify error is NOT retryable
	var backendErr *backends.BackendError
	require.ErrorAs(t, err, &backendErr)
	assert.False(t, backendErr.IsRetryable(), "Error should not be retryable")

	assert.Equal(t, 1, backend.GetMessageCount())
	assert.Equal(t, 0, backend.GetSuccessCount())
	assert.Equal(t, 1, backend.GetFailureCount())
}

func TestTestBackend_FailureModeIntermittent(t *testing.T) {
	// Configure 50% failure rate
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeIntermittent,
		FailureRate:    50,
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-004",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()
	successCount := 0
	failureCount := 0

	// Send 100 messages
	for i := 0; i < 100; i++ {
		err := backend.Handle(ctx, msg)
		if err != nil {
			failureCount++
		} else {
			successCount++
		}
	}

	// Verify roughly 50% failure rate (allow 10% variance)
	assert.Equal(t, 100, backend.GetMessageCount())
	assert.InDelta(t, 50, failureCount, 10, "Failure rate should be around 50%")
	assert.InDelta(t, 50, successCount, 10, "Success rate should be around 50%")
}

func TestTestBackend_FailureModeTimeout(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeTimeout,
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-005",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)

	require.Error(t, err, "Expected error in FailureModeTimeout")
	assert.Contains(t, err.Error(), "timeout")

	var backendErr *backends.BackendError
	require.ErrorAs(t, err, &backendErr)
	assert.True(t, backendErr.IsRetryable(), "Timeout should be retryable")
}

func TestTestBackend_FailureModeRateLimit(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeRateLimit,
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-006",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)

	require.Error(t, err, "Expected error in FailureModeRateLimit")
	assert.Contains(t, err.Error(), "rate limit")

	var backendErr *backends.BackendError
	require.ErrorAs(t, err, &backendErr)
	assert.True(t, backendErr.IsRetryable(), "Rate limit should be retryable")
}

func TestTestBackend_FailureModeFirstNFail(t *testing.T) {
	// Fail the first 3 messages, then succeed
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeFirstNFail,
		FailureRate:    3, // FailureRate is reused as N for FirstNFail mode
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-007",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()

	// First 3 should fail
	for i := 0; i < 3; i++ {
		err := backend.Handle(ctx, msg)
		require.Error(t, err, "Expected error for message %d", i+1)
	}

	// Next messages should succeed
	for i := 0; i < 5; i++ {
		err := backend.Handle(ctx, msg)
		require.NoError(t, err, "Expected success for message %d", i+4)
	}

	assert.Equal(t, 8, backend.GetMessageCount())
	assert.Equal(t, 5, backend.GetSuccessCount())
	assert.Equal(t, 3, backend.GetFailureCount())
}

func TestTestBackend_FailureModePartial(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModePartial,
		RecordMessages: true,
	})

	// Message with multiple recipients
	msg := &notifications.NotificationMessage{
		ID:   "test-008",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "user1@example.com", Name: "User 1"},
			{Email: "user2@example.com", Name: "User 2"},
			{Email: "user3@example.com", Name: "User 3"},
			{Email: "user4@example.com", Name: "User 4"},
		},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)

	require.Error(t, err, "Expected partial failure error")
	assert.Contains(t, err.Error(), "partial failure")

	// Single recipient should succeed
	singleRecipientMsg := &notifications.NotificationMessage{
		ID:   "test-009",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "single@example.com", Name: "Single User"},
		},
	}

	err = backend.Handle(ctx, singleRecipientMsg)
	require.NoError(t, err, "Single recipient should succeed")
}

func TestTestBackend_FailureDelay(t *testing.T) {
	delay := 100 * time.Millisecond
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeNone,
		FailureDelay:   delay,
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-010",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()
	start := time.Now()
	err := backend.Handle(ctx, msg)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, delay, "Should have delayed at least %v", delay)
}

func TestTestBackend_ContextCancellation(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeNone,
		FailureDelay:   1 * time.Second, // Long delay
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-011",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	// Context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := backend.Handle(ctx, msg)

	require.Error(t, err, "Expected error from context cancellation")
	assert.Contains(t, err.Error(), "context")

	var backendErr *backends.BackendError
	require.ErrorAs(t, err, &backendErr)
	assert.True(t, backendErr.IsRetryable(), "Context cancellation should be retryable")
}

func TestTestBackend_Reset(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeNone,
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-012",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()

	// Send 5 messages
	for i := 0; i < 5; i++ {
		_ = backend.Handle(ctx, msg)
	}

	assert.Equal(t, 5, backend.GetMessageCount())

	// Reset
	backend.Reset()

	assert.Equal(t, 0, backend.GetMessageCount())
	assert.Equal(t, 0, backend.GetSuccessCount())
	assert.Equal(t, 0, backend.GetFailureCount())
}

func TestTestBackend_DynamicFailureMode(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeNone,
		RecordMessages: true,
	})

	msg := &notifications.NotificationMessage{
		ID:   "test-013",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test@example.com", Name: "Test User"},
		},
	}

	ctx := context.Background()

	// Start with success
	err := backend.Handle(ctx, msg)
	require.NoError(t, err)

	// Switch to failure mode
	backend.SetFailureMode(backends.FailureModeAlways)
	err = backend.Handle(ctx, msg)
	require.Error(t, err)

	// Switch back to success
	backend.SetFailureMode(backends.FailureModeNone)
	err = backend.Handle(ctx, msg)
	require.NoError(t, err)

	assert.Equal(t, 3, backend.GetMessageCount())
	assert.Equal(t, 2, backend.GetSuccessCount())
	assert.Equal(t, 1, backend.GetFailureCount())
}

func TestTestBackend_GetMessages(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{
		FailureMode:    backends.FailureModeNone,
		RecordMessages: true,
	})

	msg1 := &notifications.NotificationMessage{
		ID:   "test-014",
		Type: notifications.NotificationTypeDocumentApproved,
		Recipients: []notifications.Recipient{
			{Email: "test1@example.com", Name: "Test User 1"},
		},
	}

	msg2 := &notifications.NotificationMessage{
		ID:   "test-015",
		Type: notifications.NotificationTypeReviewRequested,
		Recipients: []notifications.Recipient{
			{Email: "test2@example.com", Name: "Test User 2"},
		},
	}

	ctx := context.Background()
	_ = backend.Handle(ctx, msg1)
	_ = backend.Handle(ctx, msg2)

	messages := backend.GetMessages()
	require.Len(t, messages, 2)

	assert.Equal(t, "test-014", messages[0].Message.ID)
	assert.Equal(t, "test-015", messages[1].Message.ID)
	assert.True(t, messages[0].Success)
	assert.True(t, messages[1].Success)
}

func TestTestBackend_Name(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{})
	assert.Equal(t, "test", backend.Name())
}

func TestTestBackend_SupportsBackend(t *testing.T) {
	backend := backends.NewTestBackend(backends.TestBackendConfig{})

	assert.True(t, backend.SupportsBackend("test"))
	assert.False(t, backend.SupportsBackend("mail"))
	assert.False(t, backend.SupportsBackend("slack"))
}
