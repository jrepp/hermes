# Hermes Distributed Projects Architecture

**Status**: 🚧 Alpha / Design Phase  
**Version**: 1.0.0-alpha  
**Created**: October 22, 2025

## Overview

Hermes is evolving from a single-backend document management system into a **distributed, federated platform** that can aggregate and manage projects across multiple storage backends:

- **Local Workspaces** (Git repos, file systems)
- **Google Workspace** (Google Docs, Drive)
- **Remote Hermes Instances** (Enterprise/internal deployments)

This architecture enables:
- ✅ Multi-backend document management in a single interface
- ✅ Federation with internal/enterprise Hermes deployments
- ✅ Hybrid cloud + local development workflows
- ✅ Gradual migration from monolithic to distributed architecture

## Core Concepts

### 1. Project-Centric Architecture

Every document belongs to a **project**, which defines:
- Where documents are stored (providers - can be multiple during migration)
- How they're accessed (authentication)
- Search/indexing behavior
- Access control policies

**Migration Support**: Projects can have multiple providers simultaneously, enabling zero-downtime migration from one backend to another while tracking content changes and detecting conflicts.

### 2. Provider Abstraction

Projects use one of three provider types:

#### **Local Provider**
- Files on disk (Git repos, markdown, etc.)
- Ideal for: OSS docs, developer documentation, testing
- Authentication: File system permissions
- Example: Hermes docs in `./docs-cms`

#### **Google Provider**
- Google Workspace (Docs, Sheets, Slides, Drive)
- Ideal for: Internal company docs, collaborative editing
- Authentication: Service accounts, OAuth
- Example: Corporate RFCs, internal documentation

#### **Remote Hermes Provider**
- Federated connection to another Hermes instance
- Ideal for: Enterprise aggregation, multi-region deployments
- Authentication: OIDC, API keys, mutual TLS
- Example: `https://hermes.hashicorp.services` (internal deployment)

### 3. Document Identification and Versioning

Documents are identified by a **stable UUID** that persists across providers during migration. The relationship between documents, projects, providers, and revisions is many-to-many to support active migrations.

#### Document UUID (Stable Identifier)

Every document gets a **UUID** (assigned or discovered):
```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
```

**UUID Discovery/Assignment**:
1. **Frontmatter/Header**: Document declares its UUID
   ```markdown
   ---
   hermes-uuid: 550e8400-e29b-41d4-a716-446655440000
   ---
   # My Document
   ```

2. **Auto-assigned**: If no UUID exists, generate and write to frontmatter/header
3. **Tracked in DB**: UUID becomes the primary document identifier

#### Provider-Project-Document-Revision Model

A document can exist in **multiple provider-project combinations** during migration:

```
Document UUID: 550e8400-e29b-41d4-a716-446655440000

Revisions:
  1. Provider: google-workspace-old
     Project: rfc-archive
     Provider Document ID: 1a2b3c4d5e6f7890
     Content Hash: sha256:abc123...
     Last Modified: 2025-10-01T10:00:00Z
     Status: migrating-from
     
  2. Provider: local-git
     Project: rfcs-new
     Provider Document ID: docs/rfc-001.md
     Content Hash: sha256:abc123...  (same content)
     Last Modified: 2025-10-01T10:00:00Z
     Status: migrating-to
     
  3. Provider: google-workspace-old
     Project: rfc-archive
     Provider Document ID: 1a2b3c4d5e6f7890
     Content Hash: sha256:def456...  (CONFLICT!)
     Last Modified: 2025-10-15T14:30:00Z
     Status: conflict-detected
```

#### Content Hash for Drift Detection

Track SHA-256 hash of document content to detect:
- ✅ Same document across providers (migration complete)
- ⚠️ Document edited during migration (conflict)
- 🔄 Out-of-sync replicas need reconciliation

#### Hermes URI Format (Updated)

Public-facing URI for stable document access:
```
hermes://uuid/{document-uuid}
hermes://uuid/550e8400-e29b-41d4-a716-446655440000
```

Internal provider-specific reference:
```
hermes://provider/{project-id}/{provider-type}/{provider-doc-id}
hermes://provider/rfcs-new/local/docs/rfc-001.md
hermes://provider/rfc-archive/google/1a2b3c4d5e6f7890
```

**User Experience**: UI shows UUID-based links. Backend resolves to current canonical revision.

#### Revision Tracking

Revisions can be:
1. **Detected revisions**: Content hash changes trigger new revision
2. **Explicit revisions**: Git commits, Google Doc versions
3. **Migration markers**: Status flags (source, target, conflict)

**Example revision record**:
```json
{
  "revisionId": "rev-123",
  "documentUuid": "550e8400-e29b-41d4-a716-446655440000",
  "projectId": "rfcs-new",
  "providerType": "local",
  "providerDocumentId": "docs/rfc-001.md",
  "contentHash": "sha256:abc123...",
  "lastModified": "2025-10-15T14:30:00Z",
  "status": "canonical",
  "metadata": {
    "gitCommit": "a1b2c3d",
    "author": "user@example.com"
  }
}
```

**Migration Path:**
- Existing Google Doc IDs: Get UUID assigned, becomes one revision
- During migration: Document exists in multiple provider-projects
- Post-migration: Old revisions marked as "archived", new becomes "canonical"
- Conflicts: Both revisions marked "conflict", requires resolution

## Configuration: HCL Modular Structure

### Location Options

1. **Testing**: `./testing/projects.hcl` with `./testing/projects/` (this repo, safe examples only)
2. **Production**: `/etc/hermes/projects.hcl` or via `HERMES_PROJECTS_CONFIG` env var
3. **Development**: `./projects.local.hcl` (gitignored, can have real credentials)

### Modular HCL Configuration

Projects are defined in **individual HCL files** for better organization:

```
testing/
├── projects.hcl              # Main config with imports
└── projects/                 # Individual project configs
    ├── testing.hcl           # Local testing workspace
    ├── docs.hcl              # Public documentation
    ├── _template-google.hcl  # Template for Google Workspace
    └── _template-migration.hcl  # Template for migrations
```

### Configuration Structure

Main config (`projects.hcl`):
```hcl
# Import individual project definitions
import "projects/testing.hcl"
import "projects/docs.hcl"

# Global settings
projects {
  version    = "1.0.0-alpha"
  config_dir = "./projects"
}
```

Individual project (`projects/testing.hcl`):
```hcl
project "testing" {
  title         = "Hermes Testing Environment"
  friendly_name = "Hermes Testing"
  short_name    = "TEST"  # Used in TEST-001, TEST-002, etc.
  description   = "Local testing workspace"
  status        = "active"
  
  provider "local" {
    migration_status = "active"
    workspace_path   = "./testing/workspace_data"
    
    git {
      repository = "https://github.com/hashicorp-forge/hermes"
      branch     = "main"
    }
    
    indexing {
      enabled            = true
      allowed_extensions = ["md", "txt", "json", "yaml", "yml"]
    }
  }
  
  metadata {
    created_at = "2025-10-22T00:00:00Z"
    owner      = "hermes-dev-team"
    tags       = ["testing", "development", "local"]
  }
}
```

### Short Names

Each project has a **short name** (2-10 characters) used in:
- Document IDs: `RFC-001`, `PRD-042`, `TEST-123`
- URLs: `/projects/testing`, `/docs/RFC-001`
- Search facets and filtering

**Note**: Short names are NOT globally unique (display/organizational only).

### Configuration Validation

```bash
# Check HCL syntax
hclconf check testing/projects.hcl

# Validate projects configuration (when implemented)
./hermes projects validate -config=testing/projects.hcl

# List all projects
./hermes projects list -config=testing/projects.hcl
```

## Implementation Phases

### Phase 1: Foundation (Current)
- ✅ Design HCL modular structure
- ✅ Create example configurations with short names
- ✅ Document architecture
- ⏳ Implement HCL config loader in Go
- ⏳ Add validation and error handling
- ⏳ Support for `import` statements

### Phase 2: Local Provider Support
- ⏳ Implement local workspace adapter
- ⏳ File system indexing
- ⏳ Git integration (commit history, authors)
- ⏳ Markdown rendering

### Phase 3: Document ID Migration
- ⏳ New `hermes://` URI scheme
- ⏳ Database schema changes
- ⏳ Migration job for existing documents
- ⏳ Alias/redirect support for legacy IDs

### Phase 4: Multi-Provider UI
- ⏳ Project selector in UI
- ⏳ Cross-project search
- ⏳ Provider-specific document viewers
- ⏳ Project management interface

### Phase 5: Remote Federation
- ⏳ Remote Hermes provider adapter
- ⏳ Authentication (OIDC, API keys)
- ⏳ Caching layer
- ⏳ Read-only and bidirectional sync modes

## Security Considerations

### 🔴 Critical: Data Leak Prevention

This is an **open-source project** used by HashiCorp and potentially IBM. We **must not leak**:
- ❌ Internal domain names (`hashicorp.com`, `ibm.com`)
- ❌ Employee email addresses
- ❌ Internal project names
- ❌ API keys, tokens, credentials
- ❌ Google Doc IDs from internal documents
- ❌ Internal Hermes instance URLs

### Best Practices

1. **Configuration Separation**
   ```
   testing/projects.hcl           # Public examples (committed)
   testing/projects/*.hcl         # Safe project configs (committed)
   testing/projects/_template-*.hcl  # Templates only (committed)
   
   projects.local.hcl            # Private config (gitignored)
   projects.production.hcl       # Production config (secret management)
   testing/projects/*.local.hcl  # Private project configs (gitignored)
   ```

2. **Use Environment Variables with `env()`**
   ```hcl
   project "my-docs" {
     provider "google" {
       workspace_id          = env("GOOGLE_WORKSPACE_ID")
       service_account_email = env("GOOGLE_SERVICE_ACCOUNT")
       credentials_path      = env("GOOGLE_CREDENTIALS_PATH")
     }
   }
   ```

3. **Example Data Only**
   - Use `example.com`, `example-project-id`
   - Mark templates as `status = "archived"`
   - Prefix templates with `_template-` to prevent loading
   - Add notes in `metadata.notes` field

4. **Code Review Checklist**
   - [ ] No real credentials in code/config
   - [ ] No internal domain names
   - [ ] No real Google Doc IDs
   - [ ] Environment variables documented
   - [ ] Examples clearly marked as templates

## Database Schema Changes (Planned)

### New Schema (Migration-Aware)

```sql
-- Projects table (one project can have multiple providers during migration)
CREATE TABLE projects (
    id SERIAL PRIMARY KEY,
    project_id VARCHAR(64) UNIQUE NOT NULL,  -- kebab-case identifier
    title VARCHAR(200) NOT NULL,
    friendly_name VARCHAR(200),  -- "Request for Comments" (not unique in distributed system)
    tla VARCHAR(10),  -- "RFC", "PRD", "FRD" (not enforced as unique)
    description TEXT,
    status VARCHAR(20) NOT NULL,  -- active, completed, archived
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Provider configurations (many-to-many with projects during migration)
CREATE TABLE project_providers (
    id SERIAL PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(project_id),
    provider_type VARCHAR(50) NOT NULL,  -- 'local', 'google', 'remote-hermes'
    provider_config JSONB NOT NULL,
    migration_status VARCHAR(50) DEFAULT 'active',  -- 'active', 'source', 'target', 'archived'
    migration_started_at TIMESTAMP,
    migration_completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(project_id, provider_type, provider_config->>'workspaceId')
);

-- Documents identified by stable UUID
CREATE TABLE documents (
    id SERIAL PRIMARY KEY,
    document_uuid UUID UNIQUE NOT NULL,  -- Stable identifier across migrations
    title VARCHAR(500),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Document revisions (provider-project-document-revision)
CREATE TABLE document_revisions (
    id SERIAL PRIMARY KEY,
    document_uuid UUID NOT NULL REFERENCES documents(document_uuid),
    project_id VARCHAR(64) NOT NULL REFERENCES projects(project_id),
    provider_type VARCHAR(50) NOT NULL,
    provider_document_id VARCHAR(500) NOT NULL,  -- Google Doc ID, file path, etc.
    content_hash VARCHAR(64) NOT NULL,  -- SHA-256 of content
    last_modified TIMESTAMP NOT NULL,
    revision_type VARCHAR(50) DEFAULT 'detected',  -- 'detected', 'git-commit', 'google-version'
    revision_metadata JSONB,  -- Git commit hash, author, etc.
    status VARCHAR(50) DEFAULT 'active',  -- 'active', 'canonical', 'archived', 'conflict', 'stale'
    indexed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    -- Ensure one canonical revision per document UUID (enforced at app level)
    -- Allow multiple revisions during migration
    UNIQUE(document_uuid, project_id, provider_type, provider_document_id)
);

-- Indexes for performance
CREATE INDEX idx_document_revisions_uuid ON document_revisions(document_uuid);
CREATE INDEX idx_document_revisions_project ON document_revisions(project_id);
CREATE INDEX idx_document_revisions_status ON document_revisions(status);
CREATE INDEX idx_document_revisions_content_hash ON document_revisions(content_hash);
CREATE INDEX idx_document_revisions_provider_doc ON document_revisions(provider_type, provider_document_id);

-- Legacy ID aliases for backward compatibility
CREATE TABLE document_aliases (
    id SERIAL PRIMARY KEY,
    document_uuid UUID NOT NULL REFERENCES documents(document_uuid),
    alias_type VARCHAR(50) NOT NULL,  -- 'legacy-google-id', 'legacy-url', 'shortlink'
    alias_value VARCHAR(500) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_document_aliases_uuid ON document_aliases(document_uuid);
CREATE INDEX idx_document_aliases_value ON document_aliases(alias_value);

-- Migration conflicts tracking
CREATE TABLE migration_conflicts (
    id SERIAL PRIMARY KEY,
    document_uuid UUID NOT NULL REFERENCES documents(document_uuid),
    source_revision_id INTEGER REFERENCES document_revisions(id),
    target_revision_id INTEGER REFERENCES document_revisions(id),
    conflict_type VARCHAR(50) NOT NULL,  -- 'content-divergence', 'concurrent-edit', 'metadata-mismatch'
    detected_at TIMESTAMP DEFAULT NOW(),
    resolved_at TIMESTAMP,
    resolution_strategy VARCHAR(50),  -- 'manual', 'source-wins', 'target-wins', 'merged'
    resolution_notes TEXT,
    UNIQUE(document_uuid, source_revision_id, target_revision_id)
);

CREATE INDEX idx_migration_conflicts_unresolved ON migration_conflicts(document_uuid) 
    WHERE resolved_at IS NULL;
```

### Migration Strategy

1. Add new columns (nullable initially)
2. Create default project for existing documents
3. Background job to populate new fields
4. Gradual cutover to new URI scheme
5. Maintain legacy aliases for backward compatibility

## API Changes (Planned)

### New Endpoints

```
GET  /api/v2/projects              # List all projects
GET  /api/v2/projects/:id          # Get project details
POST /api/v2/projects              # Create project (admin only)

GET  /api/v2/projects/:id/documents       # Documents in project
GET  /api/v2/documents/:hermes-uri        # Get by new URI format
```

### Backward Compatibility

```
GET /api/v1/documents/:google-doc-id  # Still works, redirects internally
GET /api/v2/documents/:google-doc-id  # Still works, issues deprecation warning
```

## Testing Strategy

### Unit Tests
- Config loader and validator
- Provider adapters (mocked backends)
- URI parsing and conversion
- Migration logic

### Integration Tests
- Local provider with test files
- Project CRUD operations
- Cross-project search
- Legacy ID resolution

### End-to-End Tests
- Multi-project document browsing
- Search across providers
- Document creation in different providers
- Federation scenarios (when implemented)

## Development Workflow

### Setting Up Local Testing

1. **Use the testing configuration**
   ```bash
   export HERMES_PROJECTS_CONFIG=./testing/projects.hcl
   make up
   ```

2. **Test projects included**:
   - `testing` - Local test workspace (short name: TEST)
   - `docs` - Documentation CMS (short name: DOCS)
   - `_template-*` - Templates only (not loaded)

3. **Adding your own test project**:
   ```bash
   # Create a new project file
   cat > testing/projects/my-project.hcl <<EOF
   project "my-project" {
     title         = "My Test Project"
     friendly_name = "My Project"
     short_name    = "MYPROJ"
     status        = "active"
     
     provider "local" {
       workspace_path = "./my-test-docs"
       # ... rest of config
     }
   }
   EOF
   
   # Import in main config
   echo 'import "projects/my-project.hcl"' >> testing/projects.hcl
   ```

### Internal Deployment (HashiCorp/IBM)

For internal deployments, create a separate `projects.production.hcl`:

```json
{
  "version": "1.0.0",
  "projects": [
    {
      "projectId": "internal-rfcs",
      "provider": {
        "type": "google",
        "config": {
          "workspaceId": "${GOOGLE_WORKSPACE_ID}",
          "credentialsPath": "/run/secrets/google-credentials"
        }
      }
    }
  ]
}
```

**Store in secret management, never commit!**

## Future Enhancements

### Planned Features
- 📋 Per-project access control policies
- 📋 Provider-specific document templates
- 📋 Cross-project document references
- 📋 Federation topology management
- 📋 Caching and sync strategies
- 📋 Offline mode for local providers
- 📋 Webhook support for external updates

### Extensibility
- Plugin system for custom providers
- Provider-specific metadata extraction
- Custom indexing pipelines per project

## FAQ

### Q: Why not use subdomains for projects?
**A**: We want a single Hermes instance to manage multiple projects without DNS/TLS overhead. URI scheme is more flexible.

### Q: Can documents move between projects?
**A**: Yes, with admin approval. The document ID changes, but aliases maintain old links.

### Q: How does this affect search?
**A**: Search can be scoped to projects, or search across all accessible projects. Permissions are enforced per provider.

### Q: What about performance?
**A**: Providers implement caching. Remote Hermes federation includes configurable cache TTL.

### Q: Is this backward compatible?
**A**: Yes. Existing deployments work unchanged. New features are opt-in via config.

## Contributing

When adding provider support or modifying this architecture:

1. Update this document
2. Update JSON schema
3. Add tests for new providers
4. Document security considerations
5. Provide example configurations (safe data only!)

## Data Model Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│ Document (Stable Identity)                                      │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ UUID: 550e8400-e29b-41d4-a716-446655440000                  │ │
│ │ Title: "RFC-001: API Gateway Design"                        │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                  │
│ Revisions (Provider-Project-Document-Revision):                 │
│ ┌───────────────────────────────────────────────────────┐       │
│ │ Revision 1 (Source - During Migration)               │       │
│ │ - Provider: google-workspace                          │       │
│ │ - Project: rfc-archive                                │       │
│ │ - Provider Doc ID: 1a2b3c4d5e6f7890                  │       │
│ │ - Content Hash: sha256:abc123...                      │       │
│ │ - Status: source                                      │       │
│ │ - Last Modified: 2025-10-01 10:00                     │       │
│ └───────────────────────────────────────────────────────┘       │
│ ┌───────────────────────────────────────────────────────┐       │
│ │ Revision 2 (Target - During Migration)               │       │
│ │ - Provider: local-git                                 │       │
│ │ - Project: rfcs-new                                   │       │
│ │ - Provider Doc ID: docs/rfc-001.md                    │       │
│ │ - Content Hash: sha256:def456... ⚠️ DIFFERENT         │       │
│ │ - Status: conflict                                    │       │
│ │ - Last Modified: 2025-10-15 14:30                     │       │
│ └───────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────┘

Project Configuration:
┌─────────────────────────────────────────────────────────────────┐
│ Project: engineering-rfcs                                        │
│ Friendly Name: "Request for Comments" (not unique)              │
│ TLA: "RFC" (not unique)                                         │
│                                                                  │
│ Providers (Many-to-Many):                                       │
│ ┌───────────────────────────────────────────────────────┐       │
│ │ Provider 1: google-workspace                          │       │
│ │ - Migration Status: source (migrating FROM)           │       │
│ │ - Started: 2025-10-01                                 │       │
│ └───────────────────────────────────────────────────────┘       │
│ ┌───────────────────────────────────────────────────────┐       │
│ │ Provider 2: local-git                                 │       │
│ │ - Migration Status: target (migrating TO)             │       │
│ │ - Started: 2025-10-01                                 │       │
│ └───────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

## References

- **Main Configuration**: `./testing/projects.hcl`
- **Project Configurations**: `./testing/projects/*.hcl`
- **Project README**: `./testing/projects/README.md`
- **Legacy JSON Schema** (deprecated): `./testing/projects.schema.json`
- **Detailed Migration Design**: `docs-internal/DOCUMENT_REVISIONS_AND_MIGRATION.md` ⭐
- **Implementation Roadmap**: `docs-internal/DISTRIBUTED_PROJECTS_ROADMAP.md`
- **Workspace Adapters**: `pkg/workspace/`
- **Search Adapters**: `pkg/search/`
- **Current Google integration**: `docs-internal/README-google-workspace.md`

---

**Status**: This architecture is in **design/alpha phase**. Implementation will be incremental over multiple milestones. Feedback and contributions welcome!
