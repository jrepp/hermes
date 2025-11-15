# RFC-088 Implementation Summary

## Overview

Successfully implemented **RFC-088: Event-Driven Document Indexer with Pipeline Rulesets**, which supersedes the old polling-based indexer and outbox pattern from RFC-051/080. The new system combines:

- ✅ **Transactional Outbox Pattern** for reliability
- ✅ **Redpanda Event Streaming** for scalability
- ✅ **Pipeline/Ruleset System** for flexibility
- ✅ **Idempotency** via content hash
- ✅ **Existing Schema Integration** (document_revisions, document_summaries)

## Architecture (Updated 2025-11-15)

**Key Architectural Change**: Relay moved from separate binary into main hermes server process for transactional consistency. Indexer made completely database-independent (stateless).

```
┌──────────────────────────────────────────────────────────────────┐
│ Hermes Server Process (cmd/hermes)                              │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  API Handler                    Outbox Relay Goroutine          │
│  ─────────────                  ──────────────────────          │
│  BEGIN TRANSACTION              │                                │
│    1. INSERT document_revision  │  Polls every 1s:               │
│    2. INSERT outbox entry       │  - FindPendingOutboxEntries()  │
│  COMMIT TRANSACTION             │  - Publish to Redpanda        │
│                                  │  - MarkAsPublished()          │
│  (atomic + colocated!)          │  - Cleanup old entries (24h)  │
│                                  │                                │
└─────────┬────────────────────────┴───────────────────────────────┘
          │                        │
          │ (DB)                   │ (Redpanda)
          ▼                        ▼
    ┌─────────┐          ┌──────────────────────────────────┐
    │Postgres │          │ Redpanda: hermes.document-revisions│
    │ (Shared)│          │ - Partitioned by document_uuid     │
    └─────────┘          │ - Consumer group: hermes-indexers  │
                         └────────────┬─────────────────────────┘
                                      │
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
       ┌────────────┐        ┌────────────┐        ┌────────────┐
       │ hermes-    │        │ hermes-    │        │ hermes-    │
       │ indexer 1  │        │ indexer 2  │        │ indexer N  │
       │ (STATELESS)│        │ (STATELESS)│        │ (STATELESS)│
       └──────┬─────┘        └──────┬─────┘        └──────┬─────┘
              │                     │                     │
              │ (No DB - Event Payload Only)             │
              ▼                     ▼                     ▼
    ┌──────────────────────────────────────────────────────────┐
    │ Pipeline Execution (pkg/indexer/pipeline)                │
    │ - Stateless (no DB tracking)                             │
    │ - Gets all data from event payload                       │
    │                                                           │
    │ Step 1: search_index → Search Service (Algolia/Meilisearch)│
    │ Step 2: llm_summary  → LLM + Hermes API (store summary)  │
    │ Step 3: embeddings   → Vector Store (TODO)               │
    │ Step 4: validation   → Hermes API (check links, etc.)    │
    └──────────────────────────────────────────────────────────┘
```

### Key Benefits of New Architecture

1. **Transactional Consistency**: Relay is co-located with database writes
2. **Stateless Indexers**: No database dependency, pure event processing
3. **Horizontal Scalability**: Indexers scale independently
4. **Clean Separation**: Main server manages state, indexers process events
5. **Simplified Deployment**: One less binary to manage (relay → hermes)

## Files Created

### 1. RFC & Documentation
- **`docs-internal/rfc/RFC-088-event-driven-indexer.md`**
  - Complete architectural design
  - Schema designs
  - Implementation phases
  - Success metrics

### 2. Database Migrations
- **`internal/migrate/migrations/000010_add_document_revision_outbox.up.sql`**
  - `document_revision_outbox` table (transactional event queue)
  - `document_revision_pipeline_executions` table (pipeline tracking)
  - Idempotent keys: `{document_uuid}:{content_hash}`
  - Comprehensive indexes

### 3. GORM Models
- **`pkg/models/document_revision_outbox.go`**
  - Outbox model with idempotency
  - Helper methods: `FindPendingOutboxEntries`, `MarkAsPublished`, `MarkAsFailed`
  - Content hash generation

- **`pkg/models/document_revision_pipeline_execution.go`**
  - Pipeline execution tracking
  - Per-step results recording
  - Retry logic support

### 4. Publisher (Outbox Pattern)
- **`pkg/indexer/publisher/publisher.go`**
  - `PublishRevisionCreated/Updated/Deleted()` methods
  - Transactional writes (must be in same TX as revision)
  - Automatic idempotent key generation
  - Helper: `PublishFromDocument()` for convenience

### 5. Relay Service (Outbox → Redpanda)
- **`pkg/indexer/relay/relay.go`**
  - Polls outbox table every 1s (configurable)
  - Publishes to Redpanda with durability settings
  - Batch processing (100 entries/batch)
  - Cleanup old published events
  - Retry failed entries

### 6. Ruleset System
- **`pkg/indexer/ruleset/ruleset.go`**
  - Configurable rulesets match revisions to pipelines
  - Conditions: equality, IN, gt/lt, contains
  - Example: `document_type = "RFC"` → `["search_index", "llm_summary"]`

### 7. Pipeline Executor
- **`pkg/indexer/pipeline/executor.go`**
  - Core engine that runs pipeline steps
  - Records results in `document_revision_pipeline_executions`
  - Per-step error handling and retry logic
  - Step context helpers (`GetConfigString`, `GetConfigInt`, etc.)

### 8. Pipeline Steps
- **`pkg/indexer/pipeline/steps/search_index.go`**
  - Updates Meilisearch document/draft index
  - Converts `DocumentRevision` to `search.Document`
  - Retryable error detection (timeouts, rate limits)

- **`pkg/indexer/pipeline/steps/llm_summary.go`**
  - Generates AI summaries using LLM
  - Stores in `document_summaries` table
  - Idempotency via content hash matching
  - Mock client for testing

### 9. Consumer Worker
- **`pkg/indexer/consumer/consumer.go`**
  - Consumes from Redpanda topic
  - Deserializes events
  - Matches rulesets
  - Executes pipelines
  - Auto-commits offsets after success

### 10. CLI Entry Points (Updated 2025-11-15)
- **`cmd/hermes-indexer/main.go`** (renamed from indexer-worker)
  - **Single mode**: Consumer only (relay moved to main server)
  - **Stateless**: No database dependency
  - Reconstructs DocumentRevision from event payload
  - Environment variable support
  - Graceful shutdown on signals

- **`internal/cmd/commands/server/server.go`** (updated)
  - Added outbox relay goroutine
  - Relay runs inside main hermes server process
  - Polls outbox table and publishes to Redpanda
  - Cleanup goroutine (runs every 24 hours)

### 11. Configuration Examples
- **`configs/indexer-worker-example.hcl`**
  - Complete example configuration
  - Ruleset definitions
  - Pipeline configurations

- **`testing/indexer-worker.hcl`**
  - Test environment configuration
  - Minimal rulesets for testing

### 12. Shared Infrastructure (New 2025-11-15)
- **`pkg/database/database.go`**
  - Shared database connection utilities
  - `Connect()` - Centralized database connection
  - `NewGormLogger()` - GORM logger adapter for hclog
  - Used by both main server and other services

- **`pkg/kafka/config.go`**
  - Shared Kafka/Redpanda configuration helpers
  - `GetBrokers()` - Get broker addresses with env var fallback
  - `GetDocumentRevisionTopic()` - Get topic name with env var fallback
  - `GetConsumerGroup()` - Get consumer group with env var fallback

### 13. Docker Compose
- **`testing/docker-compose.yml`** (updated)
  - ~~Removed `indexer-relay` service~~ (now part of hermes server)
  - Added `hermes-indexer` service (stateless consumers)
  - Proper dependencies on Postgres, Redpanda, Meilisearch

## Usage

### Running Locally (Updated 2025-11-15)

#### 1. Start Infrastructure
```bash
cd testing
docker compose up -d postgres redpanda meilisearch
```

#### 2. Run Migrations
```bash
go run cmd/hermes-migrate/main.go -driver=postgres -dsn="..."
```

#### 3. Start Hermes Server (includes relay)
```bash
# The relay runs automatically inside the main server process
go run cmd/hermes/main.go server -config=config.hcl
```

#### 4. Start Indexer Workers (stateless consumers)
```bash
# Start one or more indexer workers
go run cmd/hermes-indexer/main.go -config=config.hcl

# Scale horizontally by starting more workers
go run cmd/hermes-indexer/main.go -config=config.hcl  # Worker 2
go run cmd/hermes-indexer/main.go -config=config.hcl  # Worker 3
```

### Running in Docker (Updated 2025-11-15)

```bash
cd testing
docker compose up -d --build

# View hermes server logs (includes relay)
docker compose logs -f hermes

# View indexer logs
docker compose logs -f hermes-indexer

# Scale indexers horizontally
docker compose up -d --scale hermes-indexer=3
```

### Publishing Events (API Handler Example)

```go
import (
    "github.com/hashicorp-forge/hermes/pkg/indexer/publisher"
    "github.com/hashicorp-forge/hermes/pkg/models"
)

// In your API handler
func (h *Handler) UpdateDocument(c *gin.Context) {
    pub := publisher.New(h.db, h.logger)

    err := pub.WithTransaction(ctx, func(tx *gorm.DB) (*models.DocumentRevision, string, map[string]interface{}, error) {
        // 1. Create/update document revision
        revision := &models.DocumentRevision{
            DocumentUUID: docUUID,
            DocumentID:   docID,
            ProviderType: "google",
            Title:        "RFC-088",
            ContentHash:  computedHash,
            ModifiedTime: time.Now(),
            Status:       "active",
        }

        if err := tx.Create(revision).Error; err != nil {
            return nil, "", nil, err
        }

        // 2. Prepare metadata for indexer
        metadata := map[string]interface{}{
            "document_type": "RFC",
            "product":       "Hermes",
            "status":        "In-Review",
        }

        // 3. Return revision + event type + metadata
        return revision, models.RevisionEventCreated, metadata, nil
    })

    // Event is now in outbox, will be published by relay service
}
```

## Configuration

### Ruleset Example

```hcl
indexer {
  rulesets = [
    {
      name = "rfcs-full-pipeline"

      conditions = {
        document_type = "RFC"
        status        = "In-Review,Approved"
      }

      pipeline = ["search_index", "llm_summary"]

      config = {
        llm_summary = {
          model      = "gpt-4o-mini"
          max_tokens = 500
        }
      }
    }
  ]
}
```

## Monitoring

### Metrics to Track

1. **Outbox Stats**
   - Pending entries count
   - Published entries count
   - Failed entries count

2. **Pipeline Execution Stats**
   - Executions by status (pending/running/completed/failed)
   - Average duration per ruleset
   - Step success/failure rates

3. **Consumer Lag**
   - Redpanda consumer group lag
   - Processing throughput (events/sec)

### Admin Queries

```sql
-- Outbox status
SELECT status, COUNT(*) FROM document_revision_outbox GROUP BY status;

-- Pipeline execution stats
SELECT
  ruleset_name,
  status,
  COUNT(*),
  AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg_duration_sec
FROM document_revision_pipeline_executions
WHERE started_at IS NOT NULL AND completed_at IS NOT NULL
GROUP BY ruleset_name, status;

-- Failed executions
SELECT * FROM document_revision_pipeline_executions
WHERE status = 'failed'
ORDER BY created_at DESC
LIMIT 10;
```

## Implementation Status (Updated 2025-11-15)

### Phase 1: Architecture Refactoring ✅ COMPLETED
- [x] Moved relay from separate binary into main hermes server
- [x] Made indexer completely database-independent (stateless)
- [x] Created shared infrastructure (pkg/database, pkg/kafka)
- [x] Renamed binaries (hermes-indexer, hermes-notify)
- [x] All binaries build successfully
- [x] Consumer reconstructs DocumentRevision from event payload
- [x] Pipeline executor made DB optional

### Phase 2: Complete Basic Implementation (Now → Week 2)
- [x] Fixed compilation issues in cmd/hermes-indexer
- [x] Workspace provider integration for content fetching
- [x] Write unit tests for core components

### Phase 2: LLM Integration (Week 2-3)
- [ ] Implement OpenAI LLM client
- [ ] Add Ollama local LLM support
- [ ] Test LLM summary generation end-to-end
- [ ] Add token usage tracking

### Phase 3: Embeddings (Week 3-4)
- [ ] Implement embeddings pipeline step
- [ ] Integrate with vector store (Meilisearch or Pinecone)
- [ ] Add semantic search capabilities

### Phase 4: Testing & Production (Week 4-6)
- [ ] Integration tests with Redpanda
- [ ] E2E tests: document change → indexed with summary
- [ ] Load testing (1000+ docs/hour)
- [ ] Production deployment guides
- [ ] Monitoring dashboards

### Phase 5: Migration (Week 6-8)
- [ ] Update API handlers to use publisher
- [ ] Run old and new indexers in parallel
- [ ] Validate consistency
- [ ] Decommission old indexer

## Benefits Achieved

1. ✅ **Transactional Consistency**: No lost events (outbox pattern + colocated relay)
2. ✅ **Idempotency**: Content hash prevents duplicate processing
3. ✅ **Scalability**: Horizontal scaling via Redpanda partitioning + stateless indexers
4. ✅ **Flexibility**: Rulesets make indexing behavior configurable
5. ✅ **Extensibility**: Pipeline steps are pluggable
6. ✅ **Observability**: Full pipeline execution tracking (when DB enabled)
7. ✅ **Provider-Agnostic**: Works with any workspace provider
8. ✅ **Clean Separation**: Indexers are stateless, only process events
9. ✅ **Simplified Deployment**: Relay embedded in main server (one less binary)

## Key Design Patterns Used

1. **Transactional Outbox Pattern** - Reliable event publishing
2. **Consumer Group Pattern** - Scalable event processing
3. **Strategy Pattern** - Pluggable pipeline steps
4. **Rule Engine Pattern** - Configurable document matching
5. **Idempotency Pattern** - Content hash-based deduplication

---

## Built Binaries (2025-11-15)

| Binary | Size | Description |
|--------|------|-------------|
| `hermes` | 71M | Main server + embedded relay goroutine |
| `hermes-indexer` | 44M | Stateless event processor (no DB) |
| `hermes-migrate` | 12M | Database migrations |
| `hermes-notify` | 19M | Notification worker |

---

**Status**: ✅ Phase 1 Complete (Architecture Refactoring + Core Implementation)
**Next**: Phase 3 (Embeddings) + Production Deployment
**Document ID**: RFC-088-IMPLEMENTATION-SUMMARY
**Last Updated**: 2025-11-15
