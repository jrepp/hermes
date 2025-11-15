---
id: RFC-089
title: S3-Compatible Storage Backend and Document Migration System
date: 2025-11-15
type: RFC
subtype: Architecture Design
status: Draft
tags: [storage, s3, migration, multi-backend, archival, provider]
related:
  - RFC-084
  - RFC-080
  - RFC-051
  - RFC-088
---

# RFC-089: S3-Compatible Storage Backend and Document Migration System

## Executive Summary

This RFC proposes adding S3-compatible object storage as a first-class storage backend for Hermes, enabling primary document storage, archival, and provider migration capabilities. The design leverages the existing multi-backend document model (RFC-084), outbox pattern (RFC-080/051), and extends it with a migration orchestration system that can move entire document stores between providers (Google Docs → S3 → Office365 → Markdown) while maintaining document identity, revision history, and metadata.

**Key Capabilities**:
- S3-compatible storage backend for primary document storage and archival
- Scheduled migration system to move documents between any providers
- Multi-writable storage support (documents can exist in multiple active backends simultaneously)
- Migration tracking with progress monitoring and rollback capabilities
- Admin interface for migration management and status monitoring

## Context

### Current State

Hermes currently supports two primary workspace providers:
1. **Google Workspace** - Production-ready, full-featured, direct API integration
2. **Local Filesystem** - Markdown files in Git repositories for edge deployments

**Limitations**:
- No cloud-native object storage option (S3/MinIO/Azure Blob)
- No way to migrate documents between providers
- No archival strategy for deprecated documents
- Cannot maintain documents in multiple writable stores simultaneously
- No admin interface for provider or migration management

### Existing Foundation (RFC-084)

Hermes already has excellent multi-backend support via RFC-084:

**UUID-Based Document Model**:
```go
type Document struct {
    UUID       docid.UUID  // Stable global identifier
    ProviderID string      // Backend-specific: "s3:bucket/path/doc.md"
}

type DocumentRevision struct {
    UUID            docid.UUID
    ProviderType    string  // "google", "local", "s3", "azure", "office365"
    ProviderID      string
    ContentHash     string  // SHA-256 for drift detection
    BackendRevision string  // S3 version ID, Git commit, Google rev number
    SyncStatus      string  // "canonical", "mirror", "conflict", "archived"
}
```

**Multi-Backend Document Example** (from RFC-084):
```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
Title: "RFC-001: API Gateway Design"

Revision 1: Google Workspace (canonical)
  ProviderID: google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs
  BackendRevision: 123
  ContentHash: sha256:abc123...

Revision 2: S3 (mirror for archival)
  ProviderID: s3:hermes-docs/rfcs/rfc-001.md
  BackendRevision: v1.0.4  (S3 version ID)
  ContentHash: sha256:abc123...  ✅ matches Google

Revision 3: Local Git (migration in progress)
  ProviderID: local:docs/rfc-001.md
  BackendRevision: a1b2c3d4e5f67890abcdef12
  ContentHash: sha256:def456...  ⚠️ drift detected
```

### Problem Statement

**P1: No Cloud-Native Storage Option**
- Organizations want cloud-native document storage independent of Google Workspace
- Need S3-compatible storage for cost optimization and vendor independence
- Archival requirements for compliance (7-year document retention)

**P2: No Migration Capabilities**
- Cannot migrate from Google Docs to Markdown repositories
- Cannot switch between providers without losing document history
- No way to test new providers before full cutover

**P3: No Multi-Writable Support**
- Documents can only have one canonical source
- Cannot keep documents in multiple active stores for resilience
- No gradual migration strategy (must be all-or-nothing)

**P4: No Migration Orchestration**
- No way to schedule bulk migrations
- No progress tracking or failure recovery
- No validation of migration completeness

**P5: No Admin Interface**
- No visibility into provider status or document distribution
- Cannot manage migrations through UI
- No monitoring of multi-backend sync status

## Proposed Solution

### Architecture Overview

**Event-Driven Architecture using Kafka/Redpanda + Outbox Pattern (RFC-080, RFC-087)**

```
┌─────────────────────────────────────────────────────────────────────┐
│ Hermes Core                                                           │
│                                                                       │
│ ┌─────────────────────────────────────────────────────────────────┐ │
│ │ Workspace Provider Interface (RFC-084)                           │ │
│ │ - DocumentProvider, ContentProvider, RevisionTrackingProvider    │ │
│ └─────────────────────────────────────────────────────────────────┘ │
│                                                                       │
│ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐ │
│ │ Google       │ │ Local (Git)  │ │ S3 Backend   │ │ Office365   │ │
│ │ Provider     │ │ Provider     │ │ (NEW)        │ │ Provider    │ │
│ └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘ │
│         │                │                 │                │         │
└─────────┼────────────────┼─────────────────┼────────────────┼─────────┘
          │                │                 │                │
          ▼                ▼                 ▼                ▼
   ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
   │ Google   │     │ Git Repo │     │ S3 Bucket│     │ Office365│
   │ Drive    │     │ Markdown │     │ Markdown │     │ OneDrive │
   └──────────┘     └──────────┘     └──────────┘     └──────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ Migration Orchestrator (Event-Driven)                                │
│                                                                       │
│ ┌────────────────────────────────────────────────────────────────┐  │
│ │ Migration Manager (pkg/migration/manager.go)                    │  │
│ │                                                                  │  │
│ │ Admin API → Create Migration Job                                │  │
│ │   1. Query source provider for documents                        │  │
│ │   2. BEGIN TRANSACTION                                           │  │
│ │      - INSERT migration_jobs                                     │  │
│ │      - INSERT migration_items (one per document)                 │  │
│ │      - INSERT migration_outbox (pending events)                  │  │
│ │   3. COMMIT TRANSACTION (atomic!)                                │  │
│ └────────────────────────────────────────────────────────────────┘  │
│                            │                                         │
│                            │ (transactional consistency)             │
│                            ▼                                         │
│ ┌────────────────────────────────────────────────────────────────┐  │
│ │ Outbox Relay (pkg/migration/relay)                              │  │
│ │ - Polls migration_outbox every 1s                                │  │
│ │ - Publishes pending events to Redpanda                           │  │
│ │ - Marks as published, retries on failure                         │  │
│ └────────────────────────────────────────────────────────────────┘  │
└─────────────────────────┬───────────────────────────────────────────┘
                          │
                          │ (async messaging)
                          ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Redpanda Topic: hermes.migrations                                    │
│ - Partitioned by document_uuid (ordered per document)                │
│ - Retention: 7 days                                                  │
│ - Consumer group: hermes-migration-workers                           │
└────────────┬────────────────────────────────────────────────────────┘
             │
             ├──────────────────┬──────────────────┬──────────────────┐
             ▼                  ▼                  ▼                  ▼
      ┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
      │Migration │      │Migration │      │Migration │      │Migration │
      │Worker 1  │      │Worker 2  │      │Worker 3  │      │Worker N  │
      └────┬─────┘      └────┬─────┘      └────┬─────┘      └────┬─────┘
           │                 │                 │                  │
           │ Process migration task (consume from Redpanda)       │
           ▼                 ▼                 ▼                  ▼
    ┌────────────────────────────────────────────────────────────────┐
    │ Migration Task Execution (pkg/migration/worker)                │
    │                                                                 │
    │ 1. Fetch document from source provider                         │
    │ 2. Transform content (if needed)                               │
    │ 3. Create/update document in destination provider              │
    │ 4. Verify content hash matches                                 │
    │ 5. Create document_revision entry                              │
    │ 6. Emit revision event → document_revision_outbox (RFC-088)    │
    │ 7. Update migration_items status                               │
    │                                                                 │
    │ On Failure:                                                    │
    │ - Increment retry count                                        │
    │ - Exponential backoff                                          │
    │ - Send to DLQ after max retries                                │
    └────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ Database Schema (extends existing)                                   │
│                                                                       │
│ ┌──────────────────────┐ ┌──────────────────────┐                   │
│ │ provider_storage     │ │ migration_jobs       │                   │
│ │ - Provider configs   │ │ - Source/dest        │                   │
│ │ - Capabilities       │ │ - Progress tracking  │                   │
│ │ - Status             │ │ - Scheduling         │                   │
│ └──────────────────────┘ └──────────────────────┘                   │
│                                                                       │
│ ┌──────────────────────┐ ┌──────────────────────┐                   │
│ │ migration_items      │ │ migration_outbox     │                   │
│ │ - Per-document state │ │ (NEW - RFC-080)      │                   │
│ │ - Retry count        │ │ - Pending events     │                   │
│ │ - Validation results │ │ - Publish tracking   │                   │
│ └──────────────────────┘ └──────────────────────┘                   │
│                                                                       │
│ ┌──────────────────────┐                                             │
│ │ document_revisions   │ Existing table - already supports          │
│ │ (existing)           │ multi-backend tracking!                    │
│ │ - Multi-backend      │                                             │
│ │ - content_hash       │                                             │
│ └──────────────────────┘                                             │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ Admin Interface (NEW)                                                │
│                                                                       │
│ ┌────────────────────────────────────────────────────────────────┐  │
│ │ Provider Management                                             │  │
│ │ - View all configured providers                                 │  │
│ │ - Test provider connections                                     │  │
│ │ - View document distribution                                    │  │
│ └────────────────────────────────────────────────────────────────┘  │
│                                                                       │
│ ┌────────────────────────────────────────────────────────────────┐  │
│ │ Migration Dashboard                                             │  │
│ │ - Create/schedule migration jobs                                │  │
│ │ - Monitor progress (real-time via Kafka lag monitoring)         │  │
│ │ - View migration history                                        │  │
│ │ - Pause/resume/cancel migrations                                │  │
│ │ - Rollback completed migrations                                 │  │
│ │ - Retry failed items from DLQ                                   │  │
│ └────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## Component Design

### 1. S3 Storage Backend

#### S3 Provider Implementation

```go
// pkg/workspace/adapters/s3/adapter.go
type S3Adapter struct {
    client     *s3.Client
    bucket     string
    prefix     string
    versioningEnabled bool

    // Metadata store (S3 object tags or separate manifest)
    metadataStore MetadataStore

    // Optional: DynamoDB for fast metadata queries
    dynamoDB *dynamodb.Client

    logger hclog.Logger
}

// Implements workspace.DocumentProvider
func (a *S3Adapter) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
    // providerID format: "s3:bucket/path/doc.md"
    objectKey := a.parseProviderID(providerID)

    // Get object metadata from S3 tags or DynamoDB
    metadata, err := a.metadataStore.Get(ctx, objectKey)
    if err != nil {
        return nil, err
    }

    return metadata, nil
}

func (a *S3Adapter) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
    uuid := docid.NewUUID()
    objectKey := a.buildObjectKey(destFolderID, name)
    providerID := fmt.Sprintf("s3:%s/%s", a.bucket, objectKey)

    // Fetch template content if specified
    content := ""
    if templateID != "" {
        template, err := a.GetDocument(ctx, templateID)
        if err != nil {
            return nil, fmt.Errorf("failed to get template: %w", err)
        }
        content = template.Content
    }

    // Write to S3 with versioning
    _, err := a.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: aws.String(a.bucket),
        Key:    aws.String(objectKey),
        Body:   strings.NewReader(content),
        Metadata: map[string]string{
            "hermes-uuid": uuid.String(),
            "hermes-name": name,
        },
        Tagging: aws.String(a.buildTagString(metadata)),
    })

    if err != nil {
        return nil, fmt.Errorf("failed to write to S3: %w", err)
    }

    // Get S3 version ID (for BackendRevision tracking)
    head, _ := a.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(a.bucket),
        Key:    aws.String(objectKey),
    })

    now := time.Now()
    doc := &workspace.DocumentMetadata{
        UUID:         uuid,
        ProviderType: "s3",
        ProviderID:   providerID,
        Name:         name,
        MimeType:     "text/markdown",
        CreatedTime:  now,
        ModifiedTime: now,
        SyncStatus:   "canonical",
        ContentHash:  computeHash(content),
    }

    // Store metadata (DynamoDB for fast queries)
    if err := a.metadataStore.Set(ctx, objectKey, doc); err != nil {
        return nil, err
    }

    return doc, nil
}

// Implements workspace.ContentProvider
func (a *S3Adapter) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
    objectKey := a.parseProviderID(providerID)

    result, err := a.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(a.bucket),
        Key:    aws.String(objectKey),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get S3 object: %w", err)
    }
    defer result.Body.Close()

    content, _ := io.ReadAll(result.Body)

    // Get metadata
    metadata, _ := a.metadataStore.Get(ctx, objectKey)

    return &workspace.DocumentContent{
        UUID:       metadata.UUID,
        ProviderID: providerID,
        Title:      metadata.Name,
        Body:       string(content),
        Format:     "markdown",
        BackendRevision: &workspace.BackendRevision{
            ProviderType: "s3",
            RevisionID:   aws.ToString(result.VersionId),
            ModifiedTime: aws.ToTime(result.LastModified),
            Metadata: map[string]any{
                "etag":       aws.ToString(result.ETag),
                "version_id": aws.ToString(result.VersionId),
            },
        },
        ContentHash:  computeHash(string(content)),
        LastModified: aws.ToTime(result.LastModified),
    }, nil
}

// Implements workspace.RevisionTrackingProvider
func (a *S3Adapter) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
    if !a.versioningEnabled {
        return nil, workspace.ErrNotImplemented
    }

    objectKey := a.parseProviderID(providerID)

    result, err := a.client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
        Bucket:  aws.String(a.bucket),
        Prefix:  aws.String(objectKey),
        MaxKeys: aws.Int32(int32(limit)),
    })

    if err != nil {
        return nil, err
    }

    var revisions []*workspace.BackendRevision
    for _, version := range result.Versions {
        revisions = append(revisions, &workspace.BackendRevision{
            ProviderType: "s3",
            RevisionID:   aws.ToString(version.VersionId),
            ModifiedTime: aws.ToTime(version.LastModified),
            Metadata: map[string]any{
                "etag":       aws.ToString(version.ETag),
                "size":       aws.ToInt64(version.Size),
                "is_latest":  aws.ToBool(version.IsLatest),
            },
        })
    }

    return revisions, nil
}
```

#### Configuration

```hcl
# config.hcl
workspace {
  provider = "s3"

  s3 {
    # S3 configuration
    endpoint     = "https://s3.amazonaws.com"  # Or MinIO endpoint
    region       = "us-west-2"
    bucket       = "hermes-documents"
    prefix       = "docs/"  # Optional namespace prefix

    # Versioning
    versioning_enabled = true

    # Metadata storage strategy
    metadata_store = "dynamodb"  # or "s3-tags", "manifest"

    # DynamoDB table for fast metadata queries (optional)
    dynamodb_table = "hermes-document-metadata"

    # Authentication
    auth {
      method = "iam"  # or "access-key", "sts-assume-role"
      # For access-key method:
      # access_key_id     = "..."
      # secret_access_key = "..."
    }

    # Performance tuning
    upload_concurrency = 5
    download_concurrency = 10
    multipart_threshold_mb = 100
  }
}
```

### 2. Database Schema Extensions

#### Provider Storage Registry

```sql
-- Tracks all configured storage providers
CREATE TABLE provider_storage (
    id BIGSERIAL PRIMARY KEY,

    -- Provider identification
    provider_name VARCHAR(100) NOT NULL UNIQUE,  -- "google-prod", "s3-archive", "local-edge-01"
    provider_type VARCHAR(50) NOT NULL,          -- "google", "s3", "local", "azure", "office365"

    -- Configuration
    config JSONB NOT NULL,  -- Provider-specific config (encrypted credentials)

    -- Capabilities
    capabilities JSONB,  -- {"versioning": true, "permissions": true, "search": false}

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'active',  -- 'active', 'readonly', 'disabled', 'migrating'
    is_primary BOOLEAN NOT NULL DEFAULT false,
    is_writable BOOLEAN NOT NULL DEFAULT true,

    -- Statistics
    document_count INTEGER DEFAULT 0,
    total_size_bytes BIGINT DEFAULT 0,
    last_health_check TIMESTAMP,
    health_status VARCHAR(20),  -- 'healthy', 'degraded', 'unhealthy'

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by TEXT,
    metadata JSONB
);

CREATE INDEX idx_provider_storage_type ON provider_storage(provider_type);
CREATE INDEX idx_provider_storage_status ON provider_storage(status);
```

#### Migration Jobs

```sql
-- Tracks migration jobs between providers
CREATE TABLE migration_jobs (
    id BIGSERIAL PRIMARY KEY,

    -- Job identification
    job_uuid UUID NOT NULL UNIQUE,
    job_name VARCHAR(200) NOT NULL,

    -- Source and destination
    source_provider_id INTEGER NOT NULL REFERENCES provider_storage(id),
    dest_provider_id INTEGER NOT NULL REFERENCES provider_storage(id),

    -- Scope (what to migrate)
    filter_criteria JSONB,  -- {"document_type": "RFC", "status": "Published", "project_uuid": "..."}

    -- Migration strategy
    strategy VARCHAR(50) NOT NULL DEFAULT 'copy',  -- 'copy', 'move', 'mirror'
    transform_rules JSONB,  -- Content transformation rules

    -- State
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'running', 'paused', 'completed', 'failed', 'cancelled'

    -- Progress tracking
    total_documents INTEGER DEFAULT 0,
    migrated_documents INTEGER DEFAULT 0,
    failed_documents INTEGER DEFAULT 0,
    skipped_documents INTEGER DEFAULT 0,

    -- Timing
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    estimated_completion_at TIMESTAMP,

    -- Scheduling (NEW)
    schedule_type VARCHAR(20) DEFAULT 'manual',  -- 'manual', 'scheduled', 'recurring'
    scheduled_at TIMESTAMP,                      -- When to start (for scheduled)
    cron_expression VARCHAR(100),                -- Cron schedule (for recurring)
    next_run_at TIMESTAMP,                       -- Next execution time (for recurring)
    recurrence_enabled BOOLEAN DEFAULT false,

    -- Configuration
    concurrency INTEGER DEFAULT 5,
    batch_size INTEGER DEFAULT 100,
    dry_run BOOLEAN DEFAULT false,

    -- Validation
    validate_after_migration BOOLEAN DEFAULT true,
    validation_status VARCHAR(20),  -- 'pending', 'passed', 'failed'
    validation_errors JSONB,

    -- Rollback
    rollback_enabled BOOLEAN DEFAULT true,
    rollback_data JSONB,  -- Data needed to rollback

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL,
    metadata JSONB
);

CREATE INDEX idx_migration_jobs_status ON migration_jobs(status);
CREATE INDEX idx_migration_jobs_source ON migration_jobs(source_provider_id);
CREATE INDEX idx_migration_jobs_dest ON migration_jobs(dest_provider_id);
CREATE INDEX idx_migration_jobs_created ON migration_jobs(created_at);
CREATE INDEX idx_migration_jobs_scheduled ON migration_jobs(scheduled_at) WHERE status = 'pending' AND schedule_type = 'scheduled';
CREATE INDEX idx_migration_jobs_next_run ON migration_jobs(next_run_at) WHERE recurrence_enabled = true;
```

#### Migration Items

```sql
-- Tracks individual document migration status
CREATE TABLE migration_items (
    id BIGSERIAL PRIMARY KEY,

    -- Links to migration job
    migration_job_id BIGINT NOT NULL REFERENCES migration_jobs(id) ON DELETE CASCADE,

    -- Document identification
    document_uuid UUID NOT NULL,
    source_provider_id VARCHAR(500) NOT NULL,
    dest_provider_id VARCHAR(500),

    -- State
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'in_progress', 'completed', 'failed', 'skipped'

    -- Progress
    attempt_count INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,

    -- Results
    source_content_hash VARCHAR(64),
    dest_content_hash VARCHAR(64),
    content_match BOOLEAN,

    -- Timing
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms INTEGER,

    -- Error handling
    error_message TEXT,
    error_details JSONB,
    is_retryable BOOLEAN DEFAULT true,

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX idx_migration_items_job ON migration_items(migration_job_id);
CREATE INDEX idx_migration_items_status ON migration_items(status);
CREATE INDEX idx_migration_items_uuid ON migration_items(document_uuid);
CREATE INDEX idx_migration_items_job_status ON migration_items(migration_job_id, status);
```

#### Migration Outbox (Transactional Outbox Pattern - RFC-080)

```sql
-- Outbox table for reliable migration task publishing to Redpanda
CREATE TABLE migration_outbox (
    id BIGSERIAL PRIMARY KEY,

    -- Links to migration job and item
    migration_job_id BIGINT NOT NULL REFERENCES migration_jobs(id) ON DELETE CASCADE,
    migration_item_id BIGINT NOT NULL REFERENCES migration_items(id) ON DELETE CASCADE,

    -- Document identification (for Kafka partitioning)
    document_uuid UUID NOT NULL,
    document_id VARCHAR(500) NOT NULL,

    -- Idempotency key (prevents duplicate processing)
    idempotent_key VARCHAR(128) NOT NULL UNIQUE,  -- {job_id}:{document_uuid}

    -- Event metadata
    event_type VARCHAR(50) NOT NULL,  -- 'migration.task.created', 'migration.task.retry'
    provider_source VARCHAR(100) NOT NULL,
    provider_dest VARCHAR(100) NOT NULL,

    -- Payload (all data needed for migration task)
    payload JSONB NOT NULL,  -- {job_config, item_details, transform_rules}

    -- Outbox state (RFC-080 pattern)
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- 'pending', 'published', 'failed'
    published_at TIMESTAMP,
    publish_attempts INTEGER DEFAULT 0,
    last_error TEXT,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_migration_outbox_status ON migration_outbox(status, created_at);
CREATE INDEX idx_migration_outbox_job ON migration_outbox(migration_job_id);
CREATE INDEX idx_migration_outbox_document ON migration_outbox(document_uuid);
```

**Payload Schema Example**:
```json
{
  "job_config": {
    "job_id": 123,
    "job_uuid": "550e8400-...",
    "strategy": "copy",
    "transform_rules": {...},
    "dry_run": false
  },
  "document": {
    "uuid": "7e8f4a2c-...",
    "source_provider_id": "google:1BxiMVs0XRA5nFMd...",
    "source_provider_type": "google"
  },
  "destination": {
    "provider_id": 42,
    "provider_name": "s3-archive",
    "provider_type": "s3"
  },
  "retry_config": {
    "attempt": 1,
    "max_attempts": 3
  }
}
```

#### Document Revisions Extension

```sql
-- Extend existing document_revisions table (from migration 000001)
-- No schema change needed - already supports multi-backend!

-- Example query: Get all revisions for a document across all providers
-- SELECT * FROM document_revisions
-- WHERE document_uuid = '550e8400-e29b-41d4-a716-446655440000'
-- ORDER BY revision_timestamp DESC;

-- The existing schema already has:
-- - document_uuid: Stable identifier across providers
-- - provider_type: "google", "local", "s3", "azure"
-- - provider_document_id: Provider-specific ID
-- - content_hash: For drift detection
-- - status: For sync status tracking
```

### 3. Migration Orchestrator

#### Migration Manager (Event-Driven with Outbox Pattern)

```go
// pkg/migration/manager.go
type Manager struct {
    db        *gorm.DB
    providers map[string]workspace.Provider
    logger    hclog.Logger
}

// CreateMigrationJob creates a new migration job using transactional outbox pattern
func (m *Manager) CreateMigrationJob(ctx context.Context, req *CreateMigrationRequest) (*models.MigrationJob, error) {
    // Validate source and destination providers
    sourceProvider, ok := m.providers[req.SourceProviderName]
    if !ok {
        return nil, fmt.Errorf("source provider not found: %s", req.SourceProviderName)
    }

    destProvider, ok := m.providers[req.DestProviderName]
    if !ok {
        return nil, fmt.Errorf("destination provider not found: %s", req.DestProviderName)
    }

    // Query source provider for documents matching filter
    documents, err := m.queryDocuments(ctx, sourceProvider, req.FilterCriteria)
    if err != nil {
        return nil, fmt.Errorf("failed to query documents: %w", err)
    }

    // BEGIN TRANSACTION - Everything atomic!
    var job *models.MigrationJob
    err = m.db.Transaction(func(tx *gorm.DB) error {
        // 1. Create migration job
        job = &models.MigrationJob{
            JobUUID:                uuid.New(),
            JobName:                req.JobName,
            SourceProviderID:       req.SourceProviderID,
            DestProviderID:         req.DestProviderID,
            FilterCriteria:         req.FilterCriteria,
            Strategy:               req.Strategy,
            TransformRules:         req.TransformRules,
            Status:                 "pending",
            TotalDocuments:         len(documents),
            ScheduleType:           req.ScheduleType,
            ScheduledAt:            req.ScheduledAt,
            CronExpression:         req.CronExpression,
            RecurrenceEnabled:      req.RecurrenceEnabled,
            Concurrency:            req.Concurrency,
            BatchSize:              req.BatchSize,
            DryRun:                 req.DryRun,
            ValidateAfterMigration: true,
            RollbackEnabled:        true,
            CreatedBy:              req.CreatedBy,
        }

        if err := tx.Create(job).Error; err != nil {
            return fmt.Errorf("failed to create job: %w", err)
        }

        // 2. Create migration items for each document
        for _, doc := range documents {
            item := &models.MigrationItem{
                MigrationJobID:   int64(job.ID),
                DocumentUUID:     doc.UUID,
                SourceProviderID: doc.ProviderID,
                Status:           "pending",
                MaxAttempts:      3,
            }
            if err := tx.Create(item).Error; err != nil {
                return fmt.Errorf("failed to create item: %w", err)
            }

            // 3. Create outbox event for this migration task
            outboxEvent := &models.MigrationOutbox{
                MigrationJobID:   int64(job.ID),
                MigrationItemID:  int64(item.ID),
                DocumentUUID:     doc.UUID,
                DocumentID:       doc.ProviderID,
                IdempotentKey:    fmt.Sprintf("%d:%s", job.ID, doc.UUID),
                EventType:        "migration.task.created",
                ProviderSource:   req.SourceProviderName,
                ProviderDest:     req.DestProviderName,
                Payload: map[string]any{
                    "job_config": map[string]any{
                        "job_id":          job.ID,
                        "job_uuid":        job.JobUUID.String(),
                        "strategy":        job.Strategy,
                        "transform_rules": job.TransformRules,
                        "dry_run":         job.DryRun,
                    },
                    "document": map[string]any{
                        "uuid":                 doc.UUID.String(),
                        "source_provider_id":   doc.ProviderID,
                        "source_provider_type": sourceProvider.ProviderType(),
                    },
                    "destination": map[string]any{
                        "provider_id":   job.DestProviderID,
                        "provider_name": req.DestProviderName,
                        "provider_type": destProvider.ProviderType(),
                    },
                    "retry_config": map[string]any{
                        "attempt":      0,
                        "max_attempts": 3,
                    },
                },
                Status: "pending",
            }

            if err := tx.Create(outboxEvent).Error; err != nil {
                return fmt.Errorf("failed to create outbox event: %w", err)
            }
        }

        return nil
    })

    if err != nil {
        return nil, err
    }

    // COMMIT TRANSACTION - All or nothing!
    m.logger.Info("migration job created",
        "job_id", job.ID,
        "documents", len(documents),
        "outbox_events", len(documents))

    return job, nil
}

// StartMigrationJob starts a migration job (or schedules it)
func (m *Manager) StartMigrationJob(ctx context.Context, jobID uint) error {
    var job models.MigrationJob
    if err := m.db.First(&job, jobID).Error; err != nil {
        return err
    }

    if job.Status != "pending" && job.Status != "paused" {
        return fmt.Errorf("cannot start job in status: %s", job.Status)
    }

    // Update job status
    job.Status = "running"
    job.StartedAt = ptrTime(time.Now())
    m.db.Save(&job)

    // Events are already in outbox - relay will publish them to Redpanda
    // Workers will consume and process migration tasks
    // No need to manage worker pool directly!

    m.logger.Info("migration job started", "job_id", job.ID)
    return nil
}
```

#### Outbox Relay Service

```go
// pkg/migration/relay/relay.go
type Relay struct {
    db            *gorm.DB
    kafkaProducer *kafka.Producer
    logger        hclog.Logger
    pollInterval  time.Duration
    batchSize     int
}

// Start begins polling outbox and publishing to Redpanda
func (r *Relay) Start(ctx context.Context) error {
    ticker := time.NewTicker(r.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := r.processOutbox(ctx); err != nil {
                r.logger.Error("outbox processing failed", "error", err)
            }
        }
    }
}

func (r *Relay) processOutbox(ctx context.Context) error {
    // Find pending outbox events
    var events []models.MigrationOutbox
    err := r.db.Where("status = ?", "pending").
        Order("created_at ASC").
        Limit(r.batchSize).
        Find(&events).Error

    if err != nil || len(events) == 0 {
        return err
    }

    for _, event := range events {
        if err := r.publishEvent(ctx, &event); err != nil {
            // Mark as failed, increment attempts
            event.Status = "failed"
            event.PublishAttempts++
            event.LastError = err.Error()
            r.db.Save(&event)

            r.logger.Warn("failed to publish event",
                "event_id", event.ID,
                "attempts", event.PublishAttempts,
                "error", err)
        } else {
            // Mark as published
            event.Status = "published"
            event.PublishedAt = ptrTime(time.Now())
            r.db.Save(&event)
        }
    }

    return nil
}

func (r *Relay) publishEvent(ctx context.Context, event *models.MigrationOutbox) error {
    // Serialize payload
    payload, err := json.Marshal(event.Payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %w", err)
    }

    // Publish to Redpanda topic: hermes.migrations
    // Partition by document_uuid for ordering
    return r.kafkaProducer.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{
            Topic:     kafka.StringPointer("hermes.migrations"),
            Partition: kafka.PartitionAny,
        },
        Key:   []byte(event.DocumentUUID.String()),
        Value: payload,
        Headers: []kafka.Header{
            {Key: "event_type", Value: []byte(event.EventType)},
            {Key: "job_id", Value: []byte(fmt.Sprintf("%d", event.MigrationJobID))},
            {Key: "idempotent_key", Value: []byte(event.IdempotentKey)},
        },
    }, nil)
}
```

#### Migration Worker (Kafka Consumer)

```go
// pkg/migration/worker/worker.go
type Worker struct {
    db              *gorm.DB
    providers       map[string]workspace.Provider
    kafkaConsumer   *kafka.Consumer
    logger          hclog.Logger
}

// Start begins consuming from Redpanda
func (w *Worker) Start(ctx context.Context) error {
    // Subscribe to topic
    w.kafkaConsumer.SubscribeTopics([]string{"hermes.migrations"}, nil)

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            msg, err := w.kafkaConsumer.ReadMessage(100 * time.Millisecond)
            if err != nil {
                continue // Timeout, keep polling
            }

            if err := w.processMessage(ctx, msg); err != nil {
                w.logger.Error("failed to process message",
                    "partition", msg.TopicPartition.Partition,
                    "offset", msg.TopicPartition.Offset,
                    "error", err)
                // Don't commit offset - will retry
            } else {
                // Commit offset after successful processing
                w.kafkaConsumer.CommitMessage(msg)
            }
        }
    }
}

func (w *Worker) processMessage(ctx context.Context, msg *kafka.Message) error {
    // Parse payload
    var payload map[string]any
    if err := json.Unmarshal(msg.Value, &payload); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    // Extract migration task details
    jobConfig := payload["job_config"].(map[string]any)
    document := payload["document"].(map[string]any)
    destination := payload["destination"].(map[string]any)

    jobID := int64(jobConfig["job_id"].(float64))
    documentUUID := uuid.MustParse(document["uuid"].(string))

    // Get migration item
    var item models.MigrationItem
    if err := w.db.Where("migration_job_id = ? AND document_uuid = ?",
        jobID, documentUUID).First(&item).Error; err != nil {
        return fmt.Errorf("migration item not found: %w", err)
    }

    // Execute migration
    return w.migrateDocument(ctx, &item, payload)
}

// processMigrationJob orchestrates the migration
func (m *Manager) processMigrationJob(ctx context.Context, job *models.MigrationJob) {
    defer func() {
        if r := recover(); r != nil {
            m.logger.Error("migration job panic", "job_id", job.ID, "error", r)
            job.Status = "failed"
            m.db.Save(job)
        }
    }()

    // Get source and destination providers
    sourceProvider := m.getProvider(job.SourceProviderID)
    destProvider := m.getProvider(job.DestProviderID)

    // Process items in batches
    for {
        // Check if job is paused or cancelled
        m.db.First(job, job.ID)
        if job.Status == "paused" || job.Status == "cancelled" {
            return
        }

        // Get pending items
        var items []models.MigrationItem
        err := m.db.Where("migration_job_id = ? AND status = ?", job.ID, "pending").
            Limit(job.BatchSize).
            Find(&items).Error

        if err != nil || len(items) == 0 {
            break // No more items
        }

        // Process items concurrently
        results := m.workerPool.ProcessBatch(ctx, items, func(item *models.MigrationItem) error {
            return m.migrateDocument(ctx, item, sourceProvider, destProvider, job)
        })

        // Update job progress
        for _, result := range results {
            if result.Error != nil {
                job.FailedDocuments++
            } else {
                job.MigratedDocuments++
            }
        }
        m.db.Save(job)
    }

    // Mark job as completed
    job.Status = "completed"
    job.CompletedAt = ptrTime(time.Now())
    m.db.Save(job)

    // Run validation if enabled
    if job.ValidateAfterMigration {
        go m.validateMigration(context.Background(), job)
    }
}

// migrateDocument migrates a single document
func (m *Manager) migrateDocument(
    ctx context.Context,
    item *models.MigrationItem,
    source workspace.Provider,
    dest workspace.Provider,
    job *models.MigrationJob,
) error {
    item.Status = "in_progress"
    item.StartedAt = ptrTime(time.Now())
    m.db.Save(item)

    startTime := time.Now()

    // Get source document content
    sourceContent, err := source.GetContent(ctx, item.SourceProviderID)
    if err != nil {
        item.Status = "failed"
        item.ErrorMessage = fmt.Sprintf("failed to get source: %v", err)
        item.CompletedAt = ptrTime(time.Now())
        m.db.Save(item)
        return err
    }

    item.SourceContentHash = sourceContent.ContentHash

    // Transform content if needed
    transformedContent := sourceContent.Body
    if job.TransformRules != nil {
        transformedContent, err = m.applyTransformations(transformedContent, job.TransformRules)
        if err != nil {
            item.Status = "failed"
            item.ErrorMessage = fmt.Sprintf("transformation failed: %v", err)
            m.db.Save(item)
            return err
        }
    }

    // Create document in destination if it doesn't exist
    var destDoc *workspace.DocumentMetadata
    if job.Strategy == "copy" || job.Strategy == "mirror" {
        // Try to get existing document
        destDoc, err = dest.GetDocumentByUUID(ctx, item.DocumentUUID)
        if err != nil || destDoc == nil {
            // Create new document with same UUID
            destDoc, err = dest.CreateDocumentWithUUID(
                ctx,
                item.DocumentUUID,
                "", // No template
                "", // Determine folder from metadata
                sourceContent.Title,
            )
            if err != nil {
                item.Status = "failed"
                item.ErrorMessage = fmt.Sprintf("failed to create dest: %v", err)
                m.db.Save(item)
                return err
            }
        }
    }

    // Write content to destination
    _, err = dest.UpdateContent(ctx, destDoc.ProviderID, transformedContent)
    if err != nil {
        item.Status = "failed"
        item.ErrorMessage = fmt.Sprintf("failed to write content: %v", err)
        item.AttemptCount++
        m.db.Save(item)
        return err
    }

    // Verify content hash matches
    destContent, _ := dest.GetContent(ctx, destDoc.ProviderID)
    item.DestContentHash = destContent.ContentHash
    item.ContentMatch = (item.SourceContentHash == item.DestContentHash)

    // Create document revision entry
    revision := &models.DocumentRevision{
        DocumentUUID:      item.DocumentUUID,
        ProjectUUID:       sourceContent.UUID.String(), // Or get from metadata
        ProviderType:      dest.ProviderType(),
        ProviderDocumentID: destDoc.ProviderID,
        ContentHash:       destContent.ContentHash,
        RevisionTimestamp: ptrTime(time.Now()),
        Status:            "active",
    }
    m.db.Create(revision)

    // Emit event to outbox for indexing (RFC-088)
    if !job.DryRun {
        m.outboxPublisher.PublishRevisionEvent(ctx, revision)
    }

    // Update item status
    item.Status = "completed"
    item.CompletedAt = ptrTime(time.Now())
    item.DurationMs = int(time.Since(startTime).Milliseconds())
    m.db.Save(item)

    return nil
}

// validateMigration validates that all documents migrated correctly
func (m *Manager) validateMigration(ctx context.Context, job *models.MigrationJob) {
    job.ValidationStatus = "pending"
    m.db.Save(job)

    var errors []string

    // Get all migration items
    var items []models.MigrationItem
    m.db.Where("migration_job_id = ? AND status = ?", job.ID, "completed").Find(&items)

    for _, item := range items {
        // Verify content hash matches
        if !item.ContentMatch {
            errors = append(errors, fmt.Sprintf("Content mismatch for document %s", item.DocumentUUID))
        }

        // Verify document exists in destination
        destProvider := m.getProvider(job.DestProviderID)
        _, err := destProvider.GetDocumentByUUID(ctx, item.DocumentUUID)
        if err != nil {
            errors = append(errors, fmt.Sprintf("Document not found in dest: %s", item.DocumentUUID))
        }
    }

    if len(errors) == 0 {
        job.ValidationStatus = "passed"
    } else {
        job.ValidationStatus = "failed"
        job.ValidationErrors = errors
    }

    m.db.Save(job)
}
```

### 4. Admin Interface

#### REST API Endpoints

```go
// internal/api/v2/admin/providers.go

// GET /api/v2/admin/providers
func (h *AdminHandler) ListProviders(c *gin.Context) {
    var providers []models.ProviderStorage
    h.db.Find(&providers)
    c.JSON(200, providers)
}

// POST /api/v2/admin/providers
func (h *AdminHandler) CreateProvider(c *gin.Context) {
    var req CreateProviderRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    provider := &models.ProviderStorage{
        ProviderName: req.Name,
        ProviderType: req.Type,
        Config:       req.Config,
        Status:       "active",
        IsWritable:   req.IsWritable,
    }

    if err := h.db.Create(provider).Error; err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(201, provider)
}

// POST /api/v2/admin/providers/:id/test
func (h *AdminHandler) TestProvider(c *gin.Context) {
    providerID := c.Param("id")

    // Create provider instance and test connection
    provider := h.providerFactory.Create(providerID)

    testDoc, err := provider.GetDocument(c.Request.Context(), "test-document")

    c.JSON(200, gin.H{
        "success": err == nil,
        "error":   errToString(err),
        "latency": "120ms",
    })
}

// GET /api/v2/admin/providers/:id/documents/distribution
func (h *AdminHandler) GetDocumentDistribution(c *gin.Context) {
    providerID := c.Param("id")

    var stats struct {
        TotalDocuments int64
        ByType         map[string]int64
        ByStatus       map[string]int64
        TotalSize      int64
    }

    // Query document_revisions for this provider
    h.db.Raw(`
        SELECT
            COUNT(*) as total_documents,
            SUM(size) as total_size
        FROM document_revisions
        WHERE provider_type = (SELECT provider_type FROM provider_storage WHERE id = ?)
    `, providerID).Scan(&stats)

    c.JSON(200, stats)
}

// internal/api/v2/admin/migrations.go

// POST /api/v2/admin/migrations
func (h *AdminHandler) CreateMigration(c *gin.Context) {
    var req CreateMigrationRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    job, err := h.migrationManager.CreateMigrationJob(c.Request.Context(), &req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(201, job)
}

// POST /api/v2/admin/migrations/:id/start
func (h *AdminHandler) StartMigration(c *gin.Context) {
    jobID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

    if err := h.migrationManager.StartMigrationJob(c.Request.Context(), uint(jobID)); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"status": "started"})
}

// POST /api/v2/admin/migrations/:id/pause
func (h *AdminHandler) PauseMigration(c *gin.Context) {
    jobID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

    var job models.MigrationJob
    h.db.First(&job, jobID)
    job.Status = "paused"
    h.db.Save(&job)

    c.JSON(200, gin.H{"status": "paused"})
}

// GET /api/v2/admin/migrations/:id
func (h *AdminHandler) GetMigration(c *gin.Context) {
    jobID := c.Param("id")

    var job models.MigrationJob
    if err := h.db.First(&job, jobID).Error; err != nil {
        c.JSON(404, gin.H{"error": "not found"})
        return
    }

    // Get items summary
    var itemsSummary struct {
        Pending    int64
        InProgress int64
        Completed  int64
        Failed     int64
    }
    h.db.Model(&models.MigrationItem{}).
        Where("migration_job_id = ?", jobID).
        Select("status, COUNT(*) as count").
        Group("status").
        Scan(&itemsSummary)

    c.JSON(200, gin.H{
        "job":   job,
        "items": itemsSummary,
    })
}

// GET /api/v2/admin/migrations/:id/items
func (h *AdminHandler) ListMigrationItems(c *gin.Context) {
    jobID := c.Param("id")
    status := c.Query("status") // Filter by status

    query := h.db.Where("migration_job_id = ?", jobID)
    if status != "" {
        query = query.Where("status = ?", status)
    }

    var items []models.MigrationItem
    query.Limit(100).Find(&items)

    c.JSON(200, items)
}

// POST /api/v2/admin/migrations/:id/rollback
func (h *AdminHandler) RollbackMigration(c *gin.Context) {
    jobID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

    if err := h.migrationManager.RollbackMigration(c.Request.Context(), uint(jobID)); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"status": "rolled back"})
}
```

#### UI Mockup

**Provider Management Screen**:
```
┌────────────────────────────────────────────────────────────────┐
│ Hermes Admin / Storage Providers                               │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ [+ Add Provider]                                [Refresh]      │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Provider          Type    Status    Documents   Health   │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ google-prod       google  active    5,234       ✅       │  │
│ │ s3-archive        s3      active    3,102       ✅       │  │
│ │ local-edge-nyc    local   active    1,456       ✅       │  │
│ │ azure-backup      azure   readonly  5,200       ⚠️        │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Document Distribution:                                         │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ google-prod  ████████████████████ 50%                    │  │
│ │ s3-archive   ████████████ 30%                            │  │
│ │ local-edge   ████ 15%                                    │  │
│ │ azure-backup ██ 5%                                       │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

**Migration Dashboard**:
```
┌────────────────────────────────────────────────────────────────┐
│ Hermes Admin / Migrations                                      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ [+ New Migration]                              [Refresh]       │
│                                                                 │
│ Active Migrations:                                             │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ google-to-s3-rfcs                           [Pause] [❌]   │  │
│ │ Running • Started 2h ago                                  │  │
│ │ ████████████░░░░░░░░░░  245/500 (49%)                    │  │
│ │ Source: google-prod  →  Dest: s3-archive                 │  │
│ │ Est. completion: 2h 15m                                   │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Recent Migrations:                                             │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Name                From         To          Status   ▼  │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ archive-old-docs    google-prod  s3-archive  ✅ Complete │  │
│ │ mirror-to-edge      google-prod  local-edge  ✅ Complete │  │
│ │ office365-test      google-prod  o365-dev    ❌ Failed   │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

**Migration Detail View**:
```
┌────────────────────────────────────────────────────────────────┐
│ Migration: google-to-s3-rfcs                    [Pause] [❌]    │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Status: Running                                                │
│ Progress: 245/500 documents (49%)                              │
│ ████████████░░░░░░░░░░                                         │
│                                                                 │
│ Details:                                                       │
│ • Source: google-prod (Google Workspace)                       │
│ • Destination: s3-archive (S3)                                 │
│ • Strategy: copy (keep source)                                 │
│ • Filter: document_type=RFC, status=Published                  │
│ • Started: 2h ago                                              │
│ • Est. completion: 2h 15m                                      │
│                                                                 │
│ Statistics:                                                    │
│ • Completed: 245                                               │
│ • Failed: 3                                                    │
│ • In Progress: 5                                               │
│ • Pending: 247                                                 │
│                                                                 │
│ Recent Items:                                                  │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Document              Status       Duration   Hash Match │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ RFC-001              ✅ Complete   2.3s       ✅         │  │
│ │ RFC-087              ✅ Complete   1.8s       ✅         │  │
│ │ RFC-042              ❌ Failed     0.5s       N/A        │  │
│ │ RFC-088              🔄 In Progress  ...     ...        │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Failed Items (3):                                              │
│ • RFC-042: Network timeout (retryable)                         │
│ • RFC-033: Invalid UTF-8 encoding (permanent)                  │
│ • RFC-019: Permission denied (permanent)                       │
│                                                                 │
│ [Retry Failed Items]  [Export Report]  [Rollback]             │
└────────────────────────────────────────────────────────────────┘
```

## Implementation Plan

### Phase 1: S3 Backend Foundation (Weeks 1-2)

**Week 1: S3 Adapter Implementation**
- Implement S3 provider adapter (`pkg/workspace/adapters/s3/`)
- Implement DocumentProvider interface (CRUD operations)
- Implement ContentProvider interface (content read/write)
- Implement RevisionTrackingProvider interface (S3 versioning)
- Configuration parsing and validation
- Unit tests with MinIO testcontainer

**Week 2: Metadata Store & Integration**
- Implement metadata store options (S3 tags, DynamoDB, manifest)
- Integrate S3 adapter with existing provider system
- Add S3 configuration to config.hcl
- Integration tests with real S3/MinIO
- Performance benchmarking

**Deliverables**:
- Working S3 backend adapter
- Can create/read/update/delete documents in S3
- Can track revisions with S3 versioning
- Configuration and documentation

### Phase 2: Database Schema & Models (Week 3)

- Create `provider_storage` table migration
- Create `migration_jobs` table migration
- Create `migration_items` table migration
- Implement GORM models (`pkg/models/`)
- Write model tests

**Deliverables**:
- Database migrations
- GORM models for provider registry and migrations
- Model unit tests

### Phase 3: Migration Manager (Weeks 4-5)

**Week 4: Core Migration Logic**
- Implement migration manager (`pkg/migration/manager.go`)
- Implement worker pool for concurrent migrations
- Implement document migration logic
- Content transformation system
- Error handling and retry logic

**Week 5: Validation & Rollback**
- Implement migration validation
- Implement rollback capability
- Progress tracking and metrics
- Integration tests

**Deliverables**:
- Working migration manager
- Can migrate documents between any providers
- Validation and rollback working
- Comprehensive tests

### Phase 4: Admin API & UI (Weeks 6-7)

**Week 6: REST API**
- Implement provider management endpoints
- Implement migration endpoints
- Add authentication/authorization
- API documentation (OpenAPI/Swagger)
- API integration tests

**Week 7: Admin UI**
- Provider management screen
- Migration creation wizard
- Migration dashboard with real-time updates
- Migration detail view
- UI integration tests

**Deliverables**:
- Admin REST API
- Admin UI for provider and migration management
- Real-time progress updates via WebSocket/SSE
- Documentation

### Phase 5: Multi-Writable Storage (Week 8)

- Implement multi-provider write support
- Conflict detection and resolution
- Sync status tracking
- Automatic mirroring configuration

**Deliverables**:
- Documents can exist in multiple writable stores
- Automatic sync between stores
- Conflict detection and resolution

### Phase 6: Testing & Documentation (Week 9)

- End-to-end migration tests
- Load testing (1000+ documents)
- Failure recovery testing
- Documentation and runbooks
- Migration from Google → S3 → Local → Office365 demo

**Deliverables**:
- Comprehensive test suite
- Performance benchmarks
- User documentation
- Operator runbooks

### Phase 7: Production Rollout (Week 10)

- Deploy S3 backend to staging
- Run migration from Google to S3 (RFC documents)
- Monitor and validate
- Deploy to production with feature flag
- Gradual rollout

**Deliverables**:
- S3 backend in production
- First production migration complete
- Monitoring dashboards
- Incident response procedures

## Use Cases

### Use Case 1: Archive Old Documents to S3

**Scenario**: Archive all published RFCs older than 1 year to S3 for cost savings.

```bash
# Create S3 provider
POST /api/v2/admin/providers
{
  "name": "s3-archive",
  "type": "s3",
  "config": {
    "bucket": "hermes-archive",
    "region": "us-west-2"
  },
  "is_writable": true
}

# Create migration job
POST /api/v2/admin/migrations
{
  "name": "archive-old-rfcs",
  "source_provider": "google-prod",
  "dest_provider": "s3-archive",
  "filter_criteria": {
    "document_type": "RFC",
    "status": "Published",
    "modified_before": "2024-01-01"
  },
  "strategy": "copy",
  "concurrency": 10
}

# Start migration
POST /api/v2/admin/migrations/1/start

# Monitor progress
GET /api/v2/admin/migrations/1
{
  "status": "running",
  "progress": "450/500 (90%)",
  "migrated_documents": 450,
  "failed_documents": 2
}
```

### Use Case 2: Migrate from Google Docs to Markdown in S3

**Scenario**: Migrate documentation from Google Docs to Markdown format in S3.

```bash
POST /api/v2/admin/migrations
{
  "name": "google-to-s3-markdown",
  "source_provider": "google-prod",
  "dest_provider": "s3-docs",
  "filter_criteria": {
    "document_type": "Documentation"
  },
  "strategy": "copy",
  "transform_rules": {
    "format": "markdown",
    "preserve_frontmatter": true,
    "extract_metadata": ["tags", "author", "status"]
  }
}
```

### Use Case 3: Multi-Writable Stores for Resilience

**Scenario**: Keep critical documents in both Google and S3 for resilience.

```hcl
workspace {
  providers = ["google-prod", "s3-primary"]

  multi_writable {
    enabled = true
    strategy = "dual-write"

    replication {
      source = "google-prod"
      destinations = ["s3-primary"]
      auto_sync = true
      conflict_resolution = "source-wins"
    }
  }
}
```

### Use Case 4: Test Migration Before Production

**Scenario**: Test Office365 migration with dry run.

```bash
POST /api/v2/admin/migrations
{
  "name": "test-office365-migration",
  "source_provider": "google-prod",
  "dest_provider": "office365-test",
  "filter_criteria": {
    "document_type": "RFC",
    "limit": 10
  },
  "dry_run": true,
  "validate_after_migration": true
}
```

### Use Case 5: Rollback Failed Migration

**Scenario**: Rollback a migration that produced incorrect results.

```bash
POST /api/v2/admin/migrations/5/rollback
{
  "delete_migrated_documents": true,
  "restore_original_sync_status": true
}
```

## Benefits

1. **Cost Optimization**: Archive old documents to cheaper S3 storage
2. **Vendor Independence**: Not locked into Google Workspace
3. **Data Sovereignty**: Keep sensitive documents in controlled S3 buckets
4. **Disaster Recovery**: Multi-writable stores provide resilience
5. **Flexibility**: Migrate between any providers as needs change
6. **Observability**: Full visibility into migration progress and status
7. **Safety**: Dry run, validation, and rollback capabilities
8. **Scalability**: S3 scales to billions of documents
9. **Version Control**: S3 versioning tracks all document changes
10. **Compliance**: Long-term archival with S3 Glacier

## Success Metrics

### Functional Metrics
- Successfully create/read/update/delete documents in S3
- Successfully migrate 1000+ documents between any two providers
- Validation passes for 99.9% of migrated documents
- Rollback works for all migration types
- Admin UI allows non-engineers to manage migrations

### Performance Metrics
- S3 document read latency < 200ms (p99)
- S3 document write latency < 500ms (p99)
- Migration throughput > 100 documents/minute
- Zero data loss during migrations
- < 1% migration failure rate

### Operational Metrics
- Migrations complete without manual intervention 95%+ of time
- Failed migrations can be retried automatically
- Admin UI provides real-time migration status
- Monitoring alerts on migration failures
- Runbooks exist for all failure scenarios

## Risks & Mitigations

### Risk 1: Data Loss During Migration

**Risk**: Documents lost or corrupted during migration

**Mitigation**:
- Content hash validation (SHA-256) for every document
- Atomic transactions for database updates
- Rollback capability to undo migrations
- Dry run mode to test migrations first
- Keep source documents until validation passes

### Risk 2: Performance Impact

**Risk**: Migrations slow down API performance

**Mitigation**:
- Run migrations in separate worker processes
- Rate limit migration operations
- Monitor database and API latency
- Pause migrations during peak hours
- Use separate database connection pool

### Risk 3: S3 Costs

**Risk**: S3 storage costs exceed budget

**Mitigation**:
- Implement S3 lifecycle policies (archive to Glacier)
- Monitor storage usage with alerts
- Calculate costs before migrations
- Use S3 Intelligent-Tiering
- Compress documents before storage

### Risk 4: Metadata Loss

**Risk**: Document metadata not preserved during migration

**Mitigation**:
- Store metadata in S3 object tags or DynamoDB
- Validate metadata after migration
- Use frontmatter in Markdown for embedded metadata
- Test metadata preservation in dry runs
- Document metadata schema for each provider

### Risk 5: Complex Migrations

**Risk**: Complex migrations (e.g., Google Docs → Markdown) lose formatting

**Mitigation**:
- Support transformation rules for content conversion
- Test transformations with sample documents
- Provide warnings for lossy conversions
- Allow manual review of transformed content
- Document limitations of each migration path

## Future Enhancements

### Phase 2: Additional Providers
- Azure Blob Storage backend
- Office 365 OneDrive/SharePoint backend
- GitHub repository backend
- GitLab repository backend
- Confluence backend

### Phase 3: Advanced Features
- Bi-directional sync between providers
- Incremental migrations (only changed documents)
- Scheduled migrations (cron-based)
- Migration webhooks for external integrations
- Migration templates (reusable configurations)
- Content transformation plugins (extensible)

### Phase 4: AI-Powered Migrations
- Automatic content format detection
- Smart metadata extraction from unstructured content
- Duplicate document detection
- Document classification for automatic routing

## References

- **RFC-084**: Provider Interface Refactoring - Multi-Backend Document Model
  - Establishes UUID-based document identification
  - Defines multi-backend revision tracking
  - Provides foundation for migration system

- **RFC-080**: Outbox Pattern for Document Synchronization
  - Transactional consistency pattern
  - Event-driven architecture
  - Reliable async processing

- **RFC-051**: Document Search Index Outbox Pattern
  - Document modification logging
  - Audit trail design
  - Outbox table schemas

- **RFC-088**: Event-Driven Document Indexer with Pipeline Rulesets
  - Pipeline execution system
  - Content hash for idempotency
  - Retry and error handling

- **Existing Code**:
  - `pkg/workspace/adapters/local/document_storage.go` - Local provider implementation
  - `pkg/notifications/backends/backend.go` - Backend interface pattern
  - `internal/migrate/migrations/000001_core_schema.up.sql` - Document revision schema

## Open Questions

1. **S3 Metadata Storage Strategy**
   - **Options**: S3 object tags, DynamoDB, separate manifest file
   - **Recommendation**: DynamoDB for fast queries, S3 tags for backup

2. **Content Transformation Plugins**
   - **Question**: Should transformations be plugins or built-in?
   - **Recommendation**: Start with built-in, add plugin system later

3. **Migration Scheduling**
   - **Question**: Support cron-based scheduled migrations?
   - **Recommendation**: Yes, add in Phase 3

4. **Multi-Provider Conflict Resolution**
   - **Question**: How to resolve conflicts when documents exist in multiple writable stores?
   - **Recommendation**: Configurable strategies (source-wins, last-write-wins, manual)

5. **Migration History Retention**
   - **Question**: How long to keep migration job history?
   - **Recommendation**: 90 days, then archive to S3

6. **S3 Cost Monitoring**
   - **Question**: Should we track S3 costs per provider/migration?
   - **Recommendation**: Yes, integrate with AWS Cost Explorer API

## Timeline

- **Week 1-2**: S3 backend implementation and testing
- **Week 3**: Database schema and models
- **Week 4-5**: Migration manager and orchestration
- **Week 6-7**: Admin API and UI
- **Week 8**: Multi-writable storage support
- **Week 9**: Testing and documentation
- **Week 10**: Production rollout

**Total Effort**: 10 weeks (2 backend engineers + 1 frontend engineer)

---

**Document ID**: RFC-089
**Status**: Draft
**Author**: Engineering Team
**Created**: 2025-11-15
**Last Updated**: 2025-11-15
