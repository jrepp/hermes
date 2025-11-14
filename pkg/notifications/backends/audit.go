package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
)

// AuditBackend logs all notifications for compliance and debugging
type AuditBackend struct {
	logger *log.Logger
}

// NewAuditBackend creates a new audit backend
func NewAuditBackend() *AuditBackend {
	return &AuditBackend{
		logger: log.New(os.Stdout, "[AUDIT] ", log.LstdFlags|log.Lmsgprefix),
	}
}

// Name returns the backend identifier
func (b *AuditBackend) Name() string {
	return "audit"
}

// SupportsBackend checks if this backend should process the message
func (b *AuditBackend) SupportsBackend(backend string) bool {
	return backend == "audit"
}

// Handle processes a notification message
func (b *AuditBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
	// Log notification metadata
	b.logger.Printf("Notification ID: %s", msg.ID)
	b.logger.Printf("  Type: %s", msg.Type)
	if msg.Template != "" {
		b.logger.Printf("  Template: %s", msg.Template)
	}
	b.logger.Printf("  Priority: %d", msg.Priority)
	b.logger.Printf("  Timestamp: %s", msg.Timestamp.Format(time.RFC3339))

	// Log recipients
	b.logger.Printf("  Recipients: %s", formatRecipients(msg.Recipients))

	// Log resolved content (most important for E2E testing)
	if msg.Subject != "" {
		b.logger.Printf("  Subject: %s", msg.Subject)
	}
	if msg.Body != "" {
		b.logger.Printf("  Body:\n%s", indent(msg.Body, "    "))
	}

	// Log template context (for debugging)
	if len(msg.TemplateContext) > 0 {
		contextJSON, _ := json.MarshalIndent(msg.TemplateContext, "    ", "  ")
		b.logger.Printf("  Template Context:\n    %s", string(contextJSON))
	}

	// Log document context if present
	if msg.DocumentUUID != "" {
		b.logger.Printf("  Document UUID: %s", msg.DocumentUUID)
	}
	if msg.ProjectID != "" {
		b.logger.Printf("  Project ID: %s", msg.ProjectID)
	}
	if msg.UserID != "" {
		b.logger.Printf("  User ID: %s", msg.UserID)
	}

	// Acknowledge
	b.logger.Printf("  ✓ Acknowledged at %s", time.Now().Format(time.RFC3339))

	return nil
}

// indent adds prefix to each line of text
func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func formatRecipients(recipients []notifications.Recipient) string {
	var parts []string
	for _, r := range recipients {
		if r.Name != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", r.Name, r.Email))
		} else if r.Email != "" {
			parts = append(parts, r.Email)
		}
	}
	return strings.Join(parts, ", ")
}
