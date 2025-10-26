# Simplified Local Mode - Demo

This document demonstrates the zero-config simplified mode implemented in RFC-083.

## Quick Start

```bash
# 1. Download and run - that's it!
./hermes serve

# The binary will:
# - Auto-create ./docs-cms/ workspace
# - Initialize SQLite database (embedded)
# - Start Bleve search indexer (embedded)
# - Launch browser to http://localhost:8000
# - Display colorful startup banner
```

## What Gets Created

When you run `./hermes serve` for the first time, it automatically creates:

```
docs-cms/
├── README.md              # Workspace documentation
├── config.yaml            # Optional configuration overrides
├── documents/             # Published documents
├── drafts/                # Work-in-progress documents
├── attachments/           # Images, PDFs, etc.
├── templates/             # Document templates
│   ├── rfc.md            # RFC template
│   ├── prd.md            # Product Requirements Doc
│   └── frd.md            # Functional Requirements Doc
└── data/                  # Auto-managed (do not edit)
    ├── hermes.db         # SQLite database
    └── search-index/     # Bleve full-text index
```

## Startup Banner

When you run `./hermes serve`, you see:

```
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║  Hermes CMS - Simplified Mode                                ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝

🌐 Web UI:       http://localhost:8000
📁 Workspace:    /tmp/hermes-demo/docs-cms
💾 Database:     SQLite (embedded)
                 /tmp/hermes-demo/docs-cms/data/hermes.db
🔍 Search:       Bleve (embedded)
                 /tmp/hermes-demo/docs-cms/data/search-index

💡 Quick Start:
   • Create your first document by clicking "New Document"
   • Search is automatically indexed as you create documents
   • All data is stored locally in the docs-cms directory

Press Ctrl+C to stop the server
```

## Command Options

### Zero-Config Mode (Simplified)

```bash
# Use current directory ./docs-cms/
./hermes serve

# Use specific directory
./hermes serve /path/to/my-docs

# Disable browser auto-launch
./hermes serve --browser=false
```

### Traditional Mode (Config File)

```bash
# Use explicit config file (switches to traditional mode)
./hermes serve -config=config.hcl
```

## Features Comparison

| Feature | Simplified Mode | Traditional Mode |
|---------|----------------|------------------|
| **Config Required** | No (auto-generated) | Yes (config.hcl) |
| **Database** | SQLite (embedded) | PostgreSQL (external) |
| **Search** | Bleve (embedded) | Algolia/Meilisearch (external) |
| **Workspace** | Local filesystem | Google Drive or Local |
| **Authentication** | Dex (embedded) | Google/Okta/Dex |
| **Dependencies** | None | PostgreSQL, Search service |
| **Setup Time** | 0 seconds | ~5-10 minutes |
| **Best For** | Personal use, testing, demos | Production, teams, enterprise |

## Technical Details

### What Happens Under the Hood

1. **Mode Detection**: Checks if `-config` flag is present
   - If present → Traditional server mode
   - If absent → Simplified mode

2. **Workspace Initialization** (if `./docs-cms/` doesn't exist):
   - Creates directory structure
   - Writes default templates (RFC, PRD, FRD)
   - Generates config.yaml
   - Writes README.md

3. **Auto-Configuration Generation**:
   ```go
   cfg := config.GenerateSimplifiedConfig(workspacePath)
   // Sets:
   // - SimplifiedMode: true
   // - DatabaseType: "sqlite"
   // - DBPath: "<workspace>/data/hermes.db"
   // - Search provider: Bleve
   // - Workspace provider: Local
   // - Auth provider: Dex (embedded)
   ```

4. **Server Startup**:
   - Initializes SQLite database (auto-migration)
   - Starts Bleve indexer (creates search-index/)
   - Health check polling (100ms intervals, 10s timeout)
   - Auto-launches browser (if `--browser=true`)
   - Displays colorized banner

### Database

- **Type**: SQLite (pure Go, no CGO)
- **Location**: `<workspace>/data/hermes.db`
- **Size**: Starts at ~20KB, grows with content
- **Backup**: Simple file copy

### Search Index

- **Type**: Bleve (pure Go full-text search)
- **Location**: `<workspace>/data/search-index/`
- **Features**:
  - Full-text search across documents
  - Faceted search (by type, status, project)
  - Highlighting
  - Pagination
  - Sorting
- **Indexes**: 4 separate indexes (documents, drafts, projects, links)

### Browser Launch

Platform-specific commands:
- **macOS**: `open http://localhost:8000`
- **Linux**: `xdg-open http://localhost:8000`
- **Windows**: `cmd /c start http://localhost:8000`

## Example Session

```bash
$ cd /tmp/my-project
$ /path/to/hermes serve

Initializing new Hermes workspace at /tmp/my-project/docs-cms
✓ Workspace initialized successfully

╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║  Hermes CMS - Simplified Mode                                ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝

🌐 Web UI:       http://localhost:8000
📁 Workspace:    /tmp/my-project/docs-cms
💾 Database:     SQLite (embedded)
                 /tmp/my-project/docs-cms/data/hermes.db
🔍 Search:       Bleve (embedded)
                 /tmp/my-project/docs-cms/data/search-index

💡 Quick Start:
   • Create your first document by clicking "New Document"
   • Search is automatically indexed as you create documents
   • All data is stored locally in the docs-cms directory

Press Ctrl+C to stop the server

# Browser opens automatically to http://localhost:8000
# Server is ready!
```

## Implementation Commits

All work completed in 5 commits on branch `jrepp/dev-tidy`:

1. **4405c38** - RFC-083 documentation (4 files)
2. **1953da5** - Phase 1: Zero-config serve command with SQLite
3. **fb325c8** - Phase 2: Bleve embedded search integration
4. **ed51174** - Phase 3: Browser auto-launch and startup banner
5. **45ebff1** - Phase 3: Add simplified_mode to web config API

## Files Modified/Created

**Backend**:
- `internal/cmd/commands/serve/serve.go` (new, 162 lines)
- `internal/cmd/commands/serve/browser.go` (new, 108 lines)
- `internal/workspace/init.go` (new, 150+ lines)
- `internal/config/config.go` (+50 lines)
- `internal/cmd/commands/server/server.go` (+30 lines)
- `pkg/search/adapters/bleve/adapter.go` (new, 800 lines)
- `web/web.go` (+2 lines)

**Frontend**:
- `web/app/services/config.ts` (+1 line)

**Documentation**:
- `docs-internal/rfc/RFC-083-simplified-local-mode.md` (new)
- `docs-internal/rfc/RFC-083-SUMMARY.md` (new)
- `docs-internal/rfc/RFC-083-ARCHITECTURE.md` (new)
- `docs-internal/rfc/RFC-083-IMPLEMENTATION-CHECKLIST.md` (new)

## Next Steps

**Completed** ✅:
- Phase 1: Zero-config serve command with SQLite
- Phase 2: Bleve embedded search integration
- Phase 3: Browser auto-launch and startup banner
- Phase 3: Simplified mode API detection

**Remaining** (Phase 4):
- [ ] Update README.md with Quick Start section
- [ ] Add `make serve` Makefile target
- [ ] End-to-end testing

## Try It Now

```bash
# Build the binary
make bin

# Run in any directory
cd /tmp/test-hermes
./hermes serve

# Or specify a path
./hermes serve ~/my-documents

# Open http://localhost:8000 and start creating documents!
```
