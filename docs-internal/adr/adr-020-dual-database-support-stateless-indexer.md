---
id: adr-020
title: Core+Deltas Migrations and Stateless Indexer
status: Accepted
decision_type: Architectural Pattern
created: 2026-04-24
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: c93439fc-9fd8-4e40-8765-c0a279655b3e
date: 2026-04-24
type: ADR
tags: [database, migrations, postgres, sqlite, indexer, stateless]
related: [ADR-019, ADR-017, RFC-022]
---

# ADR-020: Core+Deltas Migrations and Stateless Indexer

> Migration files use the core+deltas layout: `NNNNNN_<thing>_core.up.sql` carries portable SQL, with optional `_postgres` and `_sqlite` extras for type conversions and engine tuning. The indexer is stateless — it submits to the central Hermes via API only and never opens a database connection.

## Context

Hermes needs to run on both PostgreSQL (production, testing) and SQLite (simplified local mode, embedded scenarios), but maintaining two parallel migration trees doubled the schema surface area and produced drift bugs. Separately, the original indexer held a direct database connection, which forced the indexer binary to import GORM and the PostgreSQL driver and made it impossible to run an indexer outside the central deployment.

See [RFC-022](../rfc/rfc-022-database-deltas-and-stateless-indexer.md) for the full migration architecture, indexer-registration model, delta-maintenance metrics, file inventory, and roadmap.

## Decision

1. **Core+deltas migration layout.** Each migration version `NNNNNN` consists of:
   - `NNNNNN_<thing>_core.up.sql` / `.down.sql` — portable SQL (`INTEGER PK AUTOINCREMENT`, `TEXT` for UUIDs/strings, `INTEGER` for booleans, `TIMESTAMP` for dates; all foreign keys and indexes here).
   - Optional `NNNNNN_<thing>_postgres.up.sql` / `.down.sql` — type promotions (`TEXT`→`UUID`/`CITEXT`, `INTEGER`→`BOOLEAN`) and extension setup (`uuid-ossp`, `citext`).
   - Optional `NNNNNN_<thing>_sqlite.up.sql` / `.down.sql` — `PRAGMA` setup (`foreign_keys`, WAL mode, `mmap_size`, `synchronous`).

2. **Migration runner applies core then engine-specific deltas.** `internal/db/migrate.go::RunMigrations(db, driver)` runs the core migrations via `golang-migrate`, then `applyDatabaseSpecificMigrations(db, driver)` applies the matching `_postgres` or `_sqlite` extras for that version.

3. **Indexer is stateless and submits via API only.** No GORM, no PostgreSQL/SQLite drivers in the indexer binary. Indexer authenticates with a token (`hermes-<type>-token-<uuid>-<random>`), registers via `/api/v2/indexer/register`, then submits documents and heartbeats over HTTP.

4. **Indexer identity lives in the central database** as `indexers` and `indexer_tokens` tables (managed by the core+deltas migrations above). The central server is the only writer.

5. **Future: per-binary `go.mod`.** `cmd/hermes`, `cmd/hermes-indexer`, and `cmd/hermes-migrate` are designed to be separable modules so the indexer can ship without database drivers and the migrate binary can own SQLite (see [ADR-019](adr-019-split-server-and-migrate-binaries.md)).

## Consequences

### Positive
- ~71% reduction in duplicate SQL (50 + 8 + 0 lines vs 100 × 2).
- Single source of truth for schema structure; engine-specific concerns are small, reviewable deltas.
- Indexer binary is dependency-light and can run anywhere it can reach the central API.
- Foundation for local-developer mode (RFC-001) and remote indexers (RFC-011).
- Reviewers see "what changed in the schema" without diffing two near-identical files.

### Negative
- Two file kinds per migration version (core + extras) — slightly more files to navigate.
- Engine-specific behaviors (e.g. PostgreSQL `CITEXT` semantics) are not exercised when running on SQLite, requiring CI matrix coverage.
- Indexer-via-API has higher operational latency than direct DB writes; bulk submission and batching matter.

## Alternatives Considered

- **Two completely separate migration trees** — rejected: doubles maintenance, drift-prone, no shared review.
- **PostgreSQL only, drop SQLite** — rejected: blocks simplified local mode and embedded scenarios; SQLite is genuinely useful for single-user/local development.
- **GORM AutoMigrate instead of explicit SQL** — rejected: opaque, harder to review, no clean way to split engine-specific concerns, and we want explicit migrations for production.
- **Keep the indexer stateful (direct DB)** — rejected: forces every indexer to ship database drivers, prevents cross-network/cross-tenant indexers, and ties indexer deployment to central-database access.

## References

- [RFC-022: Database Deltas and Stateless Indexer](../rfc/rfc-022-database-deltas-and-stateless-indexer.md) — full architecture, migration files, models, metrics, roadmap.
- [ADR-019: Split Server and Migrate Binaries](adr-019-split-server-and-migrate-binaries.md) — pure-Go server, dedicated migrate binary owning SQLite.
- [ADR-017: V2 API Provider Abstraction Pattern](adr-017-api-refactoring-and-testing-strategy.md) — indexer submits through V2 endpoints.
- [RFC-001: Local Developer Mode with Central Hermes](../rfc/rfc-001-local-developer-mode-with-central-hermes.md).
- Code: `internal/db/migrate.go`, `internal/db/migrations/`, `pkg/models/indexer.go`, `pkg/models/indexer_token.go`.