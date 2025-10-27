# SQLite Driver Double Registration Conflict

**Issue**: `panic: sql: Register called twice for driver sqlite`  
**Status**: ✅ RESOLVED - Migrations moved to separate binary  
**Resolution Date**: October 27, 2025  
**Solution**: Separation of migration code into dedicated `hermes-migrate` binary

## Resolution Summary

The SQLite driver conflict has been **completely resolved** by implementing a clean architectural separation:

### What Was Done

1. **Created dedicated migration binary** (`cmd/hermes-migrate/`)
   - Handles ALL database migrations (PostgreSQL + SQLite)
   - Contains both `modernc.org/sqlite` and `lib/pq` drivers
   - Isolated from server runtime to prevent conflicts

2. **Removed SQLite from server binary** (`cmd/hermes/`)
   - Server only includes PostgreSQL driver
   - Zero SQLite-related symbols in production binary
   - Expects database to be pre-migrated

3. **Replaced `gorm.io/datatypes`** dependency
   - Created custom `models.JSON` type (`pkg/models/json.go`)
   - Implements `driver.Valuer` and `sql.Scanner`
   - Eliminates transitive dependency on `gorm.io/driver/sqlite`

4. **Moved migration code** to separate package
   - `internal/migrate/` contains all migration logic
   - Embedded SQL files moved with the package
   - Clean separation of concerns

### Verification

```bash
# ✅ Server binary has ZERO SQLite symbols
$ go tool nm build/bin/hermes | grep -c "modernc.org/sqlite"
0

# ✅ Migration binary has SQLite support
$ go tool nm build/bin/hermes-migrate | grep -c "modernc.org/sqlite"
3876

# ✅ Server starts without panic
$ ./build/bin/hermes server -config=config.hcl
# (Configuration errors expected, but NO driver panic)
```

### New Workflow

```bash
# 1. Build both binaries
make bin           # Server binary (PostgreSQL only)
make bin/migrate   # Migration binary (PostgreSQL + SQLite)

# 2. Run migrations BEFORE starting server
make migrate/postgres      # For native development
make migrate/postgres/testing  # For testing environment (port 5433)
make migrate/sqlite        # For SQLite (if needed)

# 3. Start server (expects migrated database)
./build/bin/hermes server -config=config.hcl
```

### Docker Compose Integration

The testing environment (`testing/docker-compose.yml`) now includes an automated migration service:

```yaml
services:
  migrate:
    container_name: hermes-migrate
    build:
      context: ..
      dockerfile: Dockerfile
    command:
      - /app/hermes-migrate
      - -driver=postgres
      - -dsn=host=postgres user=postgres password=postgres dbname=hermes_testing port=5432 sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy
    restart: on-failure

  hermes:
    # ... 
    depends_on:
      postgres:
        condition: service_healthy
      meilisearch:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully  # Waits for migration to finish
```

**Usage**:
```bash
cd testing
docker compose up -d --build  # Automatically runs migrations before starting server
docker compose logs migrate   # View migration logs
```

The migration service:
- ✅ Runs automatically before the server starts
- ✅ Exits with code 0 after successful migration
- ✅ Restarts on failure (useful for timing issues)
- ✅ Server won't start until migration completes successfully

### Benefits Achieved

✅ **No driver conflicts** - Server binary contains only PostgreSQL driver  
✅ **Pure-Go binary** - Server maintains `CGO_ENABLED=0` requirement  
✅ **Clean architecture** - Migrations separated from runtime  
✅ **SQLite still supported** - Available via migration binary  
✅ **No GORM datatypes** - Custom JSON type eliminates transitive dependencies  
✅ **Production-ready** - Follows 12-factor app migration pattern  

### Files Changed

- `pkg/models/json.go` - Custom JSON type (NEW)
- `pkg/models/instance.go` - Uses `models.JSON` instead of `datatypes.JSON`
- `pkg/models/document_type.go` - Uses `models.JSON` instead of `datatypes.JSON`
- `cmd/hermes-migrate/main.go` - Migration binary (NEW)
- `internal/migrate/migrate.go` - Moved from `internal/db/migrate.go`
- `internal/migrate/migrations/` - Moved from `internal/db/migrations/`
- `internal/db/db.go` - Removed SQLite support, removed RunMigrations call
- `Makefile` - Added `bin/migrate`, `migrate/postgres`, `migrate/sqlite` targets

### Related Documentation

- See `docs-internal/MAKEFILE_ROOT_TARGETS.md` for updated workflow
- Migration binary usage: `./build/bin/hermes-migrate -help`

---

## Original Problem Description (Historical Reference)

**Issue**: `panic: sql: Register called twice for driver sqlite`  
**Severity**: Critical  
**Affects**: All server commands (serve, server, setup wizard)

The Hermes binary could not start because two different SQLite drivers both attempted to register themselves with Go's `database/sql` package using the same driver name "sqlite". This caused a panic during the init phase, before any application code ran.

```
panic: sql: Register called twice for driver sqlite

goroutine 1 [running]:
database/sql.Register({0x105472b8f, 0x6}, {0x106258400, 0x10713ca30})
    /opt/homebrew/Cellar/go/1.25.2/libexec/src/database/sql/sql.go:63 +0x120
modernc.org/sqlite.init.0()
    /Users/jrepp/go/pkg/mod/modernc.org/sqlite@v1.23.1/sqlite.go:125 +0x38
```

## Root Cause Analysis

### The Conflicting Drivers

**Driver 1**: `github.com/mattn/go-sqlite3`
- **Type**: CGO-based (requires C compiler)
- **Registration**: Calls `sql.Register("sqlite", &SQLiteDriver{})` in init()
- **Location**: `github.com/mattn/go-sqlite3/sqlite3.go`
- **Used by**: `gorm.io/driver/sqlite` (GORM's official CGO driver)

**Driver 2**: `modernc.org/sqlite`
- **Type**: Pure Go (transpiled from C)
- **Registration**: Calls `sql.Register("sqlite", drv{})` in init()
- **Location**: `modernc.org/sqlite/sqlite.go:125`
- **Used by**: `github.com/glebarez/sqlite` (GORM's pure-Go driver), `golang-migrate/migrate/v4/database/sqlite`

### Why Both Are Present

Even though the code explicitly uses ONLY `github.com/glebarez/sqlite`, the Go module system pulls in BOTH drivers through multiple dependency paths:

```
Dependency Tree Analysis:

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
   
   gorm.io/gorm → (test dependencies) → gorm.io/driver/sqlite
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

Even though `tests/integration/workspace/*.go` files have `//go:build integration` tags, the dependencies are still in `go.mod` because:
1. The integration test package is part of the module
2. `go mod tidy` includes ALL dependencies across ALL build tags
3. There's no way to exclude test-only dependencies from production builds using standard Go tooling

### Why init() Causes the Panic

Both drivers have init functions that run automatically:

**mattn/go-sqlite3 (`sqlite3.go`)**:
```go
func init() {
    sql.Register("sqlite3", &SQLiteDriver{})
    sql.Register("sqlite", &SQLiteDriver{})  // Aliases both names!
}
```

**modernc.org/sqlite (`sqlite.go:125`)**:
```go
func init() {
    sql.Register("sqlite", drv{})
}
```

The panic occurs because:
1. Go's module system ensures both packages are compiled into the binary
2. Both init() functions execute before main()
3. The second call to `sql.Register("sqlite", ...)` triggers the panic in database/sql:

```go
// database/sql/sql.go:63
func Register(name string, driver driver.Driver) {
    driversMu.Lock()
    defer driversMu.Unlock()
    if driver == nil {
        panic("sql: Register driver is nil")
    }
    if _, dup := drivers[name]; dup {
        panic("sql: Register called twice for driver " + name)  // 💥
    }
    drivers[name] = driver
}
```

## Attempted Solutions

### ❌ 1. Replace gorm.io/driver/sqlite with glebarez

**Approach**:
```bash
go mod edit -droprequire gorm.io/driver/sqlite
go mod tidy
```

**Result**: FAILED  
**Reason**: `go mod tidy` immediately adds it back as a transitive dependency of:
- `gorm.io/datatypes@v1.2.6`
- `gorm.io/gorm@v1.26.4` (test dependencies)

### ❌ 2. Use go mod replace

**Approach**:
```bash
go mod edit -replace gorm.io/driver/sqlite=github.com/glebarez/sqlite@v1.11.0
```

**Result**: FAILED  
**Reason**: Replace requires the replacement to have the SAME module path. Cannot replace one module with a different module.

**Error**: 
```
go: github.com/glebarez/sqlite@v1.11.0 used for two different module paths
(gorm.io/driver/sqlite and github.com/glebarez/sqlite)
```

### ❌ 3. Import Aliasing

**Approach**: Changed `internal/db/db.go`:
```go
import glebarez_sqlite "github.com/glebarez/sqlite"
```

**Result**: FAILED  
**Reason**: Aliasing only affects how the package is referenced in code. Both packages are still compiled into the binary, and both init() functions still execute.

### ❌ 4. Downgrade modernc.org/sqlite

**Approach**:
```bash
go get modernc.org/sqlite@v1.23.1  # Match glebarez's version
go mod tidy
```

**Result**: FAILED  
**Reason**: The issue isn't version conflicts - it's that BOTH drivers (mattn + modernc) are present, regardless of version.

**Side effects**: Downgraded `glebarez/go-sqlite` from v1.22.0 to v1.21.2 (dependency resolution).

### ❌ 5. Upgrade golang-migrate

**Approach**:
```bash
go get -u github.com/golang-migrate/migrate/v4
go mod tidy
```

**Result**: FAILED  
**Reason**: golang-migrate still uses modernc.org/sqlite (correct driver), but mattn/go-sqlite3 is still pulled in by other dependencies.

### ❌ 6. Remove Direct modernc Dependency

**Approach**:
```bash
grep -r "modernc.org/sqlite" --include="*.go" .
# (Found no direct imports, only transitive)
go mod edit -droprequire modernc.org/sqlite
```

**Result**: FAILED  
**Reason**: modernc.org/sqlite is a transitive dependency of glebarez and golang-migrate. Cannot remove it without removing those packages.

### ❌ 7. Exclude Test Dependencies

**Approach**: Try to build without test dependencies
```bash
go build -tags=!integration ./cmd/hermes
```

**Result**: FAILED  
**Reason**: Build tags don't affect the module graph. `go.mod` still includes all dependencies regardless of tags.

## Why This Is Hard to Fix

1. **Go Module System Design**: The module graph includes ALL dependencies across ALL build tags
2. **init() Execution**: No way to conditionally skip init() functions
3. **Transitive Dependencies**: Can't exclude dependencies pulled in by other packages
4. **No Replace Workaround**: Can't use replace directives with different module paths
5. **GORM Ecosystem**: Multiple GORM packages depend on the official CGO driver

## Potential Solutions

### ✅ Solution 1: Remove SQLite Support (Temporary)

**Approach**: 
- Remove all SQLite-related code from internal/db/db.go
- Remove glebarez/sqlite dependency
- Document Postgres as the only supported database
- Keep SQLite support in a separate branch for future work

**Pros**:
- Immediate resolution
- No architectural changes
- Postgres is production-ready and well-tested

**Cons**:
- Loses "simplified mode" promise
- Requires Docker/external Postgres for local development

**Implementation**:
```go
// internal/db/db.go
func Connect(cfg Config) (*gorm.DB, error) {
    switch cfg.Type {
    case "postgres":
        return connectPostgres(cfg)
    case "sqlite":
        return nil, fmt.Errorf("SQLite support temporarily disabled - see docs-internal/SQLITE_DRIVER_CONFLICT.md")
    default:
        return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
    }
}
```

### ✅ Solution 2: Build with CGO Enabled

**Approach**:
- Accept mattn/go-sqlite3 as the official driver
- Remove glebarez/sqlite dependency
- Build with CGO_ENABLED=1
- Provide pre-built binaries for major platforms

**Pros**:
- Well-tested, widely-used driver
- No double-registration issues
- GORM officially supports it

**Cons**:
- Violates project's CGO_ENABLED=0 requirement
- Requires build tools on target systems
- Platform-specific binaries
- Larger binary size
- Slower builds

**Implementation**:
```makefile
# Makefile
bin:
    CGO_ENABLED=1 go build -o build/bin/hermes ./cmd/hermes
```

### ✅ Solution 3: Refactor Dependencies

**Approach**:
- Replace `gorm.io/datatypes` with custom JSON types
- Replace `golang-migrate` with custom migration code or alternative library
- Ensure NO dependencies pull in `gorm.io/driver/sqlite`
- Use only `glebarez/sqlite`

**Pros**:
- Maintains pure-Go binary
- Solves root cause
- Cleaner dependency tree

**Cons**:
- Significant refactoring required (~5-10 hours)
- Risk of breaking existing functionality
- Need to maintain custom migration code
- May affect existing data types

**Implementation Steps**:
1. Audit usage of `gorm.io/datatypes` in `pkg/models/`
2. Replace with custom GORM JSON serialization
3. Evaluate alternatives to golang-migrate:
   - goose (github.com/pressly/goose) - has SQLite support with modernc
   - atlas (ariga.io/atlas) - schema-based migrations
   - Custom SQL migration runner
4. Update all affected model files
5. Test thoroughly with existing data

### ✅ Solution 4: Build Constraints

**Approach**:
- Use build tags to conditionally compile SQLite support
- Provide two build modes: `hermes` (Postgres) and `hermes-sqlite` (SQLite)
- Or use runtime feature flags

**Pros**:
- Supports both databases
- Each binary has single driver
- Clear separation of concerns

**Cons**:
- Maintains two build configurations
- More complex CI/CD
- Documentation overhead

**Implementation**:
```go
// internal/db/db_postgres.go
//go:build !sqlite

package db

import "gorm.io/driver/postgres"

func connectDatabase(cfg Config) (*gorm.DB, error) {
    return connectPostgres(cfg)
}
```

```go
// internal/db/db_sqlite.go
//go:build sqlite

package db

import "github.com/glebarez/sqlite"

func connectDatabase(cfg Config) (*gorm.DB, error) {
    if cfg.Type == "sqlite" {
        return connectSQLite(cfg)
    }
    return connectPostgres(cfg)
}
```

```bash
# Build commands
make bin              # Postgres-only binary
make bin-sqlite       # SQLite-enabled binary
```

### ✅ Solution 5: Fork and Patch

**Approach**:
- Fork `modernc.org/sqlite`
- Remove automatic `sql.Register()` call from init()
- Manually register only when needed
- Use replace directive to use forked version

**Pros**:
- Surgical fix to exact problem
- Maintains all other functionality
- Pure-Go binary preserved

**Cons**:
- Maintain a fork
- Need to sync upstream changes
- Not a standard approach

**Implementation**:
```bash
# Fork modernc.org/sqlite
git clone https://github.com/modernc/sqlite
cd sqlite
# Edit sqlite.go: remove sql.Register() from init()
# Create github.com/hashicorp-forge/sqlite-noregister

# go.mod
replace modernc.org/sqlite => github.com/hashicorp-forge/sqlite-noregister v1.0.0
```

## Recommended Approach

### Phase 1: Immediate (This Week)

**Use Solution 1**: Remove SQLite support temporarily
- Update docs to emphasize Postgres as primary database
- Add clear error message when SQLite is selected
- Document the conflict in release notes
- Keep SQLite code in separate branch

### Phase 2: Short-term (Next Sprint)

**Implement Solution 4**: Build constraints
- Create two build targets
- Update CI/CD to produce both binaries
- Document when to use each variant
- Default to Postgres-only for simplicity

### Phase 3: Long-term (Future Release)

**Evaluate Solution 3**: Dependency refactoring
- Audit all usage of gorm.io/datatypes
- Research migration library alternatives
- Create refactoring plan
- Implement if justified by complexity/benefit ratio

## Workaround for Current Development

Until this is resolved, use the Postgres configuration:

```bash
# Start Postgres
cd testing && docker compose up -d postgres

# Use working config
cp testing/config-native.hcl config.hcl

# Build and run
make bin
./build/bin/hermes server -config=config.hcl
```

## Testing Plan After Fix

1. **Verify no double registration**:
   ```bash
   ./build/bin/hermes server -config=testing/config-sqlite.hcl
   # Should start without panic
   ```

2. **Verify only one driver loaded**:
   ```bash
   go tool nm build/bin/hermes | grep -i "sqlite.*register"
   # Should show only one registration call
   ```

3. **Verify CGO disabled** (if using pure-Go solution):
   ```bash
   go version -m build/bin/hermes | grep CGO
   # Should show CGO_ENABLED=false
   ```

4. **Integration tests**:
   ```bash
   make test/integration
   # All tests pass with both Postgres and SQLite
   ```

## Related Issues

- [ ] #123 - SQLite support for simplified mode (blocked by this)
- [ ] #124 - Pure-Go binary requirement (conflicts with CGO solution)
- [ ] #125 - gorm.io/datatypes usage audit (needed for Solution 3)

## References

- Go database/sql documentation: https://pkg.go.dev/database/sql
- modernc.org/sqlite source: https://gitlab.com/cznic/sqlite
- glebarez/sqlite GORM driver: https://github.com/glebarez/sqlite
- mattn/go-sqlite3: https://github.com/mattn/go-sqlite3
- Go Modules dependency management: https://go.dev/ref/mod

## Status Log

- **2025-10-27**: Issue discovered during embedded web assets testing
- **2025-10-27**: Attempted 7+ different solutions, all unsuccessful
- **2025-10-27**: Documented in this file, awaiting decision on approach
