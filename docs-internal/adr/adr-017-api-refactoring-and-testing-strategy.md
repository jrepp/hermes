---
id: adr-017
title: V2 API Provider Abstraction Pattern
status: Accepted
decision_type: Architectural Pattern
created: 2025-10-09
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: f7e7ed42-06a4-45f6-849f-49060f39c0c9
date: 2025-10-09
type: ADR
tags: [api, provider-abstraction, v2-api, testing, dependency-injection]
related: [ADR-009, ADR-011, ADR-016, RFC-020]
---

# ADR-017: V2 API Provider Abstraction Pattern

> V2 HTTP handlers take a single `*server.Server` and reach external systems only through provider interfaces (`search.Provider`, `workspace.Provider`). The database is the source of truth; the search index is a cache. V1 is frozen-legacy; new tests target V2.

## Context

V1 handlers took 5–7 concrete dependencies (`*algolia.Client`, `*gw.Service`, `*gorm.DB`, …) and called Algolia / Google Workspace directly. This made handlers untestable without real external services (9 integration tests skipped, 85% pass rate) and bound the API to specific vendors. We needed an abstraction that allowed mock providers, multiple backends (Algolia + Meilisearch, Google Drive + local filesystem), and a uniform handler shape.

See [RFC-020](../rfc/rfc-020-api-refactoring-and-testing-strategy.md) for the full migration narrative, integration-test framework, V1-vs-V2 analysis, and code metrics.

## Decision

1. **Single-parameter handler signature.** Every V2 HTTP handler is `func Handler(srv *server.Server) http.Handler`. `*server.Server` is the dependency-injection container holding `Config`, `DB`, `SearchProvider`, `WorkspaceProvider`, `Logger`, etc.

2. **All external access through provider interfaces.** Handlers must not import or call `pkg/algolia`, `pkg/gw`, or any concrete external client directly. Use `srv.SearchProvider` (`pkg/search`) and `srv.WorkspaceProvider` (`pkg/workspace`).

3. **Typed sentinel errors matched with `errors.Is`.** Provider implementations wrap vendor-specific errors and return typed sentinels (`search.ErrNotFound`, `workspace.ErrNotFound`, …). Handlers branch on these, not on Algolia/Google error shapes.

4. **Database is the source of truth.** V2 reads canonical data from PostgreSQL via GORM models; the search index is treated as a cache/optimization for search queries only.

5. **V1 is frozen-legacy.** No new V1 handlers. Skipped V1 tests are retargeted at the equivalent V2 endpoint rather than building a V1.5 layer or refactoring V1 in place. Existing V1 handlers remain for backward compatibility under a deprecation timeline.

6. **Integration tests use real PostgreSQL + Meilisearch via testcontainers**, with per-test isolated schemas/indexes; workspace and auth use mock adapters. Containers start once per test run (shared `MainTestSuite`).

### Canonical handler shape

```go
func DocumentHandler(srv *server.Server) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        docID := chi.URLParam(r, "id")

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

        file, err := srv.WorkspaceProvider.GetFile(doc.GoogleFileID)
        // ...
        json.NewEncoder(w).Encode(doc)
    })
}
```

## Consequences

### Positive
- 100% testability for V2 handlers via mock providers.
- Vendor swaps (Algolia ↔ Meilisearch, Google Drive ↔ local filesystem) require no handler changes.
- Uniform handler signature reduces cognitive load and simplifies route wiring.
- Database-as-source-of-truth eliminates index-drift bugs in critical reads.
- Integration tests can run hermetically in CI with testcontainers.

### Negative
- Two parallel API surfaces (V1 + V2) until V1 is sunset.
- Provider interfaces add an indirection layer — vendor-specific features must be designed into the interface or accessed via narrow adapter escape hatches.
- Migration cost was substantial (~46–57 hours for the original refactor; see RFC-020).

## Alternatives Considered

- **V1.5 parallel API** — refactored V1 mounted at `/api/v1.5/`. Rejected: code duplication, three API versions to maintain, tests would still target legacy patterns.
- **Refactor V1 in place** — modify existing V1 handlers to use providers. Rejected: high breaking-change risk, must complete fully before testing, and V2 already covered all needed endpoints.
- **Keep concrete clients, add interfaces only at the search/workspace package boundary** — rejected because handler signatures would still depend on concrete types and remain untestable.

## References

- [RFC-020: API Refactoring and Testing Strategy](../rfc/rfc-020-api-refactoring-and-testing-strategy.md) — full narrative, metrics, migration history.
- [ADR-009: Provider Abstraction Architecture](adr-009-provider-abstraction-architecture.md) — the broader Strategy + Adapter + Factory + DI pattern.
- [ADR-011: Meilisearch as Local Search Solution](adr-011-meilisearch-as-local-search-solution.md) — concrete `search.Provider` implementation.
- [ADR-016: Search and Auth Refactoring](adr-016-search-and-auth-refactoring.md) — backend-mediated search pattern.