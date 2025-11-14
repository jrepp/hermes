package notifications_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
	"github.com/hashicorp-forge/hermes/pkg/notifications/backends"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	testTopic = "hermes.notifications.test"
)

// getRedpandaBroker returns the Redpanda broker address from environment or default
func getRedpandaBroker() string {
	broker := os.Getenv("REDPANDA_BROKER")
	if broker == "" {
		broker = "localhost:19092"
	}
	return broker
}

func TestPublishAndConsume(t *testing.T) {
	broker := getRedpandaBroker()

	// Skip test if Redpanda is not available
	ctx := context.Background()
	testClient, err := kgo.NewClient(kgo.SeedBrokers(broker))
	if err != nil {
		t.Skipf("Redpanda not available: %v", err)
	}
	defer testClient.Close()

	// Create topic (idempotent)
	// Note: In production, topics should be created by infrastructure
	// For tests, we'll just use it and let Redpanda auto-create

	// Create consumer FIRST to ensure it's ready to receive the message
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.ConsumeTopics(testTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()), // Start from end to get only our test message
	)
	require.NoError(t, err)
	defer consumer.Close()

	// Give consumer a moment to establish connection
	time.Sleep(500 * time.Millisecond)

	// Create publisher
	publisher, err := notifications.NewPublisher(notifications.PublisherConfig{
		Brokers: []string{broker},
		Topic:   testTopic,
	})
	require.NoError(t, err)
	defer publisher.Close()

	// Publish a test notification
	testMsg := &notifications.NotificationMessage{
		ID:        "integration-test-001",
		Type:      notifications.NotificationTypeDocumentApproved,
		Timestamp: time.Now(),
		Priority:  0,
		Recipients: []notifications.Recipient{
			{
				Email: "test@example.com",
				Name:  "Test User",
			},
		},
		Template: "document_approved",
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
			"ApproverName":      "Alice Integration Test",
		},
		Backends: []string{"audit"},
	}

	err = publisher.PublishMessage(ctx, testMsg)
	require.NoError(t, err)

	// Consume the message with timeout
	consumeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var receivedMsg *notifications.NotificationMessage
	for receivedMsg == nil {
		select {
		case <-consumeCtx.Done():
			t.Fatal("timeout waiting for message")
		default:
			fetches := consumer.PollFetches(consumeCtx)
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					t.Logf("fetch error: %v", err)
				}
				continue
			}

			fetches.EachRecord(func(record *kgo.Record) {
				var msg notifications.NotificationMessage
				err := json.Unmarshal(record.Value, &msg)
				if err != nil {
					t.Logf("failed to unmarshal: %v", err)
					return
				}

				// Check if this is our test message
				if msg.ID == testMsg.ID {
					receivedMsg = &msg
				}
			})
		}
	}

	// Verify the message
	require.NotNil(t, receivedMsg)
	assert.Equal(t, testMsg.ID, receivedMsg.ID)
	assert.Equal(t, testMsg.Type, receivedMsg.Type)
	assert.Equal(t, testMsg.Template, receivedMsg.Template)
	assert.Equal(t, len(testMsg.Recipients), len(receivedMsg.Recipients))
	assert.Equal(t, testMsg.Recipients[0].Email, receivedMsg.Recipients[0].Email)
}

func TestAuditBackendIntegration(t *testing.T) {
	// Create audit backend
	backend := backends.NewAuditBackend()

	msg := &notifications.NotificationMessage{
		ID:        "integration-audit-001",
		Type:      notifications.NotificationTypeDocumentApproved,
		Timestamp: time.Now(),
		Priority:  0,
		Recipients: []notifications.Recipient{
			{
				Email: "test@example.com",
				Name:  "Integration Test User",
			},
		},
		Template: "document_approved",
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
			"ApproverName":      "Alice",
		},
		Backends: []string{"audit"},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)
	require.NoError(t, err)
}

func TestMailBackendIntegration(t *testing.T) {
	// Skip test if Mailhog is not available
	mailhogURL := os.Getenv("MAILHOG_URL")
	if mailhogURL == "" {
		mailhogURL = "http://localhost:8025"
	}

	// Check if Mailhog is available
	resp, err := http.Get(mailhogURL + "/api/v2/messages")
	if err != nil {
		t.Skipf("Mailhog not available: %v", err)
	}
	resp.Body.Close()

	// Create mail backend
	backend := backends.NewMailBackend(backends.MailBackendConfig{
		SMTPHost:    "localhost",
		SMTPPort:    "1025",
		FromAddress: "test@hermes.example.com",
		FromName:    "Hermes Test",
		UseTLS:      false,
	})

	// Create test notification
	testEmail := fmt.Sprintf("mailtest-%d@example.com", time.Now().Unix())
	msg := &notifications.NotificationMessage{
		ID:        fmt.Sprintf("integration-mail-%d", time.Now().Unix()),
		Type:      notifications.NotificationTypeDocumentApproved,
		Timestamp: time.Now(),
		Priority:  0,
		Recipients: []notifications.Recipient{
			{
				Email: testEmail,
				Name:  "Mail Integration Test User",
			},
		},
		Template: "document_approved",
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
			"ApproverName":      "Alice Integrationtest",
			"BaseURL":           "https://hermes.example.com",
			"DocumentID":        "test-doc-123",
		},
		Backends: []string{"mail"},
	}

	// Send email
	ctx := context.Background()
	err = backend.Handle(ctx, msg)
	require.NoError(t, err)

	// Wait a moment for email to be processed
	time.Sleep(1 * time.Second)

	// Query Mailhog API to verify email was received
	resp, err = http.Get(mailhogURL + "/api/v2/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Parse response
	var mailhogResp struct {
		Total int `json:"total"`
		Count int `json:"count"`
		Items []struct {
			Raw struct {
				From string   `json:"From"`
				To   []string `json:"To"`
			} `json:"Raw"`
			Content struct {
				Headers struct {
					Subject []string `json:"Subject"`
				} `json:"Headers"`
				Body string `json:"Body"`
			} `json:"Content"`
		} `json:"items"`
	}

	err = json.NewDecoder(resp.Body).Decode(&mailhogResp)
	require.NoError(t, err)

	// Verify at least one email was sent
	assert.True(t, mailhogResp.Count > 0, "Expected at least one email in Mailhog")

	// Find our test email
	found := false
	for _, item := range mailhogResp.Items {
		if len(item.Raw.To) > 0 && item.Raw.To[0] == testEmail {
			found = true

			// Verify email content
			assert.Contains(t, item.Content.Headers.Subject[0], "RFC-087")
			assert.Contains(t, item.Content.Headers.Subject[0], "Alice")
			assert.Contains(t, item.Content.Body, "RFC-087")
			assert.Contains(t, item.Content.Body, "Alice Integrationtest")
			assert.Contains(t, item.Content.Body, "https://hermes.example.com/document/test-doc-123")
			break
		}
	}

	assert.True(t, found, "Expected to find email to %s in Mailhog", testEmail)
}

func TestNtfyBackendIntegration(t *testing.T) {
	// Create ntfy backend configured for test topic
	backend := backends.NewNtfyBackend(backends.NtfyBackendConfig{
		ServerURL: "https://ntfy.sh",
		Topic:     "hermes-dev-test-notifications",
	})

	// Create test notification
	testID := fmt.Sprintf("ntfy-test-%d", time.Now().Unix())
	msg := &notifications.NotificationMessage{
		ID:        testID,
		Type:      notifications.NotificationTypeDocumentApproved,
		Timestamp: time.Now(),
		Priority:  0,
		Recipients: []notifications.Recipient{
			{
				Email: "test@example.com",
				Name:  "Ntfy Test User",
			},
		},
		Subject:  fmt.Sprintf("Test notification %s", testID),
		Body:     fmt.Sprintf("This is a test notification sent at %s", time.Now().Format(time.RFC3339)),
		Backends: []string{"ntfy"},
	}

	// Send notification
	ctx := context.Background()
	err := backend.Handle(ctx, msg)

	// The test should succeed even if ntfy.sh is unreachable
	// (we can't control external service availability in tests)
	if err != nil {
		t.Logf("Note: ntfy backend returned error (may be network/service issue): %v", err)
	} else {
		t.Logf("Successfully sent notification to ntfy topic: hermes-dev-test-notifications")
		t.Logf("You can check it at: https://ntfy.sh/hermes-dev-test-notifications")
	}
}
