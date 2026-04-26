---
id: adr-083
title: "Split Server and Migrate Binaries (Pure-Go Server)"
status: Accepted
decision_type: Architectural Pattern
created: 2026-04-24
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: 5fd0862b-40a9-493d-a36f-1ec405eb836d
date: 2026-04-24
type: ADR
tags: [build, binaries, sqlite, postgres, migrations, cgo]
related:
  - ADR-085
  - MEMO-127
---

# ADR-083: Split Server and Migrate Binaries (Pure-Go Server)

> The server binary `cmd/hermes` is pure-Go (`CGO_ENABLED=0`) and embeds only the PostgreSQL driver. All migrations live in a separate `cmd/hermes-migrate` binary that owns both SQLite (`modernc.org/sqlite`) and PostgreSQL drivers. `gorm.io/datatypes` is replaced by a custom `pkg/models.JSON` type to prevent transitive pull-in of `mattn/go-sqlite3`.

## Context

Hermes needs to support both PostgreSQL (production, testing) and SQLite (simplified local mode, embedded scenarios). When SQLite support was added, the server binary panicked at startup with `sql: Register called twice for driver sqlite` because Go's module graph pulled in both `mattn/go-sqlite3` (CGO) and `modernc.org/sqlite` (pure-Go) — both register themselves under the name `sqlite` in their `init()` functions, and the second registration panics.

Seven different in-place fixes were attempted (replace directives, import aliasing, version downgrades, build tags, removing direct dependencies). All failed because the Go module system includes all dependencies across all build tags, transitive dependencies cannot be excluded, and `replace` requires identical module paths. See [MEMO-127](../memo/memo-127-sqlite-driver-conflict-investigation.md) for the full investigation log.

The deeper problem: as long as a single binary tries to support both database engines and pulls in GORM ecosystem packages (`gorm.io/datatypes`, `gorm.io/driver/sqlite`), driver conflicts are unavoidable.

## Decision

1. **Server binary `cmd/hermes` is pure-Go.** Built with `CGO_ENABLED=0`. Imports only the PostgreSQL driver (`lib/pq` via `gorm.io/driver/postgres`). Contains zero SQLite symbols.

2. **Migrations live in a separate binary `cmd/hermes-migrate`.** This binary owns both `modernc.org/sqlite` and PostgreSQL drivers and runs all schema migrations for both engines. Migration code lives in `internal/migrate/` (moved from `internal/db/migrate.go`); embedded SQL files move with it.

3. **Server expects a pre-migrated database.** The server does not call `RunMigrations()` at startup. Operators must run `hermes-migrate` before `hermes server`. In Docker Compose this is wired via `depends_on: { migrate: { condition: service_completed_successfully } }`.

4. **`gorm.io/datatypes` is replaced by `pkg/models.JSON`** (a custom type implementing `driver.Valuer` and `sql.Scanner`). This eliminates the transitive pull-in of `gorm.io/driver/sqlite` → `mattn/go-sqlite3` from the model package.

5. **The boundary is permanent.** Even though the original trigger was a driver-registration panic, the split is preserved as an ongoing architectural constraint: it follows the 12-factor migrations pattern, keeps the server binary small and pure-Go, and prevents future re-introduction of `mattn/go-sqlite3` via transitive dependencies.

### Verification

```bash
# Server binary: zero SQLite symbols
$ go tool nm build/bin/hermes | grep -c "modernc.org/sqlite"
0

# Migrate binary: SQLite support present
$ go tool nm build/bin/hermes-migrate | grep -c "modernc.org/sqlite"
3876

# Server starts without driver panic
$ ./build/bin/hermes server -config=config.hcl
```

### Workflow

```bash
make bin                       # server (PostgreSQL only)
make bin/migrate               # migrate binary (PostgreSQL + SQLite)

make migrate/postgres          # native dev
make migrate/postgres/testing  # testing env (port 5433)
make migrate/sqlite            # SQLite

./build/bin/hermes server -config=config.hcl
```

## Consequences

### Positive
- Server binary is pure-Go, small, and contains zero SQLite symbols — no risk of driver-registration panics.
- Migrations are versioned, reviewable, and run as a separate operational step (12-factor compliant).
- Docker Compose `depends_on: service_completed_successfully` makes ordering explicit.
- Replacing `gorm.io/datatypes` with `models.JSON` removes a long transitive dependency chain.
- SQLite remains supported (via the migrate binary and dev workflows) without contaminating the server.

### Negative
- Two binaries to build, ship, and document.
- Operators must remember (or automate) "migrate before server start"; forgetting it leaves the server pointing at an empty/old schema.
- `models.JSON` is custom code Hermes now maintains instead of `gorm.io/datatypes`.
- Adding any future dependency that transitively pulls in `mattn/go-sqlite3` will reintroduce the panic; CI must guard against this.

## Alternatives Considered

- **Single binary, build with `CGO_ENABLED=1`** — rejected: violates pure-Go requirement, requires C toolchain on target systems, platform-specific binaries.
- **Drop SQLite entirely** — rejected: blocks simplified local mode and embedded scenarios.
- **Build tags / two server binaries (`hermes` and `hermes-sqlite`)** — rejected: doubles the release matrix and still risks driver conflicts if anyone enables both tags.
- **Fork `modernc.org/sqlite` and remove its `init()` registration** — rejected: maintenance burden, non-standard, breaks if upstream changes.
- **Keep migrations in the server binary but use `replace` directives or import aliases** — proven to fail (see MEMO-127).

## References

- [MEMO-127: SQLite Driver Conflict Investigation](../memo/memo-127-sqlite-driver-conflict-investigation.md) — full root-cause analysis and seven attempted fixes.
- [ADR-085: Core+Deltas Migrations and Stateless Indexer](adr-085-dual-database-support-stateless-indexer.md) — migration file layout that `cmd/hermes-migrate` runs.
- Code: `cmd/hermes-migrate/main.go`, `internal/migrate/`, `pkg/models/json.go`, `Makefile` (`bin/migrate`, `migrate/postgres`, `migrate/sqlite`).
