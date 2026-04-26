# RFC-083 Implementation Checklist

## Phase 1: Foundation (Week 1-2)

### Backend - Command Detection & Workspace Initialization

- [ ] **New `serve` command** (`internal/cmd/commands/serve/serve.go`)
  - [ ] Detect zero-config mode (no `-config` flag)
  - [ ] Check for `./docs-cms/` directory existence
  - [ ] Fall back to `server` command if `-config` provided
  - [ ] Accept optional path argument: `./hermes /path/to/docs`

- [ ] **Workspace initialization** (`internal/workspace/init.go`)
  - [ ] Create directory structure (`data/`, `documents/`, `drafts/`, `attachments/`, `templates/`)
  - [ ] Generate default templates (RFC, PRD, FRD markdown files)
  - [ ] Write minimal `config.yaml` (auto-generated)
  - [ ] Display initialization success message

- [ ] **SQLite database integration** (`internal/db/db.go`)
  - [ ] Add `modernc.org/sqlite` dependency to `go.mod`
  - [ ] Add conditional database provider selection:
    ```go
    if cfg.SimplifiedMode {
        return gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
    } else {
        return gorm.Open(postgres.Open(cfg.Postgres.DSN()), &gorm.Config{})
    }
    ```
  - [ ] Test GORM migrations work with SQLite (schema compatibility)
  - [ ] Handle SQLite-specific quirks (e.g., no `DROP COLUMN` support)

- [ ] **Configuration model updates** (`internal/config/config.go`)
  - [ ] Add `SimplifiedMode bool` field
  - [ ] Add `DatabaseType string` field ("sqlite" | "postgres")
  - [ ] Add `DBPath string` field (for SQLite file path)
  - [ ] Create `GenerateSimplifiedConfig()` function
  - [ ] Support YAML parsing (in addition to HCL)

- [ ] **Local workspace provider enhancements** (`pkg/workspace/local/`)
  - [ ] Auto-create directories on first use
  - [ ] Add markdown frontmatter parsing/writing
  - [ ] Handle attachments (copy to `./docs-cms/attachments/`)

### Testing

- [ ] Unit tests for workspace initialization
- [ ] SQLite database migration tests
- [ ] Config generation tests
- [ ] Command-line argument parsing tests

### Documentation

- [ ] Update `README.md` with simplified mode quick start
- [ ] Add `docs-internal/SIMPLIFIED_MODE.md` user guide
- [ ] Update Makefile with `make serve` target

**Deliverable**: `./hermes` starts server with SQLite + local workspace (no search yet)

---

## Phase 2: Embedded Search (Week 3-4)

### Backend - Bleve Integration

- [ ] **Add Bleve dependency**
  - [ ] Add `github.com/blevesearch/bleve/v2` to `go.mod`
  - [ ] Add `github.com/blevesearch/bleve/v2/mapping` for index mapping

- [ ] **Bleve search adapter** (`pkg/search/bleve/bleve.go`)
  - [ ] Implement `search.Adapter` interface:
    ```go
    type BleveAdapter struct {
        index bleve.Index
        path  string
    }
    
    func NewAdapter(indexPath string) (*BleveAdapter, error)
    func (b *BleveAdapter) IndexDocument(ctx, doc) error
    func (b *BleveAdapter) Search(ctx, query, opts) (SearchResults, error)
    func (b *BleveAdapter) DeleteDocument(ctx, id) error
    ```
  - [ ] Define index mapping (title boost, document type facets)
  - [ ] Implement search result ranking (match Algolia behavior)
  - [ ] Add highlighting support (match Algolia snippets)

- [ ] **Indexer integration** (`internal/indexer/indexer.go`)
  - [ ] Add Bleve to provider selection (alongside Algolia/Meilisearch)
  - [ ] Test incremental updates (only index changed documents)
  - [ ] Add bulk indexing for initial workspace scan

- [ ] **Search endpoint updates** (`internal/api/search.go`)
  - [ ] Update search proxy to detect Bleve backend
  - [ ] Transform Algolia query format to Bleve query
  - [ ] Transform Bleve results to Algolia response format (for frontend compatibility)

### Frontend - Search UI Adjustments

- [ ] **Config service** (`web/app/services/config.ts`)
  - [ ] Add `search_provider: "bleve"` detection
  - [ ] Conditionally adjust search UI based on provider

- [ ] **Search components** (if needed)
  - [ ] Verify faceted search works with Bleve response format
  - [ ] Verify highlighting works with Bleve snippets
  - [ ] Test search suggestions (autocomplete)

### Testing

- [ ] Unit tests for Bleve adapter
- [ ] Integration tests: index 100 documents, verify search results
- [ ] Performance tests: search latency < 100ms
- [ ] Compare Bleve vs. Algolia search quality (spot checks)

### Documentation

- [ ] Add Bleve architecture diagram to RFC
- [ ] Document search tuning options (analyzers, stemming)
- [ ] Add troubleshooting guide (index corruption, rebuilds)

**Deliverable**: Full-text search working in simplified mode

---

## Phase 3: UX Polish (Week 5)

### Backend - Startup Experience

- [ ] **Browser auto-launch** (`internal/cmd/commands/serve/serve.go`)
  - [ ] Add `--browser` flag (default: true)
  - [ ] Implement health check polling (wait for server ready)
  - [ ] Use `open` (macOS), `xdg-open` (Linux), `start` (Windows) to launch browser
  - [ ] Add startup banner with colorized output:
    ```
    ╔═══════════════════════════════════════════════════════╗
    ║  Hermes CMS - Running in Simplified Mode             ║
    ║                                                       ║
    ║  🌐 Web UI: http://localhost:8000                    ║
    ║  📁 Workspace: /Users/alice/docs-cms                 ║
    ║  💾 Database: SQLite (embedded)                      ║
    ║  🔍 Search: Bleve (embedded)                         ║
    ║                                                       ║
    ║  Press Ctrl+C to stop                                ║
    ╚═══════════════════════════════════════════════════════╝
    ```

- [ ] **Graceful shutdown**
  - [ ] Flush Bleve index on SIGINT/SIGTERM
  - [ ] Close SQLite database connection cleanly
  - [ ] Display shutdown message

### Frontend - Simplified Mode UI

- [ ] **Getting Started guide** (first-run experience)
  - [ ] Detect empty workspace (no documents)
  - [ ] Show welcome modal with:
    - [ ] "Create Your First Document" button
    - [ ] Quick tips (keyboard shortcuts, search tips)
    - [ ] Link to full documentation
  - [ ] Store "dismissed" state in localStorage

- [ ] **Document actions** (simplified mode only)
  - [ ] "Open in File Browser" button (opens native file manager)
  - [ ] "Copy File Path" button (clipboard)
  - [ ] "Export as Markdown" (with frontmatter stripped)

- [ ] **UI simplifications** (when `simplified_mode: true`)
  - [ ] Hide Google Drive integration UI
  - [ ] Hide complex permission controls
  - [ ] Show local file paths in document metadata
  - [ ] Simplify sharing UI (file paths instead of Drive links)

- [ ] **Footer/header updates**
  - [ ] Show "Simplified Mode" badge in header
  - [ ] Add "Upgrade to Enterprise" link (to documentation)

### Testing

- [ ] Manual testing: browser auto-opens on macOS, Linux, Windows
- [ ] UI testing: verify simplified mode UI changes
- [ ] First-run experience testing (empty workspace)

### Documentation

- [ ] Create 5-minute quick start video (screen recording)
- [ ] Update screenshots in README.md
- [ ] Add keyboard shortcuts reference

**Deliverable**: Polished first-run experience

---

## Phase 4: Documentation & Distribution (Week 6)

### Documentation

- [ ] **Update main README.md**
  - [ ] Move simplified mode to top (primary use case)
  - [ ] Add comparison table (simplified vs. enterprise)
  - [ ] Add architecture diagrams (ASCII + images)

- [ ] **Create user guides**
  - [ ] `docs/QUICK_START.md` (< 5 minutes)
  - [ ] `docs/SIMPLIFIED_MODE.md` (detailed guide)
  - [ ] `docs/MIGRATION_GUIDE.md` (simplified → enterprise)
  - [ ] `docs/TROUBLESHOOTING.md` (common issues)

- [ ] **Update copilot instructions** (`.github/copilot-instructions.md`)
  - [ ] Document simplified mode as default
  - [ ] Update build instructions
  - [ ] Add troubleshooting steps

### Distribution

- [ ] **Build pipeline** (`Makefile`)
  - [ ] Add `make build-simplified` target (static binaries)
  - [ ] Cross-compile for macOS (amd64, arm64), Linux (amd64, arm64), Windows (amd64)
  - [ ] Sign macOS binaries (if applicable)
  - [ ] Generate checksums (SHA256)

- [ ] **Homebrew formula** (if applicable)
  - [ ] Create `Formula/hermes.rb` in hashicorp/homebrew-tap
  - [ ] Test installation: `brew install hashicorp/tap/hermes`
  - [ ] Add caveats (usage instructions)

- [ ] **Docker image** (simplified mode variant)
  - [ ] Create `Dockerfile.simplified` with embedded mode
  - [ ] Publish to Docker Hub or GitHub Container Registry
  - [ ] Example: `docker run -v ./docs-cms:/docs-cms hashicorp/hermes:simplified`

- [ ] **GitHub Releases**
  - [ ] Automate release creation (GitHub Actions)
  - [ ] Attach binaries to releases
  - [ ] Write release notes (features, breaking changes)

### Testing

- [ ] Install from Homebrew (if applicable)
- [ ] Download binary from release, test on fresh machine
- [ ] Docker image smoke test

**Deliverable**: Public release of simplified mode

---

## Post-Launch (Ongoing)

### Monitoring & Feedback

- [ ] Add opt-in telemetry (track adoption, usage patterns)
- [ ] Create feedback form (Google Forms or GitHub Discussions)
- [ ] Monitor GitHub issues for bug reports

### Future Enhancements (Out of Scope for RFC-083)

- [ ] Multi-instance discovery (`.well-known/hermes.json`)
- [ ] Local network auth (username-based, no passwords)
- [ ] Auto-update mechanism (self-updating binary)
- [ ] Real-time collaboration (operational transform or CRDT)
- [ ] Mobile apps (iOS, Android with native file sync)
- [ ] Plugin system (custom document types, integrations)

---

## Dependencies & Prerequisites

### Go Libraries (add to `go.mod`)

```go
require (
    modernc.org/sqlite v1.28.0 // Pure Go SQLite
    github.com/blevesearch/bleve/v2 v2.3.10 // Full-text search
    gopkg.in/yaml.v3 v3.0.1 // YAML config parsing (already present)
)
```

### Development Environment

- Go 1.25.0+
- Node.js 20+ (for frontend development)
- Docker (for testing containerized deployment)

### Testing Infrastructure

- Existing `testing/` environment (for integration tests)
- Playwright (for E2E tests in simplified mode)
- GitHub Actions (for CI/CD)

---

## Success Criteria (Definition of Done)

- ✅ User can download binary, run `./hermes`, and see documents in browser (< 5 min)
- ✅ Zero configuration files required for basic usage
- ✅ Binary size < 60MB (compressed)
- ✅ All GORM tests pass with SQLite backend
- ✅ Search quality matches Algolia (spot checks on 10 queries)
- ✅ Documentation updated (README, quick start, migration guide)
- ✅ Cross-platform support (macOS, Linux, Windows)
- ✅ CI/CD pipeline builds and publishes releases

---

**Tracking**: Create GitHub Project "RFC-083 Implementation" with milestones for each phase

**Estimation**: 6 weeks (1 FTE developer) or 3 weeks (2 FTE developers in parallel)
