# Project Config Integration Plan

**Created**: 2025-10-22  
**Status**: Planning Phase  
**Target**: Hermes v2 API

## Overview

This document outlines the plan for integrating the `pkg/projectconfig` package into the Hermes server and API layer. The integration will provide workspace project configuration management, enabling multi-tenant document storage with different workspace providers (local, Google Workspace, remote Hermes).

## Current State

### Existing Projects API (Database-Backed)
- **Endpoint**: `/api/v2/projects`
- **Purpose**: Document organization projects (like folders)
- **Storage**: PostgreSQL database (`models.Project`)
- **Features**: Title, description, status, Jira integration, products
- **Use Case**: User-created document groupings within Hermes

### New Workspace Projects (Config-Based)
- **Package**: `pkg/projectconfig`
- **Purpose**: Workspace provider configuration
- **Storage**: HCL configuration files (`testing/projects/*.hcl`)
- **Features**: Provider selection, migration state, workspace paths
- **Use Case**: Multi-tenant workspace isolation, provider abstraction

**Key Distinction**: These are **different** projects:
- Database projects = user-created document collections
- Workspace projects = admin-configured storage backends

## Integration Goals

### Phase 1: Server Integration (Current Focus)
1. ✅ Load workspace project configuration at server startup
2. ✅ Add `ProjectConfig` to `server.Server` struct
3. ✅ Validate configuration before starting server
4. ⏳ Use project config to initialize workspace providers
5. ⏳ Create API endpoints to expose workspace project metadata

### Phase 2: Workspace Provider Selection (This Document)
1. Select workspace provider based on project configuration
2. Route document operations to correct provider
3. Handle migration scenarios (read from source, write to target)

### Phase 3: Migration Support (Separate - Indexer Integration)
1. **Deferred to indexer project** (separate work stream)
2. Use indexer to migrate documents between providers
3. Implement migration status tracking
4. Provide migration progress reporting

### Phase 4: Validation & Monitoring (Separate Work)
1. Runtime validation of project configurations
2. Health checks for workspace providers
3. Monitoring and alerting for migration status
4. Configuration reload without restart

## Architecture

### Server Structure Enhancement

```go
// internal/server/server.go
type Server struct {
    // Existing fields
    SearchProvider    search.Provider
    WorkspaceProvider workspace.Provider  // Primary/default provider
    Config           *config.Config
    DB               *gorm.DB
    Jira             *jira.Service
    Logger           hclog.Logger
    
    // NEW: Workspace project configuration
    ProjectConfig    *projectconfig.Config  // Multi-tenant workspace projects
}
```

### Workspace Provider Router

**New Package**: `pkg/workspace/router`

```go
// pkg/workspace/router/router.go
package router

import (
    "github.com/hashicorp-forge/hermes/pkg/projectconfig"
    "github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Router selects the appropriate workspace provider based on project configuration
type Router struct {
    projectConfig *projectconfig.Config
    providers     map[string]workspace.Provider  // Cache of initialized providers
}

// GetProvider returns the workspace provider for a given project
func (r *Router) GetProvider(projectName string) (workspace.Provider, error) {
    project, err := r.projectConfig.GetProject(projectName)
    if err != nil {
        return nil, err
    }
    
    // Get primary provider (handles migration logic)
    provider, err := project.GetPrimaryProvider()
    if err != nil {
        return nil, err
    }
    
    // Return cached or initialize new provider
    return r.getOrInitProvider(provider)
}

// GetSourceProvider returns the source provider during migration
func (r *Router) GetSourceProvider(projectName string) (workspace.Provider, error) {
    project, err := r.projectConfig.GetProject(projectName)
    if err != nil {
        return nil, err
    }
    
    if !project.IsInMigration() {
        return nil, fmt.Errorf("project %s is not in migration", projectName)
    }
    
    sourceProviderCfg, err := project.GetSourceProvider()
    if err != nil {
        return nil, err
    }
    
    return r.getOrInitProvider(sourceProviderCfg)
}

// GetTargetProvider returns the target provider during migration
func (r *Router) GetTargetProvider(projectName string) (workspace.Provider, error) {
    project, err := r.projectConfig.GetProject(projectName)
    if err != nil {
        return nil, err
    }
    
    if !project.IsInMigration() {
        return nil, fmt.Errorf("project %s is not in migration", projectName)
    }
    
    targetProviderCfg, err := project.GetTargetProvider()
    if err != nil {
        return nil, err
    }
    
    return r.getOrInitProvider(targetProviderCfg)
}

// getOrInitProvider retrieves a cached provider or initializes a new one
func (r *Router) getOrInitProvider(providerCfg *projectconfig.Provider) (workspace.Provider, error) {
    // Generate cache key: "type:workspace_path" or "type:workspace_id"
    cacheKey := r.getCacheKey(providerCfg)
    
    // Check cache
    if provider, exists := r.providers[cacheKey]; exists {
        return provider, nil
    }
    
    // Initialize new provider based on type
    var provider workspace.Provider
    var err error
    
    switch {
    case providerCfg.IsLocal():
        provider, err = r.initLocalProvider(providerCfg)
    case providerCfg.IsGoogle():
        provider, err = r.initGoogleProvider(providerCfg)
    case providerCfg.IsRemoteHermes():
        provider, err = r.initRemoteHermesProvider(providerCfg)
    default:
        return nil, fmt.Errorf("unsupported provider type: %s", providerCfg.Type)
    }
    
    if err != nil {
        return nil, err
    }
    
    // Cache provider
    r.providers[cacheKey] = provider
    
    return provider, nil
}
```

### API Endpoints

#### New Endpoint: Workspace Projects Metadata

**Purpose**: Expose workspace project configuration for UI/clients

```
GET /api/v2/workspace-projects
```

**Response**:
```json
{
  "projects": [
    {
      "name": "testing",
      "title": "Testing Environment",
      "friendly_name": "Hermes Testing",
      "short_name": "TEST",
      "status": "active",
      "is_active": true,
      "in_migration": false,
      "providers": [
        {
          "type": "local",
          "state": "active",
          "role": "Active (read/write)",
          "workspace_path": "testing",
          "git_repository": "https://github.com/hashicorp-forge/hermes",
          "indexing_enabled": true
        }
      ]
    },
    {
      "name": "docs",
      "title": "Documentation",
      "short_name": "DOCS",
      "status": "active",
      "is_active": true,
      "in_migration": true,
      "providers": [
        {
          "type": "google",
          "state": "source",
          "role": "Migration source (read-only)",
          "workspace_id": "workspace-123",
          "has_authentication": true
        },
        {
          "type": "local",
          "state": "target",
          "role": "Migration target (write destination)",
          "workspace_path": "docs",
          "indexing_enabled": true
        }
      ]
    }
  ]
}
```

**Implementation**:
```go
// internal/api/v2/workspace_projects.go
package api

func WorkspaceProjectsHandler(srv server.Server) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Authorize request
        userEmail := pkgauth.MustGetUserEmail(r.Context())
        if userEmail == "" {
            http.Error(w, "No authorization information", http.StatusUnauthorized)
            return
        }
        
        switch r.Method {
        case "GET":
            // Get all active workspace projects
            summaries := srv.ProjectConfig.GetActiveProjectSummaries()
            
            resp := WorkspaceProjectsGetResponse{
                Projects: summaries,
            }
            
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(resp)
            
        default:
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        }
    })
}
```

#### New Endpoint: Single Workspace Project

```
GET /api/v2/workspace-projects/{name}
```

**Response**:
```json
{
  "name": "testing",
  "title": "Testing Environment",
  "friendly_name": "Hermes Testing",
  "short_name": "TEST",
  "description": "Local testing workspace",
  "status": "active",
  "is_active": true,
  "in_migration": false,
  "providers": [
    {
      "type": "local",
      "state": "active",
      "role": "Active (read/write)",
      "workspace_path": "testing",
      "git_repository": "https://github.com/hashicorp-forge/hermes",
      "git_branch": "main",
      "indexing_enabled": true,
      "allowed_extensions": ["md", "txt", "json"]
    }
  ],
  "metadata": {
    "created_at": "2025-10-22T00:00:00Z",
    "owner": "hermes-dev-team",
    "tags": ["testing", "development", "local"]
  }
}
```

### Document Operation Routing

**Pattern**: Use workspace project name to select provider

```go
// Example: Document creation handler
func CreateDocumentHandler(srv server.Server) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req CreateDocumentRequest
        json.NewDecoder(r.Body).Decode(&req)
        
        // NEW: Get workspace project from request (header, query param, or default)
        workspaceProject := r.Header.Get("X-Workspace-Project")
        if workspaceProject == "" {
            workspaceProject = "default"  // Or from user preferences
        }
        
        // Get appropriate workspace provider for this project
        provider, err := srv.WorkspaceRouter.GetProvider(workspaceProject)
        if err != nil {
            http.Error(w, "Workspace project not found", http.StatusBadRequest)
            return
        }
        
        // Create document using selected provider
        doc, err := provider.CreateDocument(r.Context(), &req)
        if err != nil {
            http.Error(w, "Failed to create document", http.StatusInternalServerError)
            return
        }
        
        // ... rest of handler
    })
}
```

## Implementation Steps

### Step 1: Server Initialization ✅

**File**: `internal/cmd/commands/server/server.go`

```go
// Load project configuration
var projectConfig *projectconfig.Config
if cfg.Providers != nil && cfg.Providers.ProjectsConfigPath != "" {
    projectConfig, err = projectconfig.LoadConfig(cfg.Providers.ProjectsConfigPath)
    if err != nil {
        c.UI.Error(fmt.Sprintf("error loading projects config: %v", err))
        return 1
    }
    
    // Validate configuration
    validator := projectconfig.NewValidator()
    if err := validator.Validate(projectConfig); err != nil {
        c.UI.Error(fmt.Sprintf("invalid projects config: %v", err))
        return 1
    }
    
    c.UI.Info(fmt.Sprintf("Loaded %d workspace projects", len(projectConfig.Projects)))
}

// Create server with project config
srv := server.Server{
    Config:        cfg,
    DB:            database,
    Logger:        logger,
    ProjectConfig: projectConfig,  // NEW
    // ... other fields
}
```

### Step 2: Workspace Provider Router (NEW Package)

**File**: `pkg/workspace/router/router.go`

- Implement provider caching and initialization
- Handle migration scenarios (source/target selection)
- Provide clean API for document handlers

### Step 3: API Endpoints (NEW Handlers)

**File**: `internal/api/v2/workspace_projects.go`

- `GET /api/v2/workspace-projects` - List all workspace projects
- `GET /api/v2/workspace-projects/{name}` - Get single workspace project

### Step 4: Document Handler Updates (LATER)

Update existing document handlers to use workspace router:
- `POST /api/v2/documents` - Create document
- `GET /api/v2/documents/{id}` - Get document
- `PATCH /api/v2/documents/{id}` - Update document
- `DELETE /api/v2/documents/{id}` - Delete document

**Note**: This step is deferred - requires careful refactoring of existing handlers

## Configuration Updates

### Add Projects Config Path to Server Config

**File**: `internal/config/config.go`

```go
type Providers struct {
    // Existing fields
    Workspace string `hcl:"workspace,optional"`
    Search    string `hcl:"search,optional"`
    
    // NEW: Path to workspace projects configuration
    ProjectsConfigPath string `hcl:"projects_config_path,optional"`
}
```

### Example Server Config

```hcl
# config.hcl
providers {
  workspace          = "local"
  search            = "meilisearch"
  projects_config_path = "testing/projects.hcl"  # NEW
}
```

## Migration Handling (Deferred to Indexer Project)

### Current Plan
- Migration will be handled by the **indexer** service
- Indexer will read from source provider, write to target provider
- Document operation handlers will write to primary provider (target during migration)

### Indexer Integration Points
1. Indexer reads project configuration
2. Detects projects in migration (`IsInMigration()`)
3. For each migrating project:
   - Get source provider (`GetSourceProvider()`)
   - Get target provider (`GetTargetProvider()`)
   - Enumerate documents from source
   - Copy documents to target
   - Update search index
4. Update migration status in configuration

### Migration API Endpoints (Future)
```
POST /api/v2/workspace-projects/{name}/migrations/start
GET  /api/v2/workspace-projects/{name}/migrations/status
POST /api/v2/workspace-projects/{name}/migrations/complete
POST /api/v2/workspace-projects/{name}/migrations/rollback
```

**Status**: Not implemented yet - separate work stream

## Validation Plan (Separate Work)

### Validation Points

1. **Startup Validation** ✅ (Already implemented)
   - Load config with `projectconfig.LoadConfig()`
   - Validate with `validator.Validate(config)`
   - Fail server startup if invalid

2. **Runtime Validation** (Future)
   - Periodic config revalidation
   - Detect config file changes
   - Hot-reload configuration
   - Graceful handling of validation errors

3. **Provider Health Checks** (Future)
   - Ping workspace providers on startup
   - Periodic health checks
   - Disable unhealthy providers
   - Alert on provider failures

4. **Migration Validation** (Future - Indexer)
   - Validate source and target providers are accessible
   - Check sufficient storage space
   - Verify permissions and authentication
   - Validate migration state consistency

### Configuration Validation Rules

**Already Implemented** in `pkg/projectconfig/validator.go`:
- Version format: `X.Y` or `X.Y.Z`
- Project name: kebab-case `[a-z0-9-]+`
- Short name: UPPERCASE, max 4 characters
- Status: `active`, `archived`, `completed`
- Provider type: `local`, `google`, `remote-hermes`
- Migration status: `active`, `source`, `target`, `archived`
- Migration consistency: source requires target, target requires source
- URL validation for Git repositories and remote Hermes
- File extension validation for allowed extensions

**To Be Added** (Future):
- Provider-specific field requirements
- Workspace path existence checks (optional)
- Git repository accessibility checks
- Google Workspace API connectivity
- Remote Hermes API reachability
- Cross-project validation (unique short names)
- Migration state transition validation

## Testing Strategy

### Unit Tests
- ✅ Project config models and validation (already implemented)
- ⏳ Workspace router provider selection
- ⏳ Workspace router caching
- ⏳ API endpoint response formatting

### Integration Tests
- ⏳ Server startup with project config
- ⏳ Workspace project API endpoints
- ⏳ Provider routing for different projects
- ⏳ Migration scenario handling

### E2E Tests
- ⏳ Create document in specific workspace project
- ⏳ Read document from correct provider
- ⏳ List workspace projects via API
- ⏳ Handle migration scenarios end-to-end

## Security Considerations

### API Response Sanitization ✅
- `ToSummary()` removes all credentials
- `ProviderSummary` excludes sensitive fields
- Authentication indicated by boolean flag
- No credentials in logs or error messages

### Authorization
- Workspace project access control (Future)
- Per-project permissions (Future)
- Role-based access to migration operations (Future)

### Secrets Management
- All credentials loaded from environment variables
- No secrets in configuration files (tracked in git)
- Secure credential storage for Google service accounts
- Token management for remote Hermes authentication

## Monitoring and Observability

### Metrics (Future)
- Workspace project usage by provider type
- Provider latency and error rates
- Migration progress and duration
- Configuration reload events

### Logging
- Log workspace project selection for operations
- Log provider initialization and caching
- Log migration events and errors
- Log configuration validation results

### Tracing (Future)
- Trace document operations across providers
- Trace provider selection logic
- Trace migration operations
- Correlate operations with workspace projects

## Rollout Plan

### Phase 1: Foundation (Current Sprint)
- [x] Implement projectconfig package with models, validation, tests
- [x] Add provider state/role semantics and API sanitization
- [x] Create API usage documentation
- [ ] Add ProjectConfig to Server struct
- [ ] Implement basic workspace project API endpoints
- [ ] Create integration documentation (this document)

### Phase 2: Provider Router (Next Sprint)
- [ ] Implement `pkg/workspace/router` package
- [ ] Add provider caching and initialization
- [ ] Add migration source/target selection
- [ ] Write unit tests for router
- [ ] Integration tests with real providers

### Phase 3: Document Handler Integration (Future Sprint)
- [ ] Update document creation handlers
- [ ] Update document read handlers
- [ ] Update document update/delete handlers
- [ ] Add workspace project header/param handling
- [ ] E2E tests with multiple workspace projects

### Phase 4: Migration Support (Separate - Indexer Project)
- [ ] Implement indexer-based migration
- [ ] Add migration status tracking
- [ ] Create migration API endpoints
- [ ] Migration monitoring and alerting
- [ ] Migration rollback support

### Phase 5: Advanced Features (Future)
- [ ] Configuration hot-reload
- [ ] Provider health checks
- [ ] Per-project access control
- [ ] Migration workflow automation
- [ ] Advanced validation rules

## Open Questions

1. **Default Workspace Project**: How do we determine which workspace project to use if not specified?
   - User preference setting?
   - First active project?
   - Configured default in server config?
   
2. **Provider Lifecycle**: When should we close/cleanup workspace providers?
   - On server shutdown?
   - After inactivity timeout?
   - Never (rely on GC)?

3. **Migration Coordination**: How do we coordinate between multiple Hermes instances during migration?
   - Lock mechanism?
   - Leader election?
   - Single-instance migration only?

4. **Configuration Updates**: How do we handle config changes without restart?
   - File watcher?
   - Admin API endpoint?
   - Manual restart required?

5. **Backward Compatibility**: How do we maintain compatibility with single-provider deployments?
   - Auto-create default project?
   - Fallback to Config.Providers.Workspace?
   - Migration path for existing deployments?

## Success Criteria

### Phase 1 (Current)
- ✅ Project config package implemented and tested
- ✅ Clear provider state semantics documented
- [ ] Server loads and validates project config on startup
- [ ] API endpoints return workspace project metadata
- [ ] No secrets exposed in API responses

### Phase 2 (Provider Router)
- [ ] Workspace router selects correct provider by project
- [ ] Provider caching reduces initialization overhead
- [ ] Migration scenarios handled correctly (source/target)
- [ ] Unit test coverage >80%

### Phase 3 (Document Handlers)
- [ ] Documents created in correct workspace project
- [ ] Document operations routed to correct provider
- [ ] Multi-tenant isolation verified
- [ ] E2E tests pass for all scenarios

### Phase 4 (Migration - Indexer)
- [ ] Indexer migrates documents between providers
- [ ] Migration status tracked accurately
- [ ] Zero data loss during migration
- [ ] Migration can be rolled back

## References

- [Project Config Package Summary](./PROJECTCONFIG_PACKAGE_SUMMARY.md)
- [Project Config API Usage Guide](./PROJECTCONFIG_API_USAGE.md)
- [Workspace Provider Architecture](./README-local-workspace.md)
- [Google Workspace Integration](./README-google-workspace.md)
- [Indexer Implementation Guide](./INDEXER_IMPLEMENTATION_GUIDE.md)

## Change Log

- **2025-10-22**: Initial integration plan created
- **2025-10-22**: Added provider router design
- **2025-10-22**: Defined API endpoints and response formats
- **2025-10-22**: Documented migration and validation plans
