---
id: rfc-021
title: Document Identification System (DocID)
status: Implemented
created: 2025-10-26
author: Hermes Team
project_id: hermes
doc_uuid: 3e7af144-9069-41dc-adca-ba16e6172960
type: RFC
subtype: Architecture
tags: [document-id, uuid, distributed, multi-provider, migration]
supersedes: ADR-018 (narrative content)
related: [ADR-018, RFC-017]
---

# RFC-021: Document Identification System (DocID)

> Type hierarchy, format examples, migration phases, performance benchmarks, and use cases for the `pkg/docid` system. The binding architectural rule lives in [ADR-018](../adr/adr-018-document-identification-system.md); this RFC preserves the narrative.

## What is a DocID?

A **DocID** is a type-safe, composable document identifier that uniquely identifies documents across multiple storage providers (Google Workspace, local filesystem, remote Hermes instances). It consists of three components:

```text
┌─────────────────────────────────────────────────────────────┐
│                      Composite DocID                         │
├─────────────────────┬─────────────────────┬─────────────────┤
│   UUID              │   ProviderID        │   Project       │
│   (Stable Global)   │   (Backend-Specific)│   (Context)     │
├─────────────────────┼─────────────────────┼─────────────────┤
│ 550e8400-e29b-...   │ google:1a2b3c4d     │ rfc-archive     │
└─────────────────────┴─────────────────────┴─────────────────┘

Visual representation:
  UUID           Provider           ID              Project
   │                │                │                 │
   ▼                ▼                ▼                 ▼
uuid:550e8400-...:provider:google:id:1a2b3c4d:project:rfc-archive
```

At a glance:
- **UUID**: never changes; identifies the logical document.
- **ProviderID**: where the document is stored (`google`, `local`, `remote-hermes`).
- **Project**: which project configuration applies.

## Why DocID?

### Before

```text
Document Table
┌──────┬───────────────┬─────────────┐
│ ID   │ GoogleFileID  │ Title       │
├──────┼───────────────┼─────────────┤
│ 1    │ "1a2b3c4d"    │ "RFC-001"   │  Tied to Google
│ 2    │ "5e6f7g8h"    │ "RFC-002"   │  Can't migrate
└──────┴───────────────┴─────────────┘
```

Problems: GoogleFileID assumes Google Workspace only; no stable ID across provider migrations; can't track the same document in multiple locations; no way to detect content drift; provider-locked architecture.

### After

```text
Document Table
┌──────┬────────────────────┬──────────────┬──────────────┬─────────────┐
│ ID   │ DocumentUUID       │ ProviderType │ ProviderID   │ Title       │
├──────┼────────────────────┼──────────────┼──────────────┼─────────────┤
│ 1    │ 550e8400-e29b-...  │ google       │ 1a2b3c4d     │ "RFC-001"   │
│ 2    │ 550e8400-e29b-...  │ local        │ docs/rfc.md  │ "RFC-001"   │
└──────┴────────────────────┴──────────────┴──────────────┴─────────────┘
         ▲
         └─ Same UUID = Same logical document, different storage
```

Benefits: stable UUID survives migrations; multiple providers; track revisions across providers; detect content drift; provider-agnostic.

## Architecture Overview

### Type Hierarchy

```text
pkg/docid/
├── UUID                    ← Stable, globally unique
│   └── Methods: NewUUID, ParseUUID, String, IsZero, Equal
│
├── ProviderID              ← Backend-specific identifier
│   ├── provider: ProviderType ("google" | "local" | "remote-hermes")
│   └── id: string
│   └── Methods: NewProviderID, GoogleFileID, LocalFileID, ParseProviderID, String
│
└── CompositeID             ← Full document reference
    ├── uuid: UUID
    ├── providerID: ProviderID
    └── project: string
    └── Methods: NewCompositeID, NewCompositeIDFromUUID, ParseCompositeID,
                 ShortString, String, URIString, IsComplete, HasProvider
```

### ID Formats

```text
1. Short Format (API URLs, most common):
   uuid/550e8400-e29b-41d4-a716-446655440000
   Use: GET /api/v2/documents/uuid/550e8400-...

2. Full Format (Complete info):
   uuid:550e8400-...:provider:google:id:1a2b3c4d:project:rfcs
   Use: Internal processing, logs, debugging

3. URI Format (Query parameters):
   uuid/550e8400-...?provider=google&id=1a2b3c4d&project=rfc-archive
   Use: Web URLs, link sharing
```

### Data Flow

```text
Client Request: GET /api/v2/documents/uuid/550e8400-...
        │
        ▼
API Layer (v2/documents.go)
  1. Extract ID string from URL
  2. compositeID := docid.ParseCompositeID(idStr)
  3. uuid := compositeID.UUID()
        │
        ▼
Database Layer (models/document.go)
  SELECT * FROM documents WHERE document_uuid = ?
        │
        ▼
Workspace Provider (workspace/google.go)
  1. providerID = doc.ProviderType + doc.ProviderID
  2. provider.GetFile(providerID.ID())
  3. Return workspace.Document with CompositeID
```

## Implementation Details

### UUID

Stable, globally unique document identifier that persists across migrations.

```go
type UUID struct {
    value uuid.UUID
}

uuid := docid.NewUUID()
uuid, err := docid.ParseUUID("550e8400-e29b-41d4-a716-446655440000")
uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")

type Document struct {
    DocumentUUID docid.UUID `gorm:"type:uuid;uniqueIndex"`
}

json.Marshal(uuid) // → "550e8400-e29b-41d4-a716-446655440000"
```

Features: type-safe wrapper around `github.com/google/uuid`; implements `sql.Scanner`/`driver.Valuer`; implements `json.Marshaler`/`json.Unmarshaler`; thread-safe; zero-value support; 100% test coverage (13 cases).

### ProviderID

Identifies which storage backend holds the document.

```go
const (
    ProviderTypeGoogle       ProviderType = "google"
    ProviderTypeLocal        ProviderType = "local"
    ProviderTypeRemoteHermes ProviderType = "remote-hermes"
)

googleID, err := docid.GoogleFileID("1a2b3c4d5e6f7890")
localID, err := docid.LocalFileID("docs/rfc-001.md")
remoteID, err := docid.RemoteHermesID("https://hermes.example.com/docs/123")

providerID, err := docid.ParseProviderID("google:1a2b3c4d5e6f7890")
provider := providerID.Provider()  // "google"
id := providerID.ID()              // "1a2b3c4d5e6f7890"
```

Provider-migration example:

```text
OLD: google:1a2b3c4d5e6f7890
       │  (migrate to local)
       ▼
NEW: local:docs/archived/rfc-001.md
(Same UUID, different ProviderID)
```

### CompositeID

Full document reference with UUID, provider, and project context.

```go
uuid := docid.NewUUID()
providerID, _ := docid.GoogleFileID("1a2b3c4d")
compositeID := docid.NewCompositeID(uuid, providerID, "rfc-archive")

// UUID-only (most common)
compositeID := docid.NewCompositeIDFromUUID(uuid)

// Parse from various formats
id, err := docid.ParseCompositeID("uuid/550e8400-e29b-41d4-a716-446655440000")
id, err := docid.ParseCompositeID("uuid:550e8400...:provider:google:id:1a2b3c4d")

short := id.ShortString()   // "uuid/550e8400-..."
full := id.String()         // "uuid:550e8400...:provider:google:id:1a2b3c4d:project:rfcs"
uri := id.URIString()       // "uuid/550e8400-...?provider=google&id=1a2b3c4d&project=rfcs"
```

Completeness levels:

- **Level 1 (UUID only)** — lookup document by stable ID.
- **Level 2 (UUID + Provider)** — fetch from specific backend.
- **Level 3 (Complete)** — full document identification with project context.

## Migration Strategy

### Phase 1: Package Creation (Complete)

- Core types implemented (UUID, ProviderID, CompositeID)
- UUID tests complete (13 cases, 100% coverage)
- ProviderID tests pending (~30 cases)
- CompositeID tests pending (~40 cases)

### Phase 2: Database Schema Migration (Non-Breaking)

```sql
ALTER TABLE documents ADD COLUMN document_uuid UUID;
CREATE UNIQUE INDEX idx_documents_uuid
  ON documents(document_uuid)
  WHERE document_uuid IS NOT NULL;

ALTER TABLE documents ADD COLUMN provider_type VARCHAR(50);
ALTER TABLE documents ADD COLUMN project_id VARCHAR(64);

-- Keep google_file_id for backward compatibility during migration.
```

### Phase 3: UUID Assignment (Background Job)

```go
func assignUUIDs(db *gorm.DB, workspace workspace.Provider) error {
    var docs []models.Document
    db.Where("document_uuid IS NULL").Find(&docs)

    for _, doc := range docs {
        uuid := docid.NewUUID()
        doc.DocumentUUID = uuid
        doc.ProviderType = "google"
        doc.ProviderID = doc.GoogleFileID
        db.Save(&doc)

        workspace.SetDocumentProperty(doc.GoogleFileID, "hermesUuid", uuid.String())
    }
    return nil
}
```

Estimated rate: ~100 docs/hour (API rate limits).

### Phase 4: API Support (Backward Compatible)

```go
func resolveDocumentID(idStr string) (*models.Document, error) {
    if strings.HasPrefix(idStr, "uuid/") {
        cid, err := docid.ParseCompositeID(idStr)
        if err == nil {
            return findByUUID(cid.UUID())
        }
    }
    return findByGoogleFileID(idStr) // legacy
}
```

Both formats are accepted; no breaking changes.

### Phase 5: Workspace Adapter Updates

```go
type Document struct {
    ID          string             // Legacy (deprecated)
    CompositeID docid.CompositeID  // New
    Name        string
    Content     string
}

func (a *GoogleAdapter) GetDocument(id string) (*workspace.Document, error) {
    cid, err := a.resolveCompositeID(id)
    return &workspace.Document{
        ID:          id,
        CompositeID: cid,
    }, nil
}
```

### Phase 6: Frontend Integration

```text
Old URL: /document/1a2b3c4d5e6f7890
   ↓ (301 redirect)
New URL: /document/uuid/550e8400-e29b-41d4-a716-446655440000
```

## Use Cases

### Create New Document

```go
uuid := docid.NewUUID()
googleID, _ := docid.GoogleFileID("1a2b3c4d5e6f7890")
compositeID := docid.NewCompositeID(uuid, googleID, "rfc-archive")

doc := models.Document{
    DocumentUUID: uuid,
    ProviderType: "google",
    ProviderID:   googleID.ID(),
    ProjectID:    "rfc-archive",
    GoogleFileID: googleID.ID(), // legacy compatibility
}
db.Create(&doc)

return map[string]string{
    "id":  compositeID.ShortString(),
    "uri": compositeID.URIString(),
}
```

### Migrate Document to Local Storage

```go
originalDoc := models.Document{
    DocumentUUID: docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000"),
    ProviderType: "google",
    ProviderID:   "1a2b3c4d5e6f7890",
}

localID, _ := docid.LocalFileID("docs/archived/rfc-001.md")
localComposite := docid.NewCompositeID(
    originalDoc.DocumentUUID, localID, "rfc-archive-local",
)

// Both share the same UUID; content-drift detection via SHA256 comparison.
```

### Lookup by UUID (Cross-Provider)

```go
compositeID, err := docid.ParseCompositeID(idStr)

var doc models.Document
db.Where("document_uuid = ?", compositeID.UUID()).First(&doc)

var allRevisions []models.Document
db.Where("document_uuid = ?", compositeID.UUID()).Find(&allRevisions)
// Returns all provider locations for the same logical document.
```

## Testing Strategy

Categories: unit (parsing/validation, serialization, DB integration, edge cases), integration (schema migration, API compatibility), performance benchmarks.

Current status: UUID 100% coverage; ProviderID/CompositeID tests pending; target 95%+ overall.

## Performance Characteristics

```text
Operation                    Latency    Memory      Notes
──────────────────────────────────────────────────────────────
UUID.NewUUID()              ~100ns     16 bytes    Standard generation
UUID.ParseUUID()            <1µs       16 bytes    String parsing
UUID.String()               <1µs       36 bytes    Canonical format

ProviderID.NewProviderID()  <100ns     ~40 bytes   Type validation
ProviderID.ParseProviderID() <1µs      ~40 bytes   "type:id" parsing
ProviderID.String()         <1µs       ~40 bytes   Format "type:id"

CompositeID.New()           <200ns     ~100 bytes  Struct allocation
CompositeID.ParseShort()    <2µs       ~100 bytes  "uuid/..." parsing
CompositeID.ParseFull()     <5µs       ~100 bytes  Full format parsing
CompositeID.ShortString()   <2µs       ~50 bytes   "uuid/..." format

JSON.Marshal(UUID)          ~2µs       ~80 bytes
JSON.Unmarshal(UUID)        ~3µs       ~80 bytes

Database.Scan(UUID)         <1µs       16 bytes    sql.Scanner
Database.Value(UUID)        <1µs       36 bytes    driver.Valuer
```

Benchmarks:

```text
BenchmarkUUID_Parse-8       2000000   0.5 µs/op   16 B/op   1 allocs/op
BenchmarkUUID_String-8      5000000   0.3 µs/op   36 B/op   1 allocs/op
BenchmarkJSON_Marshal-8      500000   2.1 µs/op   80 B/op   2 allocs/op
```

## Success Criteria

- Test coverage 95%+
- ID parsing <5µs per operation
- <100 bytes per CompositeID instance
- 100% backward compatible API
- 100% UUID assignment for existing documents
- Zero downtime during rollout
- Content-drift detection within 1 hour
- Multi-provider support (local + remote)
- 50% of API calls using UUID format within 1 month

## Risks & Mitigations

- **Type Safety** — strong types prevent ID mixing; compiler-enforced.
- **Backward Compatibility** — phased rollout, dual-format support.
- **Database Migration** — background job, idempotent, resumable, rate-limited.
- **API Compatibility** — clear deprecation timeline (12+ months), extensive tests.
- **Search Index Updates** — maintain aliases during transition, gradual cutover.

## Code Locations

- Package: `pkg/docid/`
- Tests: `pkg/docid/*_test.go`
- Models: `pkg/models/document.go`
- API: `internal/api/v2/documents.go`
- Workspace: `pkg/workspace/types.go`

External dependencies: `github.com/google/uuid`, `gorm.io/gorm`, standard library.

The binding architectural rule that came out of this work is recorded in [ADR-018](../adr/adr-018-document-identification-system.md).