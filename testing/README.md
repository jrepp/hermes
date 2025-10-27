# Hermes Testing Environment

> **📍 Location**: `./testing/` directory  
> **Purpose**: Complete containerized full-stack environment for testing, development, and manual QA

This directory contains a **fully containerized testing environment** with all services needed to run Hermes:
- PostgreSQL database
- Meilisearch for search
- Dex for OIDC authentication
- Hermes backend API
- Ember web frontend

## 🚀 Quick Start

```bash
# From project root
cd testing
./quick-test.sh

# Or use Make targets
make testing/up        # Start everything
make testing/test      # Run canary test
make testing/down      # Stop everything

# From within testing directory
make up                # Start all services
make canary            # Run canary test
make down              # Stop all services
```

## When to Use This Environment

- ✅ **Integration testing** - Test full stack together
- ✅ **CI/CD pipelines** - Automated testing
- ✅ **Production-like environment** - Realistic deployment simulation
- ✅ **End-to-end testing** - Complete user workflows with playwright-mcp
- ✅ **Manual QA** - Interactive testing via browser
- ✅ **Demonstrations** - Show the full application

## 🗂️ Projects Configuration (HCL)

This testing environment uses **modular HCL configuration** for project management. Each project is defined in its own file for better organization and maintainability.

**Structure**:
```
testing/
├── projects.hcl              # Main config with imports
└── projects/                 # Individual project configs
    ├── testing.hcl           # TEST - Local test workspace
    ├── docs.hcl              # DOCS - Public documentation
    ├── _template-google.hcl  # Template (not loaded)
    ├── _template-migration.hcl  # Template (not loaded)
    └── README.md             # Detailed documentation
```

**Active test projects**:
- **testing** (short name: TEST) - Local workspace at `./workspace_data`
- **docs** (short name: DOCS) - Documentation from `./docs-cms`

**Templates** (prefixed with `_template-`):
- Google Workspace integration example
- Migration scenario (Google → Git)
- Remote Hermes federation example

See `projects/README.md` for detailed configuration guide.

**For internal deployments**: Create `projects.local.hcl` or individual `projects/*.local.hcl` files (gitignored).

## Service Ports

All services use non-standard ports to avoid conflicts with local development:

- **PostgreSQL**: `5433` (container) / `5433` (host)
- **Meilisearch**: `7701` (container) / `7701` (host)
- **Dex OIDC**: `5558` (container) / `5558` (host)
- **Hermes API**: `8000` (container) / `8001` (host)
- **Web UI**: `4200` (container) / `4201` (host)

**Access the application**: Open http://localhost:4201 in your browser

---

## Distributed Testing Enhancements 🆕

The testing environment now includes **automated distributed authoring and indexing scenarios**:

### Quick Start with Test Data

```bash
# Start environment and seed with test documents
make up
make seed              # Generate 10 test documents

# Run basic indexing scenario (end-to-end)
make scenario-basic

# Open web UI to see indexed documents
make open              # Opens http://localhost:4201
```

### Available Scenarios (Bash)

```bash
# Basic scenario: RFCs, PRDs, Meeting Notes
make seed              # 10 documents
make seed-clean        # Clean and regenerate

# Migration scenario: Same UUID in multiple workspaces
make seed-migration    # Tests conflict detection

# Conflict scenario: Modified documents
make seed-conflict     # Simulates concurrent edits

# Multi-author scenario: Realistic timeline
make seed-multi-author # Different authors, dates, statuses
```

### Python Testing Framework 🆕 (Recommended)

**NEW**: Professional Python-based testing framework with type safety, pytest integration, and rich CLI output.

```bash
# Set up Python environment (first time)
make python-setup

# Run scenarios with Python
make scenario-basic-py              # Basic distributed indexing
make scenario-migration-py          # Migration with conflict detection
make scenario-multi-author-py       # Multi-author collaboration

# Seed workspaces with Python
make python-seed                    # Basic scenario (10 docs)
make python-seed-migration          # Migration scenario (5 docs)
make python-seed-multi-author       # Multi-author (10 docs)

# Run pytest tests
make test-python                    # Unit tests only
make test-python-integration        # Integration tests (requires Hermes)
make test-python-all                # All tests
make test-python-coverage           # With coverage report

# Full distributed test
make test-distributed-py            # Start + seed + scenario + validate
```

**Why Python?**
- ✅ Type-safe API interactions via `hc-hermes` client
- ✅ Better error handling and validation
- ✅ Pytest integration for automated testing
- ✅ Rich CLI output with progress indicators
- ✅ Automatic retries for indexing waits
- ✅ Easier to maintain and extend

**Documentation**: See `python/README.md` for comprehensive guide (600+ lines)

### Available Scenarios

```bash
# Basic scenario: RFCs, PRDs, Meeting Notes
make seed              # 10 documents
make seed-clean        # Clean and regenerate

# Migration scenario: Same UUID in multiple workspaces
make seed-migration    # Tests conflict detection

# Conflict scenario: Modified documents
make seed-conflict     # Simulates concurrent edits

# Multi-author scenario: Realistic timeline
make seed-multi-author # Different authors, dates, statuses
```

### What's New

- **Seed Scripts**: Automatically generate realistic test documents
- **Scenario Automation**: End-to-end testing workflows
- **Document Templates**: RFC, PRD, Meeting Notes generators
- **Multi-Indexer Support**: Test distributed indexing (coming soon)
- **Makefile Targets**: Easy access to all scenarios

**See**: `DISTRIBUTED_TESTING_ENHANCEMENTS.md` for complete documentation

---

## Architecture

```
┌─────────────┐     ┌──────────────────┐
│  Browser    │────▶│  Web (Ember Dev) │
│ :4201       │     │  Docker :4201    │
└─────────────┘     └──────┬───────────┘
                           │ /api/* proxied
                    ┌──────▼────────────┐
                    │  Hermes Backend   │
                    │  Docker :8001     │
                    └──┬─────────────┬──┘
                       │             │
        ┌──────────────▼──┐  ┌──────▼────────┐
        │  PostgreSQL     │  │  Meilisearch  │
        │  Docker :5433   │  │  Docker :7701 │
        └─────────────────┘  └───────────────┘
        
        ┌──────────────────────────────────────┐
        │  Migration Service (one-shot)        │
        │  - Runs before server starts         │
        │  - Exits after completing migrations │
        └──────────────────────────────────────┘
        
        ┌──────────────────────────────────────┐
        │  Indexer Agent (scans workspaces)    │
        │  - workspaces/testing/               │
        │  - workspaces/docs/                  │
        └──────────────────────────────────────┘
        
        All in isolated hermes-test network
```

### Database Migrations

The testing environment includes an **automated migration service** that runs before the Hermes server starts:

- **Migration Service** (`hermes-migrate`)
  - Runs database migrations automatically on startup
  - Uses dedicated migration binary (supports PostgreSQL + SQLite)
  - Exits with code 0 after successful migration
  - Server depends on migration completion (`condition: service_completed_successfully`)
  - Restarts on failure for resilience

**Manual Migration** (if needed):
```bash
# Run migrations manually
docker compose run --rm migrate

# Or use the migration binary directly
./build/bin/hermes-migrate -driver=postgres -dsn="..."
```

---

## Recommended Workflows

### Daily Development (Local)
```bash
# From project root - for fast iteration
make bin                   # Build Hermes binary
make bin/migrate           # Build migration binary
cd testing && make up      # Start supporting services (includes auto-migration)
cd .. && ./hermes server -config=testing/config.hcl

# OR run migrations manually if needed
make migrate/postgres/testing

# In another terminal: Run web dev server
cd web
yarn start

# Validate setup
make canary
```

### Pre-Commit Testing
```bash
# Test with containerized environment
cd testing
make test

# Or from root
make testing/test
```

### CI/CD Pipeline
```bash
# In .github/workflows or similar
- name: Run integration tests
  run: |
    cd testing
    make up
    make canary
    make down
```

### Demonstrating Features
```bash
# Start complete stack
cd testing
./quick-test.sh

# Open http://localhost:4201 in browser
# Show full application running
```

---

## Quick Command Reference

### Local Development
```bash
# Services
docker-compose up -d              # Start services
docker-compose down               # Stop services
docker-compose ps                 # Check status

# Build and test
make bin                          # Build Hermes binary
make canary                       # Test local setup
./hermes server                   # Run server locally
```

### Containerized Testing
```bash
# From root
make testing/up                   # Start containers
make testing/test                 # Run tests
make testing/down                 # Stop containers
make testing/clean                # Stop and remove volumes

# From testing/
make up                           # Start with auto-build
make build                        # Rebuild containers
make logs                         # View logs
make canary                       # Run canary test
make open                         # Open in browser
make clean                        # Full cleanup
```

---

## Troubleshooting

### Port Conflicts
If you get "port already in use" errors:
- **Local dev**: Check if containerized env is running (`cd testing && make down`)
- **Containerized**: Ports are different (5433, 7701, 8001, 4201) to avoid conflicts

### Services Not Healthy
```bash
# Local dev
docker-compose logs postgres
docker-compose logs meilisearch

# Containerized
cd testing
make logs-postgres
make logs-meilisearch
```

### Build Failures
```bash
# Containerized: Rebuild without cache
cd testing
make rebuild
```

### Database Issues
```bash
# Local dev: Reset database
docker-compose down -v
docker-compose up -d

# Containerized: Reset all data
cd testing
make clean
make up
```

---

## Architecture Diagrams

### Local Development
```
┌─────────────┐     ┌──────────────┐
│  Browser    │────▶│  Web (Yarn)  │
│ :4200       │     │  localhost   │
└─────────────┘     └──────┬───────┘
                           │
                    ┌──────▼────────┐
                    │  Hermes (Go)  │
                    │  ./hermes     │
                    │  localhost    │
                    └──┬─────────┬──┘
                       │         │
        ┌──────────────▼┐  ┌────▼──────────┐
        │  PostgreSQL   │  │  Meilisearch  │
        │  Docker :5432 │  │  Docker :7700 │
        └───────────────┘  └───────────────┘
```

### Containerized Testing
```
┌─────────────┐     ┌──────────────────┐
│  Browser    │────▶│  Web (Nginx)     │
│ :4201       │     │  Docker :4201    │
└─────────────┘     └──────┬───────────┘
                           │ /api/*
                    ┌──────▼────────────┐
                    │  Hermes Backend   │
                    │  Docker :8001     │
                    └──┬─────────────┬──┘
                       │             │
        ┌──────────────▼──┐  ┌──────▼────────┐
        │  PostgreSQL     │  │  Meilisearch  │
        │  Docker :5433   │  │  Docker :7701 │
        └─────────────────┘  └───────────────┘
        
        All in isolated hermes-test network
```

---

## Files and Directories

```
hermes/
├── docker-compose.yml          # Local dev services
├── Makefile                    # Root make targets
├── scripts/
│   ├── canary-local.sh        # Local canary test
│   └── README.md
└── testing/                    # Complete containerized environment
    ├── docker-compose.yml     # Full stack definition
    ├── Dockerfile.hermes      # Backend container
    ├── Dockerfile.web         # Frontend container
    ├── nginx.conf             # Web server config
    ├── config.hcl             # Test configuration
    ├── Makefile               # Testing commands
    ├── quick-test.sh          # One-command startup
    └── README.md              # Detailed documentation
```

---

## Current Status

### ✅ Working
- **Local Development Environment**: Fully functional
  - PostgreSQL and Meilisearch services run in Docker
  - Hermes binary runs locally with hot reload
  - Web dev server with live updates
  - Canary test validates end-to-end functionality

### 🚧 In Progress
- **Containerized Testing Environment**: Build infrastructure complete, runtime blocked
  - ✅ Docker builds complete successfully (11.5s)
  - ✅ PostgreSQL container starts and passes health checks
  - ✅ Meilisearch container starts and passes health checks
  - ✅ Web container builds with pre-built assets
  - ❌ Hermes container fails at runtime (Algolia dependency)

### 🔴 Known Issues

**Hermes Server Requires Algolia Connection**

The Hermes server command currently requires a working Algolia connection even in test/development mode. This blocks the containerized testing environment from starting.

**Error**: `error initializing Algolia write client: all hosts have been contacted unsuccessfully`

**Root Cause**: Server initialization unconditionally connects to Algolia for search indexing, unlike the canary command which supports `-search-backend` flag.

**Workarounds**:
1. **Use Local Development** (recommended): Run hermes locally with local/meilisearch adapter
2. **Mock Algolia**: Set up mock Algolia service in docker-compose
3. **Code Change**: Add `-search-backend` flag to server command (like canary has)

## Next Steps

1. **For Development** (✅ RECOMMENDED): Use local dev environment
   ```bash
   docker-compose up -d
   make bin
   make canary
   ```

2. **For Testing** (🚧 BLOCKED): Containerized environment needs fix
   ```bash
   # Blocked: Requires Algolia or search backend abstraction
   cd testing
   ./quick-test.sh  # Will fail at hermes startup
   ```

3. **For Contributors**: Fix the Algolia dependency
   - Add search backend selection to server command
   - Or add conditional Algolia initialization
   - Reference: `internal/cmd/commands/canary/canary.go` (has `-search-backend` flag)

4. **Read the Docs**:
   - `scripts/README.md` - Canary test details
   - `testing/README.md` - Full containerized setup guide
   - `.github/copilot-instructions.md` - Build standards and workflows
