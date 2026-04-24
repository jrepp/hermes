package notifications

import (
	"time"
)

// NotificationType defines the type of notification
type NotificationType string

const (
	NotificationTypeEmail             NotificationType = "email"
	NotificationTypeDocumentApproved  NotificationType = "document_approved"
	NotificationTypeReviewRequested   NotificationType = "review_requested"
	NotificationTypeNewOwner          NotificationType = "new_owner"
	NotificationTypeDocumentPublished NotificationType = "document_published"
)

// NotificationMessage is the envelope for all notifications
type NotificationMessage struct {
	NextRetryAt     time.Time        `json:"next_retry_at,omitempty"`
	Timestamp       time.Time        `json:"timestamp"`
	LastRetryAt     time.Time        `json:"last_retry_at,omitempty"`
	TemplateContext map[string]any   `json:"template_context,omitempty"`
	Template        string           `json:"template,omitempty"`
	Body            string           `json:"body"`
	ProjectID       string           `json:"project_id,omitempty"`
	Type            NotificationType `json:"type"`
	ID              string           `json:"id"`
	UserID          string           `json:"user_id,omitempty"`
	Subject         string           `json:"subject"`
	DocumentUUID    string           `json:"document_uuid,omitempty"`
	BodyHTML        string           `json:"body_html"`
	LastError       string           `json:"last_error,omitempty"`
	Backends        []string         `json:"backends"`
	Recipients      []Recipient      `json:"recipients"`
	FailedBackends  []string         `json:"failed_backends,omitempty"`
	RetryCount      int              `json:"retry_count,omitempty"`
	Priority        int              `json:"priority"`
}

// Recipient defines a notification recipient
type Recipient struct {
	Email      string `json:"email,omitempty"`       // Email address
	Name       string `json:"name,omitempty"`        // Display name
	SlackID    string `json:"slack_id,omitempty"`    // Slack user ID
	TelegramID string `json:"telegram_id,omitempty"` // Telegram user ID
	DiscordID  string `json:"discord_id,omitempty"`  // Discord user ID
}
