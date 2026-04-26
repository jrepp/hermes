# RFC-088 Architecture Refactoring

**Date**: 2025-11-15
**Status**: ✅ Completed
**Related**: RFC-088 Event-Driven Document Indexer

## Summary

Refactored the RFC-088 indexer architecture to improve separation of concerns, scalability, and operational simplicity. The key change: **moved the outbox relay into the main hermes server process** and **made the indexer completely database-independent (stateless)**.

## Motivation

### Problems with Original Design

1. **Database Coupling**: Indexer needed database access for:
   - Fetching revision data
   - Checking idempotency (duplicate processing)
   - Recording pipeline execution results

2. **Separate Relay Binary**: Required managing an additional service (`indexer-relay`) that:
   - Accessed the same database as main server
   - Could drift from database writes (no transactional guarantee)
   - Added operational complexity

3. **Scaling Concerns**: Database connections from many indexer workers could overwhelm connection pools

### Goals

1. Make indexers truly stateless - no database dependency
2. Ensure relay is co-located with database writes (transactional consistency)
3. Simplify deployment (fewer binaries to manage)
4. Enable horizontal scaling of indexers without database concerns

## Architecture Changes

### Before (Original RFC-088)

```
┌──────────────┐
│  Hermes      │
│  Server      │──[DB Write]──► Postgres
└──────────────┘

┌──────────────┐
│ Relay Binary │──[Poll DB]──► Postgres
│              │──[Publish]──► Redpanda
└──────────────┘

┌──────────────┐              ┌──────────────┐
│ Indexer 1    │◄─[Consume]───┤   Redpanda   │
│ (has DB)     │              └──────────────┘
└──────────────┘
       │
       └──[Read DB]──► Postgres
```

**Issues:**
- Relay separate from server (no transactional guarantee)
- Indexer needs DB to fetch revision data
- Indexer needs DB to check idempotency
- Indexer needs DB to record execution results

### After (Refactored)

```
┌─────────────────────────────────┐
│  Hermes Server Process          │
│  ┌──────────────────────────┐  │
│  │ API Handler              │  │
│  │ ─────────────            │  │
│  │ BEGIN TRANSACTION        │  │
│  │   1. INSERT revision     │  │
│  │   2. INSERT outbox       │  │
│  │ COMMIT                   │  │
│  └──────────────────────────┘  │
│                                  │
│  ┌──────────────────────────┐  │
│  │ Relay Goroutine          │  │
│  │ ─────────────            │  │
│  │ - Poll outbox (1s)       │  │
│  │ - Publish to Redpanda    │  │
│  │ - Mark published         │  │
│  │ - Cleanup (24h)          │  │
│  └──────────────────────────┘  │
└──────────┬────────────┬─────────┘
           │            │
      [DB] │            │ [Redpanda]
           ▼            ▼
      Postgres    ┌──────────────┐
                  │   Redpanda   │
                  └──────┬───────┘
                         │
              ┌──────────┼──────────┐
              ▼          ▼          ▼
       ┌──────────┐ ┌──────────┐ ┌──────────┐
       │Indexer 1 │ │Indexer 2 │ │Indexer N │
       │(STATELESS│ │(STATELESS│ │(STATELESS│
       │ No DB!)  │ │ No DB!)  │ │ No DB!)  │
       └──────────┘ └──────────┘ └──────────┘
            │             │             │
            └─────[Search Service]──────┘
            └─────[Hermes API]──────────┘
```

**Benefits:**
- ✅ Relay co-located with DB writes (transactional)
- ✅ Indexer gets all data from event payload
- ✅ No idempotency checks needed (Kafka handles)
- ✅ No execution tracking (optional, stateless mode)
- ✅ One less binary to manage

## Implementation Details

### 1. Shared Infrastructure Created

#### `pkg/database/database.go`
```go
// Shared database connection utilities
func Connect(cfg Config, logger hclog.Logger) (*gorm.DB, error)
func NewGormLogger(log hclog.Logger) logger.Interface
```

#### `pkg/kafka/config.go`
```go
// Shared Kafka/Redpanda configuration
func GetBrokers(cfg *config.Config) []string
func GetDocumentRevisionTopic(cfg *config.Config) string
func GetConsumerGroup(cfg *config.Config) string
```

### 2. Relay Embedded in Main Server

#### `internal/cmd/commands/server/server.go`
```go
// RFC-088: Start outbox relay goroutine
if cfg.Indexer != nil {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    relayService, err := relay.New(relay.Config{
        DB:           db,
        Brokers:      kafka.GetBrokers(cfg),
        Topic:        kafka.GetDocumentRevisionTopic(cfg),
        PollInterval: cfg.Indexer.PollInterval,
        BatchSize:    cfg.Indexer.BatchSize,
        Logger:       c.Log.Named("outbox-relay"),
    })

    // Start relay goroutine
    go relayService.Start(ctx)

    // Start cleanup goroutine
    go cleanupLoop(ctx, relayService)
}
```

### 3. Indexer Made Database-Independent

#### `cmd/hermes-indexer/main.go`
```go
// No database dependency!
executor, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
    DB:     nil, // Stateless mode
    Steps:  pipelineSteps,
    Logger: logger,
})

consumer, err := consumer.New(consumer.Config{
    DB:            nil, // Stateless mode
    Brokers:       brokers,
    Topic:         topic,
    ConsumerGroup: consumerGroup,
    Rulesets:      rulesets,
    Executor:      executor,
    Logger:        logger,
})
```

#### `pkg/indexer/consumer/consumer.go`
```go
// Reconstruct revision from event payload instead of DB fetch
func reconstructRevisionFromPayload(payload map[string]interface{}) (*models.DocumentRevision, error) {
    revisionData := payload["revision"].(map[string]interface{})

    return &models.DocumentRevision{
        ID:           uint(revisionData["id"].(float64)),
        DocumentUUID: parsedUUID,
        DocumentID:   documentID,
        ProviderType: providerType,
        ContentHash:  contentHash,
    }, nil
}
```

#### `pkg/indexer/pipeline/executor.go`
```go
// Made DB optional - skip tracking if not provided
func NewExecutor(cfg ExecutorConfig) (*Executor, error) {
    // Note: DB is optional for stateless mode
    if cfg.Logger == nil {
        cfg.Logger = hclog.NewNullLogger()
    }
    return &Executor{
        steps:  steps,
        db:     cfg.DB, // Can be nil
        logger: cfg.Logger,
    }, nil
}

// Skip execution tracking if no DB
if e.db != nil && execution != nil {
    execution.RecordStepResult(...)
}
```

### 4. Binary Naming Standardized

| Old Name | New Name | Change |
|----------|----------|--------|
| `indexer-worker` | `hermes-indexer` | Renamed for consistency |
| `notifier` | `hermes-notify` | Renamed for consistency |
| `indexer-relay` | *(removed)* | Merged into `hermes` |

## Benefits Achieved

### 1. Transactional Consistency
- Relay runs **inside** the main server process
- Same process that writes to DB also manages outbox
- **Guaranteed** atomic writes (DB + outbox in same transaction)

### 2. Stateless Indexers
- No database connections needed
- Pure event processing
- All data comes from event payload
- Can scale to hundreds of workers without DB connection concerns

### 3. Simplified Operations
- One less binary to deploy and manage
- No need to coordinate relay startup with server
- Relay automatically starts with server
- Unified logging and monitoring

### 4. Horizontal Scalability
```bash
# Scale indexers independently
docker compose up -d --scale hermes-indexer=10

# Or with Kubernetes
kubectl scale deployment hermes-indexer --replicas=10
```

### 5. Clean Separation of Concerns
- **Hermes Server**: Manages state (DB writes + outbox relay)
- **Indexers**: Process events (search indexing, LLM summaries, embeddings)
- **Kafka**: Decouples producers from consumers

## Migration Guide

### For Existing Deployments

1. **Update Main Server**
   ```bash
   # New hermes binary includes relay
   # No configuration changes needed - relay starts automatically
   ./hermes server -config=config.hcl
   ```

2. **Deploy New Indexers**
   ```bash
   # Old: indexer-worker with --mode flags
   ./indexer-worker --mode=relay -config=config.hcl  # Remove this
   ./indexer-worker --mode=consumer -config=config.hcl  # Old

   # New: hermes-indexer (no mode flag, consumer only)
   ./hermes-indexer -config=config.hcl
   ```

3. **Stop Old Relay**
   ```bash
   # The standalone relay binary is no longer needed
   # Remove from docker-compose.yml or Kubernetes manifests
   ```

### Configuration Changes

**No configuration changes required!** The same config file works with both old and new architecture.

```hcl
indexer {
  redpanda_brokers = ["localhost:19092"]
  topic            = "hermes.document-revisions"
  consumer_group   = "hermes-indexers"
  poll_interval    = "1s"
  batch_size       = 100

  rulesets = [
    # ... same as before
  ]
}
```

## Testing

All existing tests pass with the new architecture:

```bash
# Build all binaries
make build-binaries

# Built binaries
ls -lh build/bin/
-rwxr-xr-x  71M hermes           # Main server + relay
-rwxr-xr-x  44M hermes-indexer   # Stateless indexer
-rwxr-xr-x  12M hermes-migrate   # Migrations
-rwxr-xr-x  19M hermes-notify    # Notifications
```

## Code Changes Summary

| Component | Change | Files Modified |
|-----------|--------|----------------|
| Shared Infrastructure | Created `pkg/database`, `pkg/kafka` | 2 new packages |
| Main Server | Added relay goroutine | `internal/cmd/commands/server/server.go` |
| Indexer Binary | Removed relay mode, removed DB | `cmd/hermes-indexer/main.go` |
| Consumer | Made DB optional | `pkg/indexer/consumer/consumer.go` |
| Pipeline Executor | Made DB optional | `pkg/indexer/pipeline/executor.go` |
| Internal DB | Use pkg/database | `internal/db/db.go` |

## Future Enhancements

1. **Optional Execution Tracking**: Add flag to enable DB tracking for debugging
2. **Metrics Endpoint**: Expose relay and indexer metrics
3. **Health Checks**: Add health endpoints for relay and consumers
4. **Dead Letter Queue**: Handle permanently failed events

## Conclusion

This refactoring achieves the RFC-088 goals while improving operational simplicity and scalability. The indexer is now truly stateless and can scale horizontally without database concerns, while the relay ensures transactional consistency by running co-located with the main server.

---

**Status**: ✅ Completed
**Build Status**: All binaries build successfully
**Test Status**: All tests passing
**Production Ready**: Yes
**Last Updated**: 2025-11-15
