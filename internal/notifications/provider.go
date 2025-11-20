package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
)

// NotificationRequest contains all data needed to create and send a notification
type NotificationRequest struct {
	Type            notifications.NotificationType
	Recipients      []notifications.Recipient
	TemplateContext map[string]any
	Backends        []string
	Priority        int
	DocumentUUID    string
	ProjectID       string
	UserID          string
}

// Provider handles notification creation and publishing
type Provider struct {
	resolver  *TemplateResolver
	publisher *notifications.Publisher
}

// NewProvider creates a new notification provider
func NewProvider(publisherConfig notifications.PublisherConfig) (*Provider, error) {
	// Initialize template resolver
	resolver, err := NewTemplateResolver()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template resolver: %w", err)
	}

	// Initialize publisher
	publisher, err := notifications.NewPublisher(publisherConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize publisher: %w", err)
	}

	return &Provider{
		resolver:  resolver,
		publisher: publisher,
	}, nil
}

// SendNotification resolves templates and publishes notification to the queue
func (p *Provider) SendNotification(ctx context.Context, req NotificationRequest) error {
	// Resolve templates
	content, err := p.resolver.Resolve(req.Type, req.TemplateContext)
	if err != nil {
		return fmt.Errorf("failed to resolve templates: %w", err)
	}

	// Create notification message with resolved content
	msg := &notifications.NotificationMessage{
		ID:              uuid.New().String(),
		Type:            req.Type,
		Timestamp:       time.Now(),
		Priority:        req.Priority,
		Recipients:      req.Recipients,
		Subject:         content.Subject,
		Body:            content.Body,
		BodyHTML:        content.BodyHTML,
		TemplateContext: req.TemplateContext, // Keep for audit/debugging
		Backends:        req.Backends,
		DocumentUUID:    req.DocumentUUID,
		ProjectID:       req.ProjectID,
		UserID:          req.UserID,
	}

	// Publish to queue
	if err := p.publisher.PublishMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish notification: %w", err)
	}

	return nil
}

// SendEmail provides backward compatibility with existing email system
// This is a simple pass-through that creates a basic notification
func (p *Provider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	// Convert to recipients
	recipients := make([]notifications.Recipient, len(to))
	for i, email := range to {
		recipients[i] = notifications.Recipient{Email: email}
	}

	// Create a simple notification with pre-rendered content
	msg := &notifications.NotificationMessage{
		ID:         uuid.New().String(),
		Type:       notifications.NotificationTypeEmail,
		Timestamp:  time.Now(),
		Recipients: recipients,
		Subject:    subject,
		Body:       body,
		BodyHTML:   body, // Assume body might be HTML
		Backends:   []string{"mail"},
	}

	// Publish to queue
	if err := p.publisher.PublishMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish email: %w", err)
	}

	return nil
}

// Close closes the provider and releases resources
func (p *Provider) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}
