# Document Revisions and Migration Tracking

**Version**: 1.0.0-alpha  
**Status**: 🚧 Design Phase  
**Related**: `DISTRIBUTED_PROJECTS_ARCHITECTURE.md`, `DISTRIBUTED_PROJECTS_ROADMAP.md`

## Problem Statement

During migration from Google Workspace to local Git repositories (or any provider-to-provider migration), we face several challenges:

1. **Hundreds of documents** need to migrate from Google to Git
2. **Active editing** continues during migration (users still edit Google Docs)
3. **Concurrent changes** on both source and target create conflicts
4. **No global uniqueness** for project names/TLAs in distributed system
5. **Need to track** which documents have migrated successfully vs. which have conflicts

## Solution: Provider-Project-Document-Revision Model

### Core Principle

**Documents are identified by stable UUIDs, not provider-specific IDs.**

A single document can exist in multiple provider-project combinations during migration, with each location tracked as a separate **revision**.

## Document Lifecycle

### Phase 1: Single Provider (Pre-Migration)

```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
Title: "RFC-001: API Gateway Design"

Revision 1:
  Provider: google-workspace
  Project: rfc-archive
  Provider Doc ID: 1a2b3c4d5e6f7890abcd
  Content Hash: sha256:abc123def456...
  Status: canonical
  Last Modified: 2025-09-15T10:00:00Z
```

### Phase 2: Migration Started (Dual Provider)

Admin starts migration:
```bash
hermes migrate start --project=rfc-archive --source=google --target=local-git
```

System creates target revision:
```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
Title: "RFC-001: API Gateway Design"

Revision 1 (SOURCE):
  Provider: google-workspace
  Project: rfc-archive
  Provider Doc ID: 1a2b3c4d5e6f7890abcd
  Content Hash: sha256:abc123def456...
  Status: source
  Last Modified: 2025-09-15T10:00:00Z
  
Revision 2 (TARGET):
  Provider: local-git
  Project: rfcs-new
  Provider Doc ID: docs/rfc-001.md
  Content Hash: sha256:abc123def456...  ✅ SAME
  Status: target
  Last Modified: 2025-10-01T09:00:00Z
  Git Commit: a1b2c3d4
```

**Status**: ✅ Content matches, migration on track

### Phase 3: Conflict Detected (Edit During Migration)

User edits Google Doc during migration:
```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
Title: "RFC-001: API Gateway Design"

Revision 1 (SOURCE):
  Provider: google-workspace
  Project: rfc-archive
  Provider Doc ID: 1a2b3c4d5e6f7890abcd
  Content Hash: sha256:999new999...  ⚠️ CHANGED!
  Status: conflict
  Last Modified: 2025-10-15T14:30:00Z
  
Revision 2 (TARGET):
  Provider: local-git
  Project: rfcs-new
  Provider Doc ID: docs/rfc-001.md
  Content Hash: sha256:abc123def456...
  Status: conflict
  Last Modified: 2025-10-01T09:00:00Z
  Git Commit: a1b2c3d4

Conflict Record:
  Type: concurrent-edit
  Detected: 2025-10-15T15:00:00Z
  Resolution: pending
```

**Status**: ⚠️ Conflict detected, requires resolution

### Phase 4: Conflict Resolution

Admin reviews and resolves:
```bash
# Option 1: Source wins (take Google Doc changes)
hermes migrate resolve --uuid=550e8400... --strategy=source-wins

# Option 2: Target wins (keep Git version)
hermes migrate resolve --uuid=550e8400... --strategy=target-wins

# Option 3: Manual merge
hermes migrate resolve --uuid=550e8400... --strategy=manual
```

After resolution (source-wins):
```
Revision 1 (SOURCE - NOW ARCHIVED):
  Provider: google-workspace
  Status: archived
  Content Hash: sha256:999new999...
  
Revision 2 (TARGET - NOW CANONICAL):
  Provider: local-git
  Status: canonical
  Content Hash: sha256:999new999...  ✅ Updated from source
  Last Modified: 2025-10-16T10:00:00Z
  Git Commit: e5f6g7h8
```

### Phase 5: Migration Complete

```
Document UUID: 550e8400-e29b-41d4-a716-446655440000

Revision 1 (OLD):
  Provider: google-workspace
  Status: archived
  Notes: "Migrated to local-git on 2025-10-16"
  
Revision 2 (CURRENT):
  Provider: local-git
  Status: canonical
  Content Hash: sha256:999new999...
  Last Modified: 2025-10-16T10:00:00Z
  Git Commit: e5f6g7h8

Alias:
  Type: legacy-google-id
  Value: 1a2b3c4d5e6f7890abcd
  Points To: UUID 550e8400-e29b-41d4-a716-446655440000
```

## UUID Discovery and Assignment

### Method 1: Document Declares UUID (Preferred)

**Markdown Frontmatter**:
```markdown
---
hermes-uuid: 550e8400-e29b-41d4-a716-446655440000
title: "RFC-001: API Gateway Design"
created: 2025-09-15
---

# API Gateway Design
...
```

**Google Doc Custom Properties**:
```json
{
  "customProperties": {
    "hermesUuid": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

### Method 2: Auto-Assignment (Discovery)

When Hermes indexes a document without a UUID:

1. **Generate UUID**: `uuid.New()`
2. **Write to document**:
   - Markdown: Add to frontmatter
   - Google Doc: Set custom property
   - Other: Store in metadata table
3. **Track in database**: Link UUID to provider document ID

### Method 3: Explicit Assignment (Migration)

During migration from legacy system:
```bash
# Assign UUID to existing Google Doc
hermes document assign-uuid \
  --google-doc-id=1a2b3c4d5e6f7890 \
  --uuid=550e8400-e29b-41d4-a716-446655440000

# Or auto-generate
hermes document assign-uuid \
  --google-doc-id=1a2b3c4d5e6f7890 \
  --auto
```

## Content Hash Calculation

### Purpose
Detect content changes without storing full document content in database.

### Algorithm
```go
func CalculateContentHash(doc *Document) string {
    // Normalize content
    normalized := NormalizeContent(doc.Content)
    
    // Include critical metadata
    hashInput := fmt.Sprintf("%s|%s|%s",
        normalized,
        doc.Title,
        doc.ModifiedTime.Format(time.RFC3339),
    )
    
    // SHA-256 hash
    hash := sha256.Sum256([]byte(hashInput))
    return fmt.Sprintf("sha256:%x", hash)
}

func NormalizeContent(content string) string {
    // Strip whitespace differences
    // Normalize line endings
    // Remove HTML comments (Google Docs)
    // Preserve semantic content only
    return normalized
}
```

### Hash Comparison

```go
func CompareRevisions(rev1, rev2 *Revision) ConflictStatus {
    if rev1.ContentHash == rev2.ContentHash {
        return NoConflict  // ✅ Content identical
    }
    
    // Check if one is ahead of the other
    if rev1.LastModified.After(rev2.LastModified) {
        return SourceAhead  // ⚠️ Source has newer changes
    } else {
        return TargetAhead  // ⚠️ Target has newer changes
    }
}
```

## Project Names and TLAs

### Problem: Not Globally Unique

In a distributed system, we **cannot enforce global uniqueness** of:
- **Friendly names**: "Request for Comments"
- **TLAs**: "RFC"

Multiple teams may use "RFC" for different purposes:
- Engineering RFCs
- Legal RFCs (Request for Comment on policy)
- Customer RFCs (Request for Change)

### Solution: Project ID is Unique, Names are Display-Only

```
Project ID: engineering-rfcs (UNIQUE, stable)
Friendly Name: Request for Comments (NOT unique, for display)
TLA: RFC (NOT unique, for display)

Project ID: legal-policy-rfcs (UNIQUE, stable)
Friendly Name: Request for Comments (SAME NAME, OK!)
TLA: RFC (SAME TLA, OK!)
```

**Document title generation**:
```
[{TLA}-{number}] {title}
[RFC-001] API Gateway Design
```

If TLA collision matters, qualify with project:
```
[Engineering RFC-001] API Gateway Design
[Legal RFC-001] Privacy Policy Review
```

## Many-to-Many Relationships

### Project ↔ Provider (Many-to-Many)

A project can have multiple providers:
```
Project: engineering-rfcs

Providers:
  1. google-workspace (source, migrating from)
  2. local-git (target, migrating to)
  3. remote-hermes (archive, read-only mirror)
```

### Document ↔ Revision (One-to-Many)

A document has multiple revisions:
```
Document UUID: 550e8400-...

Revisions:
  1. Provider: google, Project: rfcs-old, Status: archived
  2. Provider: local, Project: rfcs-new, Status: canonical
  3. Provider: remote, Project: rfcs-mirror, Status: stale
```

### Revision ↔ Project-Provider (Many-to-One)

Each revision belongs to one project-provider combination:
```
Revision 1:
  Document UUID: 550e8400-...
  Project: rfcs-old
  Provider: google-workspace
  Provider Doc ID: 1a2b3c4d5e6f
```

## Database Schema (Detailed)

### Tables

```sql
-- Documents (stable UUID identifier)
CREATE TABLE documents (
    id SERIAL PRIMARY KEY,
    document_uuid UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    title VARCHAR(500),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Projects (unique project ID)
CREATE TABLE projects (
    id SERIAL PRIMARY KEY,
    project_id VARCHAR(64) UNIQUE NOT NULL,
    title VARCHAR(200) NOT NULL,
    friendly_name VARCHAR(200),  -- NOT unique
    tla VARCHAR(10),  -- NOT unique
    description TEXT,
    status VARCHAR(20) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Project Providers (many-to-many, supports migration)
CREATE TABLE project_providers (
    id SERIAL PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(project_id),
    provider_type VARCHAR(50) NOT NULL,
    provider_config JSONB NOT NULL,
    migration_status VARCHAR(50) DEFAULT 'active',
    migration_started_at TIMESTAMP,
    migration_completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(project_id, provider_type, (provider_config->>'workspaceId'))
);

-- Document Revisions (provider-project-document-revision)
CREATE TABLE document_revisions (
    id SERIAL PRIMARY KEY,
    document_uuid UUID NOT NULL REFERENCES documents(document_uuid),
    project_id VARCHAR(64) NOT NULL REFERENCES projects(project_id),
    provider_type VARCHAR(50) NOT NULL,
    provider_document_id VARCHAR(500) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    last_modified TIMESTAMP NOT NULL,
    revision_type VARCHAR(50) DEFAULT 'detected',
    revision_metadata JSONB,
    status VARCHAR(50) DEFAULT 'active',
    indexed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(document_uuid, project_id, provider_type, provider_document_id)
);

-- Indexes
CREATE INDEX idx_document_revisions_uuid ON document_revisions(document_uuid);
CREATE INDEX idx_document_revisions_project ON document_revisions(project_id);
CREATE INDEX idx_document_revisions_status ON document_revisions(status);
CREATE INDEX idx_document_revisions_hash ON document_revisions(content_hash);

-- Get all revisions for a document
-- SELECT * FROM document_revisions WHERE document_uuid = '550e8400-...' ORDER BY last_modified DESC;

-- Find canonical revision
-- SELECT * FROM document_revisions WHERE document_uuid = '550e8400-...' AND status = 'canonical';

-- Detect conflicts (same UUID, different hash)
-- SELECT document_uuid, COUNT(DISTINCT content_hash) as hash_count
-- FROM document_revisions
-- WHERE status IN ('source', 'target')
-- GROUP BY document_uuid
-- HAVING COUNT(DISTINCT content_hash) > 1;
```

## Migration Workflow

### Step 1: Define Source and Target

In `projects.json`:
```json
{
  "projectId": "engineering-rfcs",
  "providers": [
    {
      "type": "google",
      "migrationStatus": "source",
      "migrationStartedAt": "2025-10-01T00:00:00Z",
      "config": { "workspaceId": "old-workspace" }
    },
    {
      "type": "local",
      "migrationStatus": "target",
      "migrationStartedAt": "2025-10-01T00:00:00Z",
      "config": { "workspacePath": "./rfcs" }
    }
  ]
}
```

### Step 2: Index Source Documents

```bash
hermes index --project=engineering-rfcs --provider=google
```

Hermes:
1. Discovers documents in Google Drive
2. Assigns UUIDs (or reads from custom properties)
3. Calculates content hash
4. Creates revision with `status=source`

### Step 3: Migrate Documents

```bash
hermes migrate documents \
  --project=engineering-rfcs \
  --from=google \
  --to=local \
  --batch-size=10
```

For each document:
1. Read content from Google
2. Convert format (Google Doc → Markdown)
3. Write to local Git repo with UUID in frontmatter
4. Calculate content hash
5. Create revision with `status=target`
6. Compare hashes
   - ✅ Match: Mark successful
   - ⚠️ Differ: Mark conflict

### Step 4: Monitor Progress

```bash
hermes migrate status --project=engineering-rfcs
```

Output:
```
Migration Status: engineering-rfcs
Source: google-workspace (drive-id: old-workspace)
Target: local-git (./rfcs)
Started: 2025-10-01

Progress:
  Total Documents: 347
  Migrated Successfully: 312 (90%)
  Conflicts Detected: 23 (7%)
  In Progress: 12 (3%)
  
Conflicts Requiring Resolution:
  - RFC-045: Source edited 2025-10-15 (newer than target)
  - RFC-078: Target has uncommitted changes
  - RFC-123: Content divergence detected
  ...
```

### Step 5: Resolve Conflicts

```bash
# Review specific conflict
hermes migrate conflict show --uuid=550e8400-...

# Resolve with strategy
hermes migrate conflict resolve \
  --uuid=550e8400-... \
  --strategy=source-wins \
  --note="Taking latest edits from Google Doc"
```

### Step 6: Complete Migration

```bash
hermes migrate complete --project=engineering-rfcs
```

Updates:
1. Mark source provider as `archived`
2. Mark target revisions as `canonical`
3. Keep old revisions for audit trail
4. Update search index

## UI Considerations

### Document View During Migration

```
┌─────────────────────────────────────────────────────────┐
│ RFC-045: API Gateway Design                             │
│ UUID: 550e8400-e29b-41d4-a716-446655440000             │
│                                                         │
│ ⚠️  Migration in Progress                               │
│                                                         │
│ This document exists in multiple locations:             │
│                                                         │
│ ✅ Local Git (rfcs-new)                                 │
│    docs/rfc-045.md                                      │
│    Last modified: 2025-10-01 09:00                      │
│    Status: Target (migration)                           │
│                                                         │
│ ⚠️  Google Drive (rfc-archive) - CONFLICT DETECTED      │
│    1a2b3c4d5e6f7890                                     │
│    Last modified: 2025-10-15 14:30 (NEWER!)             │
│    Status: Source (migration)                           │
│                                                         │
│ [Resolve Conflict]  [View Both]  [Migration Status]    │
└─────────────────────────────────────────────────────────┘
```

### Conflict Resolution UI

```
┌─────────────────────────────────────────────────────────┐
│ Resolve Conflict: RFC-045                               │
│                                                         │
│ Source (Google Drive) - Modified Oct 15, 14:30         │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ ## Design Changes                                   │ │
│ │ - Added new authentication flow                     │ │
│ │ - Updated API endpoints                             │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ Target (Local Git) - Modified Oct 1, 09:00             │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ ## Design Changes                                   │ │
│ │ - Initial design                                    │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ Resolution:                                             │
│ ○ Keep Source (Google Drive) - Take latest changes     │
│ ○ Keep Target (Local Git) - Discard Google changes     │
│ ○ Manual Merge - Show diff editor                      │
│                                                         │
│ [Cancel]  [Resolve]                                     │
└─────────────────────────────────────────────────────────┘
```

## Performance Considerations

### Content Hash Storage

- ✅ Store hash in database (64 bytes)
- ❌ Don't store full content in Postgres
- ✅ Use hash for quick comparison
- ✅ Fetch full content only when needed

### Indexing Strategy

```sql
-- Fast lookup by UUID
CREATE INDEX idx_revisions_uuid ON document_revisions(document_uuid);

-- Find conflicts
CREATE INDEX idx_revisions_conflict ON document_revisions(document_uuid, status) 
    WHERE status IN ('source', 'target', 'conflict');

-- Get canonical version
CREATE INDEX idx_revisions_canonical ON document_revisions(document_uuid) 
    WHERE status = 'canonical';
```

### Caching

```go
// Cache canonical revision per document
cache.Set(
    fmt.Sprintf("doc:canonical:%s", documentUUID),
    revision,
    5 * time.Minute,
)
```

## Security and Access Control

### UUID Exposure

- ✅ UUIDs are safe to expose publicly (random, not sequential)
- ✅ No information leakage from UUID
- ✅ Access control enforced at document level

### Provider Isolation

- ✅ Provider credentials isolated
- ✅ User can't access source if they don't have permission
- ✅ Migration operations require admin role

## Testing Strategy

### Unit Tests

```go
func TestContentHashCalculation(t *testing.T)
func TestRevisionComparison(t *testing.T)
func TestConflictDetection(t *testing.T)
func TestUUIDAssignment(t *testing.T)
```

### Integration Tests

```go
func TestMigrationWorkflow(t *testing.T)
func TestConflictResolution(t *testing.T)
func TestMultiProviderSync(t *testing.T)
```

### E2E Tests (Playwright)

```typescript
test('document migration creates revisions', async ({ page }) => { ... })
test('conflict detection shows warning', async ({ page }) => { ... })
test('resolve conflict updates canonical', async ({ page }) => { ... })
```

---

**Next Steps**: Implement revision tracking in Phase 2 of the roadmap (see `DISTRIBUTED_PROJECTS_ROADMAP.md`).
