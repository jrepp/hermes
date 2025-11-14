---
hermes-uuid: RFC-087-NOTIFICATION-BACKEND
document-type: RFC
document-number: RFC-087
status: draft
title: "Multi-Backend Notification System with Message Queues"
authors:
  - system
created: 2025-11-13T00:00:00Z
modified: 2025-11-13T00:00:00Z
tags:
  - rfc
  - notifications
  - architecture
  - messaging
  - redpanda
---

# RFC-087: Multi-Backend Notification System with Message Queues

## Summary

This RFC proposes migrating the synchronous email notification system to an asynchronous, template-based multi-backend notification system using Redpanda (Kafka-compatible message broker). Notifications will be defined by templates and context, published to a topic, and processed by small, independent Go consumer processes supporting multiple backends: **audit** (default), **mail**, **Slack**, **Telegram**, and **Discord**.

**Status**: Draft
**Author**: System
**Created**: 2025-11-13

## Background

### Current State

The current notification system (pkg/workspace/adapters/local/notification.go and internal/email/email.go) operates synchronously:

1. Email notifications are sent directly from API handlers via `workspace.NotificationProvider.SendEmail()`
2. The local adapter logs emails to console; production would use SMTP
3. Template rendering and sending are synchronous operations that block API responses
4. No retry mechanism, auditing, or multi-backend support
5. Notification failures can cause API request failures

**Current Architecture**:
```
API Handler → NotificationProvider.SendEmail() → SMTP / Log
              (synchronous, blocking)
```

### Problem Statement

The current synchronous notification system has several limitations:

1. **Performance**: Email sending blocks API responses, increasing latency
2. **Reliability**: No retry mechanism for failed notifications
3. **Scalability**: Cannot handle notification spikes or rate limiting
4. **Auditability**: No centralized tracking of notification events
5. **Flexibility**: Cannot easily add new notification backends (Slack, Telegram, Discord, webhooks)
6. **Testing**: Difficult to test notification behavior in integration tests

## Goals and Non-Goals

### Goals
- ✅ Decouple notification sending from API request processing
- ✅ Support multiple notification backends (audit, mail, Slack, Telegram, Discord)
- ✅ Implement template-based notification rendering with shared context
- ✅ Implement reliable message delivery with Redpanda
- ✅ Add notification auditing and observability
- ✅ Enable horizontal scaling of notification processing
- ✅ Integrate Redpanda into testing infrastructure
- ✅ Maintain backward compatibility with existing NotificationProvider interface

### Non-Goals
- ❌ Replace all SMTP configurations immediately (gradual migration)
- ❌ Build a complex notification routing system (keep it simple)
- ❌ Implement notification preferences or user subscriptions
- ❌ Add real-time notification delivery tracking UI

## Proposal

### Overview

Replace the synchronous notification system with an asynchronous, event-driven, template-based architecture:

```
┌─────────────────────────────────────────────────────────────┐
│ API Layer                                                    │
├─────────────────────────────────────────────────────────────┤
│ NotificationProvider.SendEmail()                            │
│   ↓                                                          │
│ Publisher → Redpanda Topic: "hermes.notifications"          │
│   {template: "document_approved", context: {...}}           │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ (async, decoupled)
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Redpanda Message Broker                                      │
│ Topic: hermes.notifications                                  │
│ - Partitions: 3 (for parallelism)                           │
│ - Retention: 7 days                                          │
│ - Consumer Group: hermes-notification-workers                │
└────────────┬────────────────────────────────────────────────┘
             │
             ├─────────────┬─────────────┬─────────────┬──────┐
             ▼             ▼             ▼             ▼      ▼
      ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────┐  ┌──────┐
      │Audit     │  │Mail      │  │Slack     │  │Tele-│  │Disc- │
      │Backend   │  │Backend   │  │Backend   │  │gram │  │cord  │
      └────┬─────┘  └────┬─────┘  └────┬─────┘  └──┬──┘  └──┬───┘
           │             │             │            │        │
           ▼             ▼             ▼            ▼        ▼
      ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────┐  ┌──────┐
      │Audit Log │  │SMTP      │  │Slack API │  │Tele│  │Disc. │
      │(JSON)    │  │Server    │  │          │  │API │  │API   │
      └──────────┘  └──────────┘  └──────────┘  └────┘  └──────┘
```

### Key Design Principles

1. **Template-Based**: Notifications defined by template name + context (not raw HTML/text)
2. **Multi-Backend**: Same notification rendered differently for each channel
3. **Backend Routing**: Messages specify which backends should process them
4. **Independent Workers**: Each backend runs in separate consumer processes
5. **Graceful Degradation**: Backend failures don't block other backends

### Architecture Components

**See detailed implementation in:**
- [RFC-087-MESSAGE-SCHEMA.md](./RFC-087-MESSAGE-SCHEMA.md) - Message format and templates
- [RFC-087-BACKENDS.md](./RFC-087-BACKENDS.md) - Backend implementations
- [RFC-087-DOCKER-COMPOSE.md](./RFC-087-DOCKER-COMPOSE.md) - Testing infrastructure

### Notification Flow

1. **API Layer**: Application calls `NotificationProvider.SendEmailWithTemplate()`
2. **Publisher**: Converts call to structured message with template + context
3. **Redpanda**: Routes message to all consumers in group
4. **Workers**: Multiple workers consume messages in parallel
5. **Backend Routing**: Each worker routes to enabled backends based on message.Backends field
6. **Template Rendering**: Each backend renders template in its format (HTML, Markdown, etc.)
7. **Delivery**: Backend sends notification via appropriate channel (SMTP, Slack API, etc.)
8. **Audit**: Audit backend logs all notifications for compliance

### Message Structure

```go
type NotificationMessage struct {
    ID              string                 // Unique message ID (UUID)
    Type            NotificationType       // Notification type
    Template        string                 // Template name (e.g., "document_approved")
    TemplateContext map[string]any         // Template variables
    Recipients      []Recipient            // Who receives this
    Backends        []string               // ["mail", "slack", "audit"]
    Timestamp       time.Time              // When published
    Priority        int                    // 0=normal, 1=high, 2=urgent
}

type Recipient struct {
    Email      string  // For mail backend
    Name       string  // Display name
    SlackID    string  // For Slack backend
    TelegramID string  // For Telegram backend
    DiscordID  string  // For Discord backend
}
```

### Supported Backends

| Backend  | Purpose                    | Output Format     | Configuration Required |
|----------|----------------------------|-------------------|------------------------|
| audit    | Logging/compliance         | JSON logs         | None (default)         |
| mail     | Email notifications        | HTML              | SMTP credentials       |
| slack    | Slack DMs/channels         | Slack Markdown    | Bot token              |
| telegram | Telegram messages          | Telegram Markdown | Bot token              |
| discord  | Discord DMs/channels       | Discord Markdown  | Bot token              |

### Template Example

**Template**: `document_approved`

**Context**:
```json
{
  "DocumentShortName": "RFC-087",
  "DocumentTitle": "Notification System",
  "DocumentType": "RFC",
  "DocumentURL": "https://hermes.example.com/docs/RFC-087",
  "ApproverName": "Alice Smith",
  "Product": "Hermes"
}
```

**Rendered Output**:

- **Email (HTML)**: Professional HTML email with styled headers, links, footer
- **Slack**: `:white_check_mark: *RFC-087 has been approved*\n\n*Alice Smith* approved your RFC...`
- **Telegram**: `✅ *RFC-087 has been approved*\n\n*Alice Smith* approved your RFC...`
- **Discord**: `:white_check_mark: **RFC-087 has been approved**\n\n**Alice Smith** approved your RFC...`
- **Audit**: Full JSON with all context for compliance

### API Changes

**No breaking changes**. The existing `NotificationProvider` interface remains unchanged:

```go
type NotificationProvider interface {
    SendEmail(ctx context.Context, to []string, from, subject, body string) error
    SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error
}
```

**New template-based method** (recommended):
```go
// Internal API
publisher.PublishNotification(
    ctx,
    NotificationTypeDocumentApproved,
    "document_approved",
    templateContext,
    recipients,
    []string{"mail", "slack", "audit"}, // Route to multiple backends
)
```

### Data Model

**Redpanda Topic Configuration**:
```
Topic: hermes.notifications
Partitions: 3 (allows parallel processing)
Replication Factor: 1 (testing), 3 (production)
Retention: 7 days
Cleanup Policy: delete
```

**Optional: Notification History Table**:
```sql
CREATE TABLE notification_events (
    id UUID PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    template VARCHAR(100) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    user_id VARCHAR(255),
    document_uuid UUID,
    recipients JSONB NOT NULL,
    template_context JSONB NOT NULL,
    backends TEXT[] NOT NULL,
    status VARCHAR(20) NOT NULL, -- pending, sent, failed
    retry_count INTEGER DEFAULT 0,
    last_error TEXT,
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_notification_events_type ON notification_events(type);
CREATE INDEX idx_notification_events_template ON notification_events(template);
CREATE INDEX idx_notification_events_timestamp ON notification_events(timestamp);
CREATE INDEX idx_notification_events_document ON notification_events(document_uuid);
```

### Implementation Plan

**Phase 1: Foundation** (Week 1)
- Add franz-go dependency for Kafka client
- Implement notification message schema (pkg/notifications/message.go)
- Implement publisher (pkg/notifications/publisher.go)
- Add Redpanda to docker-compose.yml
- Create topic initialization script

**Phase 2: Audit Backend** (Week 2)
- Implement audit backend consumer (pkg/notifications/backends/audit.go)
- Implement template system interfaces
- Add notification worker to Dockerfile (multi-stage build)
- Deploy audit worker in docker-compose
- Test message publishing and consumption

**Phase 3: Additional Backends** (Week 3)
- Implement mail backend with HTML templates
- Implement Slack backend with markdown templates
- Implement Telegram backend with markdown templates
- Implement Discord backend with markdown templates
- Add backend configuration and deployment

**Phase 4: Integration** (Week 4)
- Implement messaging NotificationProvider adapter
- Add configuration for switching between sync/async modes
- Update central/edge Hermes configs to use messaging provider
- Migrate existing email notification flows
- Add integration tests

**Phase 5: Production Readiness** (Week 5-6)
- Implement retry logic and dead letter queue
- Add notification history database table (optional)
- Implement monitoring and metrics (Prometheus)
- Document configuration and operations
- Create runbook for incident response

## Alternatives Considered

### Alternative 1: Direct SMTP Queue

Use a simple SMTP queue without message broker.

**Why not chosen**:
- Less flexible for adding non-email backends
- No built-in partitioning for scalability
- Would need to build retry/reliability ourselves

### Alternative 2: RabbitMQ

Use RabbitMQ instead of Redpanda/Kafka.

**Why not chosen**:
- Team has more Kafka experience
- Kafka better for high-throughput scenarios
- Redpanda is Kafka-compatible but easier to operate
- Better fit for potential event sourcing in future

### Alternative 3: Cloud Notification Service (AWS SQS + SNS)

Use cloud-native notification services.

**Why not chosen**:
- Increases cloud vendor lock-in
- Harder to test locally
- This solution works in both cloud and on-prem
- Redpanda is cloud-native and portable

### Alternative 4: Single Backend per Worker

Run separate worker processes for each backend.

**Why not chosen**:
- More operational complexity (5 different deployments)
- Harder to route same notification to multiple backends
- Current design allows flexible routing per message

## Template Management

### Template Storage in Document System

Templates are stored as Hermes documents rather than embedded in code, enabling:

**Benefits**:
- ✅ **Hot Reloading**: Edit templates without redeploying workers
- ✅ **Version Control**: Templates versioned through document revision system
- ✅ **Collaboration**: Non-developers can edit templates through Hermes UI
- ✅ **Rollback**: Revert to previous template versions instantly
- ✅ **Audit Trail**: Track who changed what and when

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

**Implementation Details**:
- Templates loaded from document storage via `TemplateLoader`
- 5-minute cache TTL with version-based invalidation
- Worker polls document versions every minute
- Cache invalidated when document version changes
- New templates take effect within 1 minute (no restart)

**See**: [RFC-087-BACKENDS.md](./RFC-087-BACKENDS.md#template-storage-in-document-system) for complete implementation details.

### Template Development Workflow

**Creating Templates**:
```bash
# Option 1: Via CLI
hermes-admin templates create \
  --name document_approved \
  --mail templates/document_approved_mail.html \
  --slack templates/document_approved_slack.md

# Option 2: Via Hermes UI
# Navigate to /notification-templates and create documents

# Option 3: Via API
# Use CreateDocument + UpdateContent
```

**Updating Templates**:
```bash
# 1. Edit template in Hermes UI
# 2. Save document (version increments)
# 3. Worker detects change within 1 minute
# 4. New notifications use updated template
```

**See**: [RFC-087-MESSAGE-SCHEMA.md](./RFC-087-MESSAGE-SCHEMA.md#template-storage) for detailed template management guide.

## Security Considerations

- **Message Encryption**: Redpanda supports TLS for broker communication (enable in production)
  - **Enhanced**: AES-256-GCM envelope encryption for PII in messages (see [RFC-087-ADDENDUM.md](./RFC-087-ADDENDUM.md#6-message-encryption-pii-protection))
- **Authentication**: Use SASL/SCRAM for Kafka authentication in production
- **Authorization**: Implement ACLs to restrict topic access
- **PII in Messages**: Email addresses and names are in messages - ensure retention policies comply with privacy requirements (7-day retention)
  - **Mitigation**: Encrypt sensitive payload data before publishing
- **Audit Trail**: All notifications logged by audit backend for compliance
- **Secrets Management**:
  - Store SMTP credentials in secrets manager
  - Store bot tokens in environment variables
  - Store encryption keys in Kubernetes secrets or KMS
  - Never include credentials in message payloads
- **Template Injection**: Validate template context to prevent injection attacks
  - **Implementation**: Input sanitization via `SanitizeTemplateContext()`
  - **Allowlist**: Only permit pre-defined template fields
  - **HTML Escaping**: Auto-escape user content in email templates
  - **See**: [RFC-087-ADDENDUM.md](./RFC-087-ADDENDUM.md#8-template-injection-prevention)
- **Template Access Control**:
  - Restrict write access to `/notification-templates` project to admins
  - Template changes tracked in document revision history
  - Implement approval workflow for production template updates

## Testing Strategy

**Unit Tests**:
- Test message serialization/deserialization
- Test template rendering for each backend
- Test publisher logic
- Test consumer routing logic

**Integration Tests**:
- Test notification flow end-to-end
- Test consumer group behavior (multiple workers)
- Test retry logic and error handling
- Test backend-specific delivery

**Load Tests**:
- Verify system handles 1000 notifications/sec
- Test consumer lag recovery
- Test backpressure handling

**E2E Tests**:
- Test actual email notifications in docker-compose environment
- Verify audit logs capture all notifications
- Test multi-backend routing

## Performance Impact

**Expected Improvements**:
- API latency reduced by 50-200ms (no blocking on SMTP)
- Can handle 10,000+ notifications/hour with 3 workers
- Horizontal scaling by adding more consumer instances

**Resource Usage**:
- Redpanda: ~512MB memory, minimal CPU (testing), 2-4GB (production)
- Each notification worker: ~50MB memory, minimal CPU
- Network: ~1KB per notification message

**Scalability**:
- Add more consumer instances for higher throughput
- Increase topic partitions for more parallelism
- Redpanda scales to millions of messages/day

## Migration Plan

1. **Add Redpanda to testing infrastructure** (backward compatible, Week 1)
2. **Deploy audit workers** alongside existing system (Week 2)
3. **Enable feature flag** to route 10% of notifications through Redpanda (Week 4)
4. **Monitor both systems** in parallel for 1 week
5. **Gradually increase** percentage routed through new system (10% → 50% → 100%)
6. **Deploy additional backends** (mail, Slack) once stable (Week 5)
7. **Remove old system** once new system proven stable (Week 6)

**Rollback Plan**: Feature flag to disable new system and revert to synchronous notifications

## Dependencies

- **franz-go**: Go Kafka client library - https://github.com/twmb/franz-go
- **Redpanda**: Kafka-compatible message broker - https://redpanda.com/
- **Docker Compose**: For local testing infrastructure
- **SMTP Server**: For mail backend (optional)
- **Slack Bot**: For Slack backend (optional)
- **Telegram Bot**: For Telegram backend (optional)
- **Discord Bot**: For Discord backend (optional)

## Operational Considerations

**Monitoring**:
- Consumer lag metrics (Redpanda metrics)
- Notification delivery success/failure rates per backend
- Template rendering errors
- Backend API failures

**Alerting**:
- Consumer lag > 1000 messages
- Backend failure rate > 5%
- Dead letter queue accumulation

**Capacity Planning**:
- Expected notification volume: 1000-5000/day
- Peak volume: 10,000/hour (document approval waves)
- Worker scaling: 1 worker per 1000 messages/hour

## Open Questions

- Q: Should we persist notification history to database?
  - **A**: Yes, implement in Phase 5 for audit and debugging (optional table)

- Q: How do we handle notification preferences (email vs Slack)?
  - **A**: Out of scope for this RFC. Address in future RFC on user preferences. For now, route to all available backends.

- Q: What's the retry policy for failed notifications?
  - **A**: Exponential backoff: 1m, 5m, 30m, 2h, then DLQ (dead letter queue)

- Q: Should we support notification batching?
  - **A**: Not initially. Add if performance profiling shows benefit.

- Q: How do we map users to Slack/Telegram/Discord IDs?
  - **A**: Phase 1: Manual configuration in user profiles. Future: OAuth integration with each platform.

## Timeline

- **Week 1**: Foundation (Redpanda + message schema + publisher)
- **Week 2**: Audit backend + template system
- **Week 3**: Additional backends (mail, Slack, Telegram, Discord)
- **Week 4**: Integration with Hermes API + testing
- **Week 5**: Production hardening (retry, DLQ, monitoring)
- **Week 6**: Gradual rollout + validation

**Target Completion**: 6 weeks from approval

## Success Metrics

- API latency for notification endpoints reduced by >100ms
- Zero notification-related API failures
- 100% of notifications captured in audit logs
- Support for 3+ notification backends
- Successful migration of all existing email notifications
- >99.9% notification delivery success rate

## References

- RFC-085: Edge-to-Central Architecture (notification delegation context)
- Franz-go Documentation: https://pkg.go.dev/github.com/twmb/franz-go
- Redpanda Quickstart: https://docs.redpanda.com/current/get-started/quick-start/
- Kafka Consumer Groups: https://kafka.apache.org/documentation/#consumergroups
- Go Kafka Best Practices: https://www.cloudkarafka.com/blog/go-kafka-consumer-best-practices.html

## Implementation Documents

This RFC is supported by detailed implementation documents:

1. **[RFC-087-MESSAGE-SCHEMA.md](./RFC-087-MESSAGE-SCHEMA.md)** - Message format, templates, template context, and template storage
2. **[RFC-087-BACKENDS.md](./RFC-087-BACKENDS.md)** - Backend implementations with template loading from document storage (audit, mail, Slack, Telegram, Discord)
3. **[RFC-087-DOCKER-COMPOSE.md](./RFC-087-DOCKER-COMPOSE.md)** - Docker Compose integration for testing
4. **[RFC-087-ADDENDUM.md](./RFC-087-ADDENDUM.md)** - Critical fixes and enhancements (retry logic, DLQ, encryption, graceful shutdown, template injection prevention)

---

**Document ID**: RFC-087
**Hermes UUID**: RFC-087-NOTIFICATION-BACKEND
**Last Updated**: 2025-11-13
