# RFC: Local Developer Mode with Central Hermes

**Status**: 🚧 Draft  
**Version**: 1.0.0-draft  
**Created**: October 24, 2025  
**Authors**: Development Team

## Overview

Enable developers to run Hermes locally in their workspace with minimal setup while synchronizing with a central Hermes instance for team collaboration. This architecture supports:

- ✅ **Zero-config local mode**: Developers run Hermes in their project directory
- ✅ **Bidirectional sync**: Local changes push to central Hermes for indexing/search
- ✅ **SQLite for local**: Lightweight embedded database, no PostgreSQL required locally
- ✅ **Indexer registration**: Central Hermes indexes content from local instances
- ✅ **Easy vending**: Drop Hermes binary into any workspace and start documenting

## Problem Statement

Currently, Hermes requires:
1. **PostgreSQL database** - heavyweight for local development
2. **Full server setup** - complex configuration
3. **No central sync** - local edits don't appear in team search
4. **Manual deployment** - difficult to "vend" Hermes into new workspaces

Developers want to:
- Run Hermes locally in their Git repo with minimal dependencies
- Have their local documents indexed by central Hermes for team discovery
- Avoid managing PostgreSQL/Meilisearch/infrastructure locally
- Spin up Hermes in new projects with one command

## Architecture

### Operating Modes

Hermes will support two deployment modes:

#### **Central Mode** (Server)
- Runs on infrastructure (K8s, VM, Docker)
- Uses PostgreSQL for persistence
- Runs Meilisearch/Algolia for search
- Aggregates content from:
  - Local Hermes instances (via indexer API)
  - Google Workspace providers
  - Other remote Hermes instances
- Provides web UI for search/browsing

#### **Local Mode** (Developer)
- Runs as local process in workspace directory
- Uses SQLite for persistence (`.hermes/hermes.db`)
- No search engine (delegates to central)
- Serves local web UI (optional)
- Registers with central Hermes for indexing
- Example: `./hermes local-server -central=https://hermes.company.com`

### Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Central Hermes                          │
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐ │
│  │ Web UI   │  │  API v2  │  │  Indexer  │  │PostgreSQL│ │
│  └──────────┘  └──────────┘  └───────────┘  └──────────┘ │
│                      │              │                       │
│                      │              │                       │
│              ┌───────┴──────────────┴────────┐            │
│              │  Indexer Registration API     │            │
│              │  /api/v2/indexer/register     │            │
│              │  /api/v2/indexer/heartbeat    │            │
│              │  /api/v2/indexer/documents    │            │
│              └───────────────────────────────┘            │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ HTTPS + Token Auth
                       │
         ┌─────────────┼─────────────┬──────────────┐
         │             │             │              │
         ▼             ▼             ▼              ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ...
│ Local Hermes │ │ Local Hermes │ │ Local Hermes │
│  (Dev 1)     │ │  (Dev 2)     │ │  (CI/CD)     │
│              │ │              │ │              │
│ SQLite DB    │ │ SQLite DB    │ │ SQLite DB    │
│ Local Files  │ │ Local Files  │ │ Local Files  │
│ Git Repo     │ │ Git Repo     │ │ Git Repo     │
└──────────────┘ └──────────────┘ └──────────────┘
```

### Database Strategy: Dual PostgreSQL + SQLite Support

#### Migration System

Use **[golang-migrate](https://github.com/golang-migrate/migrate)** for database versioning:

**Why golang-migrate?**
- ✅ Industry standard (21k+ GitHub stars)
- ✅ CLI + library interface
- ✅ Supports PostgreSQL AND SQLite with same migration files
- ✅ Version tracking in database (`schema_migrations` table)
- ✅ Up/down migrations for rollback support
- ✅ Embedded migration files via `go:embed`

**Migration Structure**:
```
internal/db/migrations/
  000001_initial_schema.up.sql
  000001_initial_schema.down.sql
  000002_add_indexer_tokens.up.sql
  000002_add_indexer_tokens.down.sql
  000003_add_document_uuids.up.sql
  000003_add_document_uuids.down.sql
```

**Implementation Pattern**:
```go
package db

import (
    "embed"
    "database/sql"
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    "github.com/golang-migrate/migrate/v4/database/sqlite"
    "github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies all pending migrations
func RunMigrations(db *sql.DB, driver string) error {
    sourceDriver, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return fmt.Errorf("failed to load migrations: %w", err)
    }
    
    var databaseDriver migrate.DatabaseDriver
    switch driver {
    case "postgres":
        databaseDriver, err = postgres.WithInstance(db, &postgres.Config{})
    case "sqlite":
        databaseDriver, err = sqlite.WithInstance(db, &sqlite.Config{})
    default:
        return fmt.Errorf("unsupported database driver: %s", driver)
    }
    
    m, err := migrate.NewWithDatabaseInstance(
        "iofs", sourceDriver,
        driver, databaseDriver,
    )
    if err != nil {
        return fmt.Errorf("failed to create migration instance: %w", err)
    }
    
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migration failed: %w", err)
    }
    
    return nil
}
```

#### Database Abstraction

**Current**: `internal/db/db.go` hardcoded for PostgreSQL  
**New**: Support driver selection via config

```go
// internal/db/db.go

type DatabaseConfig struct {
    Driver   string // "postgres" or "sqlite"
    
    // PostgreSQL config
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
    
    // SQLite config
    Path     string // e.g., ".hermes/hermes.db"
}

func NewDB(cfg DatabaseConfig) (*gorm.DB, error) {
    var dialector gorm.Dialector
    
    switch cfg.Driver {
    case "postgres":
        dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d",
            cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port)
        dialector = postgres.Open(dsn)
        
    case "sqlite":
        dialector = sqlite.Open(cfg.Path)
        
    default:
        return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
    }
    
    db, err := gorm.Open(dialector, &gorm.Config{})
    if err != nil {
        return nil, fmt.Errorf("error connecting to database: %w", err)
    }
    
    // Get underlying sql.DB for migrations
    sqlDB, err := db.DB()
    if err != nil {
        return nil, fmt.Errorf("error getting sql.DB: %w", err)
    }
    
    // Run migrations
    if err := RunMigrations(sqlDB, cfg.Driver); err != nil {
        return nil, fmt.Errorf("error running migrations: %w", err)
    }
    
    // PostgreSQL-specific setup (citext extension)
    if cfg.Driver == "postgres" {
        if err := enableCitextExtension(sqlDB); err != nil {
            return nil, err
        }
    }
    
    // Setup join tables (works for both drivers)
    if err := setupJoinTables(db); err != nil {
        return nil, err
    }
    
    return db, nil
}
```

### Indexer Registration Protocol

#### Token-Based Authentication

**Bootstrap Process**:
1. Central Hermes generates registration token on startup
2. Token written to shared volume: `/app/shared/indexer-token.txt`
3. Indexer service starts (depends_on: hermes-server)
4. Indexer reads token from shared volume
5. Indexer registers with central Hermes using token

**Token Format**:
```
hermes-indexer-token-<UUID>-<HMAC-signature>
Example: hermes-indexer-token-550e8400-e29b-41d4-a716-446655440000-a7b3c9d2e1f4
```

**Token Storage** (new table):
```sql
-- 000002_add_indexer_tokens.up.sql
CREATE TABLE indexer_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash VARCHAR(256) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    indexer_id UUID REFERENCES indexers(id),
    metadata JSONB
);

CREATE INDEX idx_indexer_tokens_hash ON indexer_tokens(token_hash);
```

#### API Endpoints

**1. Register Indexer** (POST `/api/v2/indexer/register`)

Request:
```json
{
  "token": "hermes-indexer-token-550e8400-...",
  "indexer_type": "local-workspace",
  "workspace_path": "/app/workspaces/testing",
  "metadata": {
    "hostname": "hermes-indexer-testing",
    "version": "v1.2.3"
  }
}
```

Response:
```json
{
  "indexer_id": "idx-550e8400-e29b-41d4-a716-446655440000",
  "api_token": "hermes-api-token-abc123...",
  "expires_at": "2025-11-24T00:00:00Z"
}
```

**2. Submit Documents** (POST `/api/v2/indexer/documents`)

Headers:
```
Authorization: Bearer hermes-api-token-abc123...
```

Request:
```json
{
  "indexer_id": "idx-550e8400-...",
  "documents": [
    {
      "uuid": "doc-550e8400-e29b-41d4-a716-446655440000",
      "path": "docs/rfc-001.md",
      "title": "RFC-001: Local Developer Mode",
      "content": "...",
      "content_hash": "sha256:abc123...",
      "modified_at": "2025-10-24T10:00:00Z",
      "metadata": {
        "type": "RFC",
        "status": "Draft"
      }
    }
  ]
}
```

**3. Heartbeat** (POST `/api/v2/indexer/heartbeat`)

Request:
```json
{
  "indexer_id": "idx-550e8400-...",
  "status": "healthy",
  "document_count": 42
}
```

### Docker Compose Integration

Update `testing/docker-compose.yml`:

```yaml
services:
  hermes:
    container_name: hermes-server
    # ... existing config ...
    volumes:
      # ... existing volumes ...
      - indexer_shared:/app/shared  # NEW: shared volume for token
    environment:
      # ... existing env ...
      HERMES_INDEXER_TOKEN_PATH: /app/shared/indexer-token.txt
      
  hermes-indexer:  # NEW SERVICE
    container_name: hermes-indexer
    build:
      context: ..
      dockerfile: Dockerfile
    volumes:
      # Shared token volume
      - indexer_shared:/app/shared:ro
      # Access to workspace data
      - ./workspaces/testing:/app/workspaces/testing:ro
      - ./workspaces/docs:/app/workspaces/docs:ro
    environment:
      HERMES_INDEXER_TOKEN_PATH: /app/shared/indexer-token.txt
      HERMES_CENTRAL_URL: http://hermes:8000
      HERMES_WORKSPACE_PATH: /app/workspaces
    command: ["indexer-agent", "-central=http://hermes:8000"]
    depends_on:
      hermes:
        condition: service_healthy
    networks:
      - hermes-testing

volumes:
  # ... existing volumes ...
  indexer_shared:  # NEW VOLUME
```

### Local Hermes Example: Vending into Workspace

Create `testing/local-hermes-example/`:

**Directory Structure**:
```
testing/local-hermes-example/
  README.md           # How to use local Hermes
  .hermes/
    config.hcl        # Local mode configuration
  docs/
    rfc-001.md        # Example document
    templates/
      rfc.md
```

**Configuration** (`testing/local-hermes-example/.hermes/config.hcl`):
```hcl
# Hermes Local Mode Configuration
# This config connects to the testing environment's central Hermes

# Operating mode
mode = "local"  # local | central (default: central)

# Central Hermes connection (for local mode)
central_hermes {
  url = "http://localhost:8001"  # Testing environment
  # Token will be generated on first run and saved to .hermes/token
}

# Local database (SQLite)
database {
  driver = "sqlite"
  path   = ".hermes/hermes.db"
}

# Local workspace
workspace {
  provider = "local"
  path     = "."  # Current directory
  
  # Which folders to index
  document_folders = ["docs", "rfcs"]
  template_folders = ["docs/templates"]
}

# Document types (same as central)
document_types {
  document_type "RFC" {
    long_name = "Request for Comments"
    template  = "docs/templates/rfc.md"
    # ... same config as central ...
  }
}
```

**Usage**:
```bash
# Navigate to workspace
cd testing/local-hermes-example

# Start local Hermes
../../build/bin/hermes local-server

# Output:
# ✓ SQLite database initialized at .hermes/hermes.db
# ✓ Connected to central Hermes at http://localhost:8001
# ✓ Registered as indexer: idx-550e8400-e29b-41d4-a716-446655440000
# ✓ Indexing documents in: docs/, rfcs/
# ✓ Local UI available at http://localhost:4200
# ✓ Documents synced to central Hermes every 5 minutes

# In another terminal - make changes
echo "# RFC-002: New Feature" > docs/rfc-002.md

# Hermes detects change and syncs to central
# ✓ Detected new document: docs/rfc-002.md
# ✓ Synced to central Hermes (200 OK)
```

## Implementation Plan

### Phase 1: Database Migration System ✅ (Foundation)
- [ ] Add `golang-migrate` dependency to `go.mod`
- [ ] Create `internal/db/migrations/` directory
- [ ] Convert existing GORM AutoMigrate to SQL migrations
- [ ] Implement `RunMigrations()` with embedded files
- [ ] Add SQLite driver support (`gorm.io/driver/sqlite`)
- [ ] Update `NewDB()` to accept `DatabaseConfig` with driver selection
- [ ] Test migrations on PostgreSQL (existing tests)
- [ ] Test migrations on SQLite (new test suite)

**Deliverables**:
- `internal/db/db.go` supports both PostgreSQL and SQLite
- Migration files in `internal/db/migrations/*.sql`
- Tests pass with both database drivers

### Phase 2: Indexer Registration API ✅ (Server-Side)
- [ ] Create `pkg/models/indexer.go` model
- [ ] Create `pkg/models/indexer_token.go` model
- [ ] Add migration `000002_add_indexer_tokens.up.sql`
- [ ] Implement token generation on server startup
- [ ] Implement `/api/v2/indexer/register` endpoint
- [ ] Implement `/api/v2/indexer/documents` endpoint (ingest)
- [ ] Implement `/api/v2/indexer/heartbeat` endpoint
- [ ] Add token validation middleware

**Deliverables**:
- API endpoints in `internal/api/v2/indexer.go`
- Server generates token to shared volume on startup
- Integration tests for registration flow

### Phase 3: Docker Compose Indexer ✅ (Testing Environment)
- [ ] Update `testing/docker-compose.yml` with `hermes-indexer` service
- [ ] Add `indexer_shared` volume for token exchange
- [ ] Create `cmd/hermes/commands/indexer-agent` command
- [ ] Implement indexer agent startup logic
- [ ] Implement token reading from shared volume
- [ ] Implement registration with central Hermes
- [ ] Implement document scanning and submission
- [ ] Test full flow: server startup → token generation → indexer registration → document sync

**Deliverables**:
- `hermes-indexer` container running in `testing/` environment
- Successful registration visible in logs
- Documents from `testing/workspaces/` indexed in central Hermes

### Phase 4: Local Mode Configuration ✅ (Developer Experience)
- [ ] Add `mode` field to `internal/config/config.go`
- [ ] Add `central_hermes` block to config parser
- [ ] Create `cmd/hermes/commands/local-server` command
- [ ] Implement SQLite initialization for local mode
- [ ] Implement central Hermes registration from local mode
- [ ] Create `testing/local-hermes-example/` directory
- [ ] Write example configuration and README
- [ ] Test local → central communication

**Deliverables**:
- `hermes local-server` command works
- Example in `testing/local-hermes-example/` functions
- Documentation for vending Hermes into new workspaces

### Phase 5: Documentation & Integration Tests ✅ (Quality Assurance)
- [ ] Create this RFC document
- [ ] Update `docs-internal/README.md` with local mode guide
- [ ] Create E2E test for local → central flow
- [ ] Add Playwright test for indexer registration UI
- [ ] Update `MAKEFILE_ROOT_TARGETS.md` with new commands
- [ ] Write migration guide from PostgreSQL-only to dual-database

**Deliverables**:
- Complete documentation
- Passing E2E tests
- Migration guide for existing deployments

## Configuration Schema

### Central Mode (existing + new fields)
```hcl
# mode is optional, defaults to "central"
mode = "central"

database {
  driver   = "postgres"  # postgres | sqlite
  host     = "localhost"
  port     = 5432
  user     = "postgres"
  password = "postgres"
  dbname   = "hermes"
}

indexer {
  # NEW: Enable indexer registration API
  enable_registration = true
  token_path          = "/app/shared/indexer-token.txt"
  token_ttl           = "24h"
}
```

### Local Mode (new)
```hcl
mode = "local"

central_hermes {
  url          = "https://hermes.company.com"
  token        = env("HERMES_CENTRAL_TOKEN")  # Or auto-register
  sync_interval = "5m"
}

database {
  driver = "sqlite"
  path   = ".hermes/hermes.db"
}

workspace {
  provider         = "local"
  path             = "."
  document_folders = ["docs", "rfcs"]
}
```

## Security Considerations

### Token Security
- ✅ Tokens are cryptographically secure (UUID + HMAC)
- ✅ Tokens stored as SHA-256 hash in database
- ✅ Token files have restricted permissions (0600)
- ✅ Tokens can be revoked via API
- ✅ Tokens have expiration (default: 24h, renewable)

### Network Security
- ✅ HTTPS required for production central Hermes
- ✅ Token transmitted in Authorization header (TLS encrypted)
- ✅ Optional: Mutual TLS for indexer authentication
- ✅ Rate limiting on registration endpoint

### Local Mode Isolation
- ✅ SQLite database permissions inherit from filesystem
- ✅ Local Hermes cannot access other users' data
- ✅ Central Hermes validates document ownership on ingest

## Testing Strategy

### Unit Tests
- Database migration up/down for PostgreSQL
- Database migration up/down for SQLite
- Token generation and validation
- API endpoint handlers

### Integration Tests
- Full registration flow (token → register → documents)
- PostgreSQL migration from v1 schema to v2
- SQLite database initialization from scratch
- Multi-indexer scenario (2+ indexers registering)

### E2E Tests (Playwright)
- Start testing environment with indexer
- Verify indexer registration in UI
- Add document to local workspace
- Verify document appears in search
- Test local Hermes → central sync

### Manual Testing
- Vend Hermes into new Git repo
- Run `hermes local-server`
- Verify documents sync to central
- Stop local Hermes, restart, verify re-registration

## Migration Path for Existing Deployments

### Step 1: Upgrade Database Schema
```bash
# Backup database
pg_dump hermes > hermes_backup.sql

# Run migration
./hermes migrate -config=config.hcl

# Verify
psql hermes -c "SELECT version FROM schema_migrations;"
```

### Step 2: Enable Indexer Registration (Optional)
```hcl
# config.hcl
indexer {
  enable_registration = true
  token_path          = "/var/hermes/indexer-token.txt"
}
```

### Step 3: Deploy Indexer (Optional)
```bash
# If using external indexer
docker run -v /var/hermes:/shared \
  hermes:latest indexer-agent -central=https://hermes.company.com
```

## Success Metrics

- ✅ Developer can run `hermes local-server` in <5 minutes (first time)
- ✅ Local → central sync latency <30 seconds
- ✅ Zero PostgreSQL dependency for local mode
- ✅ Indexer registration success rate >99.9%
- ✅ Migration runs complete in <5 minutes for 100k documents

## Future Enhancements

### Phase 2 (Beyond Initial RFC)
- **Bidirectional sync**: Central changes pull to local
- **Conflict resolution**: UI for resolving local vs central edits
- **Offline mode**: Queue changes when central is unreachable
- **Multi-central**: Register with multiple central Hermes instances
- **Federated search**: Local Hermes proxies search to central

### Phase 3 (Advanced)
- **P2P indexer mesh**: Indexers discover each other via mDNS
- **Edge caching**: Local Hermes caches central search results
- **GitOps integration**: Auto-commit local DB to Git for versioning

## References

- [DISTRIBUTED_PROJECTS_ARCHITECTURE.md](./DISTRIBUTED_PROJECTS_ARCHITECTURE.md)
- [DISTRIBUTED_PROJECTS_ROADMAP.md](./DISTRIBUTED_PROJECTS_ROADMAP.md)
- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [GORM SQLite Driver](https://github.com/gorm-io/sqlite)

## Open Questions

1. **Token rotation**: Should tokens auto-rotate? If so, what's the rotation period?
2. **Indexer authentication**: Use tokens only, or support OIDC for production?
3. **Document conflicts**: How to handle same document edited locally + centrally?
4. **SQLite performance**: What's the document limit before recommending PostgreSQL?
5. **Schema divergence**: How to ensure SQLite and PostgreSQL stay compatible?

## Decision Log

### 2025-10-24: Chose golang-migrate over alternatives
**Considered**: GORM AutoMigrate, go-pg/migrations, Goose, Atlas  
**Decision**: golang-migrate for industry standard, embedded support, dual driver support  
**Rationale**: Best fit for our use case, strong community, SQLite + PostgreSQL support

### 2025-10-24: Token-based auth for indexer registration
**Considered**: OIDC, mutual TLS, API keys, shared secret  
**Decision**: Token-based with optional OIDC upgrade path  
**Rationale**: Simplest for Docker Compose testing, extensible for production
