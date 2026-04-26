# Project Config API Usage Guide

**Created**: 2025-10-22  
**Package**: `pkg/projectconfig`

## Overview

This guide shows how to use the `projectconfig` package in API handlers and services to:
1. Access project and provider information with clear semantics
2. Determine provider roles and states (active, source, target, archived)
3. Return sanitized project data in API responses (without secrets)
4. Handle migration scenarios correctly

## Provider States and Their Meanings

### State Constants

```go
const (
    ProviderStateActive   = "active"   // Default: read/write operations
    ProviderStateSource   = "source"   // Migration: read-only source
    ProviderStateTarget   = "target"   // Migration: write destination
    ProviderStateArchived = "archived" // No operations
)
```

### State Semantics

| State | Role | Read Operations | Write Operations | Use Case |
|-------|------|----------------|------------------|----------|
| **active** | Primary provider | ✅ Yes | ✅ Yes | Normal operations (default) |
| **source** | Migration source | ✅ Yes | ❌ No | Migrating FROM this provider |
| **target** | Migration target | ⚠️ Limited | ✅ Yes | Migrating TO this provider |
| **archived** | Archived | ❌ No | ❌ No | Historical/inactive provider |

## Basic Usage Examples

### 1. Loading and Accessing Projects

```go
import "github.com/hashicorp-forge/hermes/pkg/projectconfig"

// Load configuration from file
config, err := projectconfig.LoadConfig("/path/to/projects.hcl")
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// Get a specific project
project, err := config.GetProject("testing")
if err != nil {
    return fmt.Errorf("project not found: %w", err)
}

// Get all active projects
activeProjects := config.GetActiveProjects()
for _, proj := range activeProjects {
    log.Printf("Active project: %s (%s)", proj.Name, proj.Title)
}

// Check project status
if project.IsActive() {
    // Project is active
} else if project.IsArchived() {
    // Project is archived
} else if project.IsCompleted() {
    // Project is completed
}
```

### 2. Working with Providers - Single Provider (Non-Migration)

```go
// Get the active provider (default for non-migration scenarios)
provider, err := project.GetActiveProvider()
if err != nil {
    return fmt.Errorf("no active provider: %w", err)
}

// Check provider type
if provider.IsLocal() {
    workspacePath := provider.ResolveWorkspacePath(config.WorkspaceBasePath)
    // Use local filesystem: /app/workspaces/testing
} else if provider.IsGoogle() {
    workspaceID := provider.WorkspaceID
    // Use Google Workspace API
} else if provider.IsRemoteHermes() {
    hermesURL := provider.HermesURL
    // Use remote Hermes API
}

// Get provider state and role information
state := provider.GetState() // "active" (or empty defaults to "active")
role := provider.GetRole()   // "Active (read/write)"

log.Printf("Using %s provider in %s state: %s", provider.Type, state, role)
```

### 3. Working with Providers - Migration Scenario

```go
// Check if project is in migration
if project.IsInMigration() {
    // Get source provider (read-only, migrating FROM)
    sourceProvider, err := project.GetSourceProvider()
    if err != nil {
        return fmt.Errorf("migration source not found: %w", err)
    }
    
    // Get target provider (write destination, migrating TO)
    targetProvider, err := project.GetTargetProvider()
    if err != nil {
        return fmt.Errorf("migration target not found: %w", err)
    }
    
    log.Printf("Migration in progress:")
    log.Printf("  Source: %s - %s", sourceProvider.Type, sourceProvider.GetRole())
    log.Printf("  Target: %s - %s", targetProvider.Type, targetProvider.GetRole())
    
    // Read from source, write to target
    documents := readFromProvider(sourceProvider)
    writeToProvider(targetProvider, documents)
}

// Get the primary provider (handles migration logic automatically)
// - In migration: returns target provider (write destination)
// - Otherwise: returns active provider
primaryProvider, err := project.GetPrimaryProvider()
if err != nil {
    return fmt.Errorf("no primary provider: %w", err)
}

// Use primary provider for all write operations
writeDocument(primaryProvider, document)
```

### 4. Querying Providers by State

```go
// Get all providers in a specific state
activeProviders := project.GetProvidersByState(projectconfig.ProviderStateActive)
sourceProviders := project.GetProvidersByState(projectconfig.ProviderStateSource)
targetProviders := project.GetProvidersByState(projectconfig.ProviderStateTarget)

// Check provider state with helper methods
for _, provider := range project.Providers {
    if provider.IsActiveState() {
        // This provider is active (read/write)
    } else if provider.IsSourceState() {
        // This provider is a migration source (read-only)
    } else if provider.IsTargetState() {
        // This provider is a migration target (write destination)
    } else if provider.IsArchivedState() {
        // This provider is archived (no operations)
    }
    
    // Get human-readable role description
    role := provider.GetRole()
    log.Printf("%s provider: %s", provider.Type, role)
}
```

## API Response Patterns

### 5. Returning Project Summaries (Without Secrets)

```go
// API handler: GET /api/v2/projects
func HandleListProjects(w http.ResponseWriter, r *http.Request) error {
    config, err := projectconfig.LoadConfigFromEnv()
    if err != nil {
        return err
    }
    
    // Get all active projects as sanitized summaries
    summaries := config.GetActiveProjectSummaries()
    
    // Safe to return in API response - no secrets included
    return json.NewEncoder(w).Encode(summaries)
}

// Response structure:
// [
//   {
//     "name": "testing",
//     "title": "Testing Environment",
//     "friendly_name": "Hermes Testing",
//     "short_name": "TEST",
//     "status": "active",
//     "is_active": true,
//     "is_archived": false,
//     "in_migration": false,
//     "providers": [
//       {
//         "type": "local",
//         "state": "active",
//         "role": "Active (read/write)",
//         "workspace_path": "testing",
//         "git_repository": "https://github.com/hashicorp-forge/hermes",
//         "git_branch": "main",
//         "indexing_enabled": true
//         // No credentials, service accounts, or secrets
//       }
//     ],
//     "metadata": {
//       "created_at": "2025-10-22T00:00:00Z",
//       "owner": "hermes-dev-team",
//       "tags": ["testing", "development"]
//     }
//   }
// ]
```

### 6. Returning Single Project Details

```go
// API handler: GET /api/v2/projects/{name}
func HandleGetProject(w http.ResponseWriter, r *http.Request) error {
    projectName := mux.Vars(r)["name"]
    
    config, err := projectconfig.LoadConfigFromEnv()
    if err != nil {
        return err
    }
    
    project, err := config.GetProject(projectName)
    if err != nil {
        http.Error(w, "Project not found", http.StatusNotFound)
        return nil
    }
    
    // Convert to sanitized summary
    summary := project.ToSummary()
    
    return json.NewEncoder(w).Encode(summary)
}
```

### 7. Project Summary Structure (What APIs Return)

```go
type ProjectSummary struct {
    Name         string             `json:"name"`
    Title        string             `json:"title"`
    FriendlyName string             `json:"friendly_name"`
    ShortName    string             `json:"short_name"`
    Description  string             `json:"description"`
    Status       string             `json:"status"`
    
    // Computed fields
    IsActive     bool               `json:"is_active"`
    IsArchived   bool               `json:"is_archived"`
    IsCompleted  bool               `json:"is_completed"`
    InMigration  bool               `json:"in_migration"`
    
    // Provider information (sanitized)
    Providers    []*ProviderSummary `json:"providers"`
    
    // Metadata (no secrets)
    Metadata     *Metadata          `json:"metadata,omitempty"`
}

type ProviderSummary struct {
    Type             string   `json:"type"`              // local, google, remote-hermes
    State            string   `json:"state"`             // active, source, target, archived
    Role             string   `json:"role"`              // Human-readable description
    
    // Type-specific fields (non-sensitive only)
    WorkspacePath    string   `json:"workspace_path,omitempty"`
    WorkspaceID      string   `json:"workspace_id,omitempty"`
    HermesURL        string   `json:"hermes_url,omitempty"`
    APIVersion       string   `json:"api_version,omitempty"`
    GitRepository    string   `json:"git_repository,omitempty"`
    GitBranch        string   `json:"git_branch,omitempty"`
    IndexingEnabled  bool     `json:"indexing_enabled"`
    
    // Authentication indicator (no credentials)
    HasAuthentication bool    `json:"has_authentication"`
    
    // Non-sensitive IDs
    SharedDriveIDs   []string `json:"shared_drive_ids,omitempty"`
}
```

## Migration Workflow Example

### 8. Complete Migration Handling

```go
// Example: Migrating from Google Workspace to Local Filesystem
func MigrateProject(projectName string) error {
    config, err := projectconfig.LoadConfigFromEnv()
    if err != nil {
        return err
    }
    
    project, err := config.GetProject(projectName)
    if err != nil {
        return err
    }
    
    // Verify project is in migration
    if !project.IsInMigration() {
        return fmt.Errorf("project %s is not in migration", projectName)
    }
    
    // Get source and target providers
    source, err := project.GetSourceProvider()
    if err != nil {
        return fmt.Errorf("no source provider: %w", err)
    }
    
    target, err := project.GetTargetProvider()
    if err != nil {
        return fmt.Errorf("no target provider: %w", err)
    }
    
    log.Printf("Migration plan:")
    log.Printf("  FROM: %s (%s) - %s", source.Type, source.GetState(), source.GetRole())
    log.Printf("  TO:   %s (%s) - %s", target.Type, target.GetState(), target.GetRole())
    
    // Initialize source (read-only)
    if source.IsGoogle() {
        sourceWorkspace, err := initGoogleWorkspace(source)
        if err != nil {
            return err
        }
        defer sourceWorkspace.Close()
        
        // Initialize target (write)
        if target.IsLocal() {
            targetWorkspace, err := initLocalWorkspace(target, config.WorkspaceBasePath)
            if err != nil {
                return err
            }
            defer targetWorkspace.Close()
            
            // Perform migration
            return migrateDocuments(sourceWorkspace, targetWorkspace)
        }
    }
    
    return fmt.Errorf("unsupported migration path: %s -> %s", source.Type, target.Type)
}
```

## Configuration Examples

### 9. Single Active Provider (No Migration)

```hcl
# testing/projects/testing.hcl
project "testing" {
  title         = "Testing Environment"
  friendly_name = "Hermes Testing"
  short_name    = "TEST"
  status        = "active"
  
  provider "local" {
    migration_status = "active"  # Or omit - defaults to "active"
    workspace_path   = "testing"
    
    git {
      repository = "https://github.com/hashicorp-forge/hermes"
      branch     = "main"
    }
    
    indexing {
      enabled = true
      allowed_extensions = ["md", "txt", "json"]
    }
  }
  
  metadata {
    created_at = "2025-10-22T00:00:00Z"
    owner      = "hermes-dev-team"
    tags       = ["testing", "local"]
  }
}
```

**API Response**:
```json
{
  "name": "testing",
  "title": "Testing Environment",
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
      "git_branch": "main",
      "indexing_enabled": true
    }
  ]
}
```

### 10. Migration Scenario (Google → Local)

```hcl
# testing/projects/docs.hcl
project "docs" {
  title         = "Documentation"
  friendly_name = "Hermes Docs"
  short_name    = "DOCS"
  status        = "active"
  
  # Source: Google Workspace (read-only during migration)
  provider "google" {
    migration_status      = "source"
    workspace_id          = env("GOOGLE_WORKSPACE_ID")
    service_account_email = env("GOOGLE_SERVICE_ACCOUNT_EMAIL")
    credentials_path      = env("GOOGLE_CREDENTIALS_PATH")
    shared_drive_ids      = [env("GOOGLE_SHARED_DRIVE_ID")]
  }
  
  # Target: Local Filesystem (write destination during migration)
  provider "local" {
    migration_status = "target"
    workspace_path   = "docs"
    
    git {
      repository = "https://github.com/hashicorp-forge/hermes-docs"
      branch     = "main"
    }
    
    indexing {
      enabled = true
      allowed_extensions = ["md", "txt"]
    }
  }
  
  metadata {
    owner = "docs-team"
    tags  = ["documentation", "migration"]
    notes = "Migrating from Google Workspace to Git repository"
  }
}
```

**API Response**:
```json
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
      "workspace_id": "workspace-abc123",
      "shared_drive_ids": ["drive-xyz789"],
      "has_authentication": true
      // No service_account_email or credentials_path (secrets excluded)
    },
    {
      "type": "local",
      "state": "target",
      "role": "Migration target (write destination)",
      "workspace_path": "docs",
      "git_repository": "https://github.com/hashicorp-forge/hermes-docs",
      "git_branch": "main",
      "indexing_enabled": true
    }
  ]
}
```

## Best Practices

### ✅ DO

1. **Use `GetPrimaryProvider()`** for write operations - it handles migration logic automatically
2. **Use `ToSummary()`** for API responses - it excludes secrets
3. **Check `IsInMigration()`** before accessing source/target providers
4. **Use state constants** (`ProviderStateActive`, etc.) instead of hardcoded strings
5. **Use `GetRole()`** for human-readable descriptions in logs/UI
6. **Validate provider states** with `IsActiveState()`, `IsSourceState()`, etc.

### ❌ DON'T

1. **Don't return raw `Provider` objects** in API responses - use `ProviderSummary`
2. **Don't hardcode provider states** - use the constants
3. **Don't assume single provider** - always check `IsInMigration()`
4. **Don't expose credentials** in logs, errors, or API responses
5. **Don't use `len(project.Providers) > 1`** to check migration - use `IsInMigration()`

## Common Patterns

### Pattern 1: Router-Level Project Selection

```go
// Select provider based on request context
func SelectProvider(project *projectconfig.Project, readOnly bool) (*projectconfig.Provider, error) {
    if project.IsInMigration() {
        if readOnly {
            // Read from source during migration
            return project.GetSourceProvider()
        } else {
            // Write to target during migration
            return project.GetTargetProvider()
        }
    }
    
    // Non-migration: use active provider
    return project.GetActiveProvider()
}
```

### Pattern 2: Service Initialization

```go
// Initialize workspace service with correct provider
func NewWorkspaceService(projectName string) (*WorkspaceService, error) {
    config, err := projectconfig.LoadConfigFromEnv()
    if err != nil {
        return nil, err
    }
    
    project, err := config.GetProject(projectName)
    if err != nil {
        return nil, err
    }
    
    // Get primary provider (handles migration)
    provider, err := project.GetPrimaryProvider()
    if err != nil {
        return nil, err
    }
    
    // Initialize based on provider type
    if provider.IsLocal() {
        return NewLocalWorkspaceService(provider, config.WorkspaceBasePath)
    } else if provider.IsGoogle() {
        return NewGoogleWorkspaceService(provider)
    } else if provider.IsRemoteHermes() {
        return NewRemoteHermesService(provider)
    }
    
    return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
}
```

### Pattern 3: Document Operations with Provider Awareness

```go
// Read document (migration-aware)
func ReadDocument(projectName, docID string) (*Document, error) {
    config, _ := projectconfig.LoadConfigFromEnv()
    project, _ := config.GetProject(projectName)
    
    var provider *projectconfig.Provider
    var err error
    
    if project.IsInMigration() {
        // During migration, read from source
        provider, err = project.GetSourceProvider()
    } else {
        // Normal operation, use active provider
        provider, err = project.GetActiveProvider()
    }
    
    if err != nil {
        return nil, err
    }
    
    return readDocumentFromProvider(provider, docID)
}

// Write document (migration-aware)
func WriteDocument(projectName string, doc *Document) error {
    config, _ := projectconfig.LoadConfigFromEnv()
    project, _ := config.GetProject(projectName)
    
    // Always write to primary provider (target during migration)
    provider, err := project.GetPrimaryProvider()
    if err != nil {
        return err
    }
    
    return writeDocumentToProvider(provider, doc)
}
```

## Troubleshooting

### Error: "no active provider found"

```go
// Cause: No provider with migration_status="active" or empty
// Solution: Ensure at least one provider has migration_status="active" or omit migration_status

provider "local" {
  migration_status = "active"  // Or omit this line
  workspace_path = "testing"
}
```

### Error: "no source provider found"

```go
// Cause: Called GetSourceProvider() but no provider has migration_status="source"
// Solution: Only call GetSourceProvider() after checking IsInMigration()

if project.IsInMigration() {
    source, err := project.GetSourceProvider()
    // Now safe to use
}
```

### Error: "project not in migration"

```go
// Cause: Trying to access source/target but project has single provider
// Solution: Check migration status first

if project.IsInMigration() {
    source, _ := project.GetSourceProvider()
    target, _ := project.GetTargetProvider()
    // Perform migration
} else {
    // Use active provider only
    provider, _ := project.GetActiveProvider()
}
```

## Summary

**Key Accessor Methods**:
- `GetActiveProvider()` - Get active provider (default for non-migration)
- `GetPrimaryProvider()` - Get primary provider (handles migration logic)
- `GetSourceProvider()` - Get migration source (read-only)
- `GetTargetProvider()` - Get migration target (write destination)
- `GetProvidersByState(state)` - Get all providers in specific state
- `ToSummary()` - Get sanitized project summary for API responses

**Key State Checkers**:
- `IsInMigration()` - Check if project is in migration (has source + target)
- `IsActive()` / `IsArchived()` / `IsCompleted()` - Check project status
- `IsActiveState()` / `IsSourceState()` / `IsTargetState()` - Check provider state

**Key Helper Methods**:
- `GetState()` - Get provider state (defaults empty to "active")
- `GetRole()` - Get human-readable role description
- `GetAllProjectSummaries()` - Get all projects (sanitized)
- `GetActiveProjectSummaries()` - Get active projects only (sanitized)

**Security**:
- Always use `ToSummary()` or `GetAllProjectSummaries()` for API responses
- Never return raw `Provider` objects with credentials
- `ProviderSummary` automatically excludes secrets
