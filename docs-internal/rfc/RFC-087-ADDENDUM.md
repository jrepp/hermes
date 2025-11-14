# RFC-087 Addendum: Critical Fixes and Implementation Details

**Parent**: [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md)
**Status**: Required for Implementation
**Date**: 2025-11-14

This addendum addresses critical issues identified during RFC-087 review that must be resolved before implementation.

## Overview

After comprehensive review, 10 critical issues were identified that could cause data loss, security breaches, or system failures. This document specifies required fixes for each issue.

---

## 1. Retry Logic and Error Handling

### Problem
No retry implementation despite mentions throughout RFC. Transient failures result in permanent notification loss.

### Solution

#### 1.1 Retry Configuration

```go
// pkg/notifications/retry.go
package notifications

import (
    "time"
)

type RetryConfig struct {
    MaxRetries      int           // Maximum retry attempts
    InitialInterval time.Duration // First retry delay
    MaxInterval     time.Duration // Maximum retry delay
    Multiplier      float64       // Backoff multiplier
}

var DefaultRetryConfig = RetryConfig{
    MaxRetries:      5,
    InitialInterval: 1 * time.Minute,
    MaxInterval:     2 * time.Hour,
    Multiplier:      2.0, // Exponential: 1m, 2m, 4m, 8m, 16m → 2h
}

// CalculateBackoff returns next retry delay
func (c *RetryConfig) CalculateBackoff(retryCount int) time.Duration {
    if retryCount >= c.MaxRetries {
        return 0 // No more retries
    }

    delay := float64(c.InitialInterval) * math.Pow(c.Multiplier, float64(retryCount))
    if time.Duration(delay) > c.MaxInterval {
        return c.MaxInterval
    }
    return time.Duration(delay)
}
```

#### 1.2 Enhanced Message Schema with Retry Metadata

```go
// pkg/notifications/message.go - Enhanced
type NotificationMessage struct {
    // ... existing fields ...

    // Retry tracking
    RetryCount    int       `json:"retry_count"`
    LastError     string    `json:"last_error,omitempty"`
    LastRetryAt   time.Time `json:"last_retry_at,omitempty"`
    NextRetryAt   time.Time `json:"next_retry_at,omitempty"`
    FailedBackends []string `json:"failed_backends,omitempty"` // Track which backends failed
}

// ShouldRetry checks if message should be retried
func (m *NotificationMessage) ShouldRetry(config RetryConfig) bool {
    return m.RetryCount < config.MaxRetries
}

// RecordFailure updates retry metadata
func (m *NotificationMessage) RecordFailure(err error, backend string, config RetryConfig) {
    m.RetryCount++
    m.LastError = err.Error()
    m.LastRetryAt = time.Now()
    m.NextRetryAt = time.Now().Add(config.CalculateBackoff(m.RetryCount))

    // Track failed backend
    if backend != "" {
        m.FailedBackends = append(m.FailedBackends, backend)
    }
}
```

#### 1.3 Retry-Enabled Consumer

```go
// cmd/notification-worker/consumer.go
func processMessageWithRetry(ctx context.Context, backends []backends.Backend, record *kgo.Record, retryConfig RetryConfig) error {
    var msg notifications.NotificationMessage
    if err := json.Unmarshal(record.Value, &msg); err != nil {
        // Parsing error - send to DLQ immediately (no retry will fix this)
        return publishToDLQ(ctx, record.Value, fmt.Errorf("parse error: %w", err))
    }

    log.Printf("Processing message: id=%s attempt=%d/%d", msg.ID, msg.RetryCount+1, retryConfig.MaxRetries)

    // Process with all backends
    var lastErr error
    successCount := 0

    for _, backend := range backends {
        for _, targetBackend := range msg.Backends {
            if backend.SupportsBackend(targetBackend) {
                if err := backend.Handle(ctx, &msg); err != nil {
                    // Check if error is retryable
                    if IsRetryableError(err) {
                        log.Printf("backend %s failed (retryable): %v", backend.Name(), err)
                        msg.RecordFailure(err, backend.Name(), retryConfig)
                        lastErr = err
                    } else {
                        // Permanent error (e.g., template not found) - send to DLQ
                        log.Printf("backend %s failed (permanent): %v", backend.Name(), err)
                        return publishToDLQ(ctx, record.Value, fmt.Errorf("permanent error in %s: %w", backend.Name(), err))
                    }
                } else {
                    log.Printf("backend %s processed message %s", backend.Name(), msg.ID)
                    successCount++
                }
            }
        }
    }

    // If all backends succeeded, commit
    if lastErr == nil {
        return nil
    }

    // Check if should retry
    if msg.ShouldRetry(retryConfig) {
        // Publish to retry topic with delay
        return publishToRetryTopic(ctx, &msg)
    }

    // Max retries exceeded - send to DLQ
    log.Printf("Max retries exceeded for message %s", msg.ID)
    return publishToDLQ(ctx, record.Value, fmt.Errorf("max retries exceeded: %w", lastErr))
}

// Error classification
func IsRetryableError(err error) bool {
    // Network errors
    if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ETIMEDOUT) {
        return true
    }

    // HTTP errors
    if httpErr, ok := err.(*HTTPError); ok {
        // 5xx errors are retryable, 4xx are not
        return httpErr.StatusCode >= 500
    }

    // Rate limit errors
    if errors.Is(err, ErrRateLimited) {
        return true
    }

    // Template errors are NOT retryable
    if errors.Is(err, ErrTemplateNotFound) {
        return false
    }

    // Default: retry
    return true
}
```

---

## 2. Dead Letter Queue (DLQ) Implementation

### Problem
Failed messages are lost permanently with no investigation or replay mechanism.

### Solution

#### 2.1 DLQ Topics

```yaml
# Kafka Topics
hermes.notifications          # Main topic
hermes.notifications.retry    # Retry topic (with delays)
hermes.notifications.dlq      # Dead letter queue
```

#### 2.2 DLQ Publisher

```go
// pkg/notifications/dlq.go
package notifications

type DLQMessage struct {
    OriginalMessage json.RawMessage `json:"original_message"`
    FailureReason   string          `json:"failure_reason"`
    FailedAt        time.Time       `json:"failed_at"`
    RetryCount      int             `json:"retry_count"`
    LastError       string          `json:"last_error"`
    FailedBackends  []string        `json:"failed_backends"`
}

func PublishToDLQ(ctx context.Context, client *kgo.Client, originalMsg []byte, err error) error {
    dlqMsg := DLQMessage{
        OriginalMessage: originalMsg,
        FailureReason:   err.Error(),
        FailedAt:        time.Now(),
    }

    // Try to parse original message for metadata
    var notif NotificationMessage
    if parseErr := json.Unmarshal(originalMsg, &notif); parseErr == nil {
        dlqMsg.RetryCount = notif.RetryCount
        dlqMsg.LastError = notif.LastError
        dlqMsg.FailedBackends = notif.FailedBackends
    }

    msgJSON, _ := json.Marshal(dlqMsg)

    record := &kgo.Record{
        Topic: "hermes.notifications.dlq",
        Value: msgJSON,
    }

    return client.ProduceSync(ctx, record).FirstErr()
}
```

#### 2.3 DLQ Monitoring

```go
// cmd/notification-dlq-monitor/main.go
package main

import (
    "context"
    "log"
    "github.com/twmb/franz-go/pkg/kgo"
)

func main() {
    // DLQ consumer that just logs and alerts
    client, _ := kgo.NewClient(
        kgo.SeedBrokers("redpanda:9092"),
        kgo.ConsumeTopics("hermes.notifications.dlq"),
    )
    defer client.Close()

    for {
        fetches := client.PollFetches(context.Background())
        fetches.EachRecord(func(record *kgo.Record) {
            var dlqMsg notifications.DLQMessage
            json.Unmarshal(record.Value, &dlqMsg)

            // Log to structured logging system
            log.Printf("DLQ MESSAGE: reason=%s retry_count=%d failed_at=%s",
                dlqMsg.FailureReason,
                dlqMsg.RetryCount,
                dlqMsg.FailedAt)

            // Send alert if DLQ accumulates messages
            alertOnDLQAccumulation()
        })
    }
}
```

#### 2.4 DLQ Replay Tool

```go
// cmd/notification-dlq-replay/main.go
package main

// Tool to replay messages from DLQ back to main topic
func main() {
    var messageID string
    flag.StringVar(&messageID, "id", "", "Message ID to replay (or 'all')")
    flag.Parse()

    // Read from DLQ, republish to main topic
    // Useful for recovering from transient issues
}
```

---

## 3. Message Ordering and Partitioning

### Problem
Messages for same user/document can be processed out of order, causing confusing notification sequences.

### Solution

#### 3.1 Partition Key Strategy

```go
// pkg/notifications/publisher.go - Enhanced
func (p *Publisher) PublishNotification(
    ctx context.Context,
    notifType NotificationType,
    template string,
    templateContext map[string]any,
    recipients []Recipient,
    backends []string,
) error {
    msg := NotificationMessage{
        ID:              uuid.New().String(),
        Type:            notifType,
        Timestamp:       time.Now(),
        Recipients:      recipients,
        Template:        template,
        TemplateContext: templateContext,
        Backends:        backends,
    }

    msgJSON, _ := json.Marshal(msg)

    // Determine partition key for ordering
    partitionKey := determinePartitionKey(&msg)

    record := &kgo.Record{
        Topic: p.topic,
        Key:   []byte(partitionKey), // Critical: ensures ordering per key
        Value: msgJSON,
    }

    return p.client.ProduceSync(ctx, record).FirstErr()
}

// determinePartitionKey ensures related messages go to same partition
func determinePartitionKey(msg *NotificationMessage) string {
    // Priority 1: Use document UUID if present (all notifications about same doc ordered)
    if msg.DocumentUUID != "" {
        return fmt.Sprintf("doc:%s", msg.DocumentUUID)
    }

    // Priority 2: Use first recipient (all notifications to same user ordered)
    if len(msg.Recipients) > 0 {
        if msg.Recipients[0].Email != "" {
            return fmt.Sprintf("user:%s", msg.Recipients[0].Email)
        }
    }

    // Fallback: random (no ordering guarantee)
    return msg.ID
}
```

#### 3.2 Sequence Numbers (Optional)

```go
// For strict ordering verification
type NotificationMessage struct {
    // ... existing fields ...
    SequenceNumber int64 `json:"sequence_number,omitempty"` // Monotonically increasing per partition key
}
```

---

## 4. Duplicate Message Handling (Idempotency)

### Problem
At-least-once delivery causes duplicate notifications to users.

### Solution

#### 4.1 Idempotency Layer

```go
// pkg/notifications/deduplication.go
package notifications

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/go-redis/redis/v8"
)

type DeduplicationCache struct {
    redis  *redis.Client
    ttl    time.Duration
}

func NewDeduplicationCache(redisAddr string) *DeduplicationCache {
    return &DeduplicationCache{
        redis: redis.NewClient(&redis.Options{
            Addr: redisAddr,
        }),
        ttl: 24 * time.Hour, // Keep dedup cache for 24 hours
    }
}

// IsDuplicate checks if message was recently processed
func (d *DeduplicationCache) IsDuplicate(ctx context.Context, msg *NotificationMessage) (bool, error) {
    key := d.generateKey(msg)

    // Try to set key with NX (only if not exists)
    success, err := d.redis.SetNX(ctx, key, time.Now().Unix(), d.ttl).Result()
    if err != nil {
        return false, err
    }

    // If SetNX failed, key already exists = duplicate
    return !success, nil
}

// generateKey creates deterministic key from message content
func (d *DeduplicationCache) generateKey(msg *NotificationMessage) string {
    // Hash of: template + recipients + context (ignore timestamp and ID)
    h := sha256.New()
    h.Write([]byte(msg.Template))

    for _, r := range msg.Recipients {
        h.Write([]byte(r.Email))
    }

    contextJSON, _ := json.Marshal(msg.TemplateContext)
    h.Write(contextJSON)

    hash := hex.EncodeToString(h.Sum(nil))
    return fmt.Sprintf("notif:dedup:%s", hash[:16])
}
```

#### 4.2 Deduplication in Consumer

```go
// cmd/notification-worker/main.go - Enhanced
func processMessage(ctx context.Context, backends []backends.Backend, record *kgo.Record, dedupCache *DeduplicationCache) error {
    var msg notifications.NotificationMessage
    json.Unmarshal(record.Value, &msg)

    // Check for duplicate
    isDup, err := dedupCache.IsDuplicate(ctx, &msg)
    if err != nil {
        log.Printf("Deduplication check failed: %v", err)
        // Continue processing to avoid message loss
    } else if isDup {
        log.Printf("Skipping duplicate message: id=%s template=%s", msg.ID, msg.Template)
        return nil // Skip processing, commit offset
    }

    // Process message normally...
}
```

---

## 5. Transaction Support and Outbox Pattern

### Problem
API operations succeed but notification publishing fails, causing inconsistency.

### Solution

#### 5.1 Notification Outbox Table

```sql
-- Database migration
CREATE TABLE notification_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_type VARCHAR(50) NOT NULL,
    template VARCHAR(100) NOT NULL,
    template_context JSONB NOT NULL,
    recipients JSONB NOT NULL,
    backends TEXT[] NOT NULL,

    -- Metadata
    document_uuid UUID,
    user_id VARCHAR(255),
    project_id VARCHAR(255),

    -- Processing state
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, published, failed
    published_at TIMESTAMP,
    error_message TEXT,
    retry_count INT DEFAULT 0,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_notification_outbox_status ON notification_outbox(status, created_at);
CREATE INDEX idx_notification_outbox_document ON notification_outbox(document_uuid);
```

#### 5.2 Write to Outbox in Transaction

```go
// internal/api/v2/documents.go - Example
func (h *Handler) approveDocument(w http.ResponseWriter, r *http.Request) {
    // Start database transaction
    tx, _ := h.db.BeginTx(r.Context(), nil)
    defer tx.Rollback()

    // 1. Update document status
    _, err := tx.Exec("UPDATE documents SET status = 'approved' WHERE id = $1", docID)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // 2. Write notification to outbox (same transaction)
    notification := NotificationOutboxEntry{
        NotificationType: "document_approved",
        Template:         "document_approved",
        TemplateContext: map[string]any{
            "DocumentShortName": doc.ShortName,
            "ApproverName":      user.Name,
            // ...
        },
        Recipients: buildRecipients(doc.Owners),
        Backends:   []string{"mail", "slack", "audit"},
    }

    _, err = tx.Exec(`
        INSERT INTO notification_outbox
        (notification_type, template, template_context, recipients, backends, document_uuid)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, notification.NotificationType, notification.Template,
       notification.TemplateContext, notification.Recipients,
       notification.Backends, docID)

    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // 3. Commit transaction (atomic: both or neither)
    if err := tx.Commit(); err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // Success - notification guaranteed to be sent (eventually)
    w.WriteHeader(200)
}
```

#### 5.3 Outbox Publisher Background Process

```go
// cmd/notification-outbox-publisher/main.go
package main

import (
    "context"
    "database/sql"
    "time"
)

func main() {
    db, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    publisher, _ := notifications.NewPublisher([]string{"redpanda:9092"}, "hermes.notifications")

    ticker := time.NewTicker(5 * time.Second)

    for range ticker.C {
        publishPendingNotifications(db, publisher)
    }
}

func publishPendingNotifications(db *sql.DB, publisher *notifications.Publisher) {
    // Fetch pending notifications
    rows, _ := db.Query(`
        SELECT id, notification_type, template, template_context, recipients, backends
        FROM notification_outbox
        WHERE status = 'pending'
        ORDER BY created_at
        LIMIT 100
    `)
    defer rows.Close()

    for rows.Next() {
        var entry NotificationOutboxEntry
        rows.Scan(&entry.ID, &entry.NotificationType, &entry.Template,
                 &entry.TemplateContext, &entry.Recipients, &entry.Backends)

        // Publish to Kafka
        err := publisher.PublishNotification(
            context.Background(),
            entry.NotificationType,
            entry.Template,
            entry.TemplateContext,
            entry.Recipients,
            entry.Backends,
        )

        if err != nil {
            // Update outbox: increment retry count
            db.Exec(`
                UPDATE notification_outbox
                SET retry_count = retry_count + 1,
                    error_message = $1,
                    updated_at = NOW()
                WHERE id = $2
            `, err.Error(), entry.ID)
        } else {
            // Mark as published
            db.Exec(`
                UPDATE notification_outbox
                SET status = 'published',
                    published_at = NOW(),
                    updated_at = NOW()
                WHERE id = $1
            `, entry.ID)
        }
    }
}
```

---

## 6. Message Encryption (PII Protection)

### Problem
Messages contain PII (emails, names, user IDs) stored unencrypted in Redpanda for 7 days.

### Solution

#### 6.1 Envelope Encryption

```go
// pkg/notifications/encryption.go
package notifications

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
)

type Encryptor struct {
    key []byte // 32 bytes for AES-256
}

func NewEncryptor(keyBase64 string) (*Encryptor, error) {
    key, err := base64.StdEncoding.DecodeString(keyBase64)
    if err != nil {
        return nil, err
    }

    if len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes for AES-256")
    }

    return &Encryptor{key: key}, nil
}

// Encrypt encrypts data using AES-256-GCM
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }

    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return ciphertext, nil
}

// Decrypt decrypts AES-256-GCM encrypted data
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, fmt.Errorf("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, err
    }

    return plaintext, nil
}
```

#### 6.2 Encrypted Message Schema

```go
// pkg/notifications/message.go - Enhanced
type NotificationMessage struct {
    // Unencrypted metadata (for routing)
    ID        string                 `json:"id"`
    Type      NotificationType       `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Priority  int                    `json:"priority"`

    // Document context (unencrypted for querying)
    DocumentUUID string `json:"document_uuid,omitempty"`
    ProjectID    string `json:"project_id,omitempty"`

    // Backend routing
    Backends []string `json:"backends"`

    // Encrypted payload (contains PII)
    EncryptedPayload string `json:"encrypted_payload"` // Base64-encoded encrypted JSON

    // Retry tracking
    RetryCount int `json:"retry_count,omitempty"`
}

// EncryptedPayloadData contains sensitive information
type EncryptedPayloadData struct {
    Recipients      []Recipient    `json:"recipients"`      // PII: emails, names, IDs
    Template        string         `json:"template"`
    TemplateContext map[string]any `json:"template_context"` // May contain PII
    UserID          string         `json:"user_id,omitempty"`
}
```

#### 6.3 Encrypt Before Publishing

```go
// pkg/notifications/publisher.go - Enhanced
func (p *Publisher) PublishNotification(
    ctx context.Context,
    notifType NotificationType,
    template string,
    templateContext map[string]any,
    recipients []Recipient,
    backends []string,
) error {
    // Prepare sensitive data for encryption
    payloadData := EncryptedPayloadData{
        Recipients:      recipients,
        Template:        template,
        TemplateContext: templateContext,
    }

    payloadJSON, _ := json.Marshal(payloadData)

    // Encrypt the payload
    encryptedPayload, err := p.encryptor.Encrypt(payloadJSON)
    if err != nil {
        return fmt.Errorf("failed to encrypt payload: %w", err)
    }

    // Build message with encrypted payload
    msg := NotificationMessage{
        ID:               uuid.New().String(),
        Type:             notifType,
        Timestamp:        time.Now(),
        Backends:         backends,
        EncryptedPayload: base64.StdEncoding.EncodeToString(encryptedPayload),
    }

    // Publish to Kafka
    msgJSON, _ := json.Marshal(msg)
    record := &kgo.Record{
        Topic: p.topic,
        Key:   []byte(msg.ID),
        Value: msgJSON,
    }

    return p.client.ProduceSync(ctx, record).FirstErr()
}
```

#### 6.4 Decrypt in Consumer

```go
// cmd/notification-worker/main.go - Enhanced
func processMessage(ctx context.Context, backends []backends.Backend, record *kgo.Record, decryptor *Encryptor) error {
    var msg notifications.NotificationMessage
    json.Unmarshal(record.Value, &msg)

    // Decrypt payload
    encryptedPayload, _ := base64.StdEncoding.DecodeString(msg.EncryptedPayload)
    decryptedPayload, err := decryptor.Decrypt(encryptedPayload)
    if err != nil {
        return fmt.Errorf("failed to decrypt payload: %w", err)
    }

    var payloadData notifications.EncryptedPayloadData
    json.Unmarshal(decryptedPayload, &payloadData)

    // Reconstruct full message for backend processing
    fullMsg := msg
    fullMsg.Recipients = payloadData.Recipients
    fullMsg.Template = payloadData.Template
    fullMsg.TemplateContext = payloadData.TemplateContext

    // Process with backends
    for _, backend := range backends {
        for _, targetBackend := range msg.Backends {
            if backend.SupportsBackend(targetBackend) {
                backend.Handle(ctx, &fullMsg)
            }
        }
    }

    return nil
}
```

#### 6.5 Key Management

```bash
# Generate encryption key
openssl rand -base64 32 > notification-encryption-key.txt

# Store in Kubernetes Secret
kubectl create secret generic notification-encryption-key \
  --from-file=key=notification-encryption-key.txt

# Or use AWS KMS, HashiCorp Vault, etc.
```

---

## 7. Context Cancellation and Graceful Shutdown

### Problem
Workers cannot shut down cleanly. In-flight messages may be lost or duplicated during deployments.

### Solution

#### 7.1 Graceful Shutdown with Timeout

```go
// cmd/notification-worker/main.go - Enhanced
func main() {
    cfg := parseConfig()

    client, _ := kgo.NewClient(
        kgo.SeedBrokers(cfg.Brokers...),
        kgo.ConsumerGroup(cfg.ConsumerGroup),
        kgo.ConsumeTopics(cfg.Topic),
    )
    defer client.Close()

    backendList := initializeBackends(cfg.EnabledBackends)

    // Setup signal handling with graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // Track in-flight messages
    var wg sync.WaitGroup
    inFlightSemaphore := make(chan struct{}, 10) // Limit concurrent processing

    go func() {
        <-sigChan
        log.Println("Shutdown signal received, stopping message consumption...")
        cancel() // Stop accepting new messages
    }()

    log.Printf("Starting notification worker (backends=%v)\n", cfg.EnabledBackends)

    // Consume messages
    for {
        select {
        case <-ctx.Done():
            log.Println("Context cancelled, waiting for in-flight messages...")

            // Wait for in-flight messages with timeout
            done := make(chan struct{})
            go func() {
                wg.Wait()
                close(done)
            }()

            select {
            case <-done:
                log.Println("All in-flight messages completed")
            case <-time.After(30 * time.Second):
                log.Println("Shutdown timeout exceeded, forcing exit")
            }

            return
        default:
            fetches := client.PollFetches(ctx)
            if errs := fetches.Errors(); len(errs) > 0 {
                for _, err := range errs {
                    log.Printf("fetch error: %v\n", err)
                }
                continue
            }

            fetches.EachPartition(func(p kgo.FetchTopicPartition) {
                for _, record := range p.Records {
                    // Acquire semaphore (limit concurrency)
                    inFlightSemaphore <- struct{}{}
                    wg.Add(1)

                    // Process in goroutine with timeout
                    go func(r *kgo.Record) {
                        defer wg.Done()
                        defer func() { <-inFlightSemaphore }()

                        // Create context with timeout for this message
                        msgCtx, msgCancel := context.WithTimeout(ctx, 30*time.Second)
                        defer msgCancel()

                        if err := processMessageWithTimeout(msgCtx, backendList, r); err != nil {
                            log.Printf("failed to process message: %v\n", err)
                        } else {
                            client.CommitRecords(ctx, r)
                        }
                    }(record)
                }
            })
        }
    }
}

func processMessageWithTimeout(ctx context.Context, backends []backends.Backend, record *kgo.Record) error {
    // Check if context already cancelled
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Process with timeout enforcement
    done := make(chan error, 1)
    go func() {
        done <- processMessage(ctx, backends, record)
    }()

    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return fmt.Errorf("message processing cancelled: %w", ctx.Err())
    }
}
```

#### 7.2 Backend Context Timeout

```go
// pkg/notifications/backends/backend.go - Enhanced
type Backend interface {
    Name() string
    Handle(ctx context.Context, msg *notifications.NotificationMessage) error
    SupportsBackend(backend string) bool
}

// Example: Mail backend with context timeout
func (b *MailBackend) sendEmail(ctx context.Context, to, subject, htmlBody string) error {
    // Check context before starting
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // SMTP with context timeout
    var d net.Dialer
    conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", b.config.SMTPHost, b.config.SMTPPort))
    if err != nil {
        return err
    }
    defer conn.Close()

    // Use context deadline for SMTP operations
    deadline, ok := ctx.Deadline()
    if ok {
        conn.SetDeadline(deadline)
    }

    // Rest of SMTP logic...
}
```

---

## 8. Template Injection Prevention

### Problem
User-provided content in template context could contain malicious template code, leading to code execution.

### Solution

#### 8.1 Input Sanitization

```go
// pkg/notifications/template_security.go
package notifications

import (
    "html"
    "regexp"
    "strings"
)

// SanitizeTemplateContext sanitizes all user-provided fields
func SanitizeTemplateContext(context map[string]any) map[string]any {
    sanitized := make(map[string]any)

    for key, value := range context {
        switch v := value.(type) {
        case string:
            sanitized[key] = sanitizeString(v)
        case map[string]any:
            sanitized[key] = SanitizeTemplateContext(v)
        default:
            sanitized[key] = value
        }
    }

    return sanitized
}

func sanitizeString(s string) string {
    // 1. Remove template syntax
    s = removeTemplateSyntax(s)

    // 2. HTML escape for email templates
    s = html.EscapeString(s)

    // 3. Remove control characters
    s = removeControlCharacters(s)

    return s
}

func removeTemplateSyntax(s string) string {
    // Remove {{ }} syntax that could be interpreted as template code
    templateRegex := regexp.MustCompile(`\{\{.*?\}\}`)
    return templateRegex.ReplaceAllString(s, "")
}

func removeControlCharacters(s string) string {
    return strings.Map(func(r rune) rune {
        if r == '\n' || r == '\r' || r == '\t' {
            return r // Keep whitespace
        }
        if r < 32 {
            return -1 // Remove control chars
        }
        return r
    }, s)
}
```

#### 8.2 Safe Template Execution

```go
// pkg/notifications/backends/mail.go - Enhanced
func (b *MailBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
    // Sanitize template context before rendering
    safeContext := SanitizeTemplateContext(msg.TemplateContext)

    // Use html/template which auto-escapes
    tmpl, ok := b.templates[msg.Template]
    if !ok {
        return ErrTemplateNotFound
    }

    var body bytes.Buffer
    if err := tmpl.Execute(&body, safeContext); err != nil {
        return fmt.Errorf("template render error: %w", err)
    }

    // Rest of implementation...
}
```

#### 8.3 Template Allowlist

```go
// Only allow pre-defined template variables
var allowedTemplateFields = map[string]bool{
    "DocumentTitle":     true,
    "DocumentShortName": true,
    "DocumentType":      true,
    "DocumentStatus":    true,
    "DocumentURL":       true,
    "ApproverName":      true,
    "OwnerName":         true,
    "Product":           true,
    // ... etc
}

func ValidateTemplateContext(context map[string]any) error {
    for key := range context {
        if !allowedTemplateFields[key] {
            return fmt.Errorf("disallowed template field: %s", key)
        }
    }
    return nil
}
```

---

## 9. Backend Error Handling and Partial Failures

### Problem
Backends log errors but return nil, causing silent failures and premature offset commits.

### Solution

#### 9.1 Proper Error Propagation

```go
// pkg/notifications/backends/backend.go - Enhanced error types
type BackendError struct {
    Backend   string
    Err       error
    Retryable bool
}

func (e *BackendError) Error() string {
    return fmt.Sprintf("%s backend error: %v", e.Backend, e.Err)
}

type MultiBackendError struct {
    Errors []BackendError
}

func (e *MultiBackendError) Error() string {
    var msgs []string
    for _, err := range e.Errors {
        msgs = append(msgs, err.Error())
    }
    return strings.Join(msgs, "; ")
}

func (e *MultiBackendError) HasRetryableError() bool {
    for _, err := range e.Errors {
        if err.Retryable {
            return true
        }
    }
    return false
}
```

#### 9.2 Enhanced Consumer Logic

```go
// cmd/notification-worker/consumer.go - Enhanced
func processMessage(ctx context.Context, backends []backends.Backend, record *kgo.Record) error {
    var msg notifications.NotificationMessage
    json.Unmarshal(record.Value, &msg)

    var errors []BackendError
    successCount := 0

    // Process all backends
    for _, backend := range backends {
        for _, targetBackend := range msg.Backends {
            if backend.SupportsBackend(targetBackend) {
                if err := backend.Handle(ctx, &msg); err != nil {
                    errors = append(errors, BackendError{
                        Backend:   backend.Name(),
                        Err:       err,
                        Retryable: IsRetryableError(err),
                    })
                    log.Printf("Backend %s failed: %v", backend.Name(), err)
                } else {
                    successCount++
                    log.Printf("Backend %s succeeded", backend.Name())
                }
            }
        }
    }

    // If no backends succeeded, return error
    if successCount == 0 && len(errors) > 0 {
        return &MultiBackendError{Errors: errors}
    }

    // Partial success is acceptable if at least one backend succeeded
    if len(errors) > 0 {
        log.Printf("Partial success: %d backends succeeded, %d failed", successCount, len(errors))
    }

    return nil
}
```

---

## 10. Producer Durability and Acknowledgments

### Problem
No configuration for required acks or idempotence. Messages could be lost during broker failures.

### Solution

#### 10.1 Producer Configuration

```go
// pkg/notifications/publisher.go - Enhanced
func NewPublisher(brokers []string, topic string) (*Publisher, error) {
    client, err := kgo.NewClient(
        kgo.SeedBrokers(brokers...),

        // Wait for all in-sync replicas to acknowledge
        kgo.RequiredAcks(kgo.AllISRAcks()),

        // Enable idempotent producer (prevents duplicates from retries)
        kgo.ProducerBatchMaxBytes(1000000),
        kgo.ProducerLinger(10 * time.Millisecond),

        // Retry configuration
        kgo.RetryBackoffFn(func(tries int) time.Duration {
            return time.Duration(tries) * 100 * time.Millisecond
        }),
        kgo.RequestRetries(3),

        // Compression
        kgo.ProducerBatchCompression(kgo.GzipCompression()),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create kafka client: %w", err)
    }

    return &Publisher{
        client: client,
        topic:  topic,
    }, nil
}
```

#### 10.2 Redpanda Topic Configuration

```yaml
# docker-compose.yml enhancement
services:
  redpanda-setup:
    image: docker.redpanda.com/redpandadata/redpanda:v24.2.11
    container_name: redpanda-setup
    depends_on:
      redpanda:
        condition: service_healthy
    command:
      - bash
      - -c
      - |
        rpk topic create hermes.notifications \
          --partitions 3 \
          --replicas 1 \
          --config retention.ms=604800000 \
          --config min.insync.replicas=1 \
          --config compression.type=gzip

        rpk topic create hermes.notifications.retry \
          --partitions 3 \
          --replicas 1

        rpk topic create hermes.notifications.dlq \
          --partitions 1 \
          --replicas 1 \
          --config retention.ms=-1
    networks:
      - hermes-testing
```

---

## Implementation Checklist

### Phase 0: Critical Fixes (Before Implementation)
- [ ] Implement retry logic with exponential backoff
- [ ] Create DLQ topics and handlers
- [ ] Implement partition key strategy for ordering
- [ ] Add deduplication layer (Redis-based)
- [ ] Implement outbox pattern for transactional guarantees
- [ ] Add message encryption (AES-256-GCM)
- [ ] Implement graceful shutdown with context cancellation
- [ ] Add template input sanitization
- [ ] Fix backend error handling (proper error propagation)
- [ ] Configure producer with proper acks and idempotence

### Phase 1: Enhanced Reliability
- [ ] Add rate limiting
- [ ] Implement circuit breakers for backends
- [ ] Add Prometheus metrics
- [ ] Configure HTTP clients with timeouts
- [ ] Add configuration validation at startup
- [ ] Implement backend health checks

### Phase 2: Operational Excellence
- [ ] Add DLQ monitoring dashboard
- [ ] Create DLQ replay tool
- [ ] Implement notification history table
- [ ] Add distributed tracing
- [ ] Create operational runbook
- [ ] Add alerting rules

## Testing Requirements

Each critical fix must include:
1. Unit tests
2. Integration test
3. Failure scenario test
4. Load test (if applicable)

## Documentation Updates Required

- [ ] Update RFC-087-NOTIFICATION-BACKEND.md with security enhancements
- [ ] Update RFC-087-MESSAGE-SCHEMA.md with encryption and retry fields
- [ ] Update RFC-087-DOCKER-COMPOSE.md with enhanced consumer implementation
- [ ] Update RFC-087-BACKENDS.md with error handling and sanitization
- [ ] Create monitoring and operations guide

---

**Related Documents**:
- [RFC-087-NOTIFICATION-BACKEND.md](./RFC-087-NOTIFICATION-BACKEND.md)
- [RFC-087-MESSAGE-SCHEMA.md](./RFC-087-MESSAGE-SCHEMA.md)
- [RFC-087-BACKENDS.md](./RFC-087-BACKENDS.md)
- [RFC-087-DOCKER-COMPOSE.md](./RFC-087-DOCKER-COMPOSE.md)
