---
id: memo-063
title: SQLite Driver Conflict Investigation
status: Reference (Resolved)
created: 2025-10-27
author: Hermes Team
project_id: hermes
doc_uuid: 5fd0862b-40a9-493d-a36f-1ec405eb836d
type: MEMO
tags: [sqlite, drivers, dependencies, debugging, history]
supersedes: ADR-019 (investigation log section)
related: [ADR-019]
---

# MEMO-063: SQLite Driver Conflict Investigation

> Extracted from ADR-019. This memo preserves the original problem analysis, the seven attempted solutions, and the alternative resolution paths that were considered before settling on the split-binary approach. The accepted decision (pure-Go server + dedicated `cmd/hermes-migrate` owning SQLite drivers + custom `models.JSON`) lives in [ADR-019](../adr/adr-019-split-server-and-migrate-binaries.md).

**Resolution date:** October 27, 2025.

## Original Problem Description

**Issue:** `panic: sql: Register called twice for driver sqlite`
**Severity:** Critical
**Affects:** All server commands (serve, server, setup wizard).

The Hermes binary could not start because two different SQLite drivers both attempted to register themselves with Go's `database/sql` package using the same driver name `sqlite`. This caused a panic during the `init` phase, before any application code ran.

```text
panic: sql: Register called twice for driver sqlite

goroutine 1 [running]:
database/sql.Register({0x105472b8f, 0x6}, {0x106258400, 0x10713ca30})
    /opt/homebrew/Cellar/go/1.25.2/libexec/src/database/sql/sql.go:63 +0x120
modernc.org/sqlite.init.0()
    /Users/jrepp/go/pkg/mod/modernc.org/sqlite@v1.23.1/sqlite.go:125 +0x38
```

## Root Cause Analysis

### The Conflicting Drivers

**Driver 1:** `github.com/mattn/go-sqlite3`
- CGO-based (requires C compiler).
- Registration: `sql.Register("sqlite", &SQLiteDriver{})` in `init()`.
- Location: `github.com/mattn/go-sqlite3/sqlite3.go`.
- Used by: `gorm.io/driver/sqlite` (GORM's official CGO driver).

**Driver 2:** `modernc.org/sqlite`
- Pure Go (transpiled from C).
- Registration: `sql.Register("sqlite", drv{})` in `init()`.
- Location: `modernc.org/sqlite/sqlite.go:125`.
- Used by: `github.com/glebarez/sqlite` (GORM's pure-Go driver) and `golang-migrate/migrate/v4/database/sqlite`.

### Why Both Are Present

Even though the code explicitly uses ONLY `github.com/glebarez/sqlite`, the Go module system pulled in both drivers through multiple dependency paths:

```text
1. Direct Usage (intended):
   internal/db/db.go → github.com/glebarez/sqlite → modernc.org/sqlite ✅

2. Migration Library (indirect):
   internal/db/migrate.go → golang-migrate/migrate/v4/database/sqlite
   → modernc.org/sqlite (blank import) ✅

3. Integration Tests (test-only):
   tests/integration/workspace/document_content_test.go
   → gorm.io/driver/sqlite → mattn/go-sqlite3 ⚠️

4. Transitive Dependencies (automatic):
   pkg/models/* → gorm.io/datatypes → gorm.io/driver/sqlite
   → mattn/go-sqlite3 ⚠️

   gorm.io/gorm → (test deps) → gorm.io/driver/sqlite
   → mattn/go-sqlite3 ⚠️
```

### The Go Module System Behavior

Key insight: **Go includes ALL dependencies in the module graph, even if they're only used in tests tagged with build constraints.**

```bash
$ go mod graph | grep "driver/sqlite"
gorm.io/gorm@v1.26.4 gorm.io/driver/sqlite@v1.6.0
gorm.io/datatypes@v1.2.6 gorm.io/driver/sqlite@v1.6.0
github.com/hashicorp-forge/hermes tests/integration/workspace → gorm.io/driver/sqlite@v1.6.0

$ go list -m all | grep sqlite
github.com/glebarez/go-sqlite v1.21.2
github.com/glebarez/sqlite v1.11.0
github.com/mattn/go-sqlite3 v1.14.22
gorm.io/driver/sqlite v1.6.0
modernc.org/sqlite v1.23.1
```

Even though `tests/integration/workspace/*.go` files have `//go:build integration` tags, the dependencies are still in `go.mod` because `go mod tidy` includes ALL dependencies across ALL build tags.

### Why init() Causes the Panic

```go
// mattn/go-sqlite3 (sqlite3.go)
func init() {
    sql.Register("sqlite3", &SQLiteDriver{})
    sql.Register("sqlite", &SQLiteDriver{})  // aliases both names
}

// modernc.org/sqlite (sqlite.go:125)
func init() {
    sql.Register("sqlite", drv{})
}

// database/sql/sql.go:63
func Register(name string, driver driver.Driver) {
    if _, dup := drivers[name]; dup {
        panic("sql: Register called twice for driver " + name) // boom
    }
}
```

## Attempted Solutions

### 1. Replace gorm.io/driver/sqlite with glebarez

```bash
go mod edit -droprequire gorm.io/driver/sqlite
go mod tidy
```

**Failed:** `go mod tidy` immediately re-adds it as a transitive dependency of `gorm.io/datatypes@v1.2.6` and `gorm.io/gorm@v1.26.4`.

### 2. Use go mod replace

```bash
go mod edit -replace gorm.io/driver/sqlite=github.com/glebarez/sqlite@v1.11.0
```

**Failed:** Replace requires the replacement to have the SAME module path.

```text
go: github.com/glebarez/sqlite@v1.11.0 used for two different module paths
(gorm.io/driver/sqlite and github.com/glebarez/sqlite)
```

### 3. Import Aliasing

```go
import glebarez_sqlite "github.com/glebarez/sqlite"
```

**Failed:** Aliasing only affects code references. Both packages are still compiled in and both `init()` functions still execute.

### 4. Downgrade modernc.org/sqlite

**Failed:** Version conflicts aren't the issue — both drivers (mattn + modernc) are present regardless of version.

### 5. Upgrade golang-migrate

**Failed:** golang-migrate uses the correct (modernc) driver, but `mattn/go-sqlite3` is still pulled in by other dependencies.

### 6. Remove Direct modernc Dependency

**Failed:** `modernc.org/sqlite` is a transitive dependency of glebarez and golang-migrate; can't remove without removing those packages.

### 7. Exclude Test Dependencies

```bash
go build -tags=!integration ./cmd/hermes
```

**Failed:** Build tags don't affect the module graph; `go.mod` still includes all dependencies regardless of tags.

## Why This Is Hard to Fix

1. **Go module system design:** the module graph includes ALL dependencies across ALL build tags.
2. **`init()` execution:** no way to conditionally skip init functions.
3. **Transitive dependencies:** can't exclude dependencies pulled in by other packages.
4. **No replace workaround:** replace directives require the same module path.
5. **GORM ecosystem:** multiple GORM packages depend on the official CGO driver.

## Potential Solutions Considered

### Solution 1: Remove SQLite Support (Temporary)

- Remove all SQLite-related code; document Postgres as the only supported database.
- Pros: immediate; no architectural changes.
- Cons: loses simplified-mode promise; requires Docker/external Postgres locally.

### Solution 2: Build with CGO Enabled

- Accept `mattn/go-sqlite3`; build with `CGO_ENABLED=1`.
- Pros: well-tested driver; no double-registration.
- Cons: violates the project's `CGO_ENABLED=0` requirement; build tools required on target systems; platform-specific binaries; slower builds.

### Solution 3: Refactor Dependencies (chosen direction)

- Replace `gorm.io/datatypes` with custom `models.JSON`.
- Audit and replace anything that pulls in `gorm.io/driver/sqlite`.
- Pros: maintains pure-Go binary; solves root cause; cleaner dep tree.
- Cons: significant refactor; risk to existing functionality; may need a custom migration runner.

### Solution 4: Build Constraints

- Use build tags to conditionally compile SQLite support; provide two build modes (`hermes`, `hermes-sqlite`).
- Pros: supports both databases; each binary has a single driver.
- Cons: two build configs; more complex CI/CD.

### Solution 5: Fork and Patch modernc.org/sqlite

- Fork; remove the automatic `sql.Register()` from `init()`; manually register when needed; use replace directive.
- Pros: surgical fix.
- Cons: maintain a fork; sync upstream; non-standard.

## Recommended Approach (At The Time)

- **Phase 1 (immediate):** Solution 1 — remove SQLite temporarily, emphasize Postgres.
- **Phase 2 (short-term):** Solution 4 — build constraints / two binaries.
- **Phase 3 (long-term):** Solution 3 — dependency refactor.

## What Was Actually Done

The accepted resolution combined Solution 3 (dependency refactor: custom `models.JSON`, removed `gorm.io/datatypes`) with a stronger version of Solution 4 (split-binary): a dedicated `cmd/hermes-migrate` binary owns both SQLite (`modernc.org/sqlite`) and PostgreSQL drivers, while the server binary `cmd/hermes` is pure-Go (`CGO_ENABLED=0`) and embeds only the PostgreSQL driver. See [ADR-019](../adr/adr-019-split-server-and-migrate-binaries.md) for the resulting architectural rule and verification.

## Status Log

- **2025-10-27** — Issue discovered during embedded web assets testing.
- **2025-10-27** — Attempted 7+ solutions, all unsuccessful; documented in this investigation log.
- **2025-10-27** — Resolved by splitting into `cmd/hermes` (pure-Go server) and `cmd/hermes-migrate` (owns SQLite drivers); replaced `gorm.io/datatypes` with `models.JSON`.

## References

- Go `database/sql`: https://pkg.go.dev/database/sql
- modernc.org/sqlite: https://gitlab.com/cznic/sqlite
- glebarez/sqlite GORM driver: https://github.com/glebarez/sqlite
- mattn/go-sqlite3: https://github.com/mattn/go-sqlite3
- Go Modules: https://go.dev/ref/mod