# RFC-087 Implementation: Message Schema and Templates

**Parent**: [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md)

This document details the message schema, template system, and publisher implementation for the notification system.

## Message Schema

### Notification Message Structure

```go
// pkg/notifications/message.go
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
    ID        string           `json:"id"`         // Unique message ID (UUID)
    Type      NotificationType `json:"type"`       // Notification type
    Timestamp time.Time        `json:"timestamp"`  // When published
    Priority  int              `json:"priority"`   // 0=normal, 1=high, 2=urgent

    // Context
    UserID       string `json:"user_id,omitempty"`       // Triggering user
    DocumentUUID string `json:"document_uuid,omitempty"` // Related document
    ProjectID    string `json:"project_id,omitempty"`    // Related project

    // Notification targets
    Recipients []Recipient `json:"recipients"` // Who receives this

    // Template-based rendering
    Template        string         `json:"template"`          // Template name (e.g., "document_approved")
    TemplateContext map[string]any `json:"template_context"`  // Template variables

    // Backend routing (which backends should process this)
    Backends []string `json:"backends"` // ["mail", "slack", "telegram", "discord"]

    // Retry tracking (set by consumers)
    RetryCount int    `json:"retry_count,omitempty"`
    LastError  string `json:"last_error,omitempty"`
}

// Recipient defines a notification recipient
type Recipient struct {
    Email      string `json:"email,omitempty"`       // Email address
    Name       string `json:"name,omitempty"`        // Display name
    SlackID    string `json:"slack_id,omitempty"`    // Slack user ID
    TelegramID string `json:"telegram_id,omitempty"` // Telegram user ID
    DiscordID  string `json:"discord_id,omitempty"`  // Discord user ID
}
```

### Template Context

```go
// TemplateContext provides standard fields for all notification templates
type TemplateContext struct {
    // Document information
    DocumentTitle     string `json:"document_title,omitempty"`
    DocumentShortName string `json:"document_short_name,omitempty"`
    DocumentType      string `json:"document_type,omitempty"`
    DocumentStatus    string `json:"document_status,omitempty"`
    DocumentURL       string `json:"document_url,omitempty"`

    // User information
    UserName     string `json:"user_name,omitempty"`
    UserEmail    string `json:"user_email,omitempty"`
    OwnerName    string `json:"owner_name,omitempty"`
    ApproverName string `json:"approver_name,omitempty"`

    // Product/Project context
    Product   string `json:"product,omitempty"`
    ProjectID string `json:"project_id,omitempty"`

    // Application context
    BaseURL     string `json:"base_url,omitempty"`
    CurrentYear int    `json:"current_year,omitempty"`

    // Additional data (template-specific)
    Extra map[string]any `json:"extra,omitempty"`
}
```

## Template System

### Template Renderer Interface

```go
// pkg/notifications/template.go
package notifications

// TemplateRenderer renders notification templates for different backends
type TemplateRenderer interface {
    Render(templateName string, context map[string]any) (string, error)
    RenderSubject(templateName string, context map[string]any) (string, error)
}
```

### Template Examples

#### Email HTML Template

**File**: `internal/email/templates/document-approved.html`

```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Document Approved</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .button { display: inline-block; padding: 10px 20px; background-color: #4CAF50; color: white; text-decoration: none; border-radius: 5px; }
        .footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>✓ Document Approved</h1>
        </div>
        <div class="content">
            <h2>{{.DocumentShortName}} has been approved</h2>
            <p><strong>{{.ApproverName}}</strong> approved your {{.DocumentType}}.</p>
            <p><strong>Document:</strong> {{.DocumentTitle}}</p>
            <p><strong>Status:</strong> {{.DocumentStatus}}</p>
            <p style="text-align: center; margin: 30px 0;">
                <a href="{{.DocumentURL}}" class="button">View Document</a>
            </p>
        </div>
        <div class="footer">
            <p>{{.Product}} &copy; {{.CurrentYear}}</p>
        </div>
    </div>
</body>
</html>
```

#### Slack Markdown Template

**File**: `pkg/notifications/templates/slack/document-approved.md`

```markdown
:white_check_mark: *{{.DocumentShortName}} has been approved*

*{{.ApproverName}}* approved your {{.DocumentType}}.

*Document:* {{.DocumentTitle}}
*Status:* {{.DocumentStatus}}

<{{.DocumentURL}}|View Document>

_{{.Product}}_
```

#### Telegram Markdown Template

**File**: `pkg/notifications/templates/telegram/document-approved.md`

```markdown
✅ *{{.DocumentShortName}} has been approved*

*{{.ApproverName}}* approved your {{.DocumentType}}.

*Document:* {{.DocumentTitle}}
*Status:* {{.DocumentStatus}}

[View Document]({{.DocumentURL}})

_{{.Product}}_
```

#### Discord Markdown Template

**File**: `pkg/notifications/templates/discord/document-approved.md`

```markdown
:white_check_mark: **{{.DocumentShortName}} has been approved**

**{{.ApproverName}}** approved your {{.DocumentType}}.

**Document:** {{.DocumentTitle}}
**Status:** {{.DocumentStatus}}

[View Document]({{.DocumentURL}})

_{{.Product}}_
```

## Publisher Implementation

### Publisher

```go
// pkg/notifications/publisher.go
package notifications

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/twmb/franz-go/pkg/kgo"
)

// Publisher publishes notifications to Redpanda
type Publisher struct {
    client *kgo.Client
    topic  string
}

// NewPublisher creates a new notification publisher
func NewPublisher(brokers []string, topic string) (*Publisher, error) {
    client, err := kgo.NewClient(
        kgo.SeedBrokers(brokers...),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create kafka client: %w", err)
    }

    return &Publisher{
        client: client,
        topic:  topic,
    }, nil
}

// PublishNotification publishes a template-based notification
func (p *Publisher) PublishNotification(
    ctx context.Context,
    notifType NotificationType,
    template string,
    templateContext map[string]any,
    recipients []Recipient,
    backends []string,
) error {
    // Build notification message
    msg := NotificationMessage{
        ID:              uuid.New().String(),
        Type:            notifType,
        Timestamp:       time.Now(),
        Priority:        0,
        Recipients:      recipients,
        Template:        template,
        TemplateContext: templateContext,
        Backends:        backends,
    }

    // Marshal to JSON
    msgJSON, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("failed to marshal notification message: %w", err)
    }

    // Publish to Redpanda
    record := &kgo.Record{
        Topic: p.topic,
        Key:   []byte(msg.ID),
        Value: msgJSON,
    }

    if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
        return fmt.Errorf("failed to publish notification: %w", err)
    }

    return nil
}

// PublishEmail helper for backward compatibility
func (p *Publisher) PublishEmail(ctx context.Context, to []string, from, subject, body string) error {
    // Build recipients
    recipients := make([]Recipient, len(to))
    for i, email := range to {
        recipients[i] = Recipient{Email: email, Name: ""}
    }

    // Use generic email template
    context := map[string]any{
        "subject": subject,
        "body":    body,
        "from":    from,
    }

    return p.PublishNotification(ctx, NotificationTypeEmail, "generic_email", context, recipients, []string{"mail"})
}

// Close closes the publisher
func (p *Publisher) Close() {
    p.client.Close()
}
```

### NotificationProvider Adapter

```go
// pkg/workspace/adapters/messaging/notification_provider.go
package messaging

import (
    "context"

    "github.com/hashicorp-forge/hermes/pkg/notifications"
)

// notificationProvider implements workspace.NotificationProvider
// by publishing to Redpanda
type notificationProvider struct {
    publisher *notifications.Publisher
}

// NewNotificationProvider creates a new messaging notification provider
func NewNotificationProvider(brokers []string, topic string) (*notificationProvider, error) {
    pub, err := notifications.NewPublisher(brokers, topic)
    if err != nil {
        return nil, err
    }

    return &notificationProvider{
        publisher: pub,
    }, nil
}

// SendEmail publishes email notification to Redpanda
func (np *notificationProvider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
    return np.publisher.PublishEmail(ctx, to, from, subject, body)
}

// SendEmailWithTemplate publishes templated email notification
func (np *notificationProvider) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
    recipients := make([]notifications.Recipient, len(to))
    for i, email := range to {
        recipients[i] = notifications.Recipient{Email: email}
    }

    // Route to all configured backends (default: mail + audit)
    backends := []string{"mail", "audit"}

    return np.publisher.PublishNotification(
        ctx,
        notifications.NotificationType(template),
        template,
        data,
        recipients,
        backends,
    )
}
```

## Example Usage

### Publishing a Notification

```go
package main

import (
    "context"
    "time"

    "github.com/hashicorp-forge/hermes/pkg/notifications"
)

func sendDocumentApprovedNotification() error {
    ctx := context.Background()

    // Create publisher
    publisher, err := notifications.NewPublisher(
        []string{"localhost:9092"},
        "hermes.notifications",
    )
    if err != nil {
        return err
    }
    defer publisher.Close()

    // Define recipients
    recipients := []notifications.Recipient{
        {
            Email:      "author@example.com",
            Name:       "Jane Author",
            SlackID:    "U123456",    // Optional
            TelegramID: "98765432",   // Optional
        },
    }

    // Define template context
    context := map[string]any{
        "DocumentShortName": "RFC-087",
        "DocumentTitle":     "Multi-Backend Notification System",
        "DocumentType":      "RFC",
        "DocumentStatus":    "Approved",
        "DocumentURL":       "https://hermes.example.com/docs/RFC-087",
        "ApproverName":      "Alice Smith",
        "Product":           "Hermes",
        "BaseURL":           "https://hermes.example.com",
        "CurrentYear":       time.Now().Year(),
    }

    // Publish to multiple backends
    return publisher.PublishNotification(
        ctx,
        notifications.NotificationTypeDocumentApproved,
        "document_approved",
        context,
        recipients,
        []string{"mail", "slack", "audit"}, // Route to mail, Slack, and audit
    )
}
```

### Message Format on the Wire

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "document_approved",
  "timestamp": "2025-11-13T10:30:00Z",
  "priority": 0,
  "document_uuid": "RFC-087",
  "recipients": [
    {
      "email": "author@example.com",
      "name": "Jane Author",
      "slack_id": "U123456",
      "telegram_id": "98765432"
    }
  ],
  "template": "document_approved",
  "template_context": {
    "DocumentShortName": "RFC-087",
    "DocumentTitle": "Multi-Backend Notification System",
    "DocumentType": "RFC",
    "DocumentStatus": "Approved",
    "DocumentURL": "https://hermes.example.com/docs/RFC-087",
    "ApproverName": "Alice Smith",
    "Product": "Hermes",
    "BaseURL": "https://hermes.example.com",
    "CurrentYear": 2025
  },
  "backends": ["mail", "slack", "audit"]
}
```

## Template Library

### Available Templates

| Template Name           | Use Case                    | Context Fields Required                                      |
|-------------------------|-----------------------------|--------------------------------------------------------------|
| `document_approved`     | Document approval           | DocumentShortName, ApproverName, DocumentType, DocumentURL   |
| `review_requested`      | Review request              | DocumentShortName, DocumentTitle, OwnerName, DocumentURL     |
| `new_owner`             | Ownership transfer          | DocumentShortName, NewOwner, OldOwner, DocumentURL           |
| `document_published`    | Document publication        | DocumentShortName, DocumentTitle, DocumentType, DocumentURL  |
| `generic_email`         | Backward compatibility      | subject, body, from                                          |

### Template Storage

Templates are stored as Hermes documents in the `/notification-templates` project:

**Structure**:
```
/notification-templates/
├── document_approved/
│   ├── mail.html       (HTML template for email)
│   ├── slack.md        (Markdown for Slack)
│   ├── telegram.md     (Markdown for Telegram)
│   └── discord.md      (Markdown for Discord)
├── review_requested/
│   └── ... (same structure)
└── ... (other templates)
```

**Benefits**:
- ✅ **Hot reload**: Edit templates without redeploying workers
- ✅ **Versioning**: Templates versioned through document revision system
- ✅ **Collaboration**: Non-developers can edit templates through Hermes UI
- ✅ **Rollback**: Revert to previous template versions instantly
- ✅ **Audit trail**: Track who changed what and when

### Adding New Templates

#### Option 1: Via Hermes UI

1. Navigate to `/notification-templates` project
2. Create folder: `{template-name}/`
3. Create documents:
   - `mail.html` - Email HTML template
   - `slack.md` - Slack markdown template
   - `telegram.md` - Telegram markdown template
   - `discord.md` - Discord markdown template
4. Set document type to `notification-template`
5. Add metadata: `template_name`, `backend`, `format`

#### Option 2: Via CLI

```bash
# Create template set
hermes-admin templates create \
  --name document_approved \
  --mail templates/document_approved_mail.html \
  --slack templates/document_approved_slack.md \
  --telegram templates/document_approved_telegram.md \
  --discord templates/document_approved_discord.md

# Update existing template
hermes-admin templates update \
  --name document_approved \
  --backend mail \
  --file templates/document_approved_mail.html

# Validate template syntax
hermes-admin templates validate \
  --name document_approved \
  --backend mail

# Preview rendered template
hermes-admin templates preview \
  --name document_approved \
  --backend mail \
  --context '{"DocumentShortName": "RFC-087", "ApproverName": "Alice"}'
```

#### Option 3: Via API

```go
// Create template document programmatically
func createTemplate(ctx context.Context, ws workspace.Workspace) error {
    templateContent := `<!DOCTYPE html>
<html>
<body>
    <h2>{{.DocumentShortName}} Approved</h2>
    <p>{{.ApproverName}} has approved <strong>{{.DocumentTitle}}</strong>.</p>
</body>
</html>`

    doc, err := ws.CreateDocument(ctx, &workspace.DocumentMetadata{
        Title: "document_approved - mail",
        Path:  "/notification-templates/document_approved/mail.html",
        Type:  "notification-template",
        Metadata: map[string]interface{}{
            "template_name": "document_approved",
            "backend":       "mail",
            "format":        "html",
            "variables":     []string{"DocumentShortName", "ApproverName", "DocumentTitle"},
        },
    })
    if err != nil {
        return err
    }

    return ws.UpdateContent(ctx, doc.ID, []byte(templateContent))
}
```

### Template Loading and Caching

Templates are loaded dynamically from document storage:

1. **Cache Duration**: 5 minutes (configurable via `TEMPLATE_CACHE_TTL`)
2. **Cache Key**: `{template_name}/{backend}.{format}`
3. **Version Polling**: Worker checks document versions every minute
4. **Invalidation**: Cache cleared when document version increments
5. **Fallback**: If template not found, error returned (fails fast)

**Example cache flow**:
```
1. Worker receives notification message for "document_approved"
2. Mail backend requests template via TemplateLoader
3. TemplateLoader checks cache → miss (first load)
4. TemplateLoader fetches from workspace: /notification-templates/document_approved/mail.html
5. Template parsed and cached with document version
6. Template returned to backend
7. Subsequent requests hit cache (for next 5 minutes)
8. After 5 minutes or version change, template reloaded
```

### Template Development Workflow

**Development**:
```bash
# 1. Create local template files
mkdir -p templates/document_approved
vim templates/document_approved/mail.html

# 2. Validate locally
hermes-admin templates validate \
  --file templates/document_approved/mail.html

# 3. Preview with test data
hermes-admin templates preview \
  --file templates/document_approved/mail.html \
  --context test-data.json

# 4. Upload to Hermes
hermes-admin templates create \
  --name document_approved \
  --mail templates/document_approved/mail.html
```

**Production Updates**:
```bash
# 1. Edit template in Hermes UI (or via API)
# 2. Save document (version increments automatically)
# 3. Worker detects version change within 1 minute
# 4. New notifications use updated template (no restart needed)
```

### Template Security

See [RFC-087-ADDENDUM.md](./RFC-087-ADDENDUM.md) section 8 for template injection prevention:

- Input sanitization via `SanitizeTemplateContext()`
- Template syntax stripping from user inputs
- HTML escaping for email templates
- Allowlist for template fields

---

**Related Documents**:
- [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md) - Main RFC
- [RFC-087-BACKENDS.md](./RFC-087-BACKENDS.md) - Backend implementations
- [RFC-087-DOCKER-COMPOSE.md](./RFC-087-DOCKER-COMPOSE.md) - Testing infrastructure
