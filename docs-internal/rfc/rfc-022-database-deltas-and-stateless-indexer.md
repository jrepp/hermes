---
id: rfc-022
title: Database Deltas and Stateless Indexer
status: Implemented
created: 2025-10-24
author: Hermes Team
project_id: hermes
doc_uuid: c93439fc-9fd8-4e40-8765-c0a279655b3e
type: RFC
subtype: Architecture Refactoring
tags: [database, migrations, postgres, sqlite, indexer, stateless]
supersedes: ADR-020 (narrative content)
related: [ADR-020, ADR-019, RFC-001]
---

# RFC-022: Database Deltas and Stateless Indexer

> Migration architecture, indexer registration models, delta-maintenance metrics, and roadmap for the database refactor that introduced core+deltas migrations and the stateless-indexer foundation. The binding architectural rule lives in [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md); this RFC preserves the narrative.

## Overview

This work refactored Hermes to support:

1. **Dual database support** (PostgreSQL + SQLite) with minimal delta maintenance.
2. **Stateless indexer architecture** (no direct database access).
3. **Modular dependencies** per binary.

## Database Migration Architecture

### Migration File Structure

**Old** (single migration file per database):

```text
internal/db/migrations/
  000001_initial_schema.up.sql       # PostgreSQL-specific
  000001_initial_schema.down.sql
  000002_add_indexer_tokens.up.sql
  000002_add_indexer_tokens.down.sql
```

**New** (core + database-specific deltas):

```text
internal/db/migrations/
  # Core schema (works for both PostgreSQL and SQLite)
  000001_core_schema.up.sql
  000001_core_schema.down.sql

  # PostgreSQL-specific enhancements
  000001_postgres_extras.up.sql      # UUID types, CITEXT, extensions
  000001_postgres_extras.down.sql

  # SQLite-specific enhancements
  000001_sqlite_extras.up.sql        # PRAGMAs, optimizations
  000001_sqlite_extras.down.sql

  # Indexer tables (core + deltas)
  000002_indexer_core.up.sql
  000002_indexer_core.down.sql
  000002_indexer_postgres.up.sql
  000002_indexer_postgres.down.sql
  000002_indexer_sqlite.up.sql
  000002_indexer_sqlite.down.sql
```

### Key Design Decisions

**Core Schema Principles:**
- `INTEGER PRIMARY KEY AUTOINCREMENT` (works for both).
- `TEXT` for UUIDs/strings (converted to proper types in extras).
- `INTEGER` for booleans (converted to `BOOLEAN` in PostgreSQL extras).
- `TIMESTAMP` for dates.
- All foreign keys and indexes in core schema.

**PostgreSQL Extras:**
- Enable extensions (`uuid-ossp`, `citext`).
- Convert `TEXT` UUIDs to `UUID` type.
- Convert `TEXT` emails to `CITEXT` type.
- Convert `INTEGER` booleans to `BOOLEAN` type.

**SQLite Extras:**
- Enable foreign keys (`PRAGMA foreign_keys = ON`).
- Enable WAL mode.
- Performance tuning (`mmap_size`, `synchronous`).

### Migration Execution Flow

```go
// internal/db/migrate.go
func RunMigrations(db *sql.DB, driver string) error {
    // 1. Apply core migrations
    m.Up()

    // 2. Apply database-specific enhancements
    applyDatabaseSpecificMigrations(db, driver)
    //   - PostgreSQL: 000001_postgres_extras.up.sql
    //   - SQLite: 000001_sqlite_extras.up.sql
}
```

### Updated Database Layer

`internal/db/db.go`:
- Removed manual extension setup (now in migrations).
- `NewDBWithConfig()` supports both PostgreSQL and SQLite.
- Migration execution happens automatically on startup.
- Backward compatible `NewDB()` for existing code.

`internal/db/migrate.go`:
- `RunMigrations()` runs core + DB-specific migrations.
- `applyDatabaseSpecificMigrations()` applies extras based on driver.
- `GetMigrationVersion()` queries current schema version.

## Indexer Models

`pkg/models/indexer.go`:
- `Indexer` model for tracking registered indexer instances.
- Fields: ID (UUID), Type, WorkspacePath, Hostname, Version, Status.
- Methods: Get, Create, Update, Delete, UpdateHeartbeat.

`pkg/models/indexer_token.go`:
- `IndexerToken` model for authentication.
- Fields: ID (UUID), TokenHash, TokenType, ExpiresAt, Revoked, IndexerID.
- Methods: Create, Get, GetByHash, GetByToken, Revoke, IsValid.
- Utilities: `GenerateToken()`, `HashToken()`.

Token format:

```text
hermes-<type>-token-<uuid>-<random-suffix>
Example: hermes-api-token-550e8400-e29b-41d4-a716-446655440000-a7b3c9d2e1f4
```

## Database Delta Maintenance

Instead of completely separate migration files for each database, we have:

1. **Core schema** (~95% of SQL) — shared.
2. **Database extras** (~5% of SQL) — type conversions and optimizations.

Example: indexer tables.

Core (`000002_indexer_core.up.sql`) — 50 lines:

```sql
CREATE TABLE indexers (
    id TEXT PRIMARY KEY,
    indexer_type TEXT NOT NULL,
    status TEXT DEFAULT 'active',
    ...
);
```

PostgreSQL extras (`000002_indexer_postgres.up.sql`) — 8 lines:

```sql
ALTER TABLE indexers ALTER COLUMN id TYPE UUID USING id::uuid;
ALTER TABLE indexer_tokens ALTER COLUMN revoked TYPE BOOLEAN;
```

SQLite extras (`000002_indexer_sqlite.up.sql`) — 1 line: no changes needed.

**Maintenance Savings:**
- Before: 100 lines × 2 databases = 200 lines to maintain.
- After: 50 core + 8 postgres + 0 sqlite = 58 lines to maintain.
- **71% reduction in duplicate SQL.**

## Stateless Indexer Roadmap

### Indexer Registration API

```go
// internal/api/v2/indexer.go
func RegisterIndexerHandler(w http.ResponseWriter, r *http.Request) {
    // Validate token
    // Create indexer record
    // Generate API token
    // Return indexer_id + api_token
}
```

### Stateless Indexer Client

```go
// pkg/indexer/client/hermes_client.go
type HermesClient struct {
    baseURL    string
    httpClient *http.Client
}

func (c *HermesClient) Register(...) (*RegisterResponse, error)
func (c *HermesClient) SubmitDocuments(...) (*SubmitResponse, error)
func (c *HermesClient) Heartbeat(...) error
```

### Separate Indexer Binary

```text
cmd/hermes-indexer/
  main.go
  go.mod      # Separate dependencies (no GORM, no PostgreSQL)
```

### Docker Compose

```yaml
services:
  hermes-indexer:
    build:
      context: .
      dockerfile: Dockerfile.indexer
    volumes:
      - indexer_shared:/app/shared
    command: ["indexer-agent", "-central=http://hermes:8000"]
```

## Testing Strategy

Unit:
- Migration up/down for both PostgreSQL and SQLite (done).
- Token generation and validation (pending).
- Indexer model CRUD (pending).
- API endpoint handlers (pending).

Integration:
- Full registration flow (token → register → documents).
- PostgreSQL migration from v1 to v2 schema.
- SQLite database initialization from scratch.
- Multi-indexer scenario.

E2E:
- Start testing environment with indexer.
- Verify indexer registration in UI.
- Add document to local workspace.
- Verify document appears in search.
- Test local Hermes → central sync.

## Files Changed

New:
- `internal/db/migrate.go`
- `internal/db/migrations/000001_core_schema.up.sql`
- `internal/db/migrations/000001_postgres_extras.up.sql`
- `internal/db/migrations/000001_sqlite_extras.up.sql`
- `internal/db/migrations/000002_indexer_core.up.sql`
- `internal/db/migrations/000002_indexer_postgres.up.sql`
- `pkg/models/indexer.go`
- `pkg/models/indexer_token.go`

Modified:
- `internal/db/db.go` — supports `DatabaseConfig`, removed manual extensions.
- `pkg/models/gorm.go` — added Indexer and IndexerToken models.
- `go.mod` — added golang-migrate dependencies.

## Future Work

### Separate Go Modules per Binary

Current (monorepo, shared dependencies):

```text
go.mod
  - All dependencies (server + indexer + operator)
```

Future (modular):

```text
cmd/hermes/go.mod
  - GORM, PostgreSQL, SQLite, Meilisearch, Algolia

cmd/hermes-indexer/go.mod
  - Google Workspace API only (no database)

cmd/hermes-operator/go.mod
  - GORM, migration tools only
```

Benefits: smaller binaries, faster builds, clear dependency boundaries, easier to vendor/distribute.

### Migration Workflow

Development:

```bash
# Create new migration
migrate create -ext sql -dir internal/db/migrations -seq add_feature

# Edit:
# - 000003_add_feature.up.sql (core)
# - 000003_add_feature_postgres.up.sql (extras)
# - 000003_add_feature_sqlite.up.sql (extras)

make up                                       # Starts PostgreSQL
./hermes server -config=testing/config.hcl
./hermes server -config=testing/config-sqlite.hcl
```

Production rollout:

```bash
pg_dump hermes > backup.sql
./hermes migrate -config=config.hcl
psql hermes -c "SELECT version FROM schema_migrations;"
./hermes migrate down -config=config.hcl  # rollback if needed
```

The binding architectural rule that came out of this work is recorded in [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md).