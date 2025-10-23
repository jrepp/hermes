# UUID-Based Document Identification Migration Guide

## Overview

This guide documents the migration from GoogleFileID-only identification to a hybrid UUID + GoogleFileID system. The migration enables:

- **Global unique identification** across providers (Google Workspace, Local, RemoteHermes)
- **Type-safe document IDs** with validation and serialization
- **Backward compatibility** with existing GoogleFileID-based code
- **Zero-downtime migration** through dual lookup mechanisms

## Architecture

### New Package: `pkg/docid`

Three core types for document identification:

1. **UUID** (`docid.UUID`)
   - Global unique identifier for documents
   - Database type: `uuid` (PostgreSQL native)
   - Serialization: string, JSON, database scanner/valuer

2. **ProviderID** (`docid.ProviderID`)
   - Provider-specific identifier (e.g., GoogleFileID)
   - Three provider types: `google`, `local`, `remote-hermes`
   - Format: `google:1abc2def3ghi`, `local:/path/to/doc.md`

3. **CompositeID** (`docid.CompositeID`)
   - Fully-qualified document reference
   - Combines UUID + ProviderID + ProjectID
   - Three serialization formats:
     - Short: `uuid:550e8400-...`
     - URI: `hermes://rfc-archive/google:1abc2def3ghi`
     - Full: `CompositeID{UUID:550e8400-..., Provider:google:1abc2def3ghi, Project:rfc-archive}`

**Test Coverage**: 96.1% (86 test cases, 250+ assertions)

## Database Changes

### Schema Additions (pkg/models/document.go)

```go
type Document struct {
    // ... existing fields ...
    
    // New fields (all nullable for gradual migration)
    DocumentUUID *docid.UUID `gorm:"type:uuid;uniqueIndex:idx_documents_uuid"`
    ProviderType *string     `gorm:"type:varchar(50)"`
    ProjectID    *string     `gorm:"type:varchar(64)"`
}
```

### Helper Methods

- `GetDocumentUUID()` - Returns existing UUID or generates new one
- `SetDocumentUUID(uuid)` - Assigns UUID to document
- `GetByUUID(db, uuid)` - Retrieves document by UUID
- `GetByGoogleFileIDOrUUID(db, id)` - Dual lookup with automatic fallback
- `HasUUID()` - Checks if document has UUID assigned

### Migration Strategy

GORM AutoMigrate handles schema changes automatically on server startup:

```go
// In internal/db/db.go
db.AutoMigrate(models.ModelsToAutoMigrate()...)
```

**Migration occurs automatically** when you:
1. Update to this branch
2. Start the server
3. PostgreSQL adds the three new columns (all nullable)
4. Existing documents continue to work with GoogleFileID

**No manual SQL required**. No downtime. Existing data unaffected.

## API Changes

### V2 Documents API (internal/api/v2/documents.go)

**URL Pattern Changes**:

```
# Existing (still works)
GET /api/v2/documents/{googleFileID}

# New formats (now supported)
GET /api/v2/documents/{uuid}
GET /api/v2/documents/uuid/{uuid}
```

**Regex Update**:
```go
// Before
`^\/api\/v2\/%s\/([0-9A-Za-z_\-]+)$`

// After (accepts UUIDs)
`^\/api\/v2\/%s\/((?:uuid\/)?[0-9A-Za-z_\-]+)$`
```

**Lookup Logic**:
```go
// Before
model := models.Document{GoogleFileID: docID}
model.Get(srv.DB)

// After (automatic fallback)
model := models.Document{}
model.GetByGoogleFileIDOrUUID(srv.DB, docID)
```

**Examples**:

```bash
# Access by GoogleFileID (existing behavior)
curl /api/v2/documents/1abc2def3ghi4jkl5mno6pqr

# Access by UUID (new)
curl /api/v2/documents/550e8400-e29b-41d4-a716-446655440000

# Access by UUID with prefix (new)
curl /api/v2/documents/uuid/550e8400-e29b-41d4-a716-446655440000
```

All three methods return the same document if it has both identifiers assigned.

### Backward Compatibility

- ✅ **All existing GoogleFileID-based API calls continue to work**
- ✅ **Documents without UUIDs are accessible via GoogleFileID**
- ✅ **No breaking changes to API contract**
- ✅ **Automatic fallback from UUID to GoogleFileID lookup**

## Workspace Layer Changes

### workspace.Document Type (pkg/workspace/types.go)

Added optional CompositeID field:

```go
type Document struct {
    ID          string              // GoogleFileID or local path
    CompositeID *docid.CompositeID  // Optional fully-qualified ID
    Name        string
    // ... other fields ...
}
```

**Integration Point**: Higher-level code (API handlers, indexer) can populate `CompositeID` when correlating workspace documents with database models.

**Adapter Impact**: Google and Local workspace adapters require **no changes** - `CompositeID` defaults to `nil`.

## Operator Commands

### Assigning UUIDs to Existing Documents

Use the `hermes operator assign-uuids` command to migrate existing documents:

```bash
# Preview what would be done (safe)
hermes operator assign-uuids --config config.hcl --dry-run

# Assign UUIDs with progress output
hermes operator assign-uuids --config config.hcl

# Verbose mode (shows each UUID assignment)
hermes operator assign-uuids --config config.hcl --verbose

# Custom batch size (default 100)
hermes operator assign-uuids --config config.hcl --batch-size 50
```

**Command Features**:
- ✅ Batch processing (configurable size, default 100)
- ✅ Progress logging (percentage complete)
- ✅ Dry-run mode (preview changes)
- ✅ Verbose mode (show each document)
- ✅ Error handling with summary
- ✅ Zero-downtime (documents remain accessible during migration)

**When to Run**:
- After deploying this branch
- After server has started (AutoMigrate creates columns)
- Can be run multiple times safely (only assigns to documents without UUIDs)
- Can be run while server is running (uses database transactions)

## Testing

### Unit Tests

**pkg/docid Tests** (96.1% coverage):
```bash
go test -v ./pkg/docid/...
```

**pkg/models Tests** (Document UUID methods):
```bash
# Non-database tests
go test -v ./pkg/models/... -run 'TestDocument.*UUID'

# Database tests (requires HERMES_TEST_POSTGRESQL_DSN)
HERMES_TEST_POSTGRESQL_DSN="..." go test -v ./pkg/models/... -run TestDocument_GetByUUID
```

### Integration Tests

**API UUID Tests** (requires integration environment):
```bash
go test -v ./tests/api/... -tags=integration -run TestDocuments.*UUID
```

Test scenarios:
- ✅ Get document by UUID (bare format)
- ✅ Get document by UUID (with uuid/ prefix)
- ✅ Get document by GoogleFileID (backward compatibility)
- ✅ Non-existent UUID returns 404
- ✅ Invalid UUID falls back to GoogleFileID
- ✅ Document with both IDs accessible by either
- ✅ Patch document by UUID
- ✅ Delete document by UUID

## Migration Workflow

### For Operators

1. **Deploy the branch**:
   ```bash
   git checkout jrepp/dev-tidy
   make build
   ```

2. **Start the server** (AutoMigrate creates columns):
   ```bash
   ./hermes server -config=config.hcl
   ```

3. **Verify schema migration**:
   ```sql
   \d documents
   -- Should show: document_uuid, provider_type, project_id columns
   ```

4. **Assign UUIDs** (optional, can be done anytime):
   ```bash
   # Preview first
   ./hermes operator assign-uuids --config config.hcl --dry-run
   
   # Execute
   ./hermes operator assign-uuids --config config.hcl
   ```

5. **Monitor**:
   - Server logs show AutoMigrate progress
   - Assign-UUIDs command shows progress and errors
   - Existing API calls continue working

### For Developers

1. **Use new UUID-based lookups** (optional, both work):
   ```go
   // Option 1: Auto-fallback (recommended)
   doc := models.Document{}
   doc.GetByGoogleFileIDOrUUID(db, idString)
   
   // Option 2: Explicit UUID lookup
   uuid, _ := docid.ParseUUID(uuidString)
   doc.GetByUUID(db, uuid)
   
   // Option 3: GoogleFileID (still works)
   doc := models.Document{GoogleFileID: fileID}
   doc.Get(db)
   ```

2. **Generate UUIDs for new documents**:
   ```go
   doc := &models.Document{
       GoogleFileID: "...",
       // ... other fields ...
   }
   
   // Option 1: Let GetDocumentUUID generate one
   uuid := doc.GetDocumentUUID()
   
   // Option 2: Explicitly assign
   uuid := docid.NewUUID()
   doc.SetDocumentUUID(uuid)
   ```

3. **Use CompositeID for cross-provider references**:
   ```go
   compositeID := docid.NewCompositeID(
       uuid,
       providerID,
       "rfc-archive",
   )
   
   // Serialization options
   short := compositeID.ShortString()       // "uuid:550e8400-..."
   uri := compositeID.URIString()           // "hermes://rfc-archive/google:1abc..."
   full := compositeID.String()             // "CompositeID{UUID:..., Provider:..., Project:...}"
   ```

## Rollback Strategy

If issues arise, rollback is safe because:

1. **Database**: New columns are nullable - existing code ignores them
2. **API**: GoogleFileID lookups still work (no breaking changes)
3. **Operator Command**: Can be re-run anytime (idempotent)

**To rollback**:
```bash
# 1. Revert to previous branch
git checkout main

# 2. Rebuild
make build

# 3. Restart server
# New columns remain in database (harmless, unused)
# All GoogleFileID-based code continues working

# 4. Optional: Remove columns (if desired)
psql -c "ALTER TABLE documents DROP COLUMN document_uuid;"
psql -c "ALTER TABLE documents DROP COLUMN provider_type;"
psql -c "ALTER TABLE documents DROP COLUMN project_id;"
```

## Performance Considerations

- **Database Indexes**: `document_uuid` has unique index (`idx_documents_uuid`)
- **Lookup Performance**: UUID lookup is as fast as GoogleFileID lookup (both indexed)
- **Batch Processing**: `assign-uuids` processes 100 documents per batch (configurable)
- **Memory Impact**: Minimal (UUID is 16 bytes, strings are small)

## Documentation References

- **Design**: `docs-internal/DOCID_PACKAGE_ANALYSIS.md` (1,100+ lines)
- **Implementation**: `docs-internal/DOCID_PACKAGE_IMPLEMENTATION.md` (550+ lines)
- **Architecture**: `docs-internal/DISTRIBUTED_PROJECTS_ARCHITECTURE.md` (original requirements)

## Commit History

- `a29c64d` - feat(docid): implement type-safe document ID package with UUID tests
- `8dcf1ec` - test(docid): add comprehensive tests for ProviderID and CompositeID (96.1% coverage)
- `66590af` - feat(models): add UUID support to Document model
- `16c86ef` - feat(api): add UUID support to v2 documents API
- `34ee67a` - feat(workspace): add CompositeID field to workspace.Document
- `fafe01a` - feat(operator): add assign-uuids command for UUID migration

## Support

For questions or issues:
1. Check test files for usage examples (`pkg/docid/*_test.go`, `tests/api/documents_uuid_test.go`)
2. Review implementation docs (`docs-internal/DOCID_PACKAGE_*.md`)
3. Examine operator command help (`./hermes operator assign-uuids --help`)

## Future Enhancements

Potential next steps (not in this PR):

- **Indexer Integration**: Populate UUID/ProviderType/ProjectID during document indexing
- **Google Docs Sync**: Write UUID to Google Doc custom properties
- **CompositeID API**: Return CompositeID in API responses
- **RemoteHermes Provider**: Implement remote document federation
- **UUID-first Lookup**: Switch default lookup order once most documents have UUIDs
