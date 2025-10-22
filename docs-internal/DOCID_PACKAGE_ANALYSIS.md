# Document ID Package - Feasibility Analysis

**Status**: ✅ Feasible with Phased Approach  
**Version**: 1.0.0-alpha  
**Created**: October 22, 2025  
**Related**: `DISTRIBUTED_PROJECTS_ARCHITECTURE.md`, `DOCUMENT_REVISIONS_AND_MIGRATION.md`

## Executive Summary

**Recommendation**: ✅ **Proceed with new `pkg/docid` package**

The new composite document ID system is **feasible and necessary** for the distributed projects architecture. It can be implemented incrementally with backward compatibility, minimal disruption to existing code, and clear migration path.

**Key Finding**: The current codebase uses `GoogleFileID` as a simple `string` throughout, making it **easy to introduce a new type-safe system** alongside the existing implementation.

---

## Current State Analysis

### 1. Document Identification Today

**Primary Identifier**: `models.Document.GoogleFileID` (string)

```go
type Document struct {
    gorm.Model
    GoogleFileID string `gorm:"index;not null;unique"`
    // ... other fields
}
```

**Usage Patterns**:
- ✅ **Database**: Single `google_file_id` column (string, unique, indexed)
- ✅ **API URLs**: `/api/v2/documents/{google-file-id}`
- ✅ **Search Index**: `objectID` field (Algolia/Meilisearch)
- ✅ **Workspace Layer**: `ID string` field in `workspace.Document`
- ✅ **Internal Logic**: Passed as `string` parameters everywhere

**Key Discovery**: ✅ **No complex ID parsing logic exists**. IDs are treated as opaque strings, making it safe to introduce structured IDs.

### 2. Usage Across Layers

#### Database Layer (`pkg/models/`)
```go
// All CRUD operations use GoogleFileID
func (d *Document) Get(db *gorm.DB) error {
    // Requires either d.ID (serial) or d.GoogleFileID (string)
    validation.Field(&d.GoogleFileID,
        validation.When(d.ID == 0,
            validation.Required.Error("either ID or GoogleFileID is required")))
}

func (d *Document) Upsert(db *gorm.DB) error {
    // Upsert by GoogleFileID
    tx.Where(Document{GoogleFileID: d.GoogleFileID}).
       FirstOrCreate(&d)
}
```

**Finding**: Database layer is **flexible** - it accepts any string as `GoogleFileID`. Can work with composite IDs if they serialize to strings.

#### API Layer (`internal/api/v2/`)
```go
// URL parsing extracts document ID as string
docID, reqType, err := parseDocumentsURLPath(r.URL.Path, "documents")

// Lookup in database
model := models.Document{
    GoogleFileID: docID,
}
if err := model.Get(srv.DB); err != nil {
    // Handle error
}
```

**Finding**: API layer is **agnostic** to ID format. Extracts string from URL, passes to database. No parsing of ID structure.

#### Workspace Layer (`pkg/workspace/`)
```go
// Storage-agnostic document
type Document struct {
    ID string  // Provider-specific identifier
    Name string
    Content string
    // ...
}

// Provider interface
type Provider interface {
    GetFile(fileID string) (*drive.File, error)
    GetDoc(fileID string) (*docs.Document, error)
    // All methods accept string IDs
}
```

**Finding**: Workspace layer uses **simple string IDs**. Each provider interprets IDs differently:
- **Google**: Google Drive file ID (e.g., `1a2b3c4d5e6f7890`)
- **Local**: File path (e.g., `docs/rfc-001.md`)
- **Future Remote**: Hermes URI (e.g., `https://hermes.example.com/api/v2/documents/{id}`)

#### Search Layer (`pkg/search/`)
```go
// Search index uses objectID
type DocumentIndex interface {
    GetObject(ctx context.Context, objectID string) (map[string]any, error)
    SaveObject(ctx context.Context, object map[string]any) error
}

// Algolia compatibility
algoDoc := map[string]any{
    "objectID": doc.GoogleFileID,  // String field
    "title": doc.Title,
    // ...
}
```

**Finding**: Search uses **objectID as string**. Can be composite ID if properly serialized.

### 3. Migration Challenges

**Current Issues**:
1. ❌ **No UUID tracking**: Documents have `gorm.Model.ID` (serial) but no stable UUID
2. ❌ **No provider tracking**: `GoogleFileID` assumes Google Workspace
3. ❌ **No project association**: Documents don't explicitly track which project they belong to (uses Product instead)
4. ❌ **No revision tracking**: No way to track same document across multiple providers
5. ❌ **No conflict detection**: Can't detect content divergence during migration

**Distributed Architecture Needs**:
1. ✅ **Stable UUID**: Persists across migrations and provider changes
2. ✅ **Provider identification**: Know which backend stores this document
3. ✅ **Project context**: Which project configuration applies
4. ✅ **Revision tracking**: Multiple representations of same document
5. ✅ **Content hashing**: Detect drift and conflicts

---

## Proposed Solution: `pkg/docid` Package

### Design Principles

1. **Type Safety**: Strong types prevent mixing UUID, provider IDs, and composite IDs
2. **Backward Compatible**: Can work with existing `GoogleFileID` strings
3. **Serializable**: Can be stored in database, URLs, JSON
4. **Parseable**: Can construct from various string formats
5. **Extensible**: Easy to add new provider types

### Core Types

#### 1. UUID (Stable Global Identifier)
```go
package docid

import "github.com/google/uuid"

// UUID is a stable, globally unique document identifier.
// This persists across provider migrations and represents the logical document.
type UUID struct {
    value uuid.UUID
}

// NewUUID generates a new random UUID.
func NewUUID() UUID {
    return UUID{value: uuid.New()}
}

// ParseUUID parses a UUID from string (e.g., "550e8400-e29b-41d4-a716-446655440000").
func ParseUUID(s string) (UUID, error) {
    u, err := uuid.Parse(s)
    if err != nil {
        return UUID{}, fmt.Errorf("invalid UUID: %w", err)
    }
    return UUID{value: u}, nil
}

// String returns the canonical UUID string.
func (u UUID) String() string {
    return u.value.String()
}

// IsZero returns true if this is the zero UUID.
func (u UUID) IsZero() bool {
    return u.value == uuid.Nil
}
```

#### 2. ProviderID (Backend-Specific Identifier)
```go
// ProviderType identifies the storage backend.
type ProviderType string

const (
    ProviderTypeGoogle       ProviderType = "google"
    ProviderTypeLocal        ProviderType = "local"
    ProviderTypeRemoteHermes ProviderType = "remote-hermes"
)

// ProviderID represents a document's identifier within a specific provider.
type ProviderID struct {
    provider ProviderType
    id       string
}

// NewProviderID creates a provider-specific ID.
func NewProviderID(provider ProviderType, id string) (ProviderID, error) {
    if err := validateProvider(provider); err != nil {
        return ProviderID{}, err
    }
    if id == "" {
        return ProviderID{}, fmt.Errorf("provider ID cannot be empty")
    }
    return ProviderID{provider: provider, id: id}, nil
}

// GoogleFileID creates a Google Drive file ID.
func GoogleFileID(id string) (ProviderID, error) {
    return NewProviderID(ProviderTypeGoogle, id)
}

// LocalFileID creates a local filesystem ID (file path).
func LocalFileID(path string) (ProviderID, error) {
    return NewProviderID(ProviderTypeLocal, path)
}

// Provider returns the provider type.
func (p ProviderID) Provider() ProviderType {
    return p.provider
}

// ID returns the provider-specific identifier.
func (p ProviderID) ID() string {
    return p.id
}

// String returns the canonical string representation: "provider:id"
func (p ProviderID) String() string {
    return fmt.Sprintf("%s:%s", p.provider, p.id)
}

// ParseProviderID parses a provider ID from string (e.g., "google:1a2b3c4d").
func ParseProviderID(s string) (ProviderID, error) {
    parts := strings.SplitN(s, ":", 2)
    if len(parts) != 2 {
        return ProviderID{}, fmt.Errorf("invalid provider ID format (expected 'provider:id'): %s", s)
    }
    return NewProviderID(ProviderType(parts[0]), parts[1])
}
```

#### 3. CompositeID (Full Document Reference)
```go
// CompositeID is a fully-qualified document identifier containing:
//   - UUID: Stable global identifier
//   - Provider: Which backend stores this document
//   - ProviderID: Backend-specific identifier
//   - Project: Which project configuration applies (optional during lookup)
type CompositeID struct {
    uuid       UUID
    providerID ProviderID
    project    string // Optional project ID
}

// NewCompositeID creates a new composite ID.
func NewCompositeID(uuid UUID, providerID ProviderID, project string) CompositeID {
    return CompositeID{
        uuid:       uuid,
        providerID: providerID,
        project:    project,
    }
}

// UUID returns the stable document UUID.
func (c CompositeID) UUID() UUID {
    return c.uuid
}

// ProviderID returns the provider-specific identifier.
func (c CompositeID) ProviderID() ProviderID {
    return c.providerID
}

// Project returns the project ID (may be empty).
func (c CompositeID) Project() string {
    return c.project
}

// String returns a canonical string representation.
// Format: "uuid:{uuid}:provider:{provider}:id:{id}:project:{project}"
func (c CompositeID) String() string {
    s := fmt.Sprintf("uuid:%s:provider:%s:id:%s",
        c.uuid.String(),
        c.providerID.Provider(),
        c.providerID.ID())
    if c.project != "" {
        s += fmt.Sprintf(":project:%s", c.project)
    }
    return s
}

// ShortString returns a human-readable short form.
// Format: "uuid/{uuid}" (most common use case)
func (c CompositeID) ShortString() string {
    return fmt.Sprintf("uuid/%s", c.uuid.String())
}

// URIString returns a URI-safe format for URLs.
// Format: "uuid/{uuid}?provider={provider}&id={id}&project={project}"
func (c CompositeID) URIString() string {
    u := url.URL{
        Path: fmt.Sprintf("uuid/%s", c.uuid.String()),
    }
    q := u.Query()
    q.Set("provider", string(c.providerID.Provider()))
    q.Set("id", c.providerID.ID())
    if c.project != "" {
        q.Set("project", c.project)
    }
    u.RawQuery = q.Encode()
    return u.String()
}
```

### Parsing and Validation

```go
// ParseCompositeID parses a composite ID from various formats.
// Supports:
//   - Full format: "uuid:{uuid}:provider:{provider}:id:{id}:project:{project}"
//   - UUID-only: "uuid/{uuid}" or just "{uuid}"
//   - Provider-only: "provider:{provider}:id:{id}"
func ParseCompositeID(s string) (CompositeID, error) {
    // Try UUID-only format first (most common)
    if strings.HasPrefix(s, "uuid/") {
        uuidStr := strings.TrimPrefix(s, "uuid/")
        uuid, err := ParseUUID(uuidStr)
        if err != nil {
            return CompositeID{}, err
        }
        return CompositeID{uuid: uuid}, nil
    }

    // Try full composite format
    if strings.Contains(s, ":") {
        return parseFullCompositeID(s)
    }

    // Assume bare UUID string
    uuid, err := ParseUUID(s)
    if err != nil {
        return CompositeID{}, err
    }
    return CompositeID{uuid: uuid}, nil
}

// parseFullCompositeID parses the full "uuid:...:provider:...:id:..." format.
func parseFullCompositeID(s string) (CompositeID, error) {
    // Implementation: Parse key:value pairs
    // ...
}
```

### Database Integration

```go
// Scan implements sql.Scanner for database reading.
func (u *UUID) Scan(value interface{}) error {
    if value == nil {
        *u = UUID{}
        return nil
    }
    
    switch v := value.(type) {
    case string:
        parsed, err := ParseUUID(v)
        if err != nil {
            return err
        }
        *u = parsed
        return nil
    case []byte:
        parsed, err := ParseUUID(string(v))
        if err != nil {
            return err
        }
        *u = parsed
        return nil
    default:
        return fmt.Errorf("cannot scan %T into UUID", value)
    }
}

// Value implements driver.Valuer for database writing.
func (u UUID) Value() (driver.Value, error) {
    if u.IsZero() {
        return nil, nil
    }
    return u.String(), nil
}
```

### JSON Serialization

```go
// MarshalJSON implements json.Marshaler.
func (c CompositeID) MarshalJSON() ([]byte, error) {
    return json.Marshal(map[string]any{
        "uuid":     c.uuid.String(),
        "provider": c.providerID.Provider(),
        "id":       c.providerID.ID(),
        "project":  c.project,
    })
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *CompositeID) UnmarshalJSON(data []byte) error {
    var obj struct {
        UUID     string `json:"uuid"`
        Provider string `json:"provider"`
        ID       string `json:"id"`
        Project  string `json:"project"`
    }
    if err := json.Unmarshal(data, &obj); err != nil {
        return err
    }
    // Parse and validate
    // ...
}
```

---

## Integration Strategy

### Phase 1: New Package Creation (Non-Breaking)

1. ✅ Create `pkg/docid/` package
2. ✅ Implement core types with 95%+ test coverage
3. ✅ Add to go.mod dependencies
4. ✅ Document API with examples

**Impact**: None - new package, no integration yet

### Phase 2: Database Schema Addition (Non-Breaking)

Add new nullable columns to `documents` table:

```sql
ALTER TABLE documents ADD COLUMN document_uuid UUID;
CREATE UNIQUE INDEX idx_documents_uuid ON documents(document_uuid) WHERE document_uuid IS NOT NULL;

ALTER TABLE documents ADD COLUMN provider_type VARCHAR(50);
ALTER TABLE documents ADD COLUMN project_id VARCHAR(64);

-- Keep google_file_id as-is (for backward compatibility)
```

**Migration Strategy**:
- ✅ New columns nullable (existing records work)
- ✅ `google_file_id` remains primary lookup key
- ✅ Gradual population via background job

### Phase 3: UUID Assignment (Background)

Create indexer job to assign UUIDs:

```go
// For existing Google Docs
for _, doc := range existingDocuments {
    if doc.DocumentUUID == nil {
        uuid := docid.NewUUID()
        doc.DocumentUUID = &uuid
        doc.ProviderType = "google"
        doc.Update(db)
        
        // Write UUID to Google Doc custom properties
        workspace.SetDocumentProperty(doc.GoogleFileID, "hermesUuid", uuid.String())
    }
}
```

**Migration Time**: ~1 hour for 1000 documents (API rate limits)

### Phase 4: API Support (Backward Compatible)

Update API to accept both formats:

```go
// Old format still works
GET /api/v2/documents/1a2b3c4d5e6f7890

// New format also works
GET /api/v2/documents/uuid/550e8400-e29b-41d4-a716-446655440000

// Resolution logic
func resolveDocumentID(idStr string) (*models.Document, error) {
    // Try parsing as composite ID
    if strings.HasPrefix(idStr, "uuid/") {
        cid, err := docid.ParseCompositeID(idStr)
        if err == nil {
            return findByUUID(cid.UUID())
        }
    }
    
    // Fall back to GoogleFileID lookup
    return findByGoogleFileID(idStr)
}
```

### Phase 5: Workspace Adapter Updates

Update adapters to return composite IDs:

```go
// workspace.Document now has composite ID
type Document struct {
    ID         string        // Legacy string (deprecated)
    CompositeID docid.CompositeID  // New composite ID
    Name       string
    // ...
}

// Google adapter
func (a *GoogleAdapter) GetDocument(id string) (*workspace.Document, error) {
    // id can be: GoogleFileID or UUID
    cid, err := a.resolveCompositeID(id)
    
    doc := &workspace.Document{
        ID: id,  // Legacy
        CompositeID: cid,  // New
        // ...
    }
}
```

### Phase 6: Frontend Updates

Update frontend to use UUIDs in URLs:

```typescript
// Old URLs still work (redirect)
/document/1a2b3c4d5e6f7890

// New URLs preferred
/document/uuid/550e8400-e29b-41d4-a716-446655440000

// Router resolves both
this.router.transitionTo('document', docId);
```

---

## Risk Assessment

### Low Risk ✅

1. **Type Safety**: Strong types prevent ID mixing
2. **Backward Compatibility**: Existing code continues working
3. **Gradual Migration**: Phased approach minimizes disruption
4. **Testing**: Comprehensive unit tests catch issues early

### Medium Risk ⚠️

1. **Database Migration**: Large-scale UUID assignment
   - **Mitigation**: Background job, idempotent, resumable
   
2. **API Compatibility**: Supporting both ID formats
   - **Mitigation**: Clear deprecation timeline, extensive testing

3. **Search Index Updates**: Updating objectID format
   - **Mitigation**: Maintain aliases, gradual cutover

### High Risk ❌

None identified - all risks have clear mitigations.

---

## Test Coverage Requirements

### Unit Tests (Target: 95%+)

1. ✅ UUID parsing and validation
2. ✅ ProviderID construction for all provider types
3. ✅ CompositeID string serialization/parsing
4. ✅ JSON marshaling/unmarshaling
5. ✅ Database scanning and valuing
6. ✅ Error handling for invalid inputs
7. ✅ Edge cases (empty strings, nil values, malformed input)

### Integration Tests

1. ✅ Database round-trip (insert UUID, read back)
2. ✅ API resolution (UUID lookup, legacy lookup)
3. ✅ Workspace adapter integration
4. ✅ Search index compatibility

### Example Test Cases

```go
func TestUUID(t *testing.T) {
    t.Run("new UUID is valid", func(t *testing.T) {
        u := docid.NewUUID()
        assert.False(t, u.IsZero())
        assert.Len(t, u.String(), 36)  // Standard UUID format
    })
    
    t.Run("parse valid UUID", func(t *testing.T) {
        u, err := docid.ParseUUID("550e8400-e29b-41d4-a716-446655440000")
        require.NoError(t, err)
        assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", u.String())
    })
    
    t.Run("parse invalid UUID", func(t *testing.T) {
        _, err := docid.ParseUUID("not-a-uuid")
        assert.Error(t, err)
    })
}

func TestProviderID(t *testing.T) {
    t.Run("Google file ID", func(t *testing.T) {
        pid, err := docid.GoogleFileID("1a2b3c4d5e6f7890")
        require.NoError(t, err)
        assert.Equal(t, docid.ProviderTypeGoogle, pid.Provider())
        assert.Equal(t, "1a2b3c4d5e6f7890", pid.ID())
        assert.Equal(t, "google:1a2b3c4d5e6f7890", pid.String())
    })
    
    t.Run("Local file path", func(t *testing.T) {
        pid, err := docid.LocalFileID("docs/rfc-001.md")
        require.NoError(t, err)
        assert.Equal(t, docid.ProviderTypeLocal, pid.Provider())
        assert.Equal(t, "docs/rfc-001.md", pid.ID())
    })
}

func TestCompositeID(t *testing.T) {
    t.Run("full composite ID", func(t *testing.T) {
        uuid := docid.NewUUID()
        pid, _ := docid.GoogleFileID("1a2b3c4d")
        cid := docid.NewCompositeID(uuid, pid, "rfc-archive")
        
        assert.Equal(t, uuid, cid.UUID())
        assert.Equal(t, pid, cid.ProviderID())
        assert.Equal(t, "rfc-archive", cid.Project())
    })
    
    t.Run("parse UUID-only format", func(t *testing.T) {
        cid, err := docid.ParseCompositeID("uuid/550e8400-e29b-41d4-a716-446655440000")
        require.NoError(t, err)
        assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cid.UUID().String())
    })
}
```

---

## Success Criteria

### Technical Metrics

1. ✅ **Test Coverage**: 95%+ unit test coverage
2. ✅ **Performance**: ID parsing <1µs per operation
3. ✅ **Memory**: <100 bytes per CompositeID instance
4. ✅ **API Compatibility**: 100% backward compatible

### Business Metrics

1. ✅ **Migration Success**: 100% documents assigned UUIDs
2. ✅ **Zero Downtime**: No service interruption during rollout
3. ✅ **Conflict Detection**: Identify conflicts within 1 hour
4. ✅ **User Experience**: No breaking changes to existing workflows

---

## Next Steps

### Immediate (This PR)

1. ✅ Create `pkg/docid/` package
2. ✅ Implement core types (UUID, ProviderID, CompositeID)
3. ✅ Write comprehensive unit tests
4. ✅ Document API with examples

### Short-Term (Next 2 Weeks)

1. ⏳ Database schema migration (add columns)
2. ⏳ UUID assignment job (background)
3. ⏳ API dual-format support

### Medium-Term (Next 1-2 Months)

1. ⏳ Workspace adapter updates
2. ⏳ Frontend integration
3. ⏳ Migration tracking UI

### Long-Term (Next 3-6 Months)

1. ⏳ Full migration to UUIDs
2. ⏳ Deprecate GoogleFileID-only APIs
3. ⏳ Multi-provider document support

---

## Conclusion

✅ **Recommendation: Proceed with Implementation**

The `pkg/docid` package is:
- ✅ **Necessary** for distributed projects architecture
- ✅ **Feasible** with backward compatibility
- ✅ **Low Risk** with phased approach
- ✅ **Well-Scoped** with clear boundaries
- ✅ **Testable** with comprehensive unit tests

**Confidence Level**: 95% - This is a well-understood problem with proven patterns from other distributed systems (Git, IPFS, blockchain). The gradual migration strategy minimizes risk.

---

## References

- **Architecture**: `DISTRIBUTED_PROJECTS_ARCHITECTURE.md`
- **Migration Design**: `DOCUMENT_REVISIONS_AND_MIGRATION.md`
- **Current Code**: `pkg/models/document.go`, `pkg/workspace/types.go`
- **API Patterns**: `internal/api/v2/documents.go`
