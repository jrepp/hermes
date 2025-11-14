# RFC-087 Implementation: Notification Backends

**Parent**: [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md)

This document details the implementation of all notification backends: audit, mail, Slack, Telegram, and Discord.

## Backend Interface

```go
// pkg/notifications/backends/backend.go
package backends

import (
    "context"
    "github.com/hashicorp-forge/hermes/pkg/notifications"
)

// Backend defines the interface for notification backends
type Backend interface {
    // Name returns the backend identifier
    Name() string

    // Handle processes a notification message
    Handle(ctx context.Context, msg *notifications.NotificationMessage) error

    // SupportsBackend checks if this backend should process the message
    SupportsBackend(backend string) bool
}
```

## Template Storage in Document System

**Overview**: Instead of embedding templates in the binary, templates are stored as Hermes documents. This enables:
- **Hot reloading**: Templates can be edited without redeploying
- **Version control**: Templates are versioned through document revisions
- **Collaboration**: Non-developers can edit templates through Hermes UI
- **Rollback**: Failed template changes can be reverted to previous versions

### Template Document Structure

Templates are stored in a special project called `notification-templates` with a standardized structure:

```
notification-templates/
├── document_approved/
│   ├── mail.html
│   ├── slack.md
│   ├── telegram.md
│   └── discord.md
├── review_requested/
│   ├── mail.html
│   ├── slack.md
│   ├── telegram.md
│   └── discord.md
└── document_published/
    ├── mail.html
    ├── slack.md
    ├── telegram.md
    └── discord.md
```

Each template document has:
- **Path**: `/notification-templates/{template_name}/{backend}.{ext}`
- **Type**: `notification-template`
- **Metadata**:
  ```json
  {
    "template_name": "document_approved",
    "backend": "mail",
    "format": "html",
    "variables": ["DocumentShortName", "ApproverName", "DocumentURL"],
    "created_by": "admin@example.com",
    "last_updated": "2025-11-13T10:00:00Z"
  }
  ```

### Template Loader Interface

```go
// pkg/notifications/backends/template_loader.go
package backends

import (
    "context"
    "fmt"
    "html/template"
    "sync"
    "time"

    "github.com/hashicorp-forge/hermes/pkg/workspace"
)

// TemplateLoader loads and caches templates from document storage
type TemplateLoader struct {
    workspace      workspace.Workspace
    cache          map[string]*CachedTemplate
    cacheMutex     sync.RWMutex
    cacheTimeout   time.Duration
    templatePrefix string // e.g., "/notification-templates"
}

type CachedTemplate struct {
    Template  *template.Template
    LoadedAt  time.Time
    DocumentID string
    Version    int64
}

func NewTemplateLoader(ws workspace.Workspace, templatePrefix string) *TemplateLoader {
    return &TemplateLoader{
        workspace:      ws,
        cache:          make(map[string]*CachedTemplate),
        cacheTimeout:   5 * time.Minute, // Cache templates for 5 minutes
        templatePrefix: templatePrefix,
    }
}

// LoadTemplate loads a template for a specific backend
func (l *TemplateLoader) LoadTemplate(ctx context.Context, templateName, backend, format string) (*template.Template, error) {
    cacheKey := fmt.Sprintf("%s/%s.%s", templateName, backend, format)

    // Check cache first
    l.cacheMutex.RLock()
    cached, ok := l.cache[cacheKey]
    l.cacheMutex.RUnlock()

    if ok && time.Since(cached.LoadedAt) < l.cacheTimeout {
        return cached.Template, nil
    }

    // Load from document storage
    templatePath := fmt.Sprintf("%s/%s/%s.%s", l.templatePrefix, templateName, backend, format)

    doc, err := l.workspace.GetDocument(ctx, templatePath)
    if err != nil {
        return nil, fmt.Errorf("template not found: %s: %w", templatePath, err)
    }

    // Get content
    content, err := l.workspace.GetContent(ctx, doc.ID)
    if err != nil {
        return nil, fmt.Errorf("failed to load template content: %w", err)
    }

    // Parse template based on format
    var tmpl *template.Template
    if format == "html" {
        tmpl, err = template.New(cacheKey).Parse(string(content))
    } else {
        // For markdown formats (Slack, Telegram, Discord)
        tmpl, err = template.New(cacheKey).Parse(string(content))
    }

    if err != nil {
        return nil, fmt.Errorf("failed to parse template: %w", err)
    }

    // Update cache
    l.cacheMutex.Lock()
    l.cache[cacheKey] = &CachedTemplate{
        Template:   tmpl,
        LoadedAt:   time.Now(),
        DocumentID: doc.ID,
        Version:    doc.Version,
    }
    l.cacheMutex.Unlock()

    return tmpl, nil
}

// InvalidateCache clears the template cache (useful for hot reload)
func (l *TemplateLoader) InvalidateCache(templateName string) {
    l.cacheMutex.Lock()
    defer l.cacheMutex.Unlock()

    for key := range l.cache {
        if strings.HasPrefix(key, templateName+"/") {
            delete(l.cache, key)
        }
    }
}

// WatchTemplates watches for template changes and invalidates cache
func (l *TemplateLoader) WatchTemplates(ctx context.Context) {
    // Poll for changes every minute
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            l.checkTemplateUpdates(ctx)
        }
    }
}

func (l *TemplateLoader) checkTemplateUpdates(ctx context.Context) {
    l.cacheMutex.Lock()
    defer l.cacheMutex.Unlock()

    for key, cached := range l.cache {
        doc, err := l.workspace.GetDocument(ctx, cached.DocumentID)
        if err != nil {
            continue
        }

        // If version changed, invalidate cache entry
        if doc.Version > cached.Version {
            delete(l.cache, key)
        }
    }
}
```

### Template Document Creation

```go
// pkg/notifications/backends/template_setup.go
package backends

import (
    "context"
    "fmt"

    "github.com/hashicorp-forge/hermes/pkg/workspace"
)

// SetupTemplateDocuments creates the template documents in Hermes
func SetupTemplateDocuments(ctx context.Context, ws workspace.Workspace) error {
    templates := []struct {
        Name    string
        Backend string
        Format  string
        Content string
    }{
        {
            Name:    "document_approved",
            Backend: "mail",
            Format:  "html",
            Content: `<!DOCTYPE html>
<html>
<body>
    <h2>{{.DocumentShortName}} Approved</h2>
    <p>{{.ApproverName}} has approved <strong>{{.DocumentTitle}}</strong>.</p>
    <p><a href="{{.DocumentURL}}">View Document</a></p>
</body>
</html>`,
        },
        {
            Name:    "document_approved",
            Backend: "slack",
            Format:  "md",
            Content: `*{{.DocumentShortName}} Approved*

{{.ApproverName}} has approved *{{.DocumentTitle}}*.

<{{.DocumentURL}}|View Document>`,
        },
        // ... more templates
    }

    for _, tmpl := range templates {
        path := fmt.Sprintf("/notification-templates/%s/%s.%s", tmpl.Name, tmpl.Backend, tmpl.Format)

        // Create document
        doc, err := ws.CreateDocument(ctx, &workspace.DocumentMetadata{
            Title:   fmt.Sprintf("%s - %s", tmpl.Name, tmpl.Backend),
            Path:    path,
            Type:    "notification-template",
            Metadata: map[string]interface{}{
                "template_name": tmpl.Name,
                "backend":       tmpl.Backend,
                "format":        tmpl.Format,
            },
        })
        if err != nil {
            return fmt.Errorf("failed to create template %s: %w", path, err)
        }

        // Write content
        if err := ws.UpdateContent(ctx, doc.ID, []byte(tmpl.Content)); err != nil {
            return fmt.Errorf("failed to write template content %s: %w", path, err)
        }
    }

    return nil
}
```

## Audit Backend

**Purpose**: Logs all notifications to stdout for compliance, debugging, and auditing.

**Configuration**: None required (default backend)

### Implementation

```go
// pkg/notifications/backends/audit.go
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

func NewAuditBackend() *AuditBackend {
    return &AuditBackend{
        logger: log.New(os.Stdout, "[AUDIT] ", log.LstdFlags|log.Lmsgprefix),
    }
}

func (b *AuditBackend) Name() string {
    return "audit"
}

func (b *AuditBackend) SupportsBackend(backend string) bool {
    return backend == "audit"
}

func (b *AuditBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
    // Log notification metadata
    b.logger.Printf("Notification ID: %s", msg.ID)
    b.logger.Printf("  Type: %s", msg.Type)
    b.logger.Printf("  Template: %s", msg.Template)
    b.logger.Printf("  Priority: %d", msg.Priority)
    b.logger.Printf("  Timestamp: %s", msg.Timestamp.Format(time.RFC3339))

    // Log recipients
    b.logger.Printf("  Recipients: %s", formatRecipients(msg.Recipients))

    // Log template context (pretty JSON)
    if len(msg.TemplateContext) > 0 {
        contextJSON, _ := json.MarshalIndent(msg.TemplateContext, "    ", "  ")
        b.logger.Printf("  Context:\n    %s", string(contextJSON))
    }

    // Log document context if present
    if msg.DocumentUUID != "" {
        b.logger.Printf("  Document UUID: %s", msg.DocumentUUID)
    }

    // Acknowledge
    b.logger.Printf("  ✓ Acknowledged at %s", time.Now().Format(time.RFC3339))

    return nil
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
```

### Output Example

```
[AUDIT] 2025-11-13T10:30:00Z Notification ID: 550e8400-e29b-41d4-a716-446655440000
[AUDIT] 2025-11-13T10:30:00Z   Type: document_approved
[AUDIT] 2025-11-13T10:30:00Z   Template: document_approved
[AUDIT] 2025-11-13T10:30:00Z   Priority: 0
[AUDIT] 2025-11-13T10:30:00Z   Timestamp: 2025-11-13T10:30:00Z
[AUDIT] 2025-11-13T10:30:00Z   Recipients: Jane Author <author@example.com>
[AUDIT] 2025-11-13T10:30:00Z   Context:
    {
      "ApproverName": "Alice Smith",
      "DocumentShortName": "RFC-087",
      "DocumentTitle": "Multi-Backend Notification System",
      "DocumentType": "RFC",
      "DocumentURL": "https://hermes.example.com/docs/RFC-087",
      "Product": "Hermes"
    }
[AUDIT] 2025-11-13T10:30:00Z   ✓ Acknowledged at 2025-11-13T10:30:01Z
```

## Mail Backend

**Purpose**: Sends email notifications via SMTP

**Configuration**:
- `SMTP_HOST`: SMTP server hostname
- `SMTP_PORT`: SMTP server port (default: 587)
- `SMTP_USERNAME`: SMTP authentication username
- `SMTP_PASSWORD`: SMTP authentication password
- `SMTP_FROM`: From email address

### Implementation

```go
// pkg/notifications/backends/mail.go
package backends

import (
    "bytes"
    "context"
    "fmt"
    "html/template"
    "net/smtp"

    "github.com/hashicorp-forge/hermes/pkg/notifications"
)

type MailBackendConfig struct {
    SMTPHost     string
    SMTPPort     int
    SMTPUsername string
    SMTPPassword string
    FromAddress  string
}

// MailBackend sends email notifications via SMTP
type MailBackend struct {
    config         MailBackendConfig
    templateLoader *TemplateLoader
}

func NewMailBackend(config MailBackendConfig, templateLoader *TemplateLoader) (*MailBackend, error) {
    return &MailBackend{
        config:         config,
        templateLoader: templateLoader,
    }, nil
}

func (b *MailBackend) Name() string {
    return "mail"
}

func (b *MailBackend) SupportsBackend(backend string) bool {
    return backend == "mail" || backend == "email"
}

func (b *MailBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
    // Load template from document storage
    tmpl, err := b.templateLoader.LoadTemplate(ctx, msg.Template, "mail", "html")
    if err != nil {
        return fmt.Errorf("failed to load template: %w", err)
    }

    var body bytes.Buffer
    if err := tmpl.Execute(&body, msg.TemplateContext); err != nil {
        return fmt.Errorf("failed to render template: %w", err)
    }

    // Render subject line
    subject := renderSubject(msg.Template, msg.TemplateContext)

    // Send to each recipient with email address
    for _, recipient := range msg.Recipients {
        if recipient.Email == "" {
            continue
        }

        if err := b.sendEmail(ctx, recipient.Email, subject, body.String()); err != nil {
            return fmt.Errorf("failed to send email to %s: %w", recipient.Email, err)
        }
    }

    return nil
}

func (b *MailBackend) sendEmail(ctx context.Context, to, subject, htmlBody string) error {
    auth := smtp.PlainAuth("", b.config.SMTPUsername, b.config.SMTPPassword, b.config.SMTPHost)

    message := fmt.Sprintf(
        "From: %s\r\n"+
            "To: %s\r\n"+
            "Subject: %s\r\n"+
            "MIME-Version: 1.0\r\n"+
            "Content-Type: text/html; charset=UTF-8\r\n"+
            "\r\n"+
            "%s",
        b.config.FromAddress,
        to,
        subject,
        htmlBody,
    )

    addr := fmt.Sprintf("%s:%d", b.config.SMTPHost, b.config.SMTPPort)
    return smtp.SendMail(addr, auth, b.config.FromAddress, []string{to}, []byte(message))
}

func renderSubject(templateName string, context map[string]any) string {
    // Generate subject line based on template
    switch templateName {
    case "document_approved":
        return fmt.Sprintf("%s approved by %s", context["DocumentShortName"], context["ApproverName"])
    case "review_requested":
        return fmt.Sprintf("Review requested for %s", context["DocumentShortName"])
    case "new_owner":
        return fmt.Sprintf("%s transferred to you", context["DocumentShortName"])
    case "document_published":
        return fmt.Sprintf("New %s: %s", context["DocumentType"], context["DocumentTitle"])
    default:
        return "Hermes Notification"
    }
}
```

## Slack Backend

**Purpose**: Sends notifications to Slack via Bot API

**Configuration**:
- `SLACK_BOT_TOKEN`: Slack Bot OAuth token (starts with `xoxb-`)

### Implementation

```go
// pkg/notifications/backends/slack.go
package backends

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/hashicorp-forge/hermes/pkg/notifications"
)

type SlackBackendConfig struct {
    BotToken string // Slack Bot OAuth token
}

// SlackBackend sends notifications to Slack
type SlackBackend struct {
    config         SlackBackendConfig
    templateLoader *TemplateLoader
    client         *http.Client
}

func NewSlackBackend(config SlackBackendConfig, templateLoader *TemplateLoader) (*SlackBackend, error) {
    return &SlackBackend{
        config:         config,
        templateLoader: templateLoader,
        client:         &http.Client{},
    }, nil
}

func (b *SlackBackend) Name() string {
    return "slack"
}

func (b *SlackBackend) SupportsBackend(backend string) bool {
    return backend == "slack"
}

func (b *SlackBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
    // Load template from document storage
    tmpl, err := b.templateLoader.LoadTemplate(ctx, msg.Template, "slack", "md")
    if err != nil {
        return fmt.Errorf("failed to load template: %w", err)
    }

    var body bytes.Buffer
    if err := tmpl.Execute(&body, msg.TemplateContext); err != nil {
        return fmt.Errorf("failed to render template: %w", err)
    }

    // Send to each recipient with Slack ID
    for _, recipient := range msg.Recipients {
        if recipient.SlackID == "" {
            continue
        }

        if err := b.sendSlackMessage(ctx, recipient.SlackID, body.String()); err != nil {
            return fmt.Errorf("failed to send slack message to %s: %w", recipient.SlackID, err)
        }
    }

    return nil
}

func (b *SlackBackend) sendSlackMessage(ctx context.Context, userID, message string) error {
    payload := map[string]interface{}{
        "channel": userID,
        "text":    message,
    }

    payloadBytes, _ := json.Marshal(payload)

    req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(payloadBytes))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+b.config.BotToken)

    resp, err := b.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("slack API returned status %d", resp.StatusCode)
    }

    return nil
}
```

## Telegram Backend

**Purpose**: Sends notifications to Telegram via Bot API

**Configuration**:
- `TELEGRAM_BOT_TOKEN`: Telegram Bot API token

### Implementation

```go
// pkg/notifications/backends/telegram.go
package backends

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/hashicorp-forge/hermes/pkg/notifications"
)

type TelegramBackendConfig struct {
    BotToken string // Telegram Bot API token
}

// TelegramBackend sends notifications to Telegram
type TelegramBackend struct {
    config         TelegramBackendConfig
    templateLoader *TemplateLoader
    client         *http.Client
}

func NewTelegramBackend(config TelegramBackendConfig, templateLoader *TemplateLoader) (*TelegramBackend, error) {
    return &TelegramBackend{
        config:         config,
        templateLoader: templateLoader,
        client:         &http.Client{},
    }, nil
}

func (b *TelegramBackend) Name() string {
    return "telegram"
}

func (b *TelegramBackend) SupportsBackend(backend string) bool {
    return backend == "telegram"
}

func (b *TelegramBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
    // Load template from document storage
    tmpl, err := b.templateLoader.LoadTemplate(ctx, msg.Template, "telegram", "md")
    if err != nil {
        return fmt.Errorf("failed to load template: %w", err)
    }

    var body bytes.Buffer
    if err := tmpl.Execute(&body, msg.TemplateContext); err != nil {
        return fmt.Errorf("failed to render template: %w", err)
    }

    // Send to each recipient with Telegram ID
    for _, recipient := range msg.Recipients {
        if recipient.TelegramID == "" {
            continue
        }

        if err := b.sendTelegramMessage(ctx, recipient.TelegramID, body.String()); err != nil {
            return fmt.Errorf("failed to send telegram message to %s: %w", recipient.TelegramID, err)
        }
    }

    return nil
}

func (b *TelegramBackend) sendTelegramMessage(ctx context.Context, chatID, message string) error {
    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.config.BotToken)

    payload := map[string]interface{}{
        "chat_id":    chatID,
        "text":       message,
        "parse_mode": "Markdown",
    }

    payloadBytes, _ := json.Marshal(payload)

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadBytes))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := b.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
    }

    return nil
}
```

## Discord Backend

**Purpose**: Sends notifications to Discord via Bot API

**Configuration**:
- `DISCORD_BOT_TOKEN`: Discord Bot token

### Implementation

```go
// pkg/notifications/backends/discord.go
package backends

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/hashicorp-forge/hermes/pkg/notifications"
)

type DiscordBackendConfig struct {
    BotToken string // Discord Bot token
}

// DiscordBackend sends notifications to Discord
type DiscordBackend struct {
    config         DiscordBackendConfig
    templateLoader *TemplateLoader
    client         *http.Client
}

func NewDiscordBackend(config DiscordBackendConfig, templateLoader *TemplateLoader) (*DiscordBackend, error) {
    return &DiscordBackend{
        config:         config,
        templateLoader: templateLoader,
        client:         &http.Client{},
    }, nil
}

func (b *DiscordBackend) Name() string {
    return "discord"
}

func (b *DiscordBackend) SupportsBackend(backend string) bool {
    return backend == "discord"
}

func (b *DiscordBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
    // Load template from document storage
    tmpl, err := b.templateLoader.LoadTemplate(ctx, msg.Template, "discord", "md")
    if err != nil {
        return fmt.Errorf("failed to load template: %w", err)
    }

    var body bytes.Buffer
    if err := tmpl.Execute(&body, msg.TemplateContext); err != nil {
        return fmt.Errorf("failed to render template: %w", err)
    }

    // Send to each recipient with Discord ID
    for _, recipient := range msg.Recipients {
        if recipient.DiscordID == "" {
            continue
        }

        if err := b.sendDiscordDM(ctx, recipient.DiscordID, body.String()); err != nil {
            return fmt.Errorf("failed to send discord message to %s: %w", recipient.DiscordID, err)
        }
    }

    return nil
}

func (b *DiscordBackend) sendDiscordDM(ctx context.Context, userID, message string) error {
    // First, create a DM channel
    createDMURL := "https://discord.com/api/v10/users/@me/channels"
    dmPayload := map[string]interface{}{
        "recipient_id": userID,
    }

    dmPayloadBytes, _ := json.Marshal(dmPayload)

    req, err := http.NewRequestWithContext(ctx, "POST", createDMURL, bytes.NewBuffer(dmPayloadBytes))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bot "+b.config.BotToken)

    resp, err := b.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var dmChannel struct {
        ID string `json:"id"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&dmChannel); err != nil {
        return err
    }

    // Send message to DM channel
    sendMessageURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", dmChannel.ID)
    msgPayload := map[string]interface{}{
        "content": message,
    }

    msgPayloadBytes, _ := json.Marshal(msgPayload)

    req, err = http.NewRequestWithContext(ctx, "POST", sendMessageURL, bytes.NewBuffer(msgPayloadBytes))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bot "+b.config.BotToken)

    resp, err = b.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("discord API returned status %d", resp.StatusCode)
    }

    return nil
}
```

## Backend Configuration Summary

| Backend  | Environment Variables          | Required | Default |
|----------|--------------------------------|----------|---------|
| audit    | None                           | No       | -       |
| mail     | SMTP_HOST                      | Yes      | -       |
|          | SMTP_PORT                      | No       | 587     |
|          | SMTP_USERNAME                  | Yes      | -       |
|          | SMTP_PASSWORD                  | Yes      | -       |
|          | SMTP_FROM                      | Yes      | -       |
| slack    | SLACK_BOT_TOKEN                | Yes      | -       |
| telegram | TELEGRAM_BOT_TOKEN             | Yes      | -       |
| discord  | DISCORD_BOT_TOKEN              | Yes      | -       |

## Worker Initialization with TemplateLoader

The notification worker must initialize the TemplateLoader and pass it to all backends:

```go
// cmd/notification-worker/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/hashicorp-forge/hermes/pkg/notifications/backends"
    "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/local"
    "github.com/twmb/franz-go/pkg/kgo"
)

func main() {
    // Initialize workspace (for loading templates)
    workspaceConfig := local.Config{
        RootDir: os.Getenv("WORKSPACE_ROOT"),
    }
    workspace, err := local.NewAdapter(workspaceConfig)
    if err != nil {
        log.Fatalf("Failed to initialize workspace: %v", err)
    }

    // Create template loader
    templateLoader := backends.NewTemplateLoader(workspace, "/notification-templates")

    // Start background watcher for template changes
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go templateLoader.WatchTemplates(ctx)

    // Initialize backends with template loader
    backendList := []backends.Backend{
        backends.NewAuditBackend(),
    }

    // Mail backend (if configured)
    if smtpHost := os.Getenv("SMTP_HOST"); smtpHost != "" {
        mailBackend, err := backends.NewMailBackend(backends.MailBackendConfig{
            SMTPHost:     smtpHost,
            SMTPPort:     getEnvInt("SMTP_PORT", 587),
            SMTPUsername: os.Getenv("SMTP_USERNAME"),
            SMTPPassword: os.Getenv("SMTP_PASSWORD"),
            FromAddress:  os.Getenv("SMTP_FROM"),
        }, templateLoader)
        if err != nil {
            log.Fatalf("Failed to create mail backend: %v", err)
        }
        backendList = append(backendList, mailBackend)
    }

    // Slack backend (if configured)
    if slackToken := os.Getenv("SLACK_BOT_TOKEN"); slackToken != "" {
        slackBackend, err := backends.NewSlackBackend(backends.SlackBackendConfig{
            BotToken: slackToken,
        }, templateLoader)
        if err != nil {
            log.Fatalf("Failed to create slack backend: %v", err)
        }
        backendList = append(backendList, slackBackend)
    }

    // Telegram backend (if configured)
    if telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN"); telegramToken != "" {
        telegramBackend, err := backends.NewTelegramBackend(backends.TelegramBackendConfig{
            BotToken: telegramToken,
        }, templateLoader)
        if err != nil {
            log.Fatalf("Failed to create telegram backend: %v", err)
        }
        backendList = append(backendList, telegramBackend)
    }

    // Discord backend (if configured)
    if discordToken := os.Getenv("DISCORD_BOT_TOKEN"); discordToken != "" {
        discordBackend, err := backends.NewDiscordBackend(backends.DiscordBackendConfig{
            BotToken: discordToken,
        }, templateLoader)
        if err != nil {
            log.Fatalf("Failed to create discord backend: %v", err)
        }
        backendList = append(backendList, discordBackend)
    }

    log.Printf("Initialized %d backends\n", len(backendList))

    // Initialize Kafka consumer
    client, err := kgo.NewClient(
        kgo.SeedBrokers(os.Getenv("KAFKA_BROKERS")),
        kgo.ConsumerGroup(os.Getenv("CONSUMER_GROUP")),
        kgo.ConsumeTopics("hermes.notifications"),
    )
    if err != nil {
        log.Fatalf("Failed to create Kafka client: %v", err)
    }
    defer client.Close()

    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // Start consuming messages
    log.Println("Notification worker started")
    for {
        select {
        case <-sigChan:
            log.Println("Shutting down...")
            return
        default:
            fetches := client.PollFetches(ctx)
            if errs := fetches.Errors(); len(errs) > 0 {
                for _, err := range errs {
                    log.Printf("Fetch error: %v\n", err)
                }
                continue
            }

            fetches.EachRecord(func(record *kgo.Record) {
                if err := processMessage(ctx, backendList, record); err != nil {
                    log.Printf("Failed to process message: %v\n", err)
                } else {
                    client.CommitRecords(ctx, record)
                }
            })
        }
    }
}
```

### Template Hot Reloading

Templates are automatically reloaded when changed:

1. **Cache TTL**: Templates cached for 5 minutes
2. **Version Polling**: Worker polls document versions every minute
3. **Invalidation**: Cache invalidated when document version changes
4. **No Restart**: Workers pick up new templates without restart

**Example template update flow**:
```bash
# 1. Admin edits template in Hermes UI
# 2. Template document version incremented
# 3. Worker detects version change (within 1 minute)
# 4. Cache invalidated for that template
# 5. Next notification uses new template
```

### Template Management via CLI

```bash
# Create template documents
hermes-admin templates init

# Update a template
hermes-admin templates update \
  --name document_approved \
  --backend mail \
  --file templates/document_approved_mail.html

# List all templates
hermes-admin templates list

# Validate template syntax
hermes-admin templates validate \
  --name document_approved \
  --backend mail
```

## Error Handling

All backends implement consistent error handling:

1. **Template Not Found**: Return error if template doesn't exist
2. **Template Rendering Error**: Return error if context data is invalid
3. **API Failures**: Return error with details for monitoring
4. **Partial Success**: If sending to multiple recipients, log failures but continue

Example error handling pattern:

```go
func (b *Backend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
    for _, recipient := range msg.Recipients {
        if err := b.send(ctx, recipient); err != nil {
            log.Printf("Failed to send to %s: %v", recipient.Email, err)
            // Continue with other recipients
        }
    }
    return nil
}
```

---

**Related Documents**:
- [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md) - Main RFC
- [RFC-087-MESSAGE-SCHEMA.md](./RFC-087-MESSAGE-SCHEMA.md) - Message format and templates
- [RFC-087-DOCKER-COMPOSE.md](./RFC-087-DOCKER-COMPOSE.md) - Testing infrastructure
