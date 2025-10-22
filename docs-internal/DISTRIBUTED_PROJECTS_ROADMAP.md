# Implementation Roadmap: Distributed Projects

**Status**: Phase 1 - Foundation  
**Target**: Hermes v0.6.0  
**Owner**: Core team

## Current Status (October 2025)

### ✅ Completed
- [x] Architecture design and documentation
- [x] JSON schema definition (`projects.schema.json`)
- [x] Example configuration (`testing/projects.json`)
- [x] Security guidelines and gitignore rules
- [x] Testing environment integration

### 🚧 In Progress
- [ ] Go config loader implementation
- [ ] Database schema updates
- [ ] Local provider adapter

### ⏳ Planned
- [ ] UI for project management
- [ ] Document ID migration
- [ ] Google provider updates
- [ ] Remote federation

## Phase 1: Foundation (Weeks 1-2)

### Goal
Load and validate `projects.json` configuration at startup.

### Tasks

#### 1.1 Config Loader (`internal/config/projects.go`)
```go
type ProjectsConfig struct {
    Version  string    `json:"version"`
    Projects []Project `json:"projects"`
}

type Project struct {
    ProjectID   string              `json:"projectId"`
    Title       string              `json:"title"`
    Description string              `json:"description"`
    Status      ProjectStatus       `json:"status"`
    Provider    ProviderConfig      `json:"provider"`
    Metadata    map[string]any      `json:"metadata"`
}

type ProviderConfig struct {
    Type   ProviderType           `json:"type"`
    Config map[string]interface{} `json:"config"`
}

func LoadProjectsConfig(path string) (*ProjectsConfig, error)
func ValidateProjectsConfig(cfg *ProjectsConfig) error
```

**Files to create**:
- `internal/config/projects.go` - Main config loader
- `internal/config/projects_test.go` - Unit tests
- `pkg/models/project_config.go` - Domain models

**Integration points**:
- `cmd/hermes/main.go` - Add `--projects-config` flag
- `internal/server/server.go` - Load at startup
- Environment variable: `HERMES_PROJECTS_CONFIG`

#### 1.2 Validation
- JSON schema validation using `go-jsonschema`
- Business logic validation:
  - Unique project IDs
  - Valid provider configurations
  - Referenced paths exist
  - No security violations (domain checks)

#### 1.3 Config Service
```go
type ProjectService interface {
    GetProject(projectID string) (*Project, error)
    ListProjects(status ProjectStatus) ([]Project, error)
    GetProjectForDocument(docID string) (*Project, error)
}
```

### Acceptance Criteria
- [ ] Hermes starts with `--projects-config=./testing/projects.json`
- [ ] Invalid configs are rejected with clear errors
- [ ] Logs show loaded projects at startup
- [ ] Config reload without restart (SIGHUP handler)
- [ ] 100% test coverage for config loader

## Phase 2: Database Schema (Weeks 3-4)

### Goal
Support project-aware document storage without breaking existing data.

### Database Changes

#### 2.1 New Tables
```sql
CREATE TABLE project_configs (
    id SERIAL PRIMARY KEY,
    project_id VARCHAR(64) UNIQUE NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    provider_config JSONB NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_project_configs_status ON project_configs(status);
CREATE INDEX idx_project_configs_provider ON project_configs(provider_type);
```

#### 2.2 Update Existing Tables
```sql
-- Documents table
ALTER TABLE documents ADD COLUMN project_id VARCHAR(64);
ALTER TABLE documents ADD COLUMN provider_document_id VARCHAR(255);
ALTER TABLE documents ADD CONSTRAINT fk_documents_project 
    FOREIGN KEY (project_id) REFERENCES project_configs(project_id);

-- Indexes for new columns
CREATE INDEX idx_documents_project_id ON documents(project_id);
CREATE INDEX idx_documents_provider_doc_id ON documents(provider_document_id);
```

#### 2.3 Migration Strategy
1. Add columns as nullable
2. Create "default" project for existing documents
3. Background migration job
4. Eventually make project_id NOT NULL

### GORM Models

Update `pkg/models/project.go`:
```go
type ProjectConfig struct {
    gorm.Model
    ProjectID      string          `gorm:"uniqueIndex;not null"`
    Title          string          `gorm:"not null"`
    Description    string
    Status         ProjectStatus   `gorm:"not null"`
    ProviderType   string          `gorm:"not null"`
    ProviderConfig datatypes.JSON  `gorm:"not null"`
    Metadata       datatypes.JSON
}

// Add to Document model
type Document struct {
    // ... existing fields ...
    ProjectID           *string `gorm:"index"`
    ProviderDocumentID  *string `gorm:"index"`
}
```

### Acceptance Criteria
- [ ] Migration scripts run without errors
- [ ] Existing documents still work
- [ ] New project-aware documents can be created
- [ ] Database constraints enforce data integrity
- [ ] Rollback strategy documented

## Phase 3: Local Provider (Weeks 5-6)

### Goal
Implement local filesystem provider for markdown/text files.

### Implementation

#### 3.1 Provider Interface
```go
// pkg/workspace/provider.go
type Provider interface {
    Type() ProviderType
    GetDocument(ctx context.Context, docID string) (*Document, error)
    ListDocuments(ctx context.Context, opts ListOptions) ([]Document, error)
    CreateDocument(ctx context.Context, doc *Document) error
    UpdateDocument(ctx context.Context, doc *Document) error
    DeleteDocument(ctx context.Context, docID string) error
    Search(ctx context.Context, query string) ([]Document, error)
}
```

#### 3.2 Local Provider Implementation
```go
// pkg/workspace/adapters/local/adapter.go
type LocalAdapter struct {
    projectID    string
    workspacePath string
    gitRepo      *git.Repository  // optional
    allowedExts  []string
}

func NewLocalAdapter(config LocalProviderConfig) (*LocalAdapter, error)
func (a *LocalAdapter) GetDocument(ctx, docID) (*Document, error)
func (a *LocalAdapter) ListDocuments(ctx, opts) ([]Document, error)
```

#### 3.3 Features
- File system traversal
- Git integration (commit history, authors, blame)
- Markdown parsing and rendering
- Metadata extraction (frontmatter)
- Watch mode for auto-indexing
- Symlink handling

### Acceptance Criteria
- [ ] Read markdown files from `./testing/workspace_data`
- [ ] Extract metadata from frontmatter
- [ ] List all documents in workspace
- [ ] Search within local documents
- [ ] Git integration (show last modified, author)
- [ ] Performance: < 100ms for typical operations

## Phase 4: Document URI Migration (Weeks 7-8)

### Goal
Migrate to `hermes://project-id/doc-id` URI scheme.

### Implementation

#### 4.1 URI Parser
```go
type HermesURI struct {
    Scheme     string  // "hermes"
    ProjectID  string  // "hermes-docs"
    DocumentID string  // "README.md" or "1a2b3c4d" (Google Doc)
}

func ParseHermesURI(uri string) (*HermesURI, error)
func (u *HermesURI) String() string
func (u *HermesURI) ToLegacyID() string  // for backward compat
```

#### 4.2 Alias Table
```sql
CREATE TABLE document_aliases (
    id SERIAL PRIMARY KEY,
    document_id INTEGER NOT NULL REFERENCES documents(id),
    alias_type VARCHAR(50) NOT NULL,  -- 'legacy-google', 'legacy-url', 'shortlink'
    alias_value VARCHAR(500) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_document_aliases_value ON document_aliases(alias_value);
```

#### 4.3 Migration Job
```go
// internal/cmd/migrate_document_ids.go
func MigrateDocumentIDs(db *gorm.DB, dryRun bool) error {
    // 1. For each existing document
    // 2. Assign to default project or detect from metadata
    // 3. Create alias for old ID
    // 4. Update document with new URI
    // 5. Update all references
}
```

### Acceptance Criteria
- [ ] All existing documents get hermes:// URIs
- [ ] Legacy IDs work via aliases
- [ ] API supports both old and new formats
- [ ] UI displays new URIs
- [ ] Search indexes updated
- [ ] No broken links

## Phase 5: Multi-Provider UI (Weeks 9-10)

### Goal
UI for browsing projects and project-aware documents.

### Frontend Changes

#### 5.1 New Routes
```
/projects                           # List all projects
/projects/:projectId                # Project dashboard
/projects/:projectId/documents      # Documents in project
/documents/:hermesUri               # New document viewer
```

#### 5.2 Components
```typescript
// web/app/components/projects/
- project-selector.gts         # Dropdown to switch projects
- project-card.gts            # Project summary card
- project-status-badge.gts    # Status indicator

// web/app/components/documents/
- document-provider-badge.gts  # Show provider type
- document-uri-display.gts     # Hermes URI with copy button
```

#### 5.3 Services
```typescript
// web/app/services/projects.ts
class ProjectsService extends Service {
    async loadProjects(): Promise<Project[]>
    async getProject(projectId: string): Promise<Project>
    async getDocumentsForProject(projectId: string): Promise<Document[]>
}
```

### Acceptance Criteria
- [ ] Project selector in nav bar
- [ ] Browse documents by project
- [ ] Search scoped to project or across all
- [ ] Provider-specific document rendering
- [ ] Copy hermes:// URI to clipboard
- [ ] Project filtering and sorting

## Phase 6: Remote Federation (Future)

### Goal
Connect to remote Hermes instances for document federation.

### Implementation
- Remote provider adapter
- Authentication (OIDC, API keys)
- Caching layer with TTL
- Sync strategies (read-only, bidirectional)
- Conflict resolution

### Deferred to v0.7.0+

## Testing Strategy

### Unit Tests
- Config loader with various valid/invalid inputs
- Provider adapters (mocked filesystem)
- URI parsing and conversion
- Migration logic with test database

### Integration Tests
- Full startup with projects.json
- Multi-project document operations
- Search across projects
- Legacy ID compatibility

### E2E Tests (Playwright)
```typescript
// tests/e2e-playwright/distributed-projects.spec.ts
test('browse projects', async ({ page }) => { ... })
test('switch between projects', async ({ page }) => { ... })
test('search within project', async ({ page }) => { ... })
test('legacy document ID redirect', async ({ page }) => { ... })
```

### Performance Tests
- 1000+ documents across 10 projects
- Search latency < 200ms
- Project switching < 100ms
- Concurrent provider operations

## Rollout Strategy

### Stage 1: Opt-In (v0.6.0-alpha)
- Feature flag: `HERMES_ENABLE_DISTRIBUTED_PROJECTS=true`
- Existing deployments unaffected
- Early adopters test in dev/staging

### Stage 2: Default On (v0.6.0-beta)
- Enabled by default
- Can be disabled with flag
- Migration tools available

### Stage 3: GA (v0.6.0)
- Fully supported
- Documentation complete
- Migration path for all users

### Stage 4: Legacy Deprecation (v0.7.0)
- Old single-backend mode deprecated
- 6-month deprecation notice
- Migration required

## Documentation Updates

- [ ] Update main README with distributed projects
- [ ] Provider configuration guides
- [ ] Migration guide for existing deployments
- [ ] API documentation with new endpoints
- [ ] Deployment examples (Docker, K8s)
- [ ] Troubleshooting guide

## Monitoring & Observability

### Metrics
- Projects loaded count
- Documents per project
- Provider operation latency
- Cache hit/miss rates
- Migration job progress

### Logging
- Project config validation errors
- Provider initialization
- Document ID resolution
- Migration events

### Alerts
- Project config failed to load
- Provider unavailable
- Migration job stalled
- High error rates per provider

## Security Review

### Checklist
- [ ] No credentials in committed configs
- [ ] Environment variable validation
- [ ] Access control per project
- [ ] Provider isolation (sandboxing)
- [ ] Audit log for project changes
- [ ] Rate limiting per provider
- [ ] Input validation for all configs

### Threat Model
- Malicious project configurations
- Provider credential leakage
- Cross-project data access
- Remote provider compromise
- DoS via expensive operations

## Open Questions

1. **Document ownership**: Can docs belong to multiple projects?
   - **Decision**: Single project, but can be referenced from others
   
2. **Provider failover**: What if a provider is unavailable?
   - **Decision**: Degrade gracefully, show cached data with warning
   
3. **Search ranking**: How to rank across providers?
   - **Decision**: Provider-agnostic ranking, metadata-based
   
4. **Permissions**: Project-level or document-level?
   - **Decision**: Both, project sets defaults, documents can override

5. **Versioning**: Support document versions across providers?
   - **Decision**: Phase 2 feature, provider-specific

## Dependencies

- Go 1.25+ (already required)
- PostgreSQL 13+ (already required)
- git2go or go-git for Git integration (new)
- JSON schema validator (new)

## Success Metrics

- 100% existing deployments can migrate without data loss
- < 5% performance degradation
- 90%+ test coverage for new code
- Zero security vulnerabilities
- Positive user feedback from early adopters

---

**Next Steps**: Implement Phase 1 config loader. See `internal/config/projects.go` (to be created).
