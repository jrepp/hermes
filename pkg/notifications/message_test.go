package notifications

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationMessageSerialization(t *testing.T) {
	// Create a sample notification message
	msg := NotificationMessage{
		ID:        "test-123",
		Type:      NotificationTypeDocumentApproved,
		Timestamp: time.Date(2025, 11, 14, 10, 30, 0, 0, time.UTC),
		Priority:  0,
		Recipients: []Recipient{
			{
				Email:   "user@example.com",
				Name:    "Test User",
				SlackID: "U123456",
			},
		},
		Template: "document_approved",
		TemplateContext: map[string]any{
			"DocumentShortName": "RFC-087",
			"DocumentTitle":     "Notification System",
			"ApproverName":      "Alice",
		},
		Backends:     []string{"mail", "slack", "audit"},
		DocumentUUID: "doc-uuid-123",
	}

	// Serialize to JSON
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize from JSON
	var decoded NotificationMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.Template, decoded.Template)
	assert.Equal(t, msg.DocumentUUID, decoded.DocumentUUID)
	assert.Equal(t, len(msg.Recipients), len(decoded.Recipients))
	assert.Equal(t, msg.Recipients[0].Email, decoded.Recipients[0].Email)
	assert.Equal(t, msg.Recipients[0].Name, decoded.Recipients[0].Name)
	assert.Equal(t, msg.Recipients[0].SlackID, decoded.Recipients[0].SlackID)
	assert.Equal(t, len(msg.Backends), len(decoded.Backends))
	assert.Equal(t, msg.Backends[0], decoded.Backends[0])
}

func TestRecipientSerialization(t *testing.T) {
	recipient := Recipient{
		Email:      "test@example.com",
		Name:       "Test User",
		SlackID:    "U123",
		TelegramID: "456",
		DiscordID:  "789",
	}

	data, err := json.Marshal(recipient)
	require.NoError(t, err)

	var decoded Recipient
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, recipient.Email, decoded.Email)
	assert.Equal(t, recipient.Name, decoded.Name)
	assert.Equal(t, recipient.SlackID, decoded.SlackID)
	assert.Equal(t, recipient.TelegramID, decoded.TelegramID)
	assert.Equal(t, recipient.DiscordID, decoded.DiscordID)
}

func TestNotificationTypeConstants(t *testing.T) {
	// Verify notification type constants are defined
	assert.Equal(t, NotificationType("email"), NotificationTypeEmail)
	assert.Equal(t, NotificationType("document_approved"), NotificationTypeDocumentApproved)
	assert.Equal(t, NotificationType("review_requested"), NotificationTypeReviewRequested)
	assert.Equal(t, NotificationType("new_owner"), NotificationTypeNewOwner)
	assert.Equal(t, NotificationType("document_published"), NotificationTypeDocumentPublished)
}

func TestTemplateContextComplexTypes(t *testing.T) {
	// Test that template context can handle various types
	msg := NotificationMessage{
		ID:       "test-context",
		Type:     NotificationTypeEmail,
		Template: "test",
		TemplateContext: map[string]any{
			"string": "value",
			"int":    42,
			"float":  3.14,
			"bool":   true,
			"array":  []string{"a", "b", "c"},
			"nested": map[string]any{"key": "value"},
		},
		Recipients: []Recipient{{Email: "test@example.com"}},
		Backends:   []string{"audit"},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded NotificationMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "value", decoded.TemplateContext["string"])
	assert.Equal(t, float64(42), decoded.TemplateContext["int"]) // JSON numbers are float64
	assert.Equal(t, 3.14, decoded.TemplateContext["float"])
	assert.Equal(t, true, decoded.TemplateContext["bool"])
}
