package backends

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailBackend(t *testing.T) {
	backend := NewMailBackend(MailBackendConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    "587",
		FromAddress: "notifications@example.com",
		FromName:    "Hermes Notifications",
		UseTLS:      true,
	})

	assert.Equal(t, "mail", backend.Name())
	assert.True(t, backend.SupportsBackend("mail"))
	assert.True(t, backend.SupportsBackend("email"))
	assert.False(t, backend.SupportsBackend("slack"))
}

func TestMailBackendBuildSubject(t *testing.T) {
	backend := NewMailBackend(MailBackendConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    "587",
		FromAddress: "notifications@example.com",
	})

	tests := []struct {
		name     string
		msg      *notifications.NotificationMessage
		expected string
	}{
		{
			name: "document approved with context",
			msg: &notifications.NotificationMessage{
				Type: notifications.NotificationTypeDocumentApproved,
				TemplateContext: map[string]any{
					"DocumentShortName": "RFC-087",
					"ApproverName":      "Alice",
				},
			},
			expected: "RFC-087 approved by Alice",
		},
		{
			name: "document approved without context",
			msg: &notifications.NotificationMessage{
				Type:            notifications.NotificationTypeDocumentApproved,
				TemplateContext: map[string]any{},
			},
			expected: "Document approved",
		},
		{
			name: "review requested with context",
			msg: &notifications.NotificationMessage{
				Type: notifications.NotificationTypeReviewRequested,
				TemplateContext: map[string]any{
					"DocumentShortName": "RFC-088",
				},
			},
			expected: "Document review requested for RFC-088",
		},
		{
			name: "document published with context",
			msg: &notifications.NotificationMessage{
				Type: notifications.NotificationTypeDocumentPublished,
				TemplateContext: map[string]any{
					"DocumentShortName": "RFC-089",
				},
			},
			expected: "Document published: RFC-089",
		},
		{
			name: "unknown notification type",
			msg: &notifications.NotificationMessage{
				Type:            "unknown",
				TemplateContext: map[string]any{},
			},
			expected: "Hermes notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject := backend.buildSubject(tt.msg)
			assert.Equal(t, tt.expected, subject)
		})
	}
}

func TestMailBackendBuildBody(t *testing.T) {
	backend := NewMailBackend(MailBackendConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    "587",
		FromAddress: "notifications@example.com",
	})

	msg := &notifications.NotificationMessage{
		Type: notifications.NotificationTypeDocumentApproved,
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
			"ApproverName":      "Alice",
		},
	}

	body, err := backend.buildBody(msg)
	require.NoError(t, err)

	// Check that HTML body contains expected elements
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "document_approved")
	assert.Contains(t, body, "RFC-087")
	assert.Contains(t, body, "Alice")
}

func TestMailBackendRenderEmail(t *testing.T) {
	backend := NewMailBackend(MailBackendConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    "587",
		FromAddress: "notifications@example.com",
		FromName:    "Hermes",
	})

	msg := &notifications.NotificationMessage{
		Type:      notifications.NotificationTypeDocumentApproved,
		Timestamp: time.Now(),
		Recipients: []notifications.Recipient{
			{Email: "user@example.com", Name: "Test User"},
		},
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
			"ApproverName":      "Alice",
			"BaseURL":           "https://hermes.example.com",
			"DocumentID":        "doc-123",
		},
	}

	subject, body, err := backend.renderEmail(msg)
	require.NoError(t, err)

	// Verify subject
	assert.Equal(t, "RFC-087 approved by Alice", subject)

	// Verify body contains key elements
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "RFC-087")
	assert.Contains(t, body, "Alice")
	assert.Contains(t, body, "https://hermes.example.com/document/doc-123")
}

func TestMailBackendHandle_NoEmailRecipients(t *testing.T) {
	backend := NewMailBackend(MailBackendConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    "587",
		FromAddress: "notifications@example.com",
	})

	msg := &notifications.NotificationMessage{
		Type:      notifications.NotificationTypeDocumentApproved,
		Timestamp: time.Now(),
		Recipients: []notifications.Recipient{
			{SlackID: "U12345"},  // No email
			{TelegramID: "T123"}, // No email
		},
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
		},
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no email recipients found")
}

func TestMailBackendHandle_WithEmailRecipients(t *testing.T) {
	// This test verifies the Handle method logic, but won't actually send emails
	// In a real integration test, you'd use a test SMTP server like MailHog
	backend := NewMailBackend(MailBackendConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    "587",
		FromAddress: "notifications@example.com",
		UseTLS:      false,
	})

	msg := &notifications.NotificationMessage{
		Type:      notifications.NotificationTypeDocumentApproved,
		Timestamp: time.Now(),
		Recipients: []notifications.Recipient{
			{Email: "user1@example.com", Name: "User One"},
			{Email: "user2@example.com", Name: "User Two"},
		},
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
			"ApproverName":      "Alice",
		},
	}

	ctx := context.Background()

	// This will fail to connect to the SMTP server, which is expected in unit tests
	// We're mainly testing that the logic runs without panicking
	err := backend.Handle(ctx, msg)

	// We expect an error because the SMTP server doesn't exist
	// but the error should be about connection, not about missing recipients or rendering
	if err != nil {
		assert.True(t,
			strings.Contains(err.Error(), "connect") ||
				strings.Contains(err.Error(), "dial") ||
				strings.Contains(err.Error(), "lookup"),
			"Expected connection error, got: %v", err)
	}
}

func TestMailBackendConfig(t *testing.T) {
	tests := []struct {
		name   string
		config MailBackendConfig
	}{
		{
			name: "full configuration",
			config: MailBackendConfig{
				SMTPHost:     "smtp.gmail.com",
				SMTPPort:     "587",
				SMTPUsername: "user@gmail.com",
				SMTPPassword: "password",
				FromAddress:  "notifications@example.com",
				FromName:     "Hermes Notifications",
				UseTLS:       true,
			},
		},
		{
			name: "minimal configuration without auth",
			config: MailBackendConfig{
				SMTPHost:    "localhost",
				SMTPPort:    "25",
				FromAddress: "test@localhost",
				UseTLS:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := NewMailBackend(tt.config)
			require.NotNil(t, backend)
			assert.Equal(t, "mail", backend.Name())
			assert.Equal(t, tt.config.SMTPHost, backend.smtpHost)
			assert.Equal(t, tt.config.SMTPPort, backend.smtpPort)
			assert.Equal(t, tt.config.FromAddress, backend.fromAddress)
		})
	}
}
