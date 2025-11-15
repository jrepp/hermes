---
id: RFC-088
title: Event-Driven Document Indexer with Pipeline Rulesets
date: 2025-11-14
type: RFC
subtype: Architecture Design
status: Draft
tags: [indexer, events, pipeline, redpanda, meilisearch, embeddings, llm]
related:
  - RFC-051
  - RFC-080
  - RFC-087
---

# RFC-088: Event-Driven Document Indexer with Pipeline Rulesets

## Summary

Refactor the document indexer to use an event-driven architecture that combines the transactional outbox pattern (RFC-051/080) with Redpanda messaging (RFC-087). Document revisions trigger pipeline executions based on configurable rulesets, enabling search indexing, embedding generation, and LLM-powered summarization.

## Background

### Current State

The current indexer (`internal/indexer/indexer.go`) operates synchronously:
1. Polls Google Drive folders for updated documents every minute
2. Fetches document content from Google Drive API
3. Updates database records
4. Updates Algolia/Meilisearch search index in-line
5. No embedding generation or AI summarization
6. Tightly coupled to Google Workspace provider

### Problems

1. **Tight Coupling**: Indexer is Google Drive-specific, doesn't work with local workspace provider
2. **No Retry Logic**: Search index failures result in inconsistent state
3. **Missing Features**: No embeddings, no AI summaries, no semantic search
4. **Polling Inefficiency**: 1-minute polling loop wastes resources
5. **No Extensibility**: Hard to add new indexing steps (embeddings, summaries, validation)
6. **Synchronous Processing**: Long-running operations (LLM calls) block the indexing loop

### Existing Schema

We already have excellent foundation schemas:

**`document_revisions`** (tracks document versions across providers):
```sql
CREATE TABLE document_revisions (
    id SERIAL PRIMARY KEY,
    document_uuid UUID NOT NULL,
    document_id VARCHAR(500) NOT NULL,  -- Provider-specific ID
    provider_type VARCHAR(50) NOT NULL,
    title VARCHAR(500),
    content_hash VARCHAR(64),           -- SHA-256 for idempotency
    modified_time TIMESTAMP,
    status VARCHAR(20) DEFAULT 'active',
    project_uuid UUID,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

**`document_summaries`** (stores AI-generated summaries):
```sql
CREATE TABLE document_summaries (
    id SERIAL PRIMARY KEY,
    document_id VARCHAR(500) NOT NULL,
    document_uuid UUID,
    executive_summary TEXT NOT NULL,
    key_points JSONB,
    topics JSONB,
    tags JSONB,
    model VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    content_hash VARCHAR(64),           -- Links to specific revision
    generated_at TIMESTAMP NOT NULL
);
```

## Proposed Solution

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ API Layer / Document Operations                                 │
├─────────────────────────────────────────────────────────────────┤
│ BEGIN TRANSACTION                                                │
│   1. INSERT/UPDATE document_revisions (with content_hash)       │
│   2. INSERT document_revision_outbox (idempotent key)           │
│ COMMIT TRANSACTION (atomic!)                                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ (transactional consistency)
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ Outbox Relay Service (pkg/indexer/relay)                        │
│ - Polls outbox table every 1s                                   │
│ - Publishes pending events to Redpanda                          │
│ - Marks as published, retries on failure                        │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ (async messaging)
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ Redpanda Topic: hermes.document-revisions                       │
│ - Partitioned by document_uuid (ordered processing per doc)     │
│ - Retention: 7 days                                              │
│ - Consumer group: hermes-indexer-workers                         │
└────────────┬────────────────────────────────────────────────────┘
             │
             ├──────────────────┬──────────────────┬──────────────┐
             ▼                  ▼                  ▼              ▼
      ┌──────────┐      ┌──────────┐      ┌──────────┐   ┌──────────┐
      │Indexer   │      │Indexer   │      │Indexer   │   │Indexer   │
      │Worker 1  │      │Worker 2  │      │Worker 3  │   │Worker N  │
      └────┬─────┘      └────┬─────┘      └────┬─────┘   └────┬─────┘
           │                 │                 │              │
           │ Apply Rulesets to determine pipeline             │
           ▼                 ▼                 ▼              ▼
    ┌────────────────────────────────────────────────────────────┐
    │ Pipeline Execution (pkg/indexer/pipeline)                  │
    │                                                             │
    │ Rule Match → Execute Pipeline Steps:                       │
    │   1. Search Index Update (Meilisearch)                     │
    │   2. Embedding Generation (OpenAI/Local)                   │
    │   3. LLM Summary Generation → document_summaries           │
    │   4. Validation (schema checks, broken links)              │
    │   5. Custom plugins (extensible)                           │
    └────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

1. **Leverage Existing Schemas**: Use `document_revisions` and `document_summaries` tables
2. **Outbox for Transactional Consistency**: New `document_revision_outbox` table
3. **Content Hash for Idempotency**: Prevent duplicate processing of same content
4. **Redpanda for Scalability**: Async processing with partitioning by document UUID
5. **Pipeline/Ruleset System**: Flexible, configurable indexing behaviors
6. **Outbox Relay Service**: Separate service publishes outbox → Redpanda

## Schema Design

### Document Revision Outbox

```sql
CREATE TABLE document_revision_outbox (
    id BIGSERIAL PRIMARY KEY,

    -- Document identification
    revision_id INTEGER NOT NULL REFERENCES document_revisions(id),
    document_uuid UUID NOT NULL,
    document_id VARCHAR(500) NOT NULL,

    -- Idempotency
    idempotent_key VARCHAR(128) NOT NULL UNIQUE,  -- {document_uuid}:{content_hash}
    content_hash VARCHAR(64) NOT NULL,

    -- Event metadata
    event_type VARCHAR(50) NOT NULL,  -- 'revision.created', 'revision.updated', 'revision.deleted'
    provider_type VARCHAR(50) NOT NULL,

    -- Payload
    payload JSONB NOT NULL,  -- Full revision data + metadata

    -- Outbox state
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'published', 'failed'
    published_at TIMESTAMP,
    publish_attempts INTEGER DEFAULT 0,
    last_error TEXT,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_revision_outbox_status ON document_revision_outbox(status, created_at);
CREATE INDEX idx_revision_outbox_document ON document_revision_outbox(document_uuid);
CREATE INDEX idx_revision_outbox_revision ON document_revision_outbox(revision_id);
```

### Pipeline Execution Tracking

```sql
CREATE TABLE document_revision_pipeline_executions (
    id BIGSERIAL PRIMARY KEY,

    -- Links to revision and outbox
    revision_id INTEGER NOT NULL REFERENCES document_revisions(id),
    outbox_id BIGINT NOT NULL REFERENCES document_revision_outbox(id),

    -- Execution metadata
    ruleset_name VARCHAR(100) NOT NULL,
    pipeline_steps JSONB NOT NULL,  -- ['search_index', 'embeddings', 'llm_summary']

    -- Execution state
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'running', 'completed', 'failed'
    started_at TIMESTAMP,
    completed_at TIMESTAMP,

    -- Results per step
    step_results JSONB,  -- {"search_index": {"status": "success", ...}, ...}
    error_details JSONB,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pipeline_exec_revision ON document_revision_pipeline_executions(revision_id);
CREATE INDEX idx_pipeline_exec_status ON document_revision_pipeline_executions(status);
```

## Ruleset System

### Ruleset Configuration

Rulesets determine which pipeline steps to execute based on document attributes:

```hcl
# config.hcl
indexer {
  rulesets = [
    {
      name = "published-rfcs"

      # Conditions to match (AND logic)
      conditions = {
        provider_type = "google"
        document_type = "RFC"
        status        = "In-Review,Approved"
      }

      # Pipeline steps to execute (in order)
      pipeline = [
        "search_index",      # Update Meilisearch
        "embeddings",        # Generate embeddings for semantic search
        "llm_summary",       # Generate AI summary
      ]

      # Pipeline configuration
      config = {
        embeddings = {
          model = "text-embedding-3-small"
          dimensions = 1536
        }
        llm_summary = {
          model = "gpt-4o-mini"
          max_tokens = 500
        }
      }
    },

    {
      name = "all-documents"

      # Default ruleset (no conditions = matches all)
      conditions = {}

      pipeline = [
        "search_index",  # Always index in search
      ]
    },

    {
      name = "design-docs-deep-analysis"

      conditions = {
        document_type = "PRD,RFC"
        content_length_gt = 5000  # Only analyze long docs
      }

      pipeline = [
        "search_index",
        "embeddings",
        "llm_summary",
        "llm_validation",  # Custom step: check for completeness
      ]

      config = {
        llm_validation = {
          checks = ["has_motivation", "has_alternatives", "has_success_metrics"]
        }
      }
    }
  ]
}
```

### Ruleset Matching Algorithm

```go
// pkg/indexer/ruleset/matcher.go
type Matcher struct {
    rulesets []Ruleset
}

func (m *Matcher) Match(revision *models.DocumentRevision, metadata map[string]any) []Ruleset {
    var matched []Ruleset

    for _, ruleset := range m.rulesets {
        if ruleset.Matches(revision, metadata) {
            matched = append(matched, ruleset)
        }
    }

    return matched
}
```

## Pipeline System

### Pipeline Steps

Each step is a self-contained unit with:
- Input: `DocumentRevision` + metadata
- Output: Result + status
- Error handling: Retryable vs permanent failures

**Built-in Steps**:

1. **`search_index`**: Update Meilisearch document index
   ```go
   type SearchIndexStep struct {
       searchProvider search.Provider
   }

   func (s *SearchIndexStep) Execute(ctx context.Context, rev *DocumentRevision) error {
       doc := convertRevisionToSearchDoc(rev)
       return s.searchProvider.DocumentIndex().Index(ctx, doc)
   }
   ```

2. **`embeddings`**: Generate vector embeddings for semantic search
   ```go
   type EmbeddingsStep struct {
       embeddingService EmbeddingService
       vectorStore      VectorStore
   }

   func (s *EmbeddingsStep) Execute(ctx context.Context, rev *DocumentRevision) error {
       content := fetchDocumentContent(rev)
       embeddings := s.embeddingService.Generate(ctx, content)
       return s.vectorStore.Store(ctx, rev.DocumentUUID, embeddings)
   }
   ```

3. **`llm_summary`**: Generate AI summary and save to `document_summaries`
   ```go
   type LLMSummaryStep struct {
       llmClient LLMClient
       db        *gorm.DB
   }

   func (s *LLMSummaryStep) Execute(ctx context.Context, rev *DocumentRevision) error {
       // Check if summary exists for this content_hash
       existing, _ := models.GetSummaryByDocumentIDAndModel(s.db, rev.DocumentID, s.model)
       if existing != nil && existing.MatchesContentHash(rev.ContentHash) {
           return nil // Already have summary for this version
       }

       content := fetchDocumentContent(rev)
       summary := s.llmClient.Summarize(ctx, content)

       // Save to document_summaries table
       return (&models.DocumentSummary{
           DocumentID:       rev.DocumentID,
           DocumentUUID:     &rev.DocumentUUID,
           ExecutiveSummary: summary.Executive,
           KeyPoints:        summary.KeyPoints,
           Topics:           summary.Topics,
           ContentHash:      rev.ContentHash,
           Model:            s.model,
           Provider:         s.provider,
           GeneratedAt:      time.Now(),
       }).Create(s.db)
   }
   ```

4. **`validation`**: Validate document structure, links, metadata
   ```go
   type ValidationStep struct {
       validators []Validator
   }

   func (s *ValidationStep) Execute(ctx context.Context, rev *DocumentRevision) error {
       content := fetchDocumentContent(rev)

       for _, validator := range s.validators {
           if err := validator.Validate(ctx, rev, content); err != nil {
               // Record validation issues but don't fail pipeline
               recordValidationIssue(rev, validator.Name(), err)
           }
       }
       return nil
   }
   ```

### Pipeline Executor

```go
// pkg/indexer/pipeline/executor.go
type Executor struct {
    steps map[string]PipelineStep
    db    *gorm.DB
}

type PipelineStep interface {
    Name() string
    Execute(ctx context.Context, rev *models.DocumentRevision, config map[string]any) error
    IsRetryable(err error) bool
}

func (e *Executor) ExecutePipeline(ctx context.Context, rev *models.DocumentRevision, ruleset *Ruleset) error {
    execution := &models.DocumentRevisionPipelineExecution{
        RevisionID:    int(rev.ID),
        RulesetName:   ruleset.Name,
        PipelineSteps: ruleset.Pipeline,
        Status:        "running",
        StartedAt:     ptrTime(time.Now()),
        StepResults:   make(map[string]any),
    }

    if err := e.db.Create(execution).Error; err != nil {
        return err
    }

    // Execute each step in order
    for _, stepName := range ruleset.Pipeline {
        step, ok := e.steps[stepName]
        if !ok {
            return fmt.Errorf("unknown pipeline step: %s", stepName)
        }

        stepConfig := ruleset.Config[stepName]

        if err := step.Execute(ctx, rev, stepConfig); err != nil {
            execution.Status = "failed"
            execution.ErrorDetails = map[string]any{
                "step":  stepName,
                "error": err.Error(),
            }
            e.db.Save(execution)

            // Decide whether to continue or fail pipeline
            if !step.IsRetryable(err) {
                return err  // Permanent failure
            }
            // Continue to next step for retryable errors
        }

        execution.StepResults[stepName] = map[string]any{
            "status":       "success",
            "completed_at": time.Now(),
        }
    }

    execution.Status = "completed"
    execution.CompletedAt = ptrTime(time.Now())
    e.db.Save(execution)

    return nil
}
```

## Implementation

### Phase 1: Schema & Outbox (Week 1)

- Create `document_revision_outbox` table migration
- Create `document_revision_pipeline_executions` table migration
- Implement outbox models in `pkg/models/document_revision_outbox.go`
- Update API handlers to write to outbox on document changes

**Deliverables**:
- Database migrations
- GORM models
- Outbox write logic in API handlers

### Phase 2: Outbox Relay Service (Week 2)

- Implement outbox relay service in `pkg/indexer/relay`
- Polls outbox every 1s
- Publishes events to Redpanda topic `hermes.document-revisions`
- Marks as published, handles failures with retry

**Deliverables**:
- `pkg/indexer/relay/worker.go`
- Redpanda publisher integration
- Outbox cleanup job (delete old published events)

### Phase 3: Indexer Consumer & Ruleset System (Week 3)

- Implement indexer consumer in `pkg/indexer/consumer`
- Implement ruleset matcher in `pkg/indexer/ruleset`
- Parse ruleset configuration from HCL
- Match revisions to rulesets

**Deliverables**:
- `pkg/indexer/consumer/worker.go`
- `pkg/indexer/ruleset/matcher.go`
- Ruleset configuration parser

### Phase 4: Pipeline Steps (Week 4-5)

- Implement `search_index` step (Meilisearch)
- Implement `embeddings` step (OpenAI API)
- Implement `llm_summary` step (saves to `document_summaries`)
- Implement `validation` step

**Deliverables**:
- `pkg/indexer/pipeline/steps/search_index.go`
- `pkg/indexer/pipeline/steps/embeddings.go`
- `pkg/indexer/pipeline/steps/llm_summary.go`
- `pkg/indexer/pipeline/steps/validation.go`

### Phase 5: Integration & Testing (Week 6)

- Update docker-compose with indexer consumer
- Integration tests with Redpanda
- End-to-end tests: document change → indexed with summary
- Load testing

**Deliverables**:
- Integration tests
- E2E test suite
- Performance benchmarks

## Benefits

1. **Transactional Consistency**: Outbox pattern ensures no lost events
2. **Idempotency**: Content hash prevents duplicate processing
3. **Scalability**: Redpanda partitioning enables horizontal scaling
4. **Flexibility**: Rulesets make indexing behavior configurable
5. **Extensibility**: Pipeline steps are pluggable
6. **AI-Powered**: LLM summaries and embeddings enable semantic search
7. **Provider-Agnostic**: Works with Google, local, S3, Azure providers
8. **Observability**: Pipeline execution tracking in database

## Success Metrics

- 99.9% indexing reliability (no lost documents)
- < 5s latency from document change to search index update
- < 30s latency for LLM summary generation
- Zero duplicate summary generation (idempotency works)
- Support for 1000+ document updates/hour
- Successful migration from old indexer with zero downtime

## Migration Strategy

1. **Deploy new schema** (outbox tables) alongside old indexer
2. **Start outbox relay** publishing events (old indexer still running)
3. **Deploy indexer consumers** processing events (parallel to old indexer)
4. **Validate consistency** between old and new indexers for 1 week
5. **Switch over** API handlers to write to outbox
6. **Decommission old indexer** after validation period

## Open Questions

1. **Vector store for embeddings**: Meilisearch native support or external (Pinecone, Weaviate)?
   - **Proposal**: Start with Meilisearch built-in vector search (v1.3+), migrate to dedicated store if needed

2. **LLM provider**: OpenAI, local Ollama, or both?
   - **Proposal**: Make it configurable per ruleset, support both

3. **Outbox cleanup**: How long to keep published events?
   - **Proposal**: 7 days retention, then delete or archive to cold storage

4. **Pipeline parallelization**: Execute steps in parallel or sequential?
   - **Proposal**: Sequential initially, add DAG-based parallelization in v2

5. **Failure handling**: DLQ for permanently failed events?
   - **Proposal**: Yes, add dead letter queue after 5 retry attempts

## References

- RFC-051: Document Search Index Outbox Pattern
- RFC-080: Outbox Pattern for Document Synchronization
- RFC-087: Multi-Backend Notification System with Message Queues
- Existing schema: `document_revisions`, `document_summaries`
- Meilisearch Vector Search: https://www.meilisearch.com/docs/learn/experimental/vector-search

---

**Document ID**: RFC-088
**Status**: Draft
**Last Updated**: 2025-11-14
