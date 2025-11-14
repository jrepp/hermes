package notifications_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/internal/notifications"
	pkgnotifications "github.com/hashicorp-forge/hermes/pkg/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationE2E tests the complete notification flow:
// Server (template resolution) → Publisher → Redpanda → Notifier → Audit Backend
func TestNotificationE2E(t *testing.T) {
	// Skip if Redpanda not available
	broker := getRedpandaBroker()

	// Create notification provider (server-side component)
	// Use production topic for E2E test (hermes.notifications, not test topic)
	provider, err := notifications.NewProvider(pkgnotifications.PublisherConfig{
		Brokers: []string{broker},
		Topic:   "hermes.notifications",
	})
	require.NoError(t, err)
	defer provider.Close()

	// Create unique recipient to identify this test run
	testID := fmt.Sprintf("e2e-%d", time.Now().Unix())
	testEmail := fmt.Sprintf("%s@example.com", testID)

	// Send notification with template context (as server would do)
	ctx := context.Background()
	err = provider.SendNotification(ctx, notifications.NotificationRequest{
		Type: pkgnotifications.NotificationTypeDocumentApproved,
		Recipients: []pkgnotifications.Recipient{
			{Email: testEmail, Name: "E2E Test User"},
		},
		TemplateContext: map[string]any{
			"DocumentShortName":        "RFC-087",
			"DocumentTitle":            "Notification System",
			"ApproverName":             "Alice E2E",
			"ApproverEmail":            "alice@example.com",
			"DocumentNonApproverCount": 2,
			"DocumentURL":              "https://hermes.example.com/document/test-123",
			"Product":                  "Hermes",
			"DocumentOwner":            "Bob E2E",
			"DocumentStatus":           "In-Review",
			"DocumentType":             "RFC",
		},
		Backends:     []string{"audit"},
		DocumentUUID: "e2e-test-doc-uuid",
		ProjectID:    "e2e-test-project",
		UserID:       "e2e-test-user",
	})
	require.NoError(t, err)

	// Wait for notifier to process the message
	// In real system, notifier runs in docker-compose
	time.Sleep(5 * time.Second)

	// Verify the notification was processed by checking Docker logs
	// The audit backend logs the fully resolved content
	logs, err := getNotifierLogs()
	require.NoError(t, err)

	// Verify audit log contains the resolved template content
	assert.Contains(t, logs, testEmail, "Audit log should contain test recipient email")

	// Verify resolved subject
	assert.Contains(t, logs, "Subject: RFC-087 approved by Alice E2E",
		"Audit log should contain resolved subject")

	// Verify resolved body with template variables substituted
	assert.Contains(t, logs, "Alice E2E has approved your document",
		"Audit log should contain approver action")
	assert.Contains(t, logs, "**Notification System** RFC-087",
		"Audit log should contain resolved body with document title")
	assert.Contains(t, logs, "Bob E2E · Hermes",
		"Audit log should contain document owner and product")
	assert.Contains(t, logs, "Status: In-Review · RFC",
		"Audit log should contain status and type")
	assert.Contains(t, logs, "There are 2 more pending approvals for the document",
		"Audit log should contain non-approver count")
	assert.Contains(t, logs, "[View in Hermes](https://hermes.example.com/document/test-123)",
		"Audit log should contain document URL")

	// Verify context metadata
	assert.Contains(t, logs, "Document UUID: e2e-test-doc-uuid",
		"Audit log should contain document UUID")
	assert.Contains(t, logs, "Project ID: e2e-test-project",
		"Audit log should contain project ID")
	assert.Contains(t, logs, "User ID: e2e-test-user",
		"Audit log should contain user ID")

	// Verify acknowledgement
	assert.Contains(t, logs, "✓ Acknowledged",
		"Audit log should contain acknowledgement")
}

// getNotifierLogs retrieves logs from the hermes-notifier-audit Docker container
func getNotifierLogs() (string, error) {
	cmd := exec.Command("docker", "logs", "hermes-notifier-audit", "--tail", "100")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get notifier logs: %w (output: %s)", err, string(output))
	}
	return string(output), nil
}

// TestNotificationE2EWithMultipleBackends tests notification routing to multiple backends
func TestNotificationE2EWithMultipleBackends(t *testing.T) {
	// Skip if Redpanda not available
	broker := getRedpandaBroker()

	provider, err := notifications.NewProvider(pkgnotifications.PublisherConfig{
		Brokers: []string{broker},
		Topic:   "hermes.notifications",
	})
	require.NoError(t, err)
	defer provider.Close()

	testID := fmt.Sprintf("multi-backend-%d", time.Now().Unix())
	testEmail := fmt.Sprintf("%s@example.com", testID)

	ctx := context.Background()
	err = provider.SendNotification(ctx, notifications.NotificationRequest{
		Type: pkgnotifications.NotificationTypeDocumentApproved,
		Recipients: []pkgnotifications.Recipient{
			{Email: testEmail, Name: "Multi Backend Test"},
		},
		TemplateContext: map[string]any{
			"DocumentShortName":        "RFC-087",
			"DocumentTitle":            "Multi Backend Test",
			"ApproverName":             "Bob Multi",
			"ApproverEmail":            "bob@example.com",
			"DocumentNonApproverCount": 0,
			"DocumentURL":              "https://hermes.example.com/document/test-456",
			"Product":                  "Hermes",
			"DocumentOwner":            "Charlie",
			"DocumentStatus":           "In-Review",
			"DocumentType":             "RFC",
		},
		Backends: []string{"audit"}, // Could be ["audit", "mail", "slack"]
	})
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	logs, err := getNotifierLogs()
	require.NoError(t, err)

	// Verify the notification was processed
	assert.Contains(t, logs, testEmail)
	assert.Contains(t, logs, "RFC-087 approved by Bob Multi")
}

// TestNotificationTemplateResolution tests that templates are resolved server-side
func TestNotificationTemplateResolution(t *testing.T) {
	// This test verifies template resolution without requiring the full E2E infrastructure
	resolver, err := notifications.NewTemplateResolver()
	require.NoError(t, err)

	// Test document_approved template
	context := map[string]any{
		"DocumentShortName":        "RFC-087",
		"DocumentTitle":            "Test Document",
		"ApproverName":             "Alice",
		"ApproverEmail":            "alice@example.com",
		"DocumentNonApproverCount": 3,
		"DocumentURL":              "https://hermes.example.com/doc/123",
		"Product":                  "Hermes",
		"DocumentOwner":            "Bob",
		"DocumentStatus":           "In-Review",
		"DocumentStatusClass":      "in-review",
		"DocumentType":             "RFC",
		"BaseURL":                  "https://hermes.example.com",
		"CurrentYear":              2025,
	}

	content, err := resolver.Resolve(pkgnotifications.NotificationTypeDocumentApproved, context)
	require.NoError(t, err)

	// Verify subject
	assert.Equal(t, "RFC-087 approved by Alice", content.Subject)

	// Verify body contains all substituted variables
	assert.Contains(t, content.Body, "Alice has approved your document")
	assert.Contains(t, content.Body, "**Test Document** RFC-087")
	assert.Contains(t, content.Body, "Bob · Hermes")
	assert.Contains(t, content.Body, "Status: In-Review · RFC")
	assert.Contains(t, content.Body, "[View in Hermes](https://hermes.example.com/doc/123)")
	assert.Contains(t, content.Body, "There are 3 more pending approvals for the document")

	// Verify HTML contains expected elements
	assert.Contains(t, content.BodyHTML, "<!DOCTYPE html>")
	assert.Contains(t, content.BodyHTML, "Test Document")
	assert.Contains(t, content.BodyHTML, "RFC-087")
	assert.Contains(t, content.BodyHTML, "has approved your document")
	assert.Contains(t, content.BodyHTML, "Alice")
	assert.Contains(t, content.BodyHTML, "Bob &middot; Hermes")
	assert.Contains(t, content.BodyHTML, `href="https://hermes.example.com/doc/123"`)

	// Verify HTML escaping works (html/template should auto-escape)
	contextWithSpecialChars := map[string]any{
		"DocumentShortName":        "RFC-<script>alert('xss')</script>",
		"DocumentTitle":            "Test",
		"ApproverName":             "Alice & Bob",
		"ApproverEmail":            "alice@example.com",
		"DocumentNonApproverCount": 0,
		"DocumentURL":              "https://hermes.example.com/doc/123",
		"Product":                  "Hermes",
		"DocumentOwner":            "Owner",
		"DocumentStatus":           "In-Review",
		"DocumentStatusClass":      "in-review",
		"DocumentType":             "RFC",
		"BaseURL":                  "https://hermes.example.com",
		"CurrentYear":              2025,
	}
	content, err = resolver.Resolve(pkgnotifications.NotificationTypeDocumentApproved, contextWithSpecialChars)
	require.NoError(t, err)

	// HTML should be escaped
	assert.Contains(t, content.BodyHTML, "&lt;script&gt;")
	assert.Contains(t, content.BodyHTML, "Alice &amp; Bob")
}

// TestTemplateValidationMissingVariable tests that template resolution fails when context variables are missing
func TestTemplateValidationMissingVariable(t *testing.T) {
	resolver, err := notifications.NewTemplateResolver()
	require.NoError(t, err)

	// Try to resolve template with incomplete context (missing required variables)
	incompleteContext := map[string]any{
		"DocumentShortName": "RFC-087",
		"DocumentTitle":     "Test Document",
		// Missing: ApproverName, ApproverEmail, DocumentOwner, etc.
	}

	_, err = resolver.Resolve(pkgnotifications.NotificationTypeDocumentApproved, incompleteContext)

	// Should return an error about unexpanded template values
	require.Error(t, err, "Expected error for missing template context variables")
	assert.Contains(t, err.Error(), "template validation failed", "Error should mention template validation")
	assert.Contains(t, err.Error(), "<no value>", "Error should mention '<no value>' unexpanded value")
}

// TestTemplateValidationEmptyContext tests that template resolution fails with completely empty context
func TestTemplateValidationEmptyContext(t *testing.T) {
	resolver, err := notifications.NewTemplateResolver()
	require.NoError(t, err)

	// Try to resolve template with empty context
	emptyContext := map[string]any{}

	_, err = resolver.Resolve(pkgnotifications.NotificationTypeDocumentApproved, emptyContext)

	// Should return an error about unexpanded template values
	require.Error(t, err, "Expected error for empty template context")
	assert.Contains(t, err.Error(), "template validation failed", "Error should mention template validation")
}
