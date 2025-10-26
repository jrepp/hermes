---
id: RFC-083
title: Simplified Local Mode - Zero-Config Document CMS
date: 2025-10-26
type: RFC
subtype: Architecture
status: Proposed
tags: [local-mode, ux, zero-config, embedded, standalone]
related:
  - README-local-workspace.md
  - README-indexer.md
  - RFC-007
---

# Simplified Local Mode - Zero-Config Document CMS

## Executive Summary

Transform Hermes from an enterprise Google Workspace integration into a **standalone document CMS** that can be run with a single command. Users download a binary, run `./hermes` in any directory, and instantly get a full-featured document management system in their browser—no configuration files, no external dependencies, no cloud services required.

**Vision**: `./hermes` → Web browser opens → Start writing documents

## Problem Statement

### Current Complexity Barrier

Today, running Hermes requires:
- ✅ Writing a 200+ line `config.hcl` file
- ✅ Setting up PostgreSQL database
- ✅ Configuring Google Workspace credentials OR setting up Dex OIDC
- ✅ Setting up Algolia OR Meilisearch
- ✅ Understanding workspace providers, auth providers, search providers
- ✅ Managing multiple services (database, search, auth)

**Result**: 30-60 minute setup time for experienced developers. Impossible for non-technical users.

### Use Cases Being Missed

1. **Solo Developer**: Wants personal document management for RFCs, design docs, notes
2. **Small Team**: 2-5 people collaborating on project documentation
3. **Education**: Students learning document-driven development
4. **Evaluation**: Enterprise users wanting to try Hermes before committing to full deployment
5. **Offline Work**: Developers needing document management without internet connectivity

### Competitive Landscape

- **Obsidian**: Download, run, instant markdown editor
- **Notion Local**: Desktop app with embedded database
- **GitBook**: Install CLI, run in directory, get documentation site
- **Docusaurus**: `npx docusaurus init` → full docs site

**Hermes should match this simplicity.**

## Goals

### Primary Goals

1. **Zero-Config Launch**: `./hermes` or `./hermes /path/to/docs` starts a full CMS
2. **Embedded Everything**: Database, search, auth all embedded in single binary
3. **Well-Known Directory**: Store all CMS data in `./docs-cms/` (data, config, indexes)
4. **Instant Browser UX**: Auto-open browser to http://localhost:8000 on startup
5. **No External Dependencies**: No PostgreSQL, no Algolia, no Dex required
6. **Production-Ready**: Same binary scales from solo use to enterprise deployment

### Non-Goals (Out of Scope)

- ❌ Removing enterprise features (Google Workspace, Okta, Algolia still supported)
- ❌ Multi-user auth in simplified mode (single-user or trust-based local network)
- ❌ Real-time collaboration (future enhancement)
- ❌ Mobile apps (browser-based only)

## Proposed Architecture

### Command-Line Interface

```bash
# Mode 1: Run in current directory (discovers ./docs-cms/)
./hermes
./hermes serve

# Mode 2: Explicit path
./hermes /path/to/my-documents
./hermes serve /path/to/my-documents

# Mode 3: Traditional config file (backwards compatible)
./hermes server -config=config.hcl

# Behavior:
# - If no arguments AND ./docs-cms/ exists → Use ./docs-cms/
# - If path argument provided → Use that path
# - If -config flag provided → Traditional enterprise mode
# - If none of above → Create ./docs-cms/ and initialize
```

### Well-Known Directory Structure

```
./docs-cms/
├── config.yaml              # Auto-generated minimal config (optional overrides)
├── data/
│   ├── hermes.db           # Embedded SQLite database
│   └── fts.index           # Full-text search index (Bleve/Meilisearch embedded)
├── documents/              # Published documents (markdown + metadata)
│   ├── RFC-001.md
│   ├── RFC-001.meta.json
│   └── ...
├── drafts/                 # Draft documents
│   └── ...
├── attachments/            # Binary attachments (images, PDFs)
│   └── ...
└── templates/              # Document templates
    ├── rfc.md
    ├── prd.md
    └── frd.md
```

### Embedded Components

#### 1. Database: SQLite (Embedded)

**Replace**: PostgreSQL for simplified mode

**Library**: `modernc.org/sqlite` (pure Go, no CGO)

**Benefits**:
- ✅ Zero configuration
- ✅ Single file database (`docs-cms/data/hermes.db`)
- ✅ GORM already supports SQLite
- ✅ Reliable, proven, fast for single-node workloads
- ✅ File-based backups (just copy the file)

**Migration Strategy**:
```go
// internal/db/db.go
func NewDatabase(cfg *config.Config) (*gorm.DB, error) {
    if cfg.SimplifiedMode {
        // SQLite
        return gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
    } else {
        // PostgreSQL (existing code)
        return gorm.Open(postgres.Open(cfg.Postgres.DSN()), &gorm.Config{})
    }
}
```

#### 2. Search: Bleve (Embedded Full-Text Search)

**Replace**: Algolia/Meilisearch for simplified mode

**Library**: `github.com/blevesearch/bleve/v2` (pure Go)

**Benefits**:
- ✅ Pure Go, no external service
- ✅ Full-text search with stemming, highlighting
- ✅ Faceted search (by document type, status, author)
- ✅ Single file index
- ✅ Proven in production (Sourcegraph, Gitea, many others)

**Alternative**: Embedded Meilisearch (via `github.com/meilisearch/meilisearch-go` + bundled binary)

**Interface**:
```go
// pkg/search/search.go (existing interface)
type Adapter interface {
    IndexDocument(ctx context.Context, doc Document) error
    Search(ctx context.Context, query string, opts SearchOptions) (SearchResults, error)
    DeleteDocument(ctx context.Context, id string) error
}

// pkg/search/bleve/bleve.go (new adapter)
type BleveAdapter struct {
    index bleve.Index
    path  string
}
```

#### 3. Authentication: Trust-Based Local Mode

**Mode 1: Single-User (Default)**
- No login required
- All documents owned by "Local User"
- Email: `user@localhost`
- User ID: Deterministic UUID based on hostname

**Mode 2: Local Network (Optional)**
- Simple username entry (no passwords)
- Trust-based (assumes trusted network)
- User switching via dropdown in UI

**Configuration**:
```yaml
# docs-cms/config.yaml
auth:
  mode: single-user  # or "local-network"
  domain: localhost
```

#### 4. Workspace: Local Filesystem Provider

**Already Implemented!** (See `pkg/workspace/local/`)

**Enhancements Needed**:
- ✅ Auto-create `documents/`, `drafts/`, `templates/` directories
- ✅ Default document templates (RFC, PRD, FRD)
- ✅ Markdown file format with frontmatter metadata
- ✅ Attachment handling (images, PDFs linked from markdown)

### Frontend Adaptations

#### Auto-Detection of Simplified Mode

```typescript
// web/app/services/config.ts
interface AppConfig {
  auth_provider: "google" | "okta" | "dex" | "local";
  workspace_provider: "google" | "local";
  search_provider: "algolia" | "meilisearch" | "bleve";
  simplified_mode: boolean;  // NEW
}
```

#### UI Simplifications for Local Mode

When `simplified_mode: true`:
- ✅ Hide Google Drive integration UI
- ✅ Hide complex permission controls (everyone has access)
- ✅ Show local file path in document metadata
- ✅ Add "Open in Finder/Explorer" button (opens native file browser)
- ✅ Simplified sharing (copy markdown file path)

### Multi-Instance Discovery (Future Enhancement)

**Problem**: Team members on same network running separate Hermes instances

**Solution**: Well-known `.well-known/hermes.json` discovery protocol

```bash
# Start with discovery enabled
./hermes --discovery

# Advertises on local network:
# http://192.168.1.100:8000/.well-known/hermes.json
```

```json
{
  "version": "2.0.0",
  "instance_id": "uuid-of-this-instance",
  "workspace_path": "/Users/alice/docs-cms",
  "owner": "Alice Smith",
  "capabilities": ["read", "write"],
  "auth_mode": "local-network"
}
```

**Frontend**: "Connect to Team Instance" dropdown shows discovered instances

## Implementation Plan

### Phase 1: Foundation (Week 1-2)

**Backend**:
- [ ] Add `serve` command as alias/wrapper for `server` with zero-config detection
- [ ] Implement `./docs-cms/` auto-discovery and initialization
- [ ] Add SQLite database support with GORM (conditional on simplified mode)
- [ ] Create default `config.yaml` generator (minimal overrides only)

**Frontend**:
- [ ] Add `simplified_mode` field to `/api/v2/web/config`
- [ ] Conditionally hide enterprise-only UI components

**Deliverable**: `./hermes` starts server with SQLite, existing local workspace, no search

### Phase 2: Embedded Search (Week 3-4)

**Backend**:
- [ ] Implement Bleve search adapter (`pkg/search/bleve/`)
- [ ] Add document indexing on create/update (via existing indexer hooks)
- [ ] Migrate search proxy endpoints to support Bleve backend

**Frontend**:
- [ ] Update search UI to handle Bleve response format (likely same as Algolia)

**Deliverable**: Full-text search working in simplified mode

### Phase 3: Browser Auto-Open & UX Polish (Week 5)

**Backend**:
- [ ] Add `--browser` flag (default: true) to auto-open browser on startup
- [ ] Implement health check endpoint polling before browser launch
- [ ] Add startup banner with URL (colorized terminal output)

**Frontend**:
- [ ] Add "Getting Started" guide for new users (simplified mode only)
- [ ] Add "Export to Markdown" bulk operation
- [ ] Add "Open in File Browser" for individual documents

**Deliverable**: Polished first-run experience

### Phase 4: Documentation & Distribution (Week 6)

**Documentation**:
- [ ] Update `README.md` with simplified mode as primary use case
- [ ] Create "Quick Start" guide (< 5 minutes to first document)
- [ ] Add architecture diagrams (simplified vs. enterprise mode)

**Distribution**:
- [ ] Build static binaries for macOS, Linux, Windows (via `make build`)
- [ ] Add Homebrew formula (`brew install hashicorp/tap/hermes`)
- [ ] Create Docker image with embedded mode (`docker run -v ./docs-cms:/docs-cms hermes`)

**Deliverable**: Public release of simplified mode

## Technical Decisions

### Decision 1: SQLite vs. Embedded PostgreSQL

**Options**:
1. **SQLite** (chosen)
2. Embedded PostgreSQL (via `github.com/fergusstrange/embedded-postgres`)
3. BoltDB/BadgerDB (key-value stores)

**Rationale**:
- ✅ SQLite is mature, well-tested, zero-config
- ✅ GORM already supports SQLite (minimal code changes)
- ✅ SQL semantics match PostgreSQL (easier migration path)
- ✅ Single-file database (easy backups)
- ❌ Embedded PostgreSQL adds 50MB+ to binary, complex startup
- ❌ BoltDB/BadgerDB require rewriting all database logic

### Decision 2: Bleve vs. Embedded Meilisearch

**Options**:
1. **Bleve** (chosen)
2. Embedded Meilisearch binary
3. Pure Go full-text search (e.g., `github.com/go-ego/riot`)

**Rationale**:
- ✅ Bleve is pure Go (single binary, no child processes)
- ✅ Battle-tested in production (Sourcegraph, Gitea)
- ✅ Excellent Go API (aligns with existing search adapter interface)
- ✅ Supports advanced features (facets, highlighting, stemming)
- ❌ Meilisearch requires bundling separate binary (10MB+)
- ❌ Riot lacks maturity and advanced features

### Decision 3: Single Binary vs. Separate Modes

**Options**:
1. **Single binary with mode detection** (chosen)
2. Separate `hermes-local` binary
3. Plugin architecture

**Rationale**:
- ✅ Single binary simplifies distribution and maintenance
- ✅ Easier to transition from simplified to enterprise mode
- ✅ Code reuse across modes (same models, API handlers)
- ✅ Users can switch modes via config file
- ❌ Separate binary fragments ecosystem
- ❌ Plugin architecture adds complexity

### Decision 4: Authentication in Simplified Mode

**Chosen**: Trust-based local mode (single-user default, optional local-network)

**Rejected Alternatives**:
- ❌ **Passwords**: Overkill for local/solo use, security theater without TLS
- ❌ **OAuth Device Flow**: Requires internet, adds complexity
- ❌ **Client Certificates**: Too complex for target audience

**Security Posture**:
- Simplified mode is **NOT** for public internet exposure
- Documented clearly: "Run on localhost or trusted networks only"
- Enterprise mode (with Okta/Dex/Google) required for production

### Decision 5: Configuration Format for Simplified Mode

**Chosen**: YAML for `config.yaml` (HCL for enterprise `config.hcl`)

**Rationale**:
- ✅ YAML more familiar to non-developers
- ✅ Simpler syntax for simple overrides
- ✅ Differentiate simplified mode visually (`.yaml` vs. `.hcl`)
- ✅ Still supports HCL for advanced users (backwards compatible)

**Example**:
```yaml
# docs-cms/config.yaml (auto-generated)
server:
  port: 8000
  auto_open_browser: true

auth:
  mode: single-user

workspace:
  base_path: ./docs-cms
```

## Migration & Compatibility

### Backwards Compatibility

**Existing Deployments**: Unaffected. Simplified mode only activates when:
- No `-config` flag provided, AND
- `./docs-cms/` directory detected or created

**Enterprise Mode**: Remains unchanged. All existing configurations work as-is.

### Migration Path: Simplified → Enterprise

**Scenario**: User starts with simplified mode, team grows, needs enterprise features

**Migration Script**:
```bash
# Export from SQLite to PostgreSQL
./hermes operator migrate-local-to-postgresql \
  --sqlite-path=./docs-cms/data/hermes.db \
  --postgres-dsn="postgresql://user:pass@localhost/hermes"

# Generate enterprise config
./hermes operator generate-config \
  --from-local=./docs-cms/config.yaml \
  --output=config.hcl
```

### Data Portability

**Export Formats**:
- Markdown files (already native format)
- JSON bulk export (all documents + metadata)
- SQLite database (direct file copy)

**Import from Other Systems**:
- Markdown directory → Hermes (bulk import command)
- Notion export → Hermes (conversion tool)

## Risks & Mitigations

### Risk 1: Binary Size Inflation

**Concern**: Embedding SQLite + Bleve + web assets increases binary size

**Mitigation**:
- Current binary: ~40MB (with web assets)
- SQLite (modernc.org/sqlite): +2MB (pure Go, no CGO)
- Bleve: +5MB
- **Total**: ~50MB (acceptable for single binary)
- Use `upx` for compression if needed (reduces by 40-60%)

### Risk 2: Performance at Scale

**Concern**: SQLite/Bleve may not scale to thousands of documents

**Benchmarks Needed**:
- SQLite: Proven to 100K+ rows (sufficient for most teams)
- Bleve: Used by Sourcegraph for millions of documents (with tuning)

**Mitigation**:
- Document recommended limits (< 10,000 documents in simplified mode)
- Provide migration path to enterprise mode (PostgreSQL + Algolia)
- Add telemetry to detect when users hit limits

### Risk 3: User Expectations

**Concern**: Users expect Google Drive integration (core Hermes feature)

**Mitigation**:
- Clear messaging: "Simplified mode for local/small teams, Enterprise mode for Google Workspace"
- Prominent "Upgrade to Enterprise" CTA in simplified mode UI
- Documentation: side-by-side comparison of modes

### Risk 4: Search Quality Degradation

**Concern**: Bleve may not match Algolia's search quality

**Mitigation**:
- Implement same ranking signals (title boost, freshness, document type)
- Extensive testing against Algolia results
- Tune Bleve analyzers (stemming, stop words) to match Algolia
- Document differences transparently

## Success Metrics

### Adoption Metrics (3 months post-launch)

- **Downloads**: > 1,000 unique downloads of standalone binary
- **New Users**: > 500 first-time Hermes users (via telemetry opt-in)
- **Conversion**: > 10% of simplified mode users upgrade to enterprise mode

### User Experience Metrics

- **Time to First Document**: < 5 minutes (download → first saved document)
- **Setup Complexity**: 0 configuration files required
- **Browser Auto-Open**: > 90% success rate (cross-platform)

### Technical Metrics

- **Binary Size**: < 60MB (compressed)
- **Memory Usage**: < 100MB RAM (idle), < 500MB RAM (100 documents indexed)
- **Search Performance**: < 100ms for typical queries (< 1,000 documents)

## Open Questions

1. **Default Port**: 8000 (current) or 3000 (common for local dev tools)?
2. **Telemetry**: Opt-in usage stats to track adoption? (Privacy-preserving)
3. **Auto-Update**: Built-in update mechanism or rely on package managers?
4. **Templates**: Ship with RFC/PRD/FRD templates, or minimal + download more?
5. **Multi-Platform**: Windows support in Phase 1 or Phase 2?

## Alternatives Considered

### Alternative 1: Docker Compose as Default

**Proposal**: Provide `docker-compose.yml` for one-command setup

**Pros**:
- ✅ Can use real PostgreSQL, Meilisearch (no embedded limits)
- ✅ Easier to add services later (Redis, etc.)

**Cons**:
- ❌ Requires Docker installed (not "zero dependencies")
- ❌ More complex for non-technical users
- ❌ Slower startup (container pulls, network setup)

**Decision**: Rejected. Docker Compose is great for enterprise dev, but contradicts "zero dependencies" goal.

### Alternative 2: SaaS Offering (Hermes Cloud)

**Proposal**: Hosted Hermes with free tier (like Notion, Obsidian Sync)

**Pros**:
- ✅ Zero installation
- ✅ Multi-device sync
- ✅ Revenue opportunity

**Cons**:
- ❌ Requires hosting infrastructure
- ❌ Data privacy concerns (defeats "local-first" value prop)
- ❌ Ongoing operational costs

**Decision**: Rejected. Out of scope for this RFC. Possible future direction.

### Alternative 3: Progressive Web App (PWA)

**Proposal**: Browser-based app using IndexedDB for storage

**Pros**:
- ✅ No installation required
- ✅ Works offline
- ✅ Cross-platform

**Cons**:
- ❌ Limited storage (browser quotas)
- ❌ No native file system access
- ❌ Cannot run as team server

**Decision**: Rejected. Doesn't solve the "team collaboration" use case.

## References

### Internal Documentation
- [README-local-workspace.md](../README-local-workspace.md) - Existing local workspace provider
- [README-indexer.md](../README-indexer.md) - Document indexing architecture
- [RFC-007: Multi-Provider Auth](RFC-007-multi-provider-auth-architecture.md) - Authentication patterns

### External Inspiration
- [Obsidian](https://obsidian.md) - Local-first markdown editor
- [Logseq](https://logseq.com) - Graph-based knowledge management
- [Docusaurus](https://docusaurus.io) - Static site generator with local dev mode
- [Bleve Search](https://blevesearch.com) - Full-text search library
- [SQLite](https://sqlite.org) - Embedded database

### Technical References
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) - Pure Go SQLite
- [github.com/blevesearch/bleve](https://github.com/blevesearch/bleve) - Go full-text search
- [GORM SQLite Driver](https://gorm.io/docs/connecting_to_the_database.html#SQLite)

## Appendix: Code Examples

### Simplified Mode Detection

```go
// internal/cmd/commands/serve/serve.go
package serve

import (
    "os"
    "path/filepath"
)

func (c *Command) Run(args []string) int {
    // Parse flags
    flags := c.Flags()
    configPath := flags.Lookup("config").Value.String()
    
    // If explicit config provided, use traditional enterprise mode
    if configPath != "" {
        return c.runEnterpriseMode(configPath)
    }
    
    // Check for explicit path argument
    var workspacePath string
    if len(args) > 0 {
        workspacePath = args[0]
    } else {
        // Check for ./docs-cms/ in current directory
        cwd, _ := os.Getwd()
        workspacePath = filepath.Join(cwd, "docs-cms")
    }
    
    // Check if workspace path exists
    if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
        // Initialize new workspace
        c.UI.Info(fmt.Sprintf("Initializing new Hermes workspace at %s", workspacePath))
        if err := c.initializeWorkspace(workspacePath); err != nil {
            c.UI.Error(err.Error())
            return 1
        }
    }
    
    return c.runSimplifiedMode(workspacePath)
}
```

### Auto-Generated Config

```go
// internal/config/simplified.go
package config

func GenerateSimplifiedConfig(workspacePath string) *Config {
    return &Config{
        SimplifiedMode: true,
        Server: ServerConfig{
            Addr:            ":8000",
            BaseURL:         "http://localhost:8000",
            AutoOpenBrowser: true,
        },
        Database: DatabaseConfig{
            Type: "sqlite",
            Path: filepath.Join(workspacePath, "data", "hermes.db"),
        },
        LocalWorkspace: LocalWorkspaceConfig{
            BasePath:    workspacePath,
            DocsPath:    filepath.Join(workspacePath, "documents"),
            DraftsPath:  filepath.Join(workspacePath, "drafts"),
            Domain:      "localhost",
        },
        Search: SearchConfig{
            Provider: "bleve",
            Bleve: BleveConfig{
                IndexPath: filepath.Join(workspacePath, "data", "fts.index"),
            },
        },
        Auth: AuthConfig{
            Provider: "local",
            Local: LocalAuthConfig{
                Mode: "single-user",
            },
        },
    }
}
```

### Workspace Initialization

```go
// internal/workspace/init.go
package workspace

func InitializeWorkspace(basePath string) error {
    // Create directory structure
    dirs := []string{
        filepath.Join(basePath, "data"),
        filepath.Join(basePath, "documents"),
        filepath.Join(basePath, "drafts"),
        filepath.Join(basePath, "attachments"),
        filepath.Join(basePath, "templates"),
    }
    
    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %w", dir, err)
        }
    }
    
    // Create default templates
    templates := map[string]string{
        "rfc.md":  defaultRFCTemplate,
        "prd.md":  defaultPRDTemplate,
        "frd.md":  defaultFRDTemplate,
    }
    
    for name, content := range templates {
        path := filepath.Join(basePath, "templates", name)
        if err := os.WriteFile(path, []byte(content), 0644); err != nil {
            return fmt.Errorf("failed to create template %s: %w", name, err)
        }
    }
    
    // Write minimal config.yaml
    configPath := filepath.Join(basePath, "config.yaml")
    if err := writeDefaultConfig(configPath); err != nil {
        return err
    }
    
    return nil
}
```

---

**Next Steps**:
1. Review and approve RFC (team discussion)
2. Create GitHub issue for tracking implementation
3. Spike: Bleve integration (2 days)
4. Spike: SQLite migration (1 day)
5. Implement Phase 1 (2 weeks)

**Authors**: [@jrepp](https://github.com/jrepp)  
**Reviewers**: [TBD]  
**Last Updated**: 2025-10-26
