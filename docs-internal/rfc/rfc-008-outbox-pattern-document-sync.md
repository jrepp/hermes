---
id: rfc-008
created: 2025-10-09
author: Hermes Team
project_id: hermes
doc_uuid: 51518baa-f9b3-418f-8e36-1cc23b37eb3b
status: Accepted
title: Outbox Pattern for Document Synchronization
type: RFC
subtype: Architecture Proposal
tags: [architecture, document-sync, meilisearch, outbox-pattern, search]
related: [ADR-009, ADR-011, ADR-016, ADR-017, ADR-020]
---

# Outbox Pattern for Document Synchronization

## Summary

Implement the **Transactional Outbox Pattern** to ensure reliable synchronization between the PostgreSQL database and search index (Algolia/Meilisearch), preventing data inconsistencies and enabling eventual consistency guarantees.

This RFC is accepted with the hardened contract below. It follows ADR-009 and ADR-017 by keeping search access behind `search.Provider` and treating the database as the source of truth; it follows ADR-020 by using explicit core+deltas migrations and keeping the external indexer stateless/API-only.

## Motivation

### Current Architecture Problems

**Problem 1: Inconsistent State**

```go
// Current implementation (internal/api/v2/documents.go)
func (h *DocumentsHandler) CreateDocument(c *gin.Context) {
    // 1. Write to database
    err := h.db.Create(&doc).Error
    if err != nil {
        return c.JSON(500, gin.H{"error": "database error"})
    }

    // 2. Write to search index
    err = h.search.Index(ctx, "documents", []Document{doc})
    // ❌ PROBLEM: If this fails, database has doc but search doesn't!
    // Users can view document but can't find it via search
}

```

**Problem 2: Partial Failures**
- Database commit succeeds
- Search index update fails (network issue, API rate limit, quota exceeded)
- Document exists in DB but not searchable
- No automatic retry mechanism

**Problem 3: Race Conditions**
- Multiple workers updating same document
- Search index may have stale data
- No ordering guarantees

**Problem 4: Operational Complexity**
- Manual intervention required to fix inconsistencies
- No visibility into failed synchronization
- Hard to debug "document not found in search" issues

### Real-World Scenarios

**Scenario 1: Algolia Rate Limit**

```text
11:23:45 POST /api/v2/documents (create RFC 456)
11:23:46 DB: Document RFC 456 created ✅
11:23:47 Algolia: Rate limit exceeded (429 Too Many Requests) ❌
11:23:48 User: "Document saved successfully" ✅ (misleading)
11:24:30 User: Search for "RFC 456" → 0 results ❌
```

**Scenario 2: Network Partition**

```text
14:15:00 PUT /api/v2/documents/123 (update content)
14:15:01 DB: Update successful ✅
14:15:02 Meilisearch: Connection timeout ❌
14:15:30 Search shows old content ❌
14:16:00 Another user finds document via search, sees outdated info

```

**Scenario 3: Bulk Import**

```text
09:00:00 Admin: Import 500 documents from CSV
09:00:05 DB: 500 documents inserted ✅
09:00:10 Algolia: 247/500 indexed, then quota exceeded ❌
09:05:00 Users see 253 documents in DB but can't find them via search
```

### User Stories

**Story 1: Developer**
> "When I create a document, I expect it to be searchable within seconds. Currently, sometimes documents are 'lost' and never appear in search results."

**Story 2: Operations Engineer**
> "I need visibility into synchronization failures. How many documents are pending sync? Which ones failed? Can I retry them?"

**Story 3: End User**
> "I just saved a document, but when I search for it, it's not there. My colleague found it by browsing the list. Why is search unreliable?"

## Proposed Solution: Transactional Outbox Pattern

### Architecture Overview

```text
┌───────────────────────────────────────────────────────────────┐
│ API Handler (internal/api/v2/documents.go)                    │
│                                                                │
│  1. Begin Transaction                                         │
│  2. Write Document to documents table                         │
│  3. Write Event to outbox table                               │
│  4. Commit Transaction (atomic!)                              │
└───────────────────────────────────────────────────────────────┘
                            │
                            │ Transaction committed
                            ▼
┌───────────────────────────────────────────────────────────────┐
│ Search Outbox Relay (pkg/search/outbox or pkg/outbox)          │
│                                                                │
│  1. Poll outbox table (every 1s)                              │
│  2. Read pending events                                       │
│  3. Process events (update search index)                      │
│  4. Mark events as processed                                  │
│  5. Retry failed events (exponential backoff)                 │
└───────────────────────────────────────────────────────────────┘
                            │
                            │ Index update
                            ▼
┌───────────────────────────────────────────────────────────────┐
│ Search Provider (Algolia/Meilisearch)                         │
│                                                                │
│  Receives document updates from outbox worker                 │
│  Eventually consistent with database                          │
└───────────────────────────────────────────────────────────────┘

```

### Database Schema

**Outbox Table** (`pkg/models/search_outbox_event.go`):

```go
type SearchOutboxEvent struct {
    ID             uint       `gorm:"primaryKey"`
    EventType      string     `gorm:"type:varchar(80);not null;index"`
    AggregateID    string     `gorm:"type:varchar(255);not null;index"` // stable document UUID when available, provider file ID until migration completes
    AggregateType  string     `gorm:"type:varchar(50);not null"`        // "document", "draft", "project"
    IndexName      string     `gorm:"type:varchar(50);not null;index"`  // "documents", "drafts", "projects"
    Operation      string     `gorm:"type:varchar(20);not null"`        // "upsert" or "delete"
    Sequence       int64      `gorm:"not null"`
    IdempotencyKey string     `gorm:"type:varchar(255);not null;uniqueIndex"`
    Payload        []byte     `gorm:"type:jsonb"`
    Status         string     `gorm:"type:varchar(20);not null;index"`  // "pending", "processing", "completed", "failed", "dlq"
    AttemptCount   int        `gorm:"default:0"`
    AvailableAt    time.Time  `gorm:"index"`
    LockedAt       *time.Time
    LockedBy       string     `gorm:"type:varchar(100)"`
    LastAttemptAt  *time.Time
    CompletedAt    *time.Time
    ErrorMessage   string     `gorm:"type:text"`
    CreatedAt      time.Time  `gorm:"index"`
    UpdatedAt      time.Time
}
```

Use a dedicated `search_outbox_events` table rather than extending `document_revision_outbox`. `document_revision_outbox` belongs to revision/indexer submission workflows; T1 owns search projection events. Sharing the table would couple two event models with different consumers, idempotency keys, retention, and replay semantics. This preserves the ADR-020 stateless-indexer boundary: the external indexer still submits via API only, and the central server writes `search_outbox_events` inside its own database transactions.

**Indexes**:

```sql
CREATE INDEX idx_search_outbox_status_created ON search_outbox_events(status, created_at);
CREATE INDEX idx_search_outbox_event_type ON search_outbox_events(event_type);
CREATE INDEX idx_search_outbox_aggregate_id ON search_outbox_events(aggregate_id);
CREATE INDEX idx_search_outbox_available ON search_outbox_events(status, available_at, created_at);
CREATE INDEX idx_search_outbox_ordering ON search_outbox_events(aggregate_type, aggregate_id, sequence);

```

The actual migration must use ADR-020 core+deltas files under `internal/db/migrations/`, not GORM AutoMigrate.

### Event Contract

The outbox records search projection events, not revision events. Each row says which cache projection must be brought into convergence with database truth.

| Event type | Aggregate | Index | Operation | Payload source | Idempotency key |
|------------|-----------|-------|-----------|----------------|-----------------|
| `document.created` | document UUID/provider ID | documents | upsert | canonical DB document + reviews + project/product fields needed by `search.Document` | `documents:{aggregate_id}:created:{sequence}` |
| `document.updated` | document UUID/provider ID | documents | upsert | canonical DB document snapshot | `documents:{aggregate_id}:updated:{sequence}` |
| `document.published` | document UUID/provider ID | documents | upsert | canonical published DB document snapshot | `documents:{aggregate_id}:published:{sequence}` |
| `document.deleted` | document UUID/provider ID | documents | delete | tombstone only: aggregate ID, provider ID, project ID | `documents:{aggregate_id}:deleted:{sequence}` |
| `draft.created` | document UUID/provider ID | drafts | upsert | canonical draft DB document snapshot | `drafts:{aggregate_id}:created:{sequence}` |
| `draft.updated` | document UUID/provider ID | drafts | upsert | canonical draft DB document snapshot | `drafts:{aggregate_id}:updated:{sequence}` |
| `draft.deleted` | document UUID/provider ID | drafts | delete | tombstone only: aggregate ID, provider ID, project ID | `drafts:{aggregate_id}:deleted:{sequence}` |
| `draft.published` | document UUID/provider ID | drafts | delete | tombstone only for draft projection removal | `drafts:{aggregate_id}:published-delete:{sequence}` |
| `review_state.changed` | document UUID/provider ID | documents | upsert | canonical DB document + current review rows | `documents:{aggregate_id}:review-state:{sequence}` |
| `project.created` | project ID | projects | upsert | canonical DB project snapshot | `projects:{aggregate_id}:created:{sequence}` |
| `project.updated` | project ID | projects | upsert | canonical DB project snapshot | `projects:{aggregate_id}:updated:{sequence}` |
| `search.backfill` | document UUID/provider ID or project ID | target index | upsert/delete | DB snapshot at enqueue time | `{index}:{aggregate_id}:backfill:{batch_id}:{sequence}` |

`sequence` is a monotonically increasing per-aggregate value allocated in the same transaction as the authoritative mutation and the outbox insert. It can be a persisted projection version on the aggregate row or a database-derived counter, but it must not depend on wall-clock ordering alone. Delete and metadata-only events must not use content hashes as their only identity.

### Transaction Boundaries

Every handler mutation that changes a searchable projection must use one `*gorm.DB` transaction for the authoritative database write and the `search_outbox_events` insert. A request may call workspace providers before the DB transaction when necessary to obtain provider IDs/content, but after the database commit it must not perform a best-effort direct search write. If a workspace call and DB transaction cannot be made atomic, database truth still wins: the transaction records the resulting DB state and the outbox event that projects that state.

Direct search writes are allowed only in the relay package, backfill/reindex tooling, tests, and read-only search endpoints. API handlers must enqueue search projection events instead of calling `SearchProvider.*Index().Index` or `Delete` directly.

### Ordering, Idempotency, and Convergence

Relay processing is at-least-once. Duplicate delivery is expected and must be safe.

- Each event has a unique `idempotency_key`; duplicate enqueue attempts become no-ops.
- The relay processes events for the same `(aggregate_type, aggregate_id)` in `sequence` order.
- Later events for one aggregate do not pass an earlier pending, processing, failed, or DLQ event for that same aggregate unless an operator explicitly marks the earlier event skipped.
- Events for unrelated aggregates may continue when one aggregate is failed or in DLQ.
- Upsert processing rehydrates or validates against database state before writing search when practical. If the database row no longer exists, the relay converts the stale upsert into a delete/tombstone outcome and marks the event completed with that reason.
- Search writes must be idempotent by object ID. Replaying the same event must leave the same final index object or tombstone.

The required convergence proof is database-to-cache, not cache-only: tests must compare final search contents against the canonical database rows after relay recovery.

### Retry, DLQ, and Replay

Events start as `pending`. The relay claims due events by setting `processing`, `locked_at`, and `locked_by` inside a transaction, then processes them outside the claim transaction. Crashed `processing` events whose lock is older than the configured visibility timeout return to `pending`.

Retries use exponential backoff by setting `available_at`. After `max_retries`, an event moves to `dlq` with the last error. DLQ rows are durable operational records, not just a counter.

DLQ behavior:

- A DLQ event blocks later events for the same aggregate to preserve per-document ordering.
- DLQ events do not block unrelated aggregates.
- Operators can inspect DLQ rows, retry a row after fixing the cause, or explicitly mark a row `skipped`/`completed` with an operator note if accepting that ordering break is safer than blocking the aggregate.
- Replay uses the original `idempotency_key` for duplicate-safe retry, or a new `search.backfill` event when rebuilding from current database truth.

Replay modes:

- `retry`: move failed or DLQ rows back to `pending` without changing payload or idempotency key.
- `rebuild-current`: enqueue `search.backfill` events from current database rows; this is the preferred recovery when payload schemas changed.
- `replay-range`: re-run completed rows for a time/id range for audit recovery; must remain duplicate-safe.

### Observability Contract

The operational surface must expose more than a DLQ counter:

- Pending row count by index and project.
- Oldest pending row age.
- Oldest failed/DLQ row age.
- Retry count and DLQ count by event type.
- Processing rate and processing latency.
- Per-project lag when the event payload carries project identity.
- Health output that distinguishes `relay_stalled`, `backlog_high`, and `dlq_present`.

### Implementation

#### Step 1: Emit Events in Handlers

The snippets below show the control flow only. The accepted implementation must use `search_outbox_events`, `SearchOutboxEvent`, per-aggregate `sequence`, and the idempotency keys defined in the event contract above.

**Before (Current)**:

```go
func (h *DocumentsHandler) CreateDocument(c *gin.Context) {
    // Write to DB
    if err := h.db.Create(&doc).Error; err != nil {
        return c.JSON(500, err)
    }

    // Update search (MAY FAIL!)
    h.search.Index(ctx, "documents", []Document{doc})

    return c.JSON(201, doc)
}
```

**After (With Outbox)**:

```go
func (h *DocumentsHandler) CreateDocument(c *gin.Context) {
    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // Write to DB
    if err := tx.Create(&doc).Error; err != nil {
        tx.Rollback()
        return c.JSON(500, err)
    }

    // Emit outbox event
    payload, _ := json.Marshal(doc)
    event := &SearchOutboxEvent{
        EventType:      "document.created",
        AggregateID:    doc.ID,
        AggregateType:  "document",
        IndexName:      "documents",
        Operation:      "upsert",
        Sequence:       nextSequence,
        IdempotencyKey: fmt.Sprintf("documents:%s:created:%d", doc.ID, nextSequence),
        Payload:        payload,
        Status:         "pending",
        AvailableAt:    time.Now(),
    }
    if err := tx.Create(event).Error; err != nil {
        tx.Rollback()
        return c.JSON(500, err)
    }

    // Commit transaction (atomic!)
    if err := tx.Commit().Error; err != nil {
        return c.JSON(500, err)
    }

    // ✅ Both DB and outbox event are committed atomically
    return c.JSON(201, doc)
}

```

#### Step 2: Outbox Worker

**Relay Implementation** (`pkg/search/outbox/relay.go` or `pkg/outbox/worker.go`):

```go
type Worker struct {
    db     *gorm.DB
    search search.Provider
    logger hclog.Logger

    pollInterval time.Duration
    batchSize    int
    maxRetries   int
}

func (w *Worker) Start(ctx context.Context) error {
    ticker := time.NewTicker(w.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := w.processEvents(ctx); err != nil {
                w.logger.Error("failed to process events", "error", err)
            }
        }
    }
}

func (w *Worker) processEvents(ctx context.Context) error {
    // Lock events for processing (prevent duplicate processing)
    var events []SearchOutboxEvent
    err := w.db.Transaction(func(tx *gorm.DB) error {
        // Find pending events
        if err := tx.Where("status = ?", "pending").
            Or("status = ? AND attempt_count < ?", "failed", w.maxRetries).
            Order("created_at ASC").
            Limit(w.batchSize).
            Find(&events).Error; err != nil {
            return err
        }

        // Mark as processing
        eventIDs := make([]uint, len(events))
        for i, e := range events {
            eventIDs[i] = e.ID
        }
        return tx.Model(&SearchOutboxEvent{}).
            Where("id IN ?", eventIDs).
            Update("status", "processing").Error
    })

    if err != nil {
        return err
    }

    // Process each event
    for _, event := range events {
        if err := w.processEvent(ctx, &event); err != nil {
            w.logger.Warn("event processing failed",
                "event_id", event.ID,
                "event_type", event.EventType,
                "error", err)

            // Update failure
            w.db.Model(&event).Updates(map[string]interface{}{
                "status":          "failed",
                "attempt_count":   event.AttemptCount + 1,
                "last_attempt_at": time.Now(),
                "error_message":   err.Error(),
            })
        } else {
            // Mark completed
            w.db.Model(&event).Updates(map[string]interface{}{
                "status":       "completed",
                "completed_at": time.Now(),
            })
        }
    }

    return nil
}

func (w *Worker) processEvent(ctx context.Context, event *SearchOutboxEvent) error {
    switch event.EventType {
    case "document.created", "document.updated":
        var doc Document
        if err := json.Unmarshal(event.Payload, &doc); err != nil {
            return fmt.Errorf("unmarshal payload: %w", err)
        }
        return w.search.Index(ctx, "documents", []Document{doc})

    case "document.deleted":
        return w.search.Delete(ctx, "documents", []string{event.AggregateID})

    default:
        return fmt.Errorf("unknown event type: %s", event.EventType)
    }
}
```

#### Step 3: Worker Startup

**Server Initialization** (`cmd/hermes/main.go`):

```go
func main() {
    // ... existing setup ...

    // Start outbox worker
    outboxWorker := outbox.NewWorker(
        db,
        searchProvider,
        logger.Named("outbox"),
        outbox.WithPollInterval(1 * time.Second),
        outbox.WithBatchSize(100),
        outbox.WithMaxRetries(5),
    )

    // Run worker in background
    go func() {
        if err := outboxWorker.Start(ctx); err != nil {
            logger.Error("outbox worker stopped", "error", err)
        }
    }()

    // ... start HTTP server ...
}

```

### Retry Strategy

**Exponential Backoff**:

```text
Attempt | Delay | Cumulative
--------|-------|------------
1       | 1s    | 1s
2       | 2s    | 3s
3       | 4s    | 7s
4       | 8s    | 15s
5       | 16s   | 31s
Failed  | -     | Give up
```

**Configuration**:

```hcl
outbox {
  poll_interval = "1s"
  batch_size    = 100
  max_retries   = 5
  backoff_base  = 2  # Exponential base
}

```

## Monitoring & Observability

### Metrics

**Prometheus Metrics** (`pkg/outbox/metrics.go`):

```go
var (
    outboxEventsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "hermes_outbox_events_total",
            Help: "Total number of outbox events",
        },
        []string{"event_type", "status"},
    )

    outboxProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "hermes_outbox_processing_duration_seconds",
            Help:    "Time spent processing outbox events",
            Buckets: prometheus.DefBuckets,
        },
        []string{"event_type"},
    )

    outboxPendingEvents = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "hermes_outbox_pending_events",
            Help: "Number of pending outbox events",
        },
    )

    outboxFailedEvents = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "hermes_outbox_failed_events",
            Help: "Number of failed outbox events",
        },
    )
)
```

**Grafana Dashboard Queries**:

```promql
# Pending events (should be < 100)
hermes_outbox_pending_events

# Processing rate
rate(hermes_outbox_events_total{status="completed"}[5m])

# Failure rate (should be < 1%)
rate(hermes_outbox_events_total{status="failed"}[5m])
  / rate(hermes_outbox_events_total[5m])

# Processing latency (p99 should be < 5s)
histogram_quantile(0.99,
  rate(hermes_outbox_processing_duration_seconds_bucket[5m]))

```

### Admin Endpoints

**Outbox Status API** (`internal/api/v2/outbox.go`):

```go
// GET /api/v2/admin/outbox/stats
func (h *OutboxHandler) GetStats(c *gin.Context) {
    var stats struct {
        Pending    int64 `json:"pending"`
        Processing int64 `json:"processing"`
        Completed  int64 `json:"completed"`
        Failed     int64 `json:"failed"`
        OldestPending time.Time `json:"oldest_pending"`
    }

    h.db.Model(&SearchOutboxEvent{}).Where("status = ?", "pending").Count(&stats.Pending)
    h.db.Model(&SearchOutboxEvent{}).Where("status = ?", "processing").Count(&stats.Processing)
    h.db.Model(&SearchOutboxEvent{}).Where("status = ?", "completed").Count(&stats.Completed)
    h.db.Model(&SearchOutboxEvent{}).Where("status = ?", "failed").Count(&stats.Failed)

    h.db.Model(&SearchOutboxEvent{}).
        Where("status = ?", "pending").
        Order("created_at ASC").
        Limit(1).
        Pluck("created_at", &stats.OldestPending)

    c.JSON(200, stats)
}

// GET /api/v2/admin/outbox/events?status=failed&limit=50
func (h *OutboxHandler) ListEvents(c *gin.Context) {
    status := c.Query("status")
    limit := c.GetInt("limit", 50)

    var events []SearchOutboxEvent
    query := h.db.Model(&SearchOutboxEvent{})
    if status != "" {
        query = query.Where("status = ?", status)
    }
    query.Order("created_at DESC").Limit(limit).Find(&events)

    c.JSON(200, events)
}

// POST /api/v2/admin/outbox/retry
func (h *OutboxHandler) RetryFailed(c *gin.Context) {
    result := h.db.Model(&SearchOutboxEvent{}).
        Where("status = ?", "failed").
        Updates(map[string]interface{}{
            "status":        "pending",
            "attempt_count": 0,
            "error_message": "",
        })

    c.JSON(200, gin.H{"retried": result.RowsAffected})
}
```

### Alerts

**Alertmanager Rules**:

```yaml
groups:
  - name: outbox
    interval: 30s
    rules:
      - alert: OutboxBacklog
        expr: hermes_outbox_pending_events > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Outbox backlog exceeds 1000 events"

      - alert: OutboxProcessingStalled
        expr: increase(hermes_outbox_events_total[5m]) == 0
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "Outbox worker not processing events"

      - alert: OutboxHighFailureRate
        expr: |
          rate(hermes_outbox_events_total{status="failed"}[5m])
          / rate(hermes_outbox_events_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Outbox failure rate exceeds 5%"

```

## Implementation Plan

### Phase 0: Inventory and Contract Lock

- Inventory all direct API-layer search writes and classify each by event type, aggregate, transaction boundary, and test coverage.
- Confirm the `search_outbox_events` schema, idempotency keys, ordering rules, retry/DLQ behavior, and replay modes in this RFC before coding.
- Add the CI guard that fails on direct API-layer search writes while allowing relay, backfill/reindex tooling, read-only search endpoints, and tests.

### Phase 1: Schema, Model, Relay

- Add `search_outbox_events` via ADR-020 core+deltas migrations under `internal/db/migrations/`.
- Implement `pkg/models/search_outbox_event.go` and enqueue helpers that require a transaction handle.
- Implement the relay claim/retry/DLQ loop and search projection executor behind `search.Provider`.
- Add unit tests for duplicate enqueue, same-aggregate ordering, stale upsert-to-delete convergence, retry backoff, visibility-timeout recovery, and DLQ transitions.

### Phase 2: V2 Handler Migration

- Migrate v2 document, draft, review-state, approval, publish, delete, and project mutation paths from direct search writes to transactional outbox enqueues.
- Remove post-response goroutine search writes from handlers; any remaining post-response work must not be required for search convergence.
- Add integration tests using real PostgreSQL and Meilisearch that stop the relay, perform create/update/publish/review-state/delete flows, restart the relay, and compare search contents with database truth.

### Phase 3: V1/Legacy Surface Migration

- Audit any remaining v1 or compatibility helpers, including functions that hide a `search.Provider` write.
- Migrate or explicitly exempt each path in the audit matrix.
- Keep reads/search queries allowed; remove only direct writes from mutation handlers.

### Phase 4: Operations and Backfill

- Expose the observability contract metrics and health fields.
- Add DLQ inspection, retry, skip, and rebuild-current runbook under `docs-internal/guides/`.
- Add backfill/reindex tooling that emits `search.backfill` events from database truth.
- Run load/restart testing and record the result as a memo.

## Success Metrics

### Reliability
| Metric | Current | Target |
|--------|---------|--------|
| Search consistency | unknown until audit fixture lands | 99.9% |
| Documents "lost" in search | known risk from dual writes | < 0.1% |
| Manual interventions/month | unknown until DLQ metrics land | 0 avoidable manual reindexes |

### Performance
| Metric | Target |
|--------|--------|
| Event processing latency after relay recovery (p99) | < 30s under the T1 load test |
| Worker throughput | >= 1,000 mutations/min sustained |
| DB overhead from pending outbox rows | bounded by retention and cleanup SLO |

### Operational
| Metric | Target |
|--------|--------|
| Oldest pending event age | visible in health/metrics; alert threshold documented |
| Oldest failed/DLQ event age | visible in health/metrics; alert threshold documented |
| Per-project lag | visible when project identity is present |

## Alternatives Considered

### 1. ❌ Synchronous Dual Writes (Current Approach)
**Pros**: Simple, immediate consistency
**Cons**: Partial failures, no retry, data loss
**Rejected**: Current problems too severe

### 2. ❌ Event Sourcing
**Pros**: Complete audit trail, time travel
**Cons**: Complex, requires full rewrite, overkill
**Rejected**: Too disruptive for existing system

### 3. ❌ Change Data Capture (CDC)
**Pros**: Automatic, no code changes
**Cons**: Requires Debezium/Kafka, complex infrastructure
**Rejected**: Too much operational overhead

### 4. ❌ Two-Phase Commit (2PC)
**Pros**: Strong consistency
**Cons**: Requires XA transactions, not supported by Algolia/Meilisearch
**Rejected**: Not feasible with search providers

### 5. ❌ Saga Pattern
**Pros**: Handles distributed transactions
**Cons**: Complex compensation logic, harder to reason about
**Rejected**: Outbox pattern is simpler and sufficient

## Risks & Mitigation

### Risk 1: Outbox Table Growth
**Problem**: Outbox table grows unbounded
**Mitigation**:
- Cleanup job deletes completed events after 7 days
- Archive old events to S3 for audit
- Partition table by month

### Risk 2: Worker Failure
**Problem**: Worker crashes, events not processed
**Mitigation**:
- Worker restarts automatically (Kubernetes/systemd)
- Multiple workers for redundancy (with locking)
- Alerting on processing stall

### Risk 3: Event Ordering
**Problem**: Events processed out of order
**Mitigation**:
- Allocate a per-aggregate `sequence` in the same transaction as the mutation.
- Process same-aggregate events in sequence order and block later same-aggregate events behind failed/DLQ rows.
- Allow unrelated aggregates to continue so one poison document does not stop the relay globally.

### Risk 4: Payload Size
**Problem**: Large documents exceed JSON field size
**Mitigation**:
- Store reference instead of full payload (future optimization)
- Compress payload with gzip
- PostgreSQL JSONB handles up to 1GB

### Risk 5: Handler Transaction Gaps
**Problem**: Existing handlers mix workspace calls, DB writes, post-response goroutines, and search writes.
**Mitigation**:
- Phase 0 inventory records the transaction owner for each mutation.
- Outbox enqueue helpers require a transaction handle.
- Tests crash or stop the relay between request success and search projection to prove the durable row exists.

## Future Enhancements

- **Multiple Workers**: Scale horizontally with leader election
- **Payload Compression**: Reduce storage for large documents
- **Webhooks**: Trigger webhooks for external systems
- **Snapshot Strategy**: Periodic full reindex from DB

## Related Documentation

- Martin Fowler: Transactional Outbox Pattern
- [ADR-009: Provider Abstraction Architecture](../adr/adr-009-provider-abstraction-architecture.md)
- [ADR-011: Meilisearch as Local Search Solution](../adr/adr-011-meilisearch-as-local-search-solution.md)
- [ADR-016: Search and Auth Refactoring](../adr/adr-016-search-and-auth-refactoring.md)
- [ADR-017: API Refactoring and Testing Strategy](../adr/adr-017-api-refactoring-and-testing-strategy.md)
- [ADR-020: Core+Deltas Migrations and Stateless Indexer](../adr/adr-020-dual-database-support-stateless-indexer.md)
- `pkg/search/readme.md` - Search provider architecture

## Resolved Questions

1. **Should we use a separate database for the outbox?**
   - No. The outbox must live in the same database as the authoritative mutation so the mutation and enqueue commit atomically.

2. **How long should we keep completed events?**
   - Keep completed events for a short operational window, default 7 days, then delete or archive. Failed and DLQ events remain until resolved or explicitly archived by an operator.

3. **Should we support event replay?**
   - Yes. Supported modes are `retry`, `rebuild-current`, and `replay-range`.

4. **How to handle schema changes in payload?**
   - Version event payloads and prefer `rebuild-current` from database truth when payload schemas change.

5. **Should we have multiple workers?**
   - The initial implementation may use one worker, but the claim/lock model must be safe for multiple workers.

## Timeline

- **Week 1**: Phase 0 inventory, CI guard, schema/model contract lock.
- **Week 2**: Relay and enqueue helper implementation.
- **Week 3**: V2 handler migration and integration tests.
- **Week 4**: V1/legacy migration and backfill tooling.
- **Week 5**: Operations runbook, metrics, load/restart testing, and stabilization.

**Total Effort**: 5 weeks baseline for one backend engineer, with operations review during Phase 4.
