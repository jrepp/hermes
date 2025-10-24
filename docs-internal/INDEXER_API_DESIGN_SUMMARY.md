# Indexer API Design Summary

**Date**: October 23, 2025  
**Status**: ✅ Design Complete - Ready for Implementation  
**Documents Updated**:
- `INDEXER_IMPLEMENTATION_GUIDE.md` - Added complete API specification
- `INDEXER_REFACTOR_PLAN.md` - Added API-Based Architecture section

## Design Overview

The indexer now uses an **API-based architecture** instead of direct database access. This fundamental shift enables external document sources (GitHub, local files, remote Hermes) and proper separation of concerns.

## Key API Endpoints Defined

### 1. Document Management

**POST /api/v2/indexer/documents** - Create/upsert document reference
- Supports multiple workspace provider types: `github`, `local`, `hermes`, `google`
- Includes workspace metadata (repo, path, commit SHA, etc.)
- Upsert semantics (create if new, update if exists by UUID)
- Returns document ID and creation/update status

**GET /api/v2/indexer/documents/:uuid** - Get document by UUID
- Returns document metadata and latest revision info
- Used by indexer for verification and duplicate detection

### 2. Revision Tracking

**POST /api/v2/indexer/documents/:uuid/revisions** - Create document revision
- Full metadata: content hash, commit SHA, revision reference
- Automatic duplicate detection (by content hash)
- Links revision to document UUID
- Supports content length, type, modified_by, modified_at

### 3. AI-Generated Content

**PUT /api/v2/indexer/documents/:uuid/summary** - Update AI summary
- Ties summary to specific revision (by ID or content hash)
- Includes AI model metadata (model name, version, tokens used)
- Validation: ensures revision exists before saving summary

**PUT /api/v2/indexer/documents/:uuid/embeddings** - Store vector embeddings
- Supports chunked embeddings (multiple per document)
- Model and dimension tracking
- Linked to specific revision for consistency

## Workspace Provider Types

The API supports multiple external sources:

### GitHub
```json
{
  "workspace_provider": {
    "type": "github",
    "repository": "hashicorp/hermes",
    "branch": "main",
    "path": "docs-internal/RFC-001.md",
    "commit_sha": "abc123def456",
    "remote_url": "https://github.com/hashicorp/hermes"
  }
}
```

### Local Filesystem
```json
{
  "workspace_provider": {
    "type": "local",
    "path": "docs-internal/RFC-001.md",
    "absolute_path": "/Users/jrepp/hc/hermes/docs-internal/RFC-001.md",
    "project_root": "/Users/jrepp/hc/hermes"
  }
}
```

### Remote Hermes Instance
```json
{
  "workspace_provider": {
    "type": "hermes",
    "endpoint": "https://hermes.example.com",
    "document_id": "550e8400-e29b-41d4-a716-446655440000",
    "api_key": "...",
    "workspace_id": "production"
  }
}
```

## Architecture Flow

```
Indexer Service (External Client)
    │
    │ 1. Discover documents from project workspace
    │ 2. Extract content and metadata
    │ 3. Calculate content hash
    │ 4. Generate AI summary (if enabled)
    │
    ▼
POST /api/v2/indexer/documents
    │ (Create document reference with workspace provider metadata)
    │
    ▼
POST /api/v2/indexer/documents/:uuid/revisions
    │ (Create revision with content hash, commit SHA)
    │
    ▼
PUT /api/v2/indexer/documents/:uuid/summary
    │ (Save AI-generated summary tied to revision)
    │
    ▼
PUT /api/v2/indexer/documents/:uuid/embeddings
    │ (Store vector embeddings for semantic search)
    │
    ▼
Database (PostgreSQL)
    • documents table (with workspace_provider_type, workspace_provider_metadata)
    • document_revisions table (content_hash, commit_sha, revision_reference)
    • document_embeddings table (vector data)
```

## Project Config Integration

The indexer uses **project configuration** to resolve workspace providers:

```hcl
# testing/projects/docs-internal.hcl
project "docs-internal" {
  short_name  = "DOCS"
  description = "Internal documentation"
  
  workspace "local" {
    type = "local"
    root = "./docs-internal"
  }
}
```

**CLI Usage**:
```bash
# Index specific project
./hermes indexer -config=config.hcl -project=docs-internal

# Index all active projects
./hermes indexer -config=config.hcl -all-projects
```

## Database Schema Changes

New fields required in `documents` table:
```sql
ALTER TABLE documents ADD COLUMN workspace_provider_type VARCHAR(50);
ALTER TABLE documents ADD COLUMN workspace_provider_metadata JSONB;
ALTER TABLE documents ADD COLUMN indexed_at TIMESTAMP;
ALTER TABLE documents ADD COLUMN indexer_version VARCHAR(50);
```

New `document_revisions` table:
```sql
CREATE TABLE document_revisions (
  id SERIAL PRIMARY KEY,
  document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  content_hash VARCHAR(255) NOT NULL,
  revision_reference VARCHAR(255),
  commit_sha VARCHAR(255),
  content_length BIGINT,
  content_type VARCHAR(100),
  summary TEXT,
  modified_by VARCHAR(255),
  modified_at TIMESTAMP,
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE(document_id, content_hash)
);
```

New `document_embeddings` table:
```sql
CREATE TABLE document_embeddings (
  id SERIAL PRIMARY KEY,
  document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  revision_id INTEGER REFERENCES document_revisions(id) ON DELETE CASCADE,
  model VARCHAR(100) NOT NULL,
  model_version VARCHAR(50),
  dimensions INTEGER NOT NULL,
  embeddings vector(768), -- pgvector extension
  chunk_metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

## Advantages Over Direct DB Access

| Aspect | Direct DB | API-Based ✅ |
|--------|-----------|--------------|
| **Coupling** | Tight coupling to schema | Loose coupling via contracts |
| **Testing** | Requires DB setup | Can mock API responses |
| **Security** | Full DB access | Scoped permissions via API |
| **Validation** | Manual | API enforces rules |
| **Audit** | Manual logging | Built-in API logs |
| **Deployment** | Must share DB | Can deploy independently |
| **Scaling** | Single process | Indexer can scale horizontally |
| **External Sources** | Difficult | Natural (API accepts metadata) |

## Authentication

**Service Token** (production):
```bash
./hermes admin create-service-token --name="indexer-service" --scopes="indexer:write"
export HERMES_INDEXER_TOKEN="svc_abc123..."
```

**OIDC/Dex** (testing):
```go
token, err := auth.GetOIDCToken(ctx, dexURL, clientID, clientSecret)
apiClient := &IndexerAPIClient{
    BaseURL:   "http://localhost:8001",
    AuthToken: token,
}
```

## Migration Strategy

**Phase 1**: Add API client parameter to commands (keep DB for backward compat)
```go
type TrackRevisionCommand struct {
    DB        *gorm.DB              // Legacy (deprecated)
    APIClient *IndexerAPIClient     // New (preferred)
}
```

**Phase 2**: Deprecate DB parameter, log warnings

**Phase 3**: Remove DB parameter entirely

## Integration Test Changes

The integration test (`tests/integration/indexer/full_pipeline_test.go`) will:

1. **Create API client** instead of direct DB connection
2. **Use project config** to resolve workspace (not direct filesystem path)
3. **Update commands** to use API client:
   - `TrackCommand` → POST to `/api/v2/indexer/documents`
   - `TrackRevisionCommand` → POST to `/api/v2/indexer/documents/:uuid/revisions`
   - `SummarizeCommand` → PUT to `/api/v2/indexer/documents/:uuid/summary`

## Next Steps

1. ✅ **Design Complete** (this document)
2. ⏳ **Implement API handlers** (`internal/api/v2/indexer.go`)
3. ⏳ **Create API client** (`tests/integration/indexer/api_client.go`)
4. ⏳ **Refactor commands** to use API client
5. ⏳ **Update integration test** with project config
6. ⏳ **Run end-to-end test** and validate

## References

- **Full API Specification**: `docs-internal/INDEXER_IMPLEMENTATION_GUIDE.md` (lines 1200+)
- **Architecture Details**: `docs-internal/INDEXER_REFACTOR_PLAN.md` (API-Based Architecture section)
- **Command Pattern**: `docs-internal/INDEXER_REFACTOR_PLAN.md` (Design Patterns section)
- **Project Config**: `docs-internal/DISTRIBUTED_PROJECTS_ARCHITECTURE.md`
- **Testing Environment**: `testing/README.md`
