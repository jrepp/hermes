---
id: memo-042
created: 2026-04-24
title: Simplified Local Mode - Architecture Diagram
author: Hermes Team
project_id: hermes
doc_uuid: 68aea158-5847-4527-8083-0f4fa1f386d6
status: Draft
---

# Simplified Local Mode - Architecture Diagram

## Comparison: Enterprise vs. Simplified Mode

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                          ENTERPRISE MODE                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌─────────────┐                                                         │
│  │   Hermes    │  (40MB binary)                                         │
│  │   Binary    │                                                         │
│  └──────┬──────┘                                                         │
│         │                                                                 │
│         │ Reads config.hcl (200+ lines)                                  │
│         │                                                                 │
│         ├────────────────┬──────────────┬───────────────┬────────────────┤
│         │                │              │               │                │
│         ▼                ▼              ▼               ▼                │
│  ┌──────────┐    ┌──────────┐   ┌──────────┐   ┌─────────────┐         │
│  │PostgreSQL│    │  Algolia │   │  Google  │   │ Okta / Dex  │         │
│  │ Database │    │  Search  │   │ Workspace│   │    Auth     │         │
│  │(external)│    │(external)│   │(external)│   │ (external)  │         │
│  └──────────┘    └──────────┘   └──────────┘   └─────────────┘         │
│                                                                           │
│  Setup Time: 30-60 minutes                                               │
│  Config: Manual (HCL file)                                               │
│  Dependencies: 4 external services                                       │
│                                                                           │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                         SIMPLIFIED MODE                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌─────────────────────────────────────────────────────────────┐        │
│  │              Hermes Binary (50MB)                            │        │
│  │  ┌─────────────────────────────────────────────────────┐    │        │
│  │  │  Embedded Components:                               │    │        │
│  │  │  • SQLite Database (modernc.org/sqlite)             │    │        │
│  │  │  • Bleve Search (blevesearch/bleve)                 │    │        │
│  │  │  • Local Auth (trust-based)                         │    │        │
│  │  │  • Web Assets (Ember app)                           │    │        │
│  │  └─────────────────────────────────────────────────────┘    │        │
│  └────────────────────────┬────────────────────────────────────┘        │
│                           │                                              │
│                           │ Writes/Reads                                 │
│                           ▼                                              │
│                  ┌────────────────┐                                      │
│                  │  ./docs-cms/   │                                      │
│                  ├────────────────┤                                      │
│                  │ data/          │                                      │
│                  │  ├ hermes.db   │ (SQLite)                             │
│                  │  └ fts.index   │ (Bleve)                              │
│                  │ documents/     │ (Markdown)                           │
│                  │ drafts/        │ (Markdown)                           │
│                  │ attachments/   │ (Files)                              │
│                  │ templates/     │ (RFC/PRD/FRD)                        │
│                  │ config.yaml    │ (Auto-gen)                           │
│                  └────────────────┘                                      │
│                                                                           │
│  Setup Time: < 5 minutes (download + run)                                │
│  Config: Automatic (zero-config)                                         │
│  Dependencies: NONE (everything embedded)                                │
│                                                                           │
└─────────────────────────────────────────────────────────────────────────┘

```

## Data Flow: Simplified Mode

```text
┌──────────────┐
│   Browser    │
│              │
│ localhost:   │
│   8000       │
└──────┬───────┘
       │
       │ HTTP (REST API)
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│              Hermes Server (Embedded)                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐    ┌─────────────┐    ┌─────────────┐        │
│  │   API        │    │   Search    │    │  Workspace  │        │
│  │  Handler     │◄──►│  (Bleve)    │◄──►│   (Local)   │        │
│  │              │    │             │    │             │        │
│  └──────┬───────┘    └──────┬──────┘    └──────┬──────┘        │
│         │                   │                  │                │
│         │                   │                  │                │
│         ▼                   ▼                  ▼                │
│  ┌─────────────────────────────────────────────────────┐        │
│  │         SQLite Database (GORM)                      │        │
│  │  ┌──────────┬───────────┬────────────┬──────────┐  │        │
│  │  │Documents │  Drafts   │   Users    │ Metadata │  │        │
│  │  └──────────┴───────────┴────────────┴──────────┘  │        │
│  └─────────────────────────────────────────────────────┘        │
│                          │                                       │
└──────────────────────────┼───────────────────────────────────────┘
                           │
                           │ Persist to disk
                           ▼
                  ┌────────────────────┐
                  │   ./docs-cms/      │
                  │   └── data/        │
                  │       └── hermes.db│
                  └────────────────────┘
```

## Startup Sequence

```text
┌──────────────────────────────────────────────────────────────────┐
│ $ ./hermes                                                       │
└──────────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Detect Mode                                                  │
│    • No -config flag? → Simplified Mode                         │
│    • Check ./docs-cms/ exists?                                  │
│      - Yes → Use existing workspace                             │
│      - No  → Initialize new workspace                           │
└──────────────────────┬──────────────────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Initialize Workspace (if needed)                             │
│    • Create ./docs-cms/ directory structure                     │
│    • Generate config.yaml (minimal)                             │
│    • Create default templates (RFC, PRD, FRD)                   │
│    • Initialize SQLite database (schema migration)              │
│    • Create empty Bleve search index                            │
└──────────────────────┬──────────────────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Start Services                                               │
│    • Connect to SQLite (./docs-cms/data/hermes.db)              │
│    • Open Bleve index (./docs-cms/data/fts.index)               │
│    • Initialize local workspace provider                        │
│    • Register HTTP routes (API + static assets)                 │
│    • Start HTTP server on :8000                                 │
└──────────────────────┬──────────────────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Wait for Server Ready                                        │
│    • Health check polling (5s timeout)                          │
│    • Wait for /health endpoint to return 200 OK                 │
└──────────────────────┬──────────────────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Launch Browser                                               │
│    • Open http://localhost:8000 in default browser              │
│    • Display startup banner in terminal:                        │
│                                                                  │
│      ╔═══════════════════════════════════════════════════════╗ │
│      ║  Hermes CMS - Running in Simplified Mode             ║ │
│      ║                                                       ║ │
│      ║  🌐 Web UI: http://localhost:8000                    ║ │
│      ║  📁 Workspace: /Users/alice/docs-cms                 ║ │
│      ║  💾 Database: SQLite (embedded)                      ║ │
│      ║  🔍 Search: Bleve (embedded)                         ║ │
│      ║                                                       ║ │
│      ║  Press Ctrl+C to stop                                ║ │
│      ╚═══════════════════════════════════════════════════════╝ │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘

```

## Database Schema: Simplified vs. Enterprise

```text
┌─────────────────────────────────────────────────────────────────┐
│             SAME GORM MODELS (shared code)                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Documents                    Drafts                            │
│  ├─ ID (UUID)                 ├─ ID (UUID)                      │
│  ├─ GoogleFileID*             ├─ GoogleFileID*                  │
│  ├─ Title                     ├─ Title                          │
│  ├─ DocType                   ├─ DocType                        │
│  ├─ Status                    ├─ Status                         │
│  ├─ Contributors              ├─ Contributors                   │
│  ├─ ApprovedBy               ├─ ModifiedTime                   │
│  ├─ ModifiedTime             └─ ...                            │
│  └─ ...                                                         │
│                                                                  │
│  Users                        Projects                          │
│  ├─ EmailAddress              ├─ ID                             │
│  ├─ GivenName                 ├─ Name                           │
│  ├─ FamilyName                ├─ Description                    │
│  ├─ PhotoUrl                  └─ ...                            │
│  └─ ...                                                         │
│                                                                  │
│  * GoogleFileID nullable in simplified mode (local files)       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
       │                                │
       ▼                                ▼
┌──────────────┐              ┌─────────────────┐
│  PostgreSQL  │              │     SQLite      │
│  (Prodution) │              │  (Simplified)   │
│              │              │                 │
│  • Multi-GB  │              │  • < 100MB      │
│  • HA/Backup │              │  • File-based   │
│  • Concurrent│              │  • Single-node  │
└──────────────┘              └─────────────────┘
```

## Provider Abstraction (Already Exists!)

```go
// pkg/workspace/workspace.go
type Adapter interface {
    CreateDocument(ctx context.Context, doc *Document) error
    GetDocument(ctx context.Context, id string) (*Document, error)
    UpdateDocument(ctx context.Context, doc *Document) error
    DeleteDocument(ctx context.Context, id string) error
    ListDocuments(ctx context.Context) ([]*Document, error)
}

// Enterprise: pkg/workspace/google/google.go
type GoogleAdapter struct {
    service *drive.Service
    // ... Google Drive API calls
}

// Simplified: pkg/workspace/local/local.go (already implemented!)
type LocalAdapter struct {
    basePath string
    // ... Filesystem operations
}

// Runtime selection (same pattern as auth/search):
func NewAdapter(cfg *config.Config) (Adapter, error) {
    switch cfg.Providers.Workspace {
    case "google":
        return google.NewAdapter(cfg.GoogleWorkspace)
    case "local":
        return local.NewAdapter(cfg.LocalWorkspace) // ✅ Already exists!
    default:
        return nil, fmt.Errorf("unknown workspace provider: %s", cfg.Providers.Workspace)
    }
}

```

**Key Insight**: Local workspace provider is already implemented. Just need to:
1. Make it the default in simplified mode
2. Add SQLite/Bleve alternatives
3. Simplify configuration

---

**See**: [rfc-009-simplified-local-mode.md](rfc-009-simplified-local-mode.md) for full specification