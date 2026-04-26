---
id: adr-003
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

## What Belongs in an ADR

An ADR records a **decision with ongoing force** — a rule, pattern, or constraint that future contributors must follow or explicitly supersede. Every ADR must declare a `decision_type`:

| Decision Type | Example |
|---------------|---------|
| **Architectural Pattern** | Provider abstraction (ADR-073), V2 handler shape (ADR-081), `withTimeout()` policy (ADR-065) |
| **Configuration Choice** | HCL over YAML/JSON (ADR-086), version-pinning policy (ADR-029) |
| **Integration / Boundary** | Split server vs migrate binaries (ADR-083), `pkg/docid` identity (ADR-082) |
| **Bug Workaround** | Should rarely become an ADR — prefer a memo. Only promote when the workaround imposes a *lasting* contract on other code. |

If a write-up is "we hit bug X and fixed it by changing line Y," it belongs in `docs-internal/memo/`, not here. ADRs explain the *rule* a future contributor must respect, not the incident that produced it.

## Quick Stats

- **Total ADRs**: 22 (006 and 036 demoted to MEMO-125 and MEMO-124)
- **Categories**: Frontend (4), Auth (5), Provider/Storage (6), Search (3), Config (2), Infra/Tooling (2)
- **Date Range**: October 2025 – April 2026

## Cross-Cutting Principles

These principles recur across multiple ADRs and govern new design work. Cite the ADR(s) when invoking them.

| Principle | Source ADRs |
|-----------|-------------|
| Provider abstraction via Strategy/Adapter/Factory + DI (`auth.Provider`, `workspace.Provider`, `search.Provider`) | 048, 071, 072, 073, 075, 076, 078, 080, 081 |
| Configuration-driven runtime provider selection (`providers { auth, workspace, search }`) | 048, 071, 073, 075, 077, 080 |
| Database is source of truth; search index is a cache | 071, 081 |
| Backend-centric OIDC; client-centric Google OAuth (header: `Authorization: Bearer` vs `Hermes-Google-Access-Token`) | 048, 072, 076, 077, 078, 080, 084 |
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
| Intentional dependency-version pinning where the ecosystem forces a mismatch (`ember-power-select` 8.x + `ember-concurrency` 2.x) | 029 |

## Index by Category

### Frontend

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [001](adr-001-stay-with-classic-ember-build.md) | Stay with Classic Ember Build | Architectural Pattern | Accepted | Keep Ember CLI/Broccoli; defer Embroider+Vite migration. |
| [029](adr-029-ember-concurrency-version-pinning-policy.md) | Ember Concurrency Version Pinning Policy | Configuration Choice | Accepted | `ember-power-select` 8.x + `ember-concurrency` 2.x version mismatch is intentional. |
| [032](adr-032-direct-fetch-for-singleton-endpoints.md) | Direct fetch() for Singleton API Endpoints | Architectural Pattern | Accepted | Singleton endpoints (e.g. `/me`) use direct `fetch()` with `credentials: "include"`, not `store.findAll()`. |
| [065](adr-065-frontend-async-timeout-policy.md) | Frontend Async Timeout & Fallback Policy | Architectural Pattern | Accepted | Wrap every async API call in `withTimeout()` with a graceful fallback; no infinite spinners. |

### Authentication & Authorization

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [072](adr-072-dex-oidc-authentication-for-development.md) | Dex OIDC for Development | Integration | Accepted | Dex `v2.41.1` as local OIDC provider with static-password connector. |
| [076](adr-076-multi-provider-auth-architecture.md) | Multi-Provider Auth Architecture | Architectural Pattern | Implemented | Runtime provider selection via `/api/v2/web/config`; provider-specific headers; no Torii. |
| [077](adr-077-auth-provider-selection.md) | Auth Provider Selection | Configuration Choice | Implemented | `-auth-provider` flag and `HERMES_AUTH_PROVIDER` env var; priority flag > env > config. |
| [078](adr-078-dex-authentication-implementation.md) | Dex Authentication Implementation | Integration | Implemented | `/auth/login`, `/auth/callback`, `/auth/logout`; HttpOnly session cookies; web dev proxy for `/auth/*`. |
| [084](adr-084-multi-provider-auth-diagrams.md) | Multi-Provider Auth Diagrams | Architectural Pattern (documentation) | Accepted | Sequence diagrams documenting Google OAuth (popup) vs OIDC (redirect) flows. |

### Provider & Storage Architecture

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [048](adr-048-environment-agnostic-oauth-redirect.md) | Environment-Agnostic OAuth Redirect | Architectural Pattern | Accepted | OAuth `redirect_uri` always backend; backend redirects to frontend via env-agnostic `base_url`. |
| [071](adr-071-local-file-workspace-system.md) | Local File Workspace System | Architectural Pattern | Accepted | `pkg/workspace/local/` Markdown + YAML frontmatter as drop-in for Google Workspace. |
| [073](adr-073-provider-abstraction-architecture.md) | Provider Abstraction Architecture | Architectural Pattern | Accepted | Three core interfaces: `auth.Provider`, `workspace.Provider`, `search.Provider`. |
| [079](adr-079-document-editor-implementation.md) | Document Editor Implementation | Architectural Pattern | Implemented | Adaptive editor: textarea for local, iframe + open-in-tab for Google; `GET/PUT /api/v2/documents/:id/content`. |
| [082](adr-082-document-identification-system.md) | Document Identification System | Integration | Implemented (Phase 1) | `pkg/docid` with `UUID`, `ProviderID`, `CompositeID`; phased non-breaking DB migration. |
| [085](adr-085-dual-database-support-stateless-indexer.md) | Dual Database Support & Stateless Indexer | Architectural Pattern | Accepted | Core+deltas migrations (`*_core.up.sql` + `_postgres`/`_sqlite` extras); `Indexer`+`IndexerToken` models. |

### Search & Indexing

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [075](adr-075-meilisearch-as-local-search-solution.md) | Meilisearch as Local Search | Integration | Accepted | Meilisearch `v1.11` for dev/CI, Algolia for prod, both behind `search.Provider`. |
| [080](adr-080-search-and-auth-refactoring.md) | Search & Auth Refactoring | Architectural Pattern | Implemented | All search proxied through backend `/1/indexes/*`; no frontend Algolia credentials. |
| [081](adr-081-api-refactoring-and-testing-strategy.md) | API Refactoring & Testing Strategy | Architectural Pattern | Implemented | V2 single-param `func Handler(srv *server.Server) http.Handler`; testcontainers (real PG + Meilisearch). |

### Configuration

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [083](adr-083-split-server-and-migrate-binaries.md) | Split Server and Migrate Binaries (Pure-Go Server) | Architectural Pattern | Accepted | `cmd/hermes` is pure-Go PG-only; `cmd/hermes-migrate` owns SQLite + PG migrations. |
| [086](adr-086-hcl-projects-configuration.md) | HCL Projects Configuration | Configuration Choice | Accepted | HCL per-project files under `testing/projects/*.hcl`; `_template-` files skipped. |

### Infrastructure & Tooling

| ID | Title | Type | Status | Decision |
|----|-------|------|--------|----------|
| [070](adr-070-testing-docker-compose-environment.md) | Testing Docker Compose Environment | Configuration Choice | Accepted | `testing/docker-compose.yml` with +1-offset ports (8001/5433/7701/4201). |
| [074](adr-074-playwright-for-local-iteration.md) | Playwright for Local Iteration | Architectural Pattern | Accepted | Dual strategy: `playwright-mcp` interactive + headless CI; `data-test-*` selectors. |

### Demoted to Memos

| Was | Now | Reason |
|-----|-----|--------|
| ADR-006 (Animated Components Fix) | [MEMO-125](../memo/memo-125-ember-animated-stub-components.md) | Scoped to one ecosystem migration; no general pattern. |
| ADR-036 (Fix Location Type) | [MEMO-124](../memo/memo-124-fix-ember-location-type.md) | One-line config fix; no ongoing constraint. |

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
