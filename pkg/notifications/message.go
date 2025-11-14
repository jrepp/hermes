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
	// Message metadata
	ID        string           `json:"id"`        // Unique message ID (UUID)
	Type      NotificationType `json:"type"`      // Notification type
	Timestamp time.Time        `json:"timestamp"` // When published
	Priority  int              `json:"priority"`  // 0=normal, 1=high, 2=urgent

	// Context
	UserID       string `json:"user_id,omitempty"`       // Triggering user
	DocumentUUID string `json:"document_uuid,omitempty"` // Related document
	ProjectID    string `json:"project_id,omitempty"`    // Related project

	// Notification targets
	Recipients []Recipient `json:"recipients"` // Who receives this

	// Template-based rendering (for server-side resolution)
	Template        string         `json:"template,omitempty"`         // Template name (e.g., "document_approved") - deprecated, use resolved fields
	TemplateContext map[string]any `json:"template_context,omitempty"` // Template variables - kept for audit/debugging

	// Resolved content (populated by server before publishing)
	Subject  string `json:"subject"`   // Fully resolved subject line
	Body     string `json:"body"`      // Fully resolved body (markdown)
	BodyHTML string `json:"body_html"` // Fully resolved HTML body

	// Backend routing (which backends should process this)
	Backends []string `json:"backends"` // ["mail", "slack", "telegram", "discord", "audit"]

	// Retry tracking (set by consumers)
	RetryCount     int       `json:"retry_count,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	LastRetryAt    time.Time `json:"last_retry_at,omitempty"`
	NextRetryAt    time.Time `json:"next_retry_at,omitempty"`
	FailedBackends []string  `json:"failed_backends,omitempty"` // Track which backends failed
}

// Recipient defines a notification recipient
type Recipient struct {
	Email      string `json:"email,omitempty"`       // Email address
	Name       string `json:"name,omitempty"`        // Display name
	SlackID    string `json:"slack_id,omitempty"`    // Slack user ID
	TelegramID string `json:"telegram_id,omitempty"` // Telegram user ID
	DiscordID  string `json:"discord_id,omitempty"`  // Discord user ID
}
