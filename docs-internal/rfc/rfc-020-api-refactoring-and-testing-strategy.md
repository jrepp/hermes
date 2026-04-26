---
id: rfc-020
title: API Refactoring and Testing Strategy
status: Implemented
created: 2025-10-09
author: Hermes Team
project_id: hermes
doc_uuid: f7e7ed42-06a4-45f6-849f-49060f39c0c9
type: RFC
subtype: Architecture Refactoring
tags: [api, provider-abstraction, refactoring, testing, v2-api]
supersedes: ADR-017 (narrative content)
related: [ADR-017, ADR-009, RFC-010, RFC-011]
---

# RFC-020: API Refactoring and Testing Strategy

> Migration history, integration-test framework, V1-vs-V2 analysis, and code metrics for the V2 provider-abstraction refactor. The binding architectural rule lives in [ADR-017](../adr/adr-017-api-refactoring-and-testing-strategy.md); this RFC preserves the narrative.

## Executive Summary

The V2 API refactor decoupled handlers from concrete Algolia and Google Workspace clients by introducing `search.Provider` and `workspace.Provider` interfaces and a unified `*server.Server` dependency container. All eight V2 handlers were migrated, and a testcontainer-based integration suite reached 154 passing tests.

Key outcomes:

1. **Provider Abstraction** decouples handlers from specific search/storage implementations.
2. **V2 API Architecture** uses dependency injection via `*server.Server`.
3. **Integration Test Framework** with isolated PostgreSQL schemas + Meilisearch indexes per test.
4. **Strategic Migration Path** — skipped V1 tests retargeted at existing V2 endpoints rather than creating a V1.5 layer.

---

## Background & Motivation

### Problem Statement

The original Hermes API had concrete external-service coupling:

```go
// V1 API Pattern (Legacy)
func DocumentHandler(
    cfg *config.Config,
    l hclog.Logger,
    ar *algolia.Client,      // tightly coupled to Algolia
    aw *algolia.Client,      // cannot mock for tests
    s *gw.Service,           // tightly coupled to Google Workspace
    db *gorm.DB) http.Handler {

    file, err := s.GetFile(docID)
    err = ar.Docs.GetObject(docID, &algoObj)
}
```

Issues:
- ~25 Google Workspace direct calls across 8 V1 handler files
- ~16 Algolia direct calls across 5 V1 handler files
- 12 function signatures requiring concrete external dependencies
- 9 integration tests skipped due to inability to mock external services
- 85% test pass rate (50/59), with 9 skipped

### Goals

1. Enable 100% testability — all API handlers testable with mock providers
2. Provider abstraction — support multiple search/storage backends
3. Clean architecture — dependency injection via unified `server.Server` struct
4. Backward compatibility — maintain V1 API for existing clients
5. Future-proof — V2 API as primary development target

---

## Architecture Overview

### Hermes API Stack

```text
┌─────────────────────────────────────────────────────────────┐
│                     HTTP Layer                              │
│  /api/v1/*  (Legacy)        /api/v2/*  (Modern)            │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Handler Layer                             │
│  internal/api/            internal/api/v2/                  │
│  - Concrete deps          - Provider abstraction            │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                  Provider Abstractions                       │
│  pkg/search/             pkg/workspace/                     │
│  - DocumentIndex()       - GetFile()                        │
│  - DraftIndex()          - ShareFile()                      │
│  - Search()              - MoveFile()                       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│            Provider Implementations                          │
│  Algolia     Meilisearch    Google Drive    Local Storage  │
└─────────────────────────────────────────────────────────────┘
```

### Key Abstractions

#### Search Provider (`pkg/search/`)

```go
type Provider interface {
    DocumentIndex() DocumentIndex
    DraftIndex() DraftIndex
}

type DocumentIndex interface {
    GetObject(ctx context.Context, id string) (*Document, error)
    Index(ctx context.Context, doc *Document) error
    Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error)
    Delete(ctx context.Context, id string) error
}
```

Implementations: `algolia.Provider`, `meilisearch.Provider`, `mock.Provider`.

#### Workspace Provider (`pkg/workspace/`)

```go
type Provider interface {
    GetFile(id string) (*File, error)
    ShareFile(id, email, role string) error
    MoveFile(id, folderID string) (*File, error)
    CopyFile(templateID, title, folderID string) (*File, error)
    RenameFile(id, newName string) error
    // ... 15+ methods total
}
```

Implementations: `google.Adapter`, `local.Adapter`, `mock.Adapter`.

#### Server Struct (Dependency Container)

```go
// internal/server/server.go
type Server struct {
    Config            *config.Config
    DB                *gorm.DB
    SearchProvider    search.Provider
    WorkspaceProvider workspace.Provider
    Logger            hclog.Logger
}
```

---

## V2 API Pattern

### Handler Structure

```go
// internal/api/v2/documents.go
func DocumentHandler(srv *server.Server) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        userEmail := r.Context().Value("userEmail").(string)

        doc, err := srv.SearchProvider.DocumentIndex().GetObject(ctx, docID)
        if err != nil {
            if errors.Is(err, search.ErrNotFound) {
                http.Error(w, "Document not found", http.StatusNotFound)
                return
            }
            srv.Logger.Error("failed to get document", "error", err)
            http.Error(w, "Internal server error", http.StatusInternalServerError)
            return
        }

        file, err := srv.WorkspaceProvider.GetFile(doc.GoogleFileID)
        json.NewEncoder(w).Encode(doc)
    })
}
```

### V2 API Endpoints (All Migrated)

| Endpoint | Handler | Lines | Provider Usage |
|----------|---------|-------|----------------|
| `/api/v2/documents/` | DocumentHandler | ~1142 | DB + Search + Workspace |
| `/api/v2/drafts` | DraftsHandler | ~800 | DB + Search + Workspace |
| `/api/v2/drafts/{id}` | DraftsDocumentHandler | ~600 | DB + Search + Workspace |
| `/api/v2/reviews/` | ReviewsHandler | ~700 | DB + Search + Workspace + Email |
| `/api/v2/approvals/` | ApprovalsHandler | ~500 | DB + Search |
| `/api/v2/people` | PeopleHandler | ~200 | DB + Search (was 501) |
| `/api/v2/me` | MeHandler | ~300 | DB + Workspace |
| `/api/v2/groups` | GroupsHandler | ~150 | DB |

Migration totals: 8 handlers fully migrated, ~47 direct Algolia/Google calls replaced with provider calls, zero breaking changes to API contracts.

### Data Source Strategy

V2 uses the database as the primary source; the search index is a cache:

```go
// V2: Database is source of truth
model := models.Document{GoogleFileID: docID}
if err := model.Get(srv.DB); err != nil { /* ... */ }

// Search provider for search operations only
results, err := srv.SearchProvider.DocumentIndex().Search(ctx, query, opts)
```

V1 used Algolia as the primary source:

```go
// V1: Algolia is source of truth (problematic)
var algoObj map[string]any
err = ar.Docs.GetObject(docID, &algoObj)
```

---

## V1 API Migration Strategy

### Current V1 State

Test coverage: 50/59 tests passing (85%), 9 tests skipped.

Coupling analysis:

| File | Workspace Calls | Algolia Calls | Priority | Complexity |
|------|----------------|---------------|----------|------------|
| `drafts.go` | 8 | 5 | High | Very High (1442 lines) |
| `reviews.go` | 11 | 4 | High | High (700+ lines) |
| `documents.go` | 2 | 3 | Medium | Medium (780 lines) |
| `approvals.go` | 2 | 4 | Medium | Medium (500 lines) |
| `me.go` | 2 | - | Low | Low (300 lines) |

Total: ~25 workspace calls + ~16 Algolia calls across 5 main handlers.

### Migration Options Evaluated

#### Option A: V1.5 Parallel API
Create `internal/api/v1_5/` with refactored handlers mounted at `/api/v1.5/`.
- Pros: zero risk to V1, side-by-side testing, easy rollback.
- Cons: code duplication, three API versions, 4–8 hours, tests legacy patterns.
- Verdict: not recommended.

#### Option B: Direct V1 Refactoring
Modify existing V1 handlers in place with provider abstractions.
- Pros: no duplication, single modernized version.
- Cons: higher breaking-change risk, must complete fully before testing, 8–13 hours.
- Verdict: possible but risky.

#### Option C: Migrate Tests to V2 (chosen)
Update skipped V1 tests to target existing V2 endpoints.

| Skipped V1 Test | V1 Endpoint | V2 Endpoint |
|-----------------|-------------|-------------|
| TestDocuments_Get | `/api/v1/documents/{id}` | `/api/v2/documents/{id}` |
| TestDocuments_Patch | `/api/v1/documents/{id}` | `/api/v2/documents/{id}` |
| TestDocuments_Delete | `/api/v1/documents/{id}` | `/api/v2/documents/{id}` |
| TestDocuments_List | `/api/v1/documents` | `/api/v2/documents` |
| TestAPI_DraftsHandler | `/api/v1/drafts` | `/api/v2/drafts` |
| TestAPI_ReviewsHandler | `/api/v1/reviews/` | `/api/v2/reviews/` |
| TestAPI_ApprovalsHandler | `/api/v1/approvals/` | `/api/v2/approvals/` |
| TestAPI_MeHandler | `/api/v1/me` | `/api/v2/me` |
| TestAPI_ProductsHandler | `/api/v1/products` | `/api/v2/products` |

Verdict: chosen — most efficient, tests the correct API.

### Response Format Differences

V1 (from Algolia):

```json
{
  "objectID": "abc123",
  "docNumber": "RFC-001",
  "docType": "RFC",
  "title": "Test Doc",
  "status": "In Review"
}
```

V2 (from Database):

```json
{
  "id": 42,
  "googleFileID": "abc123",
  "docNumber": "RFC-001",
  "docType": {"name": "RFC", "longName": "Request for Comments"},
  "title": "Test Doc",
  "status": "In Review",
  "createdAt": "2025-10-05T10:00:00Z",
  "modifiedAt": "2025-10-05T12:30:00Z"
}
```

Key differences: `objectID` → `googleFileID`; nested objects (`docType`, `product`); additional metadata; more complete data from database.

---

## Integration Test Framework

### Test Suite Architecture

```text
tests/api/
├── suite_main.go            # MainTestSuite (shared infrastructure)
│   ├── Docker containers (PostgreSQL, Meilisearch)
│   └── Started once per test run
│
├── suite_v1_test.go         # V1 API test suite
│   └── Each test gets isolated:
│       ├── Unique database schema (test_<timestamp>)
│       ├── Unique search indexes (test-docs-<timestamp>)
│       └── Mock workspace provider
│
├── suite_v2_test.go         # V2 API test suite
├── suite_complete_test.go   # Unified runner (V1 + V2)
├── client.go                # Fluent test client
└── fixtures/                # Test data builders
```

### Shared Docker Containers

```go
type MainTestSuite struct {
    postgresContainer *postgres.PostgresContainer
    meilisearchURL    string
}

func (s *MainTestSuite) SetupSuite() {
    s.postgresContainer, _ = postgres.Run(ctx, "postgres:17-alpine")
    meilisearchContainer, _ = testcontainers.GenericContainer(ctx, ...)
}
```

47% faster (71s vs 138s) compared to per-test containers.

### Per-Test Isolation

```go
func NewV2TestSuite(t *testing.T) *V2TestSuite {
    dbName := fmt.Sprintf("test_%d", time.Now().UnixNano())
    db := createIsolatedDB(dbName)

    timestamp := time.Now().UnixNano()
    docIndexName := fmt.Sprintf("test-docs-%d", timestamp)
    searchProvider := createMeilisearchProvider(docIndexName)

    workspaceProvider := local.NewAdapter(testDir)

    return &V2TestSuite{
        DB: db,
        SearchProvider: searchProvider,
        WorkspaceProvider: workspaceProvider,
    }
}
```

### Component Injection

```go
srv := &server.Server{
    Config:            suite.Config,
    DB:                suite.DB,
    SearchProvider:    suite.SearchProvider,
    WorkspaceProvider: suite.WorkspaceProvider,
    Logger:            log,
}
handler := apiv2.DocumentHandler(srv)
```

### Fluent Test Client

```go
resp := suite.Client.
    WithAuth("alice@hashicorp.com").
    Get("/api/v2/documents/test-123").
    ExpectStatus(200).
    ExpectJSON()

var doc map[string]interface{}
resp.DecodeJSON(&doc)
assert.Equal(t, "Test Document", doc["title"])
```

### Integration Tests Created

`tests/api/api_complete_integration_test.go` (395 lines):

1. **TestCompleteIntegration_DocumentLifecycle** — create draft, persist, index, search, retrieve, authorize.
2. **TestCompleteIntegration_ProductsEndpoint** — multi-product associations.
3. **TestCompleteIntegration_DocumentTypesV1** — simple v1 endpoint (no auth).
4. **TestCompleteIntegration_DocumentTypesV2** — authenticated v2 endpoint.
5. **TestCompleteIntegration_AnalyticsEndpoint** — analytics POST variants.
6. **TestCompleteIntegration_MultiUserScenario** — Alice + Bob ownership isolation.

### Real vs Mock Components

| Component | Implementation | Reason |
|-----------|---------------|--------|
| Database | Real PostgreSQL | Test actual GORM models and queries |
| Search | Real Meilisearch | Test actual search behavior and filters |
| Workspace | Mock adapter | No Google Drive needed for tests |
| Auth | Mock adapter | No OAuth setup needed |
| Email | Empty service | Not needed for API tests |

### Test Execution Metrics

```bash
# Unit Tests
115 tests passing, ~3s, 11.3% coverage

# API Integration Tests
154 tests passing, 8 expected failures (V1 Algolia-coupled), 28 skipped, ~270s

# New API Test Suite (TestAPIComplete)
71s runtime (47% faster than old suite)

# Search / Workspace Integration Tests
All passing, ~14s / ~0.2s
```

Total: 289 tests across all suites; 269/289 (93%) passing; parallel execution 4× faster locally.

---

## Implementation Results

### V2 API Migration

8 handlers, 3500+ lines refactored:

| Handler | Before | After | Provider Calls |
|---------|--------|-------|----------------|
| DocumentHandler | 6 params, Algolia-coupled | 1 param, providers | 15 workspace, 8 search |
| DraftsHandler | 6 params, 1442 lines | 1 param, providers | 24 workspace, 13 search |
| ReviewsHandler | 7 params, GW-coupled | 1 param, providers | 15 workspace, 7 search |
| ApprovalsHandler | 6 params | 1 param, providers | 2 workspace, 8 search |
| PeopleHandler | Was 501 error | Functional | 0 workspace, 3 search |
| MeHandler | 6 params | 1 param, providers | 5 workspace |
| GroupsHandler | 5 params | 1 param, providers | 0 workspace, 2 DB |
| ProjectsHandler | 6 params | 1 param, providers | 3 search |

Totals: ~47 direct Google Workspace calls → `srv.WorkspaceProvider.*`; ~39 direct Algolia calls → `srv.SearchProvider.*`; 8 function signatures simplified from 5–7 params to 1 param.

### Test Coverage Achievements

Before: 50/59 (85%) passing, 9 skipped, no mock infrastructure.
After: 154/162 (95%) passing, 8 expected V1 failures, full mock infrastructure for Search + Workspace + Auth.

---

## Best Practices & Patterns

### Provider Abstraction

```go
// DO: provider abstraction with context
ctx := r.Context()
doc, err := srv.SearchProvider.DocumentIndex().GetObject(ctx, docID)
if err != nil {
    if errors.Is(err, search.ErrNotFound) {
        http.Error(w, "Document not found", http.StatusNotFound)
        return
    }
}

// DON'T: concrete Algolia client
var algoObj map[string]any
err = ar.Docs.GetObject(docID, &algoObj)
```

### Handler Structure

```go
func MyHandler(srv *server.Server) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        docID := chi.URLParam(r, "id")
        userEmail, ok := ctx.Value("userEmail").(string)
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        if docID == "" {
            http.Error(w, "Missing document ID", http.StatusBadRequest)
            return
        }
        doc, err := srv.SearchProvider.DocumentIndex().GetObject(ctx, docID)
        if err != nil { /* handle */ return }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(doc)
    })
}
```

### Error Handling

```go
doc, err := srv.SearchProvider.DocumentIndex().GetObject(ctx, docID)
if err != nil {
    if errors.Is(err, search.ErrNotFound) {
        http.Error(w, "Document not found", http.StatusNotFound)
        return
    }
    srv.Logger.Error("search error", "error", err, "docID", docID)
    http.Error(w, "Internal server error", http.StatusInternalServerError)
    return
}
```

### Integration Test Pattern

```go
func testV2MyEndpoint(t *testing.T) {
    suite := NewV2TestSuite(t)
    defer suite.Cleanup()

    doc := fixtures.NewDocument().
        WithGoogleFileID(suite.GetUniqueDocID("test-doc")).
        WithTitle("Test Document").
        WithStatus(models.ApprovedDocumentStatus).
        Create(t, suite.DB)

    searchDoc := ModelToSearchDocument(doc)
    err := suite.SearchProvider.DocumentIndex().Index(context.Background(), searchDoc)
    require.NoError(t, err)

    srv := &server.Server{
        DB: suite.DB,
        SearchProvider: suite.SearchProvider,
        WorkspaceProvider: suite.WorkspaceProvider,
        Config: suite.Config,
        Logger: hclog.NewNullLogger(),
    }
    handler := apiv2.MyHandler(srv)

    req := httptest.NewRequest("GET", "/api/v2/my-endpoint/"+doc.GoogleFileID, nil)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}
```

### Database as Source of Truth

```go
// V2 (correct): get from DB, use search only for search operations
doc := models.Document{GoogleFileID: docID}
if err := doc.Get(srv.DB); err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        http.Error(w, "Document not found", http.StatusNotFound)
        return
    }
}
results, err := srv.SearchProvider.DocumentIndex().Search(ctx, query, search.SearchOptions{
    Filters: map[string]interface{}{"status": "In-Review"},
    Limit:   10,
})
```

---

## Future Work

### V1 API Options

- **Maintain as-is (recommended)** — V1 functional for backward compatibility, all new development on V2, deprecation in 6–12 months.
- **Gradual V1 refactoring** — refactor V1 handlers one-by-one to provider pattern; 8–13 h initial + 3–5 h fixes.
- **Deprecate and remove V1** — 1–2 mo announce, 3–6 mo migration support, 6 mo remove.

### Additional Enhancements

- Search provider: bulk indexing, retry logic, metrics/observability.
- Workspace provider: batch operations, file caching, additional backends (Dropbox, OneDrive).
- Testing infrastructure: performance benchmarks, load tests, contract tests for provider interfaces.
- Documentation: API versioning guide, provider implementation guide, V1→V2 migration guide.

---

## Migration Statistics

### Code Metrics

V2 API handlers:
- Lines refactored: ~3,500
- Functions updated: 8 handlers
- Direct calls replaced: 86 (47 workspace + 39 search)
- Function parameters reduced: 48 → 8 (6:1 ratio)

Test suite:
- Integration tests created: 154
- Pass rate improvement: 85% → 95%
- Execution speedup: 47% (138s → 71s)
- Mock infrastructure components: 3 (Search, Workspace, Auth)

### Time Investment

- V2 API migration: ~16–20 h
- Test suite creation: ~12–15 h
- Mock infrastructure: ~8–10 h
- Documentation: ~10–12 h

Total: ~46–57 hours.

### Success Metrics

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Test Pass Rate | 85% | 95% | +10% |
| Mockable APIs | 0% | 100% (V2) | +100% |
| Test Runtime | 138s | 71s | 47% faster |
| Provider Coupling | High | None (V2) | Decoupled |
| API Versions | 1 (V1) | 2 (V1+V2) | Modern API |

---

## Conclusion

The refactor achieved provider abstraction, 100% testability for V2, comprehensive integration tests, and a clear migration path. **The V2 API is the recommended path forward for all new development.** V1 remains functional for backward compatibility with a defined deprecation path.

The binding architectural rule that came out of this work is recorded in [ADR-017](../adr/adr-017-api-refactoring-and-testing-strategy.md).