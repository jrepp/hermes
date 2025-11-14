package backends

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
)

// NtfyBackend sends push notifications via ntfy.sh
type NtfyBackend struct {
	serverURL string
	topic     string
	client    *http.Client
}

// NtfyBackendConfig holds configuration for the ntfy backend
type NtfyBackendConfig struct {
	// ServerURL is the ntfy server URL (e.g., "https://ntfy.sh")
	ServerURL string

	// Topic is the ntfy topic to send notifications to
	Topic string

	// Timeout for HTTP requests (optional, defaults to 10s)
	Timeout time.Duration
}

// NewNtfyBackend creates a new ntfy backend
func NewNtfyBackend(cfg NtfyBackendConfig) *NtfyBackend {
	// Default values
	if cfg.ServerURL == "" {
		cfg.ServerURL = "https://ntfy.sh"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	return &NtfyBackend{
		serverURL: cfg.ServerURL,
		topic:     cfg.Topic,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name returns the backend identifier
func (b *NtfyBackend) Name() string {
	return "ntfy"
}

// SupportsBackend checks if this backend should process the message
func (b *NtfyBackend) SupportsBackend(backend string) bool {
	return backend == "ntfy"
}

// Handle processes a notification message
func (b *NtfyBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
	// Use the resolved body (markdown format)
	messageBody := msg.Body
	if messageBody == "" {
		messageBody = fmt.Sprintf("Notification: %s", msg.Type)
	}

	// Create the ntfy notification URL
	url := fmt.Sprintf("%s/%s", b.serverURL, b.topic)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(messageBody))
	if err != nil {
		return fmt.Errorf("failed to create ntfy request: %w", err)
	}

	// Set ntfy headers
	// Title: Use the subject
	if msg.Subject != "" {
		req.Header.Set("Title", msg.Subject)
	}

	// Priority: Map notification priority to ntfy priority (1=min, 3=default, 5=max)
	ntfyPriority := "3" // default
	if msg.Priority > 0 {
		ntfyPriority = "5" // urgent
	} else if msg.Priority < 0 {
		ntfyPriority = "1" // low
	}
	req.Header.Set("Priority", ntfyPriority)

	// Tags: Add notification type as tag
	req.Header.Set("Tags", string(msg.Type))

	// Send the request
	resp, err := b.client.Do(req)
	if err != nil {
		// Network errors are retryable (RFC-087-ADDENDUM Section 9)
		return NewBackendError("ntfy", "send", true, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Classify error as retryable or permanent
		retryable := isRetryableHTTPStatus(resp.StatusCode)
		return NewBackendError("ntfy", "send", retryable,
			fmt.Errorf("ntfy request failed with status %d", resp.StatusCode))
	}

	return nil
}

// isRetryableHTTPStatus determines if an HTTP status code represents a retryable error
func isRetryableHTTPStatus(status int) bool {
	// Retryable: 5xx (server errors), 429 (rate limit), 408 (timeout)
	// Permanent: 4xx (client errors, except 429 and 408)
	switch {
	case status >= 500: // 5xx server errors
		return true
	case status == 429: // Too Many Requests
		return true
	case status == 408: // Request Timeout
		return true
	case status >= 400 && status < 500: // Other 4xx errors (bad request, auth, etc.)
		return false
	default:
		return false
	}
}
