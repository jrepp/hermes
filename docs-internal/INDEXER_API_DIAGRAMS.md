# Indexer API Architecture Diagrams

## Quick Reference: API-Based Indexer Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                    INDEXER SERVICE (Client)                          │
│                                                                      │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐              │
│  │  Load       │   │  Discover   │   │  Extract    │              │
│  │  Project    │──▶│  Documents  │──▶│  Content    │              │
│  │  Config     │   │  (Workspace)│   │  + Metadata │              │
│  └─────────────┘   └─────────────┘   └──────┬──────┘              │
│                                              │                      │
│  ┌─────────────┐   ┌─────────────┐   ┌──────▼──────┐              │
│  │  Generate   │   │  Calculate  │   │  Assign     │              │
│  │  Summary    │◀──│  Hash       │◀──│  UUID       │              │
│  │  (Ollama)   │   │  (SHA256)   │   │             │              │
│  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘              │
└─────────┼─────────────────┼─────────────────┼─────────────────────┘
          │                 │                 │
          │                 │                 │ HTTP POST/PUT
          │                 │                 │ Authorization: Bearer <token>
          │                 │                 │
          │                 │                 ▼
┌─────────┼─────────────────┼─────────────────────────────────────────┐
│         │                 │         HERMES API SERVER               │
│         │                 │                                         │
│         │                 │  ┌────────────────────────────────┐    │
│         │                 └─▶│ POST /api/v2/indexer/documents │    │
│         │                    │  • Create/upsert document      │    │
│         │                    │  • Workspace provider metadata │    │
│         │                    └────────────┬───────────────────┘    │
│         │                                 │                        │
│         │                    ┌────────────▼────────────────────┐   │
│         │                    │ POST /api/v2/indexer/documents/ │   │
│         │                    │      :uuid/revisions            │   │
│         │                    │  • Content hash                 │   │
│         │                    │  • Commit SHA / version         │   │
│         │                    │  • Modified by/at               │   │
│         │                    └────────────┬────────────────────┘   │
│         │                                 │                        │
│         │                    ┌────────────▼────────────────────┐   │
│         └───────────────────▶│ PUT /api/v2/indexer/documents/  │   │
│                              │      :uuid/summary               │   │
│                              │  • AI-generated summary          │   │
│                              │  • Model metadata                │   │
│                              └────────────┬────────────────────┘   │
│                                           │                        │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │              Authentication & Authorization                  │  │
│  │  • Service token (production)                               │  │
│  │  • OIDC/Dex (testing)                                       │  │
│  │  • Scope: indexer:write                                     │  │
│  └─────────────────────────────────────────────────────────────┘  │
└─────────────────────────────┬────────────────────────────────────┘
                              │
                              ▼ SQL INSERT/UPDATE
┌──────────────────────────────────────────────────────────────────┐
│                    POSTGRESQL DATABASE                            │
│                                                                   │
│  ┌────────────────────┐  ┌──────────────────────┐               │
│  │  documents         │  │  document_revisions  │               │
│  │  • id (serial)     │  │  • id (serial)       │               │
│  │  • uuid (uuid)     │  │  • document_id       │               │
│  │  • title           │  │  • content_hash      │               │
│  │  • doc_type        │  │  • commit_sha        │               │
│  │  • status          │  │  • revision_ref      │               │
│  │  • workspace_*     │  │  • summary (text)    │               │
│  │  • indexed_at      │  │  • modified_at       │               │
│  └────────────────────┘  └──────────────────────┘               │
│                                                                   │
│  ┌────────────────────┐                                          │
│  │  document_embeddings│                                         │
│  │  • id (serial)      │                                         │
│  │  • document_id      │                                         │
│  │  • revision_id      │                                         │
│  │  • embeddings       │                                         │
│  │    (vector(768))    │                                         │
│  └────────────────────┘                                          │
└──────────────────────────────────────────────────────────────────┘
```

## Workspace Provider Types

```
┌──────────────────────────────────────────────────────────────────┐
│                     WORKSPACE PROVIDERS                           │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   GitHub     │  │    Local     │  │   Hermes     │          │
│  │              │  │  Filesystem  │  │  (Remote)    │          │
│  │ • repository │  │ • path       │  │ • endpoint   │          │
│  │ • branch     │  │ • root       │  │ • document_id│          │
│  │ • path       │  │ • project    │  │ • workspace  │          │
│  │ • commit_sha │  │              │  │              │          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │
│         │                 │                 │                   │
│         └─────────────────┼─────────────────┘                   │
│                           │                                     │
│                           ▼                                     │
│              ┌────────────────────────┐                         │
│              │  Indexer discovers     │                         │
│              │  documents via         │                         │
│              │  provider API          │                         │
│              └────────────────────────┘                         │
└──────────────────────────────────────────────────────────────────┘
```

## Project Config → Workspace Resolution

```
┌──────────────────────────────────────────────────────────────────┐
│                     PROJECT CONFIGURATION                         │
│                  testing/projects/docs-internal.hcl               │
│                                                                   │
│  project "docs-internal" {                                       │
│    short_name  = "DOCS"                                          │
│    description = "Internal documentation"                        │
│    status      = "active"                                        │
│                                                                   │
│    workspace "local" {                                           │
│      type = "local"                                              │
│      root = "./docs-internal"                                    │
│                                                                   │
│      folders {                                                   │
│        docs   = "."                                              │
│        drafts = ".drafts"                                        │
│      }                                                           │
│    }                                                             │
│  }                                                               │
└───────────────────────────┬──────────────────────────────────────┘
                            │
                            │ projectconfig.LoadConfig()
                            │ cfg.GetProject("docs-internal")
                            │
                            ▼
┌──────────────────────────────────────────────────────────────────┐
│                     WORKSPACE PROVIDER                            │
│                                                                   │
│  provider, err := workspace.NewProvider(project.Workspace)       │
│                                                                   │
│  docs, err := provider.ListDocuments(ctx,                        │
│      project.Workspace.Folders.Docs, nil)                        │
│                                                                   │
│  for _, doc := range docs {                                      │
│      // Create document via API                                  │
│      req := &CreateDocumentRequest{                              │
│          UUID:  doc.UUID,                                        │
│          Title: doc.Title,                                       │
│          WorkspaceProvider: {                                    │
│              Type:      "local",                                 │
│              Path:      doc.Path,                                │
│              ProjectID: "docs-internal",                         │
│          },                                                      │
│      }                                                           │
│      apiClient.CreateDocument(ctx, req)                          │
│  }                                                               │
└──────────────────────────────────────────────────────────────────┘
```

## Command Pipeline with API Client

```
┌──────────────────────────────────────────────────────────────────┐
│                     INDEXER PIPELINE                              │
│                                                                   │
│  ┌──────────────────┐                                            │
│  │ DiscoverCommand  │ (Read from workspace)                      │
│  └────────┬─────────┘                                            │
│           │ DocumentContext                                      │
│           ▼                                                       │
│  ┌──────────────────┐                                            │
│  │ AssignUUID       │ (Generate or load UUID)                    │
│  └────────┬─────────┘                                            │
│           │ + UUID                                               │
│           ▼                                                       │
│  ┌──────────────────┐                                            │
│  │ ExtractContent   │ (Get markdown, parse frontmatter)          │
│  └────────┬─────────┘                                            │
│           │ + Content                                            │
│           ▼                                                       │
│  ┌──────────────────┐                                            │
│  │ CalculateHash    │ (SHA256 of content)                        │
│  └────────┬─────────┘                                            │
│           │ + Hash                                               │
│           ▼                                                       │
│  ┌──────────────────┐    ┌─────────────────────────┐            │
│  │ TrackCommand     │───▶│ POST /api/v2/indexer/   │            │
│  │ (APIClient)      │    │      documents           │            │
│  └────────┬─────────┘    └─────────────────────────┘            │
│           │ Document created in DB                               │
│           ▼                                                       │
│  ┌──────────────────┐    ┌─────────────────────────┐            │
│  │ TrackRevision    │───▶│ POST /api/v2/indexer/   │            │
│  │ (APIClient)      │    │      documents/:uuid/   │            │
│  │                  │    │      revisions           │            │
│  └────────┬─────────┘    └─────────────────────────┘            │
│           │ Revision created in DB                               │
│           ▼                                                       │
│  ┌──────────────────┐                                            │
│  │ Summarize        │ (Call Ollama API)                          │
│  │ (Ollama)         │                                            │
│  └────────┬─────────┘                                            │
│           │ + Summary                                            │
│           ▼                                                       │
│  ┌──────────────────┐    ┌─────────────────────────┐            │
│  │ UpdateSummary    │───▶│ PUT /api/v2/indexer/    │            │
│  │ (APIClient)      │    │      documents/:uuid/   │            │
│  │                  │    │      summary             │            │
│  └────────┬─────────┘    └─────────────────────────┘            │
│           │ Summary saved to revision                            │
│           ▼                                                       │
│  ┌──────────────────┐                                            │
│  │ Index            │ (Update Meilisearch)                       │
│  └──────────────────┘                                            │
└──────────────────────────────────────────────────────────────────┘
```

## Migration Strategy: DB → API

```
PHASE 1: Dual Support
┌─────────────────────────────────────────────────────────────────┐
│  type TrackRevisionCommand struct {                             │
│      DB        *gorm.DB              // ⚠️  Legacy (deprecated) │
│      APIClient *IndexerAPIClient     // ✅ New (preferred)      │
│  }                                                              │
│                                                                 │
│  func (c *TrackRevisionCommand) Execute(...) {                 │
│      if c.APIClient != nil {                                   │
│          return c.createRevisionViaAPI(...)  // New path       │
│      }                                                         │
│      return c.createRevisionDirectDB(...)    // Legacy path    │
│  }                                                             │
└─────────────────────────────────────────────────────────────────┘
                             │
                             │ Deprecation period
                             ▼
PHASE 2: API Only
┌─────────────────────────────────────────────────────────────────┐
│  type TrackRevisionCommand struct {                             │
│      APIClient *IndexerAPIClient     // ✅ Required            │
│  }                                                              │
│                                                                 │
│  func (c *TrackRevisionCommand) Execute(...) {                 │
│      return c.createRevisionViaAPI(...)                        │
│  }                                                             │
└─────────────────────────────────────────────────────────────────┘
```

## Testing Environment Integration

```
┌──────────────────────────────────────────────────────────────────┐
│                    TESTING DOCKER COMPOSE                         │
│                  (./testing/docker-compose.yml)                   │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  PostgreSQL  │  │ Meilisearch  │  │     Dex      │          │
│  │  :5433       │  │    :7701     │  │  :5558/:5559 │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│                                                                   │
│  ┌────────────────────────────────────────────────────────┐     │
│  │              Hermes Backend                            │     │
│  │              :8001                                     │     │
│  │  • API endpoints (including /api/v2/indexer/*)        │     │
│  │  • OIDC auth via Dex                                  │     │
│  │  • Database migrations                                │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                   │
│  ┌────────────────────────────────────────────────────────┐     │
│  │              Web Frontend                              │     │
│  │              :4201                                     │     │
│  │  • Proxies /api/* to backend :8001                    │     │
│  └────────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────────┘
                             │
                             │ Integration test connects to
                             │ http://localhost:8001
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                 INTEGRATION TEST                                  │
│          tests/integration/indexer/full_pipeline_test.go          │
│                                                                   │
│  1. Setup:                                                       │
│     • Get auth token from Dex                                    │
│     • Create IndexerAPIClient(http://localhost:8001)            │
│     • Load project config (testing/projects.hcl)                │
│                                                                   │
│  2. Execute Pipeline:                                            │
│     • Discover docs from project workspace                       │
│     • Process through commands (with API client)                 │
│     • POST documents to API                                      │
│     • POST revisions to API                                      │
│     • PUT summaries to API                                       │
│                                                                   │
│  3. Verify:                                                      │
│     • Query database for documents                               │
│     • Check document_revisions table                             │
│     • Verify summaries stored correctly                          │
│     • Validate Meilisearch index                                 │
└──────────────────────────────────────────────────────────────────┘
```

## References

- **Design Summary**: `INDEXER_API_DESIGN_SUMMARY.md`
- **API Spec**: `INDEXER_IMPLEMENTATION_GUIDE.md` (line 1200+)
- **Architecture**: `INDEXER_REFACTOR_PLAN.md` (API-Based Architecture section)
