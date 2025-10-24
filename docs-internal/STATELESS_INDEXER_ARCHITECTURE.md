# Stateless Indexer Architecture

**Status**: 🚧 Design  
**Version**: 2.0.0  
**Created**: October 24, 2025

## Overview

The indexer is being refactored from a **stateful database-dependent service** into a **stateless API client** that submits all data to the central Hermes server.

## Current Architecture (v1) ❌

```
┌───────────────────────────────────────┐
│          Indexer Process              │
│                                       │
│  ┌──────────┐  ┌──────────────────┐  │
│  │ GORM DB  │  │ Google Workspace │  │
│  │ (direct) │  │   API Client     │  │
│  └────┬─────┘  └────┬─────────────┘  │
│       │             │                 │
│       ▼             ▼                 │
│  ┌────────────────────────┐          │
│  │  Index Documents       │          │
│  │  - Read from GW        │          │
│  │  - Write to DB         │          │
│  │  - Write to Algolia    │          │
│  └────────────────────────┘          │
└───────────────────────────────────────┘
         │              │
         ▼              ▼
   PostgreSQL      Algolia/Meilisearch
```

**Problems**:
- ❌ Indexer requires PostgreSQL connection
- ❌ Indexer has write access to database
- ❌ Cannot run indexer without full infrastructure
- ❌ Tight coupling between indexer and database schema
- ❌ Difficult to version/update indexers independently

## New Architecture (v2) ✅

```
┌───────────────────────────────────────────────┐
│            Indexer Agent (Stateless)          │
│                                               │
│  ┌──────────────┐  ┌─────────────────────┐   │
│  │   Provider   │  │  Hermes API Client  │   │
│  │   Adapter    │  │  (HTTP only)        │   │
│  │ - Google     │  └──────────┬──────────┘   │
│  │ - Local FS   │             │               │
│  └──────┬───────┘             │               │
│         │                     │               │
│         ▼                     ▼               │
│  ┌────────────────────────────────────────┐  │
│  │  Document Discovery & Processing       │  │
│  │  - Scan provider for documents         │  │
│  │  - Extract metadata                    │  │
│  │  - Generate summaries (optional)       │  │
│  │  - Generate embeddings (optional)      │  │
│  │  - Submit via API to central Hermes   │  │
│  └────────────────────────────────────────┘  │
└───────────────────────┬───────────────────────┘
                        │ HTTPS + Bearer Token
                        ▼
           ┌────────────────────────────┐
           │    Central Hermes Server   │
           │                            │
           │  /api/v2/indexer/*         │
           │    - register              │
           │    - heartbeat             │
           │    - documents (batch)     │
           │                            │
           │  ┌──────────┐  ┌─────────┐│
           │  │PostgreSQL│  │Meilisrch││
           │  └──────────┘  └─────────┘│
           └────────────────────────────┘
```

**Benefits**:
- ✅ Indexer has zero database dependencies
- ✅ Central Hermes owns all database writes
- ✅ Indexer can run anywhere (local dev, CI/CD, edge)
- ✅ Easy to version/update indexers independently
- ✅ Simpler testing (mock HTTP API only)

## API Design

### POST `/api/v2/indexer/register`

Register a new indexer with central Hermes.

**Request**:
```json
{
  "token": "hermes-registration-token-...",
  "indexer_type": "local-workspace",
  "workspace_path": "/workspace",
  "metadata": {
    "hostname": "dev-machine",
    "version": "v1.2.3",
    "provider": "google-workspace"
  }
}
```

**Response**:
```json
{
  "indexer_id": "550e8400-e29b-41d4-a716-446655440000",
  "api_token": "hermes-api-token-abc123...",
  "expires_at": "2025-11-24T00:00:00Z",
  "config": {
    "heartbeat_interval": "5m",
    "batch_size": 100
  }
}
```

### POST `/api/v2/indexer/heartbeat`

Send periodic heartbeat to signal indexer is alive.

**Headers**:
```
Authorization: Bearer hermes-api-token-abc123...
```

**Request**:
```json
{
  "indexer_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "healthy",
  "document_count": 142,
  "last_scan_at": "2025-10-24T10:00:00Z",
  "metrics": {
    "documents_processed": 42,
    "errors": 0
  }
}
```

**Response**:
```json
{
  "acknowledged": true,
  "server_time": "2025-10-24T10:05:00Z"
}
```

### POST `/api/v2/indexer/documents`

Submit discovered/updated documents to central Hermes.

**Headers**:
```
Authorization: Bearer hermes-api-token-abc123...
Content-Type: application/json
```

**Request**:
```json
{
  "indexer_id": "550e8400-e29b-41d4-a716-446655440000",
  "documents": [
    {
      "action": "upsert",
      "document": {
        "uuid": "doc-550e8400-e29b-41d4-a716-446655440000",
        "provider_type": "google-workspace",
        "provider_document_id": "1a2b3c4d5e6f",
        "project_id": "engineering-rfcs",
        "title": "RFC-001: Distributed Architecture",
        "summary": "This RFC proposes...",
        "content": "# RFC-001\n\n...",
        "content_hash": "sha256:abc123...",
        "document_type": "RFC",
        "status": "In-Review",
        "metadata": {
          "created_at": "2025-10-01T00:00:00Z",
          "modified_at": "2025-10-24T10:00:00Z",
          "owner": "user@example.com",
          "contributors": ["user1@example.com", "user2@example.com"],
          "approvers": ["manager@example.com"],
          "custom_fields": {
            "Current Version": "1.0",
            "Stakeholders": "Engineering, Product"
          }
        },
        "embedding": {
          "model": "openai-text-embedding-3-small",
          "vector": [0.1, 0.2, ..., 0.9],
          "dimensions": 1536
        }
      }
    },
    {
      "action": "delete",
      "provider_document_id": "old-doc-id",
      "reason": "Document deleted from provider"
    }
  ],
  "provider_metadata": {
    "scan_completed_at": "2025-10-24T10:00:00Z",
    "total_discovered": 2,
    "provider_version": "v3"
  }
}
```

**Response**:
```json
{
  "accepted": 2,
  "rejected": 0,
  "results": [
    {
      "uuid": "doc-550e8400-e29b-41d4-a716-446655440000",
      "status": "created",
      "indexed_at": "2025-10-24T10:05:00Z"
    },
    {
      "provider_document_id": "old-doc-id",
      "status": "deleted"
    }
  ],
  "errors": []
}
```

### Document Submission States

Documents can be in different states during indexer operations:

```go
type DocumentAction string

const (
    DocumentActionUpsert  DocumentAction = "upsert"  // Create or update
    DocumentActionDelete  DocumentAction = "delete"  // Mark as deleted
    DocumentActionRefresh DocumentAction = "refresh" // Refresh metadata only
)
```

## Indexer Workflow

### 1. Registration Phase
```go
// On startup
client := NewHermesClient(centralURL)
resp := client.Register(token, indexerType, workspacePath)
indexerID := resp.IndexerID
apiToken := resp.APIToken
```

### 2. Discovery Phase
```go
// Scan provider for documents
provider := NewGoogleWorkspaceProvider(...)
documents := provider.ListDocuments()

for doc := range documents {
    // Extract metadata
    metadata := provider.GetMetadata(doc.ID)
    
    // Optional: Generate summary
    summary := ai.Summarize(doc.Content)
    
    // Optional: Generate embedding
    embedding := ai.Embed(doc.Content)
    
    // Build submission
    submission := IndexerDocument{
        UUID: doc.UUID,
        ProviderDocumentID: doc.ID,
        Title: metadata.Title,
        Content: doc.Content,
        Summary: summary,
        Embedding: embedding,
        Metadata: metadata,
    }
    
    batch = append(batch, submission)
    
    // Submit in batches
    if len(batch) >= batchSize {
        client.SubmitDocuments(indexerID, apiToken, batch)
        batch = nil
    }
}
```

### 3. Heartbeat Phase
```go
// Periodic heartbeat
ticker := time.NewTicker(5 * time.Minute)
for range ticker.C {
    client.Heartbeat(indexerID, apiToken, HeartbeatRequest{
        Status: "healthy",
        DocumentCount: totalDocuments,
        LastScanAt: lastScanTime,
    })
}
```

## Code Organization

### Old Structure ❌
```
internal/indexer/
  indexer.go         // Depends on gorm.DB, algolia, googleworkspace
```

### New Structure ✅
```
cmd/hermes-indexer/        # Separate binary
  main.go
  
pkg/indexer/
  client/
    hermes_client.go       # HTTP client for central Hermes API
    types.go               # Request/response types
  
  providers/
    provider.go            # Interface
    google.go              # Google Workspace implementation
    local.go               # Local filesystem implementation
    remote.go              # Remote Hermes implementation
  
  processor/
    metadata.go            # Metadata extraction
    summary.go             # AI summarization (optional)
    embedding.go           # AI embedding generation (optional)
  
  agent/
    agent.go               # Main indexer agent orchestration
    config.go              # Agent configuration
```

## Migration Strategy

### Phase 1: New API Endpoints ✅
- Implement `/api/v2/indexer/*` endpoints on server
- Keep old indexer working

### Phase 2: Stateless Indexer ✅
- Create `pkg/indexer/client/` with HTTP client
- Create new `cmd/hermes-indexer/` binary
- No gorm.DB dependency

### Phase 3: Provider Refactoring ✅
- Extract provider interfaces
- Support Google, Local, Remote providers

### Phase 4: Testing ✅
- Test indexer agent → central Hermes flow
- E2E tests with Docker Compose

### Phase 5: Deprecation ✅
- Mark old `internal/indexer/indexer.go` as deprecated
- Migrate to new indexer agent
- Remove database dependency from indexer

## Dependencies per Binary

### `cmd/hermes` (Server)
```go
require (
    gorm.io/gorm
    gorm.io/driver/postgres
    gorm.io/driver/sqlite
    github.com/golang-migrate/migrate/v4
    github.com/meilisearch/meilisearch-go
    google.golang.org/api  // For workspace provider
)
```

### `cmd/hermes-indexer` (Agent)
```go
require (
    google.golang.org/api  // For Google Workspace provider only
    // NO database dependencies
    // NO search engine dependencies
)
```

This separation ensures:
- ✅ Smaller binary size for indexer
- ✅ Faster build times
- ✅ Clear dependency boundaries
- ✅ Easier to vendor/distribute indexer separately

## Security Considerations

### Token Security
- ✅ Registration tokens are one-time use
- ✅ API tokens expire and can be rotated
- ✅ Tokens transmitted over HTTPS only
- ✅ Rate limiting on API endpoints

### Data Integrity
- ✅ Content hashes prevent corruption
- ✅ Idempotent upsert operations
- ✅ Conflict detection via content hash comparison

### Access Control
- ✅ Indexer can only write to its registered projects
- ✅ Central Hermes validates all submissions
- ✅ Document ownership enforced server-side

## Future Enhancements

### Streaming API
Use Server-Sent Events (SSE) for real-time updates:
```
GET /api/v2/indexer/stream?indexer_id=...
Authorization: Bearer hermes-api-token-...

data: {"type":"command","action":"rescan","project_id":"rfcs"}
data: {"type":"ack","heartbeat_received":true}
```

### Webhook Support
Allow central Hermes to push commands to indexers:
```
POST https://indexer-agent.local/webhook
{
  "command": "rescan",
  "project_id": "rfcs",
  "signature": "sha256:..."
}
```

### Distributed Indexing
Multiple indexers for same project (sharded by document ranges):
```
Indexer A: documents 1-1000
Indexer B: documents 1001-2000
Indexer C: documents 2001-3000
```
