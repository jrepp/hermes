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

## Quick Stats

- **Total ADRs**: 24
- **Categories**: Frontend (5), Auth (5), Provider/Storage (6), Search (3), Config (2), Infra/Tooling (3)
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
| Graceful frontend degradation: timeouts, fallbacks, no infinite spinners; `data-test-*` selectors | 065, 074 |
| `server.Server` as the single DI container for V2 handlers (`func Handler(srv *server.Server) http.Handler`) | 081 |
| Dual Playwright strategy: `playwright-mcp` for exploration, headless for CI; never `--headed` in automation | 074 |
| Error wrapping with `errors.Is` against typed sentinels (`search.ErrNotFound`, `workspace.ErrNotFound`) | 073, 081 |
| Environment-agnostic `base_url` for OAuth; OAuth `redirect_uri` always backend, then redirect to frontend | 048 |

## Index by Category

### Frontend

| ID | Title | Status | Decision |
|----|-------|--------|----------|
| [001](adr-001-stay-with-classic-ember-build.md) | Stay with Classic Ember Build | Accepted | Keep Ember CLI/Broccoli; defer Embroider+Vite migration. |
| [006](adr-006-animated-components-fix.md) | Animated Components Fix | Accepted | Passthrough stub components for ember-animated migration. |
| [029](adr-029-ember-concurrency-compat.md) | Ember Concurrency Compatibility | Accepted | `ember-power-select` 8.x + `ember-concurrency` 2.x version mismatch (intentional). |
| [032](adr-032-ember-data-store-fix.md) | Ember Data Store Fix | Accepted | Direct `fetch()` instead of `store.findAll()` for `/me`; safe optional chaining on session headers. |
| [036](adr-036-fix-location-type.md) | Fix Location Type | Accepted | `locationType` from `auto` → `history` for Ember 6.x. |
| [065](adr-065-promise-timeout-hang.md) | Promise Timeout Hang Fix | Accepted | `withTimeout()` wrapper + graceful fallback on every async API call. |

### Authentication & Authorization

| ID | Title | Status | Decision |
|----|-------|--------|----------|
| [072](adr-072-dex-oidc-authentication-for-development.md) | Dex OIDC for Development | Accepted | Dex `v2.41.1` as local OIDC provider with static-password connector. |
| [076](adr-076-multi-provider-auth-architecture.md) | Multi-Provider Auth Architecture | Implemented | Runtime provider selection via `/api/v2/web/config`; provider-specific headers; no Torii. |
| [077](adr-077-auth-provider-selection.md) | Auth Provider Selection | Implemented | `-auth-provider` flag and `HERMES_AUTH_PROVIDER` env var; priority flag > env > config. |
| [078](adr-078-dex-authentication-implementation.md) | Dex Authentication Implementation | Implemented | `/auth/login`, `/auth/callback`, `/auth/logout`; HttpOnly session cookies; web dev proxy for `/auth/*`. |
| [084](adr-084-multi-provider-auth-diagrams.md) | Multi-Provider Auth Diagrams | Accepted | Sequence diagrams documenting Google OAuth (popup) vs OIDC (redirect) flows. |

### Provider & Storage Architecture

| ID | Title | Status | Decision |
|----|-------|--------|----------|
| [048](adr-048-local-workspace-user-info.md) | Local Workspace User Info Fix | Accepted | Real `ProviderAdapter` for local workspace; environment-agnostic `base_url` for OAuth. |
| [071](adr-071-local-file-workspace-system.md) | Local File Workspace System | Accepted | `pkg/workspace/local/` Markdown + YAML frontmatter as drop-in for Google Workspace. |
| [073](adr-073-provider-abstraction-architecture.md) | Provider Abstraction Architecture | Accepted | Three core interfaces: `auth.Provider`, `workspace.Provider`, `search.Provider`. |
| [079](adr-079-document-editor-implementation.md) | Document Editor Implementation | Implemented | Adaptive editor: textarea for local, iframe + open-in-tab for Google; `GET/PUT /api/v2/documents/:id/content`. |
| [082](adr-082-document-identification-system.md) | Document Identification System | Implemented (Phase 1) | `pkg/docid` with `UUID`, `ProviderID`, `CompositeID`; phased non-breaking DB migration. |
| [085](adr-085-dual-database-support-stateless-indexer.md) | Dual Database Support & Stateless Indexer | Accepted | Core+deltas migrations (`*_core.up.sql` + `_postgres`/`_sqlite` extras); `Indexer`+`IndexerToken` models. |

### Search & Indexing

| ID | Title | Status | Decision |
|----|-------|--------|----------|
| [075](adr-075-meilisearch-as-local-search-solution.md) | Meilisearch as Local Search | Accepted | Meilisearch `v1.11` for dev/CI, Algolia for prod, both behind `search.Provider`. |
| [080](adr-080-search-and-auth-refactoring.md) | Search & Auth Refactoring | Implemented | All search proxied through backend `/1/indexes/*`; no frontend Algolia credentials. |
| [081](adr-081-api-refactoring-and-testing-strategy.md) | API Refactoring & Testing Strategy | Implemented | V2 single-param `func Handler(srv *server.Server) http.Handler`; testcontainers (real PG + Meilisearch). |

### Configuration

| ID | Title | Status | Decision |
|----|-------|--------|----------|
| [083](adr-083-sqlite-driver-registration-conflict.md) | SQLite Driver Registration Conflict | Accepted | Split binaries: `cmd/hermes` (PG only) vs `cmd/hermes-migrate` (PG + SQLite); custom `models.JSON`. |
| [086](adr-086-hcl-projects-configuration.md) | HCL Projects Configuration | Accepted | HCL per-project files under `testing/projects/*.hcl`; `_template-` files skipped. |

### Infrastructure & Tooling

| ID | Title | Status | Decision |
|----|-------|--------|----------|
| [070](adr-070-testing-docker-compose-environment.md) | Testing Docker Compose Environment | Accepted | `testing/docker-compose.yml` with +1-offset ports (8001/5433/7701/4201). |
| [074](adr-074-playwright-for-local-iteration.md) | Playwright for Local Iteration | Accepted | Dual strategy: `playwright-mcp` interactive + headless CI; `data-test-*` selectors. |

## Decision Format

Each ADR follows this structure:

```markdown
# ADR-NNN: Title

**Status**: Accepted | Implemented | Superseded | Deprecated
**Date**: YYYY-MM-DD
**Related**: Links to related RFCs/ADRs

## Context
## Decision
## Consequences
## Alternatives Considered
## Implementation
## References
```

## Contributing

1. Assign next sequential ID (NNN format).
2. Use kebab-case filename: `adr-NNN-description.md`.
3. Include context, decision, consequences, alternatives.
4. Link related RFCs and ADRs.
5. **If a decision contradicts a [Cross-Cutting Principle](#cross-cutting-principles), explicitly mark the source ADR(s) as Superseded.**
6. Update this index — both the category table and (if applicable) the principles table.
