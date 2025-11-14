# RFC-087 Addendum: Notification Template Scheme

**Status**: Implementation
**Author**: System
**Date**: 2025-11-14

## Overview

This addendum defines the template resolution scheme for RFC-087 notifications. Templates are fully resolved on the Hermes server before being published to the notification queue, ensuring that notification workers remain stateless and backend-agnostic.

## Design Principles

1. **Server-Side Resolution**: All templates are resolved on the Hermes server
2. **Backend Agnostic**: Workers receive fully-formatted notifications
3. **Separation of Concerns**: Template logic stays in server, delivery logic in workers
4. **Testability**: E2E tests verify complete notification flow

## Template Scheme

### Message Structure

Each notification consists of:

```go
type NotificationMessage struct {
    // Identity
    ID              string           `json:"id"`              // Unique notification ID
    Type            NotificationType `json:"type"`            // e.g., "document_approved"
    Timestamp       time.Time        `json:"timestamp"`
    Priority        int              `json:"priority"`        // 0=normal, 1=high, 2=urgent

    // Recipients
    Recipients      []Recipient      `json:"recipients"`      // Target users

    // Resolved Content (NEW)
    Subject         string           `json:"subject"`         // Fully resolved subject line
    Body            string           `json:"body"`            // Fully resolved body (markdown)
    BodyHTML        string           `json:"body_html"`       // Fully resolved HTML body

    // Context (for audit/debugging)
    DocumentUUID    string           `json:"document_uuid,omitempty"`
    ProjectID       string           `json:"project_id,omitempty"`
    UserID          string           `json:"user_id,omitempty"`

    // Backend routing
    Backends        []string         `json:"backends"`        // ["mail", "slack", "audit"]

    // Metadata
    TemplateID      string           `json:"template_id"`     // Template used (for auditing)
    TemplateContext map[string]any   `json:"template_context"` // Context used (for auditing)

    // Retry tracking
    RetryCount      int              `json:"retry_count,omitempty"`
    LastError       string           `json:"last_error,omitempty"`
    LastRetryAt     time.Time        `json:"last_retry_at,omitempty"`
    NextRetryAt     time.Time        `json:"next_retry_at,omitempty"`
    FailedBackends  []string         `json:"failed_backends,omitempty"`
}
```

### Template Resolution Flow

```
┌─────────────────┐
│  Hermes Server  │
│                 │
│  1. Event       │──┐
│     (doc        │  │
│     approved)   │  │
└─────────────────┘  │
                     │
                     ▼
┌──────────────────────────────────────┐
│  NotificationProvider                │
│                                      │
│  2. Load Template                    │
│     - subject.tmpl                   │
│     - body.md.tmpl                   │
│     - body.html.tmpl                 │
│                                      │
│  3. Resolve with Context             │
│     {                                │
│       DocumentShortName: "RFC-087"   │
│       ApproverName: "Alice"          │
│       DocumentURL: "https://..."     │
│       ...                            │
│     }                                │
│                                      │
│  4. Generate NotificationMessage     │
│     - subject: "RFC-087 approved..." │
│     - body: "Alice approved..."      │
│     - body_html: "<html>..."         │
└──────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────┐
│  Publisher                          │
│                                     │
│  5. Publish to Redpanda             │
│     topic: hermes.notifications     │
│     partition: by doc UUID          │
└─────────────────────────────────────┘
                     │
                     ▼
              ┌──────────┐
              │ Redpanda │
              └──────────┘
                     │
                     ▼
┌─────────────────────────────────────┐
│  Notifier (Worker)                  │
│                                     │
│  6. Consume & Route                 │
│     - Audit: Log notification       │
│     - Mail: Send email (pre-formed) │
│     - Slack: Post message           │
└─────────────────────────────────────┘
```

## Template Types

### 1. Subject Template
- **Format**: Plain text with Go template syntax
- **Output**: Single line subject string
- **Example**: `{{.DocumentShortName}} approved by {{.ApproverName}}`

### 2. Body Template (Markdown)
- **Format**: Markdown with Go template syntax
- **Output**: Markdown text (for Slack, Telegram, Discord)
- **Example**:
  ```markdown
  **{{.DocumentShortName}}** has been approved!

  Approver: {{.ApproverName}}

  [View Document]({{.DocumentURL}})
  ```

### 3. Body Template (HTML)
- **Format**: HTML with Go template syntax
- **Output**: Full HTML email
- **Example**:
  ```html
  <!DOCTYPE html>
  <html>
  <body>
    <h1>{{.DocumentShortName}} Approved</h1>
    <p>{{.ApproverName}} has approved this document.</p>
    <a href="{{.DocumentURL}}">View Document</a>
  </body>
  </html>
  ```

## Template Storage

Templates are stored in embedded file system:

```
internal/notifications/templates/
├── document_approved/
│   ├── subject.tmpl
│   ├── body.md.tmpl
│   └── body.html.tmpl
├── review_requested/
│   ├── subject.tmpl
│   ├── body.md.tmpl
│   └── body.html.tmpl
├── document_published/
│   ├── subject.tmpl
│   ├── body.md.tmpl
│   └── body.html.tmpl
└── new_owner/
    ├── subject.tmpl
    ├── body.md.tmpl
    └── body.html.tmpl
```

## NotificationProvider Interface

```go
// NotificationProvider handles notification creation and publishing
type NotificationProvider interface {
    // SendNotification resolves templates and publishes notification
    SendNotification(ctx context.Context, req NotificationRequest) error

    // SendEmail provides backward compatibility with existing email system
    SendEmail(to []string, from, subject, body string) error
}

// NotificationRequest contains all data needed to create a notification
type NotificationRequest struct {
    Type            NotificationType
    Recipients      []Recipient
    TemplateContext map[string]any
    Backends        []string
    Priority        int
    DocumentUUID    string
    ProjectID       string
    UserID          string
}
```

## Backward Compatibility

The existing `workspace.Provider.SendEmail()` interface is preserved:

```go
// Existing interface (unchanged)
type Provider interface {
    // ... other methods
    SendEmail(to []string, from, subject, body string) error
}

// New notification-aware provider
type NotificationProvider interface {
    Provider  // Embed existing interface

    // New notification method
    SendNotification(ctx context.Context, req NotificationRequest) error
}
```

## Testing Strategy

### Unit Tests
- Template resolution logic
- Context variable substitution
- Subject/body generation

### Integration Tests
- **E2E Test**: Server → Publisher → Redpanda → Notifier → Audit Logs
  - Create document approval event
  - Send notification via provider
  - Verify audit log output contains resolved content
  - Signal: Audit backend logs with complete formatted message

### E2E Test Flow

```go
func TestNotificationE2E(t *testing.T) {
    // 1. Setup: Start server, Redpanda, notifier with audit backend

    // 2. Trigger notification on server
    provider.SendNotification(ctx, NotificationRequest{
        Type: NotificationTypeDocumentApproved,
        Recipients: []Recipient{{Email: "test@example.com"}},
        TemplateContext: map[string]any{
            "DocumentShortName": "RFC-087",
            "ApproverName": "Alice",
            "DocumentURL": "https://hermes.example.com/doc/123",
        },
        Backends: []string{"audit"},
    })

    // 3. Wait for processing
    time.Sleep(2 * time.Second)

    // 4. Verify: Check audit logs contain resolved template
    logs := getNotifierLogs()
    assert.Contains(t, logs, "RFC-087 approved by Alice")
    assert.Contains(t, logs, "https://hermes.example.com/doc/123")
}
```

## Migration Path

1. **Phase 1** (Current): Implement NotificationProvider with template resolution
2. **Phase 2**: Create E2E integration test
3. **Phase 3**: Update document approval/review flows to use new provider
4. **Phase 4**: Deprecate old email-specific code
5. **Phase 5**: Add Slack/Telegram/Discord template support

## Benefits

1. **Worker Simplicity**: Workers only handle delivery, no template logic
2. **Consistency**: All backends receive identical content
3. **Testability**: Template rendering tested separately from delivery
4. **Performance**: Template resolution happens once on server, not per-backend
5. **Auditability**: Full message content logged before delivery
6. **Debuggability**: Audit logs show exactly what was sent

## Security Considerations

- Template context sanitization on server
- HTML escaping in templates
- No user-provided template execution in workers
- Audit trail of all notification content

## Examples

### Document Approval Notification

**Input (Server):**
```go
NotificationRequest{
    Type: NotificationTypeDocumentApproved,
    Recipients: []Recipient{
        {Email: "owner@example.com", Name: "Doc Owner"},
        {SlackID: "U12345", Name: "Doc Owner"},
    },
    TemplateContext: map[string]any{
        "DocumentShortName": "RFC-087",
        "DocumentTitle": "Notification System",
        "ApproverName": "Alice",
        "ApproverEmail": "alice@example.com",
        "DocumentURL": "https://hermes.example.com/document/123",
        "NonApproverCount": 2,
    },
    Backends: []string{"mail", "slack", "audit"},
}
```

**Output (Queue):**
```json
{
    "id": "notif-12345",
    "type": "document_approved",
    "timestamp": "2025-11-14T10:00:00Z",
    "subject": "RFC-087 approved by Alice",
    "body": "**RFC-087: Notification System** has been approved!\n\nApprover: Alice (alice@example.com)\nRemaining approvers: 2\n\n[View Document](https://hermes.example.com/document/123)",
    "body_html": "<!DOCTYPE html><html>...",
    "recipients": [
        {"email": "owner@example.com", "name": "Doc Owner"},
        {"slack_id": "U12345", "name": "Doc Owner"}
    ],
    "backends": ["mail", "slack", "audit"],
    "template_id": "document_approved",
    "template_context": {...}
}
```

**Audit Log Output:**
```
[AUDIT] Notification ID: notif-12345
[AUDIT]   Type: document_approved
[AUDIT]   Subject: RFC-087 approved by Alice
[AUDIT]   Recipients: Doc Owner <owner@example.com>, Doc Owner (Slack: U12345)
[AUDIT]   Body:
**RFC-087: Notification System** has been approved!

Approver: Alice (alice@example.com)
Remaining approvers: 2

[View Document](https://hermes.example.com/document/123)
[AUDIT]   ✓ Acknowledged at 2025-11-14T10:00:01Z
```

## Implementation Notes

- Use `text/template` for subject and markdown bodies
- Use `html/template` for HTML bodies (auto-escaping)
- Store templates in `embed.FS` for atomic deployments
- Support template overrides via configuration
- Cache parsed templates for performance
