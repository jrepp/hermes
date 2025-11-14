package backends

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditBackend(t *testing.T) {
	backend := NewAuditBackend()

	assert.Equal(t, "audit", backend.Name())
	assert.True(t, backend.SupportsBackend("audit"))
	assert.False(t, backend.SupportsBackend("mail"))
}

func TestAuditBackendHandle(t *testing.T) {
	backend := NewAuditBackend()

	msg := &notifications.NotificationMessage{
		ID:        "test-audit-001",
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
			"ApproverName":      "Alice",
		},
		Backends:     []string{"audit"},
		DocumentUUID: "doc-123",
	}

	ctx := context.Background()
	err := backend.Handle(ctx, msg)
	require.NoError(t, err)
}

func TestFormatRecipients(t *testing.T) {
	tests := []struct {
		name       string
		recipients []notifications.Recipient
		expected   string
	}{
		{
			name: "single recipient with name",
			recipients: []notifications.Recipient{
				{Email: "user@example.com", Name: "Test User"},
			},
			expected: "Test User <user@example.com>",
		},
		{
			name: "single recipient without name",
			recipients: []notifications.Recipient{
				{Email: "user@example.com"},
			},
			expected: "user@example.com",
		},
		{
			name: "multiple recipients",
			recipients: []notifications.Recipient{
				{Email: "user1@example.com", Name: "User One"},
				{Email: "user2@example.com"},
			},
			expected: "User One <user1@example.com>, user2@example.com",
		},
		{
			name:       "no recipients",
			recipients: []notifications.Recipient{},
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRecipients(tt.recipients)
			assert.Equal(t, tt.expected, result)
		})
	}
}
