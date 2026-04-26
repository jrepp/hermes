---
id: adr-002
created: 2026-04-24
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: 6c1b6d39-46b6-4705-b1c8-9cf1bc02d281
status: Accepted
title: ADR Index (Architectural Decision Records)
---

# ADR Index (Architectural Decision Records)

Authoritative record of architectural decisions for Hermes. Each ADR documents the context, the decision, alternatives considered, and consequences.

**This index is the ground-truth map of binding architectural commitments.** New work must conform to the principles listed in [Cross-Cutting Principles](#cross-cutting-principles) unless an ADR is explicitly superseded.

> **Authoring a new ADR or demoting one?** Start from [`docs-internal/templates/readme.md`](../templates/readme.md). It pins the templates, controlled vocabularies for `status` / `decision_type`, and the demotion / RFC-migration checklists.

## What Belongs in an ADR

An ADR records a **decision with ongoing force** — a rule, pattern, or constraint that future contributors must follow or explicitly supersede. Every ADR must declare a `decision_type`:

| Decision Type | Example |
|---------------|---------|
| **Architectural Pattern** | Provider abstraction (ADR-009), V2 handler shape (ADR-017), `withTimeout()` policy (ADR-005) |
| **Configuration Choice** | HCL over YAML/JSON (ADR-021) |
| **Integration / Boundary** | Split server vs migrate binaries (ADR-019), `pkg/docid` identity (ADR-018) |
| **Bug Workaround** | Should rarely become an ADR — prefer a memo. Only promote when the workaround imposes a *lasting* contract on other code. |

If a write-up is "we hit bug X and fixed it by changing line Y," it belongs in `docs-internal/memo/`, not here. ADRs explain the *rule* a future contributor must respect, not the incident that produced it.

## Quick Stats

- **Total ADRs**: 21 (006, 029, 036, 084 demoted to memos; 081/082/085 narrative migrated to RFCs, tight ADRs retained)
- **Categories**: Frontend (3), Auth (4), Provider/Storage (6), Search (3), Config (2), Infra/Tooling (2)
- **Date Range**: October 2025 – April 2026

## Cross-Cutting Principles

These principles recur across multiple ADRs and govern new design work. Cite the ADR(s) when invoking them.

| Principle | Source ADRs |
|-----------|-------------|
| Provider abstraction via Strategy/Adapter/Factory + DI (`auth.Provider`, `workspace.Provider`, `search.Provider`) | 048, 071, 072, 073, 075, 076, 078, 080, 081 |
| Configuration-driven runtime provider selection (`providers { auth, workspace, search }`) | 048, 071, 073, 075, 077, 080 |
| Database is source of truth; search index is a cache | 071, 081 |
| Backend-centric OIDC; client-centric Google OAuth (header: `Authorization: Bearer` vs `Hermes-Google-Access-Token`) | 048, 072, 076, 077, 078, 080 |
| Session cookies HttpOnly + Secure + SameSite=Lax (never localStorage) | 076, 078 |
| Pure-Go server binary (`CGO_ENABLED=0`); migrations live in a separate binary | 083, 085 |
| Port-isolation strategy (+1 offset) across native / integration / testing / production | 070, 072, 078 |
| HCL (not YAML/JSON) for configuration; native `env()`, comments, `import` | 086 (used by 048, 070, 072, 075, 077) |
| Stable UUID identity for documents that survives provider migrations (`pkg/docid`) | 082 |
| Stateless indexer; API-only data submission (no direct DB access from indexer) | 085 |
| Core+deltas migration layout (shared SQL core, small per-DB extras) | 085 |
| Direct `fetch()` (with `credentials: "include"`) for singleton API endpoints; not Ember Data `store.findAll()` | 032 |
| Frontend async timeout & graceful-fallback policy (`withTimeout()` on every API call) | 065 |
| Use `data-test-*` selectors; dual Playwright strategy (`playwright-mcp` + headless CI) | 074 |
| `server.Server` as the single DI container for V2 handlers (`func Handler(srv *server.Server) http.Handler`) | 081 |
| Error wrapping with `errors.Is` against typed sentinels (`search.ErrNotFound`, `workspace.ErrNotFound`) | 073, 081 |
| Environment-agnostic `base_url` for OAuth; OAuth `redirect_uri` always backend, then redirect to frontend | 048 |

## Index by Category

### Frontend

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [001](adr-001-stay-with-classic-ember-build.md) | Stay with Classic Ember Build | Architectural Pattern | Accepted | Keep Ember CLI/Broccoli; defer Embroider+Vite migration. |
| [032](adr-002-direct-fetch-for-singleton-endpoints.md) | Direct fetch() for Singleton API Endpoints | Architectural Pattern | Accepted | Singleton endpoints (e.g. `/me`) use direct `fetch()` with `credentials: "include"`, not `store.findAll()`. |
| [065](adr-005-frontend-async-timeout-policy.md) | Frontend Async Timeout & Fallback Policy | Architectural Pattern | Accepted | Wrap every async API call in `withTimeout()` with a graceful fallback; no infinite spinners. |

### Authentication & Authorization

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [072](adr-008-dex-oidc-authentication-for-development.md) | Dex OIDC for Development | Integration | Accepted | Dex `v2.41.1` as local OIDC provider with static-password connector. |
| [076](adr-012-multi-provider-auth-architecture.md) | Multi-Provider Auth Architecture | Architectural Pattern | Implemented | Runtime provider selection via `/api/v2/web/config`; provider-specific headers; no Torii. |
| [077](adr-013-auth-provider-selection.md) | Auth Provider Selection | Configuration Choice | Implemented | `-auth-provider` flag and `HERMES_AUTH_PROVIDER` env var; priority flag > env > config. |
| [078](adr-014-dex-authentication-implementation.md) | Dex Authentication Implementation | Integration | Implemented | `/auth/login`, `/auth/callback`, `/auth/logout`; HttpOnly session cookies; web dev proxy for `/auth/*`. |

### Provider & Storage Architecture

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [048](adr-004-environment-agnostic-oauth-redirect.md) | Environment-Agnostic OAuth Redirect | Architectural Pattern | Accepted | OAuth `redirect_uri` always backend; backend redirects to frontend via env-agnostic `base_url`. |
| [071](adr-007-local-file-workspace-system.md) | Local File Workspace System | Architectural Pattern | Accepted | `pkg/workspace/local/` Markdown + YAML frontmatter as drop-in for Google Workspace. |
| [073](adr-009-provider-abstraction-architecture.md) | Provider Abstraction Architecture | Architectural Pattern | Accepted | Three core interfaces: `auth.Provider`, `workspace.Provider`, `search.Provider`. |
| [079](adr-015-document-editor-implementation.md) | Document Editor Implementation | Architectural Pattern | Implemented | Adaptive editor: textarea for local, iframe + open-in-tab for Google; `GET/PUT /api/v2/documents/:id/content`. |
| [082](adr-018-document-identification-system.md) | Document Identification System | Integration | Implemented (Phase 1) | `pkg/docid` with `UUID`, `ProviderID`, `CompositeID`; phased non-breaking DB migration. |
| [085](adr-020-dual-database-support-stateless-indexer.md) | Dual Database Support & Stateless Indexer | Architectural Pattern | Accepted | Core+deltas migrations (`*_core.up.sql` + `_postgres`/`_sqlite` extras); `Indexer`+`IndexerToken` models. |

### Search & Indexing

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [075](adr-011-meilisearch-as-local-search-solution.md) | Meilisearch as Local Search | Integration | Accepted | Meilisearch `v1.11` for dev/CI, Algolia for prod, both behind `search.Provider`. |
| [080](adr-016-search-and-auth-refactoring.md) | Search & Auth Refactoring | Architectural Pattern | Implemented | All search proxied through backend `/1/indexes/*`; no frontend Algolia credentials. |
| [081](adr-017-api-refactoring-and-testing-strategy.md) | API Refactoring & Testing Strategy | Architectural Pattern | Implemented | V2 single-param `func Handler(srv *server.Server) http.Handler`; testcontainers (real PG + Meilisearch). |

### Configuration

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [083](adr-019-split-server-and-migrate-binaries.md) | Split Server and Migrate Binaries (Pure-Go Server) | Architectural Pattern | Accepted | `cmd/hermes` is pure-Go PG-only; `cmd/hermes-migrate` owns SQLite + PG migrations. |
| [086](adr-021-hcl-projects-configuration.md) | HCL Projects Configuration | Configuration Choice | Accepted | HCL per-project files under `testing/projects/*.hcl`; `_template-` files skipped. |

### Infrastructure & Tooling

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [070](adr-006-testing-docker-compose-environment.md) | Testing Docker Compose Environment | Configuration Choice | Accepted | `testing/docker-compose.yml` with +1-offset ports (8001/5433/7701/4201). |
| [074](adr-010-playwright-for-local-iteration.md) | Playwright for Local Iteration | Architectural Pattern | Accepted | Dual strategy: `playwright-mcp` interactive + headless CI; `data-test-*` selectors. |

### Demoted to Memos

| Was | Now | Reason |
|-----|-----|--------|
| ADR-006 (Animated Components Fix) | [MEMO-061](../memo/memo-061-ember-animated-stub-components.md) | Scoped to one ecosystem migration; no general pattern. |
| ADR 029 (Ember Concurrency Version Pinning Policy) | [MEMO-064](../memo/memo-064-ember-concurrency-power-select-incompatibility.md) | ADRs should not pin specific package versions; the rule belongs in `package.json`. |
| ADR 036 (Fix Location Type) | [MEMO-060](../memo/memo-060-fix-ember-location-type.md) | One-line config fix; no ongoing constraint. |
| MEMO-062 (Multi-Provider Auth Diagrams) | [MEMO-062](../memo/memo-062-multi-provider-auth-diagrams.md) | Diagrams are reference material, not a decision; ADR-012 owns the rule. |
| ADR-019 investigation log | [MEMO-063](../memo/memo-063-sqlite-driver-conflict-investigation.md) | Incident narrative belongs in a memo; the rule lives in the tight ADR-019. |

### Narrative Migrated to RFCs

Some ADRs originally contained extensive narrative, deltas, and process detail. The decision content stays in the ADR (tightened); the narrative now lives in a new RFC.

| Tight ADR | Narrative now in |
|-----------|------------------|
| [ADR-017](adr-017-api-refactoring-and-testing-strategy.md) (V2 handler shape, testcontainers) | [RFC-020](../rfc/rfc-020-api-refactoring-and-testing-strategy.md) |
| [ADR-018](adr-018-document-identification-system.md) (`pkg/docid`) | [RFC-021](../rfc/rfc-021-document-identification-system.md) |
| [ADR-020](adr-020-dual-database-support-stateless-indexer.md) (core+deltas, stateless indexer) | [RFC-022](../rfc/rfc-022-database-deltas-and-stateless-indexer.md) |

### Known-Missing Legacy References

These IDs are referenced in older docs but **do not exist** in the repo. Do not link to them; treat occurrences as cleanup candidates.

- `RFC-007`, `RFC-020`, `RFC 034`, `ADR-007`, `ADR-016`
- `MEMO-001`, `MEMO 075`

## Decision Format

Each ADR follows this structure:

```markdown
---
id: adr-NNN
title: <Architectural-rule title, NOT "fix X" or "issue Y">
decision_type: Architectural Pattern | Configuration Choice | Integration | Bug Workaround
status: Proposed | Accepted | Implemented | Superseded | Deprecated
date: YYYY-MM-DD
---

# ADR-NNN: <Title>

**Decision Type**: <one of the four above>

## Context
## Decision
## Consequences
## Alternatives Considered
## Implementation
## References
```

## Contributing

1. Assign next sequential ID (NNN format).
2. Use kebab-case filename. **Lead with the rule, not the bug**: prefer `adr-NNN-frontend-async-timeout-policy.md` over `adr-NNN-promise-timeout-hang.md`.
3. Set `decision_type` in frontmatter and as a one-line note under the H1. If the only honest type is "Bug Workaround," reconsider whether this should be a memo.
4. Include context, decision, consequences, alternatives.
5. Link related RFCs and ADRs.
6. **If a decision contradicts a [Cross-Cutting Principle](#cross-cutting-principles), explicitly mark the source ADR(s) as Superseded.**
7. Update this index — both the category table and (if applicable) the principles table.