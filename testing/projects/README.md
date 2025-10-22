# Hermes Projects Configuration

This directory contains HCL configuration files for Hermes projects. Each project defines document collections with their workspace providers and indexing settings.

## Structure

```
testing/
├── projects.hcl              # Main config with imports and global settings
└── projects/                 # Individual project configurations
    ├── testing.hcl           # Hermes testing environment (active)
    ├── docs.hcl              # Public documentation (active)
    ├── _template-google.hcl  # Template for Google Workspace
    ├── _template-migration.hcl  # Template for migration scenarios
    └── _template-remote.hcl  # Template for remote Hermes federation
```

## Quick Start

### Active Projects

Only these projects are active in the OSS deployment:

- **testing** (`projects/testing.hcl`): Local testing workspace
- **docs** (`projects/docs.hcl`): Public documentation

### Templates (Prefixed with `_template-`)

Template files are **NOT loaded** by default. They serve as examples for:
- Google Workspace integration
- Migration from Google to Git
- Federation with remote Hermes instances

## Project Configuration

### Basic Structure

```hcl
project "short-name" {
  title         = "Human-Readable Title"
  friendly_name = "Display Name"
  short_name    = "ABBR"  # Used in document IDs: ABBR-001
  description   = "Project description"
  status        = "active"  # active, completed, archived
  
  provider "local" {
    # Provider configuration
  }
  
  metadata {
    # Project metadata
  }
}
```

### Short Names

Short names are used in:
- **Document identifiers**: `RFC-001`, `PRD-042`, `TEST-123`
- **URLs**: `/projects/testing`, `/docs/RFC-001`
- **Search facets**: Filter by project short name
- **Display**: Compact project reference

**Important**: Short names should be:
- 2-10 uppercase characters
- Memorable and descriptive
- Unique within your Hermes instance (not globally unique)

### Global Settings

The main `projects.hcl` file defines global settings:

```hcl
projects {
  version = "1.0.0-alpha"
  config_dir = "./projects"
  
  # Base path for all workspace data
  # Container: /app/workspaces
  # Native dev: ./testing/workspaces (override with env var or local config)
  workspace_base_path = "/app/workspaces"
}
```

Individual projects use **relative paths** under this base:

```hcl
provider "local" {
  workspace_path = "testing"  # Resolves to /app/workspaces/testing
}
```

### Provider Types

#### Local Provider

```hcl
provider "local" {
  migration_status = "active"
  
  # Relative to workspace_base_path
  workspace_path = "my-project"  # → /app/workspaces/my-project
  
  git {
    repository = "https://github.com/org/repo"
    branch     = "main"
  }
  
  indexing {
    enabled            = true
    allowed_extensions = ["md", "txt"]
    public_read_access = false
  }
}
```

**Container Mapping** (docker-compose.yml):
```yaml
volumes:
  - ./workspaces/my-project:/app/workspaces/my-project
```

**Native Development**:
```bash
export HERMES_WORKSPACE_BASE_PATH="./testing/workspaces"
# Or create projects.local.hcl with workspace_base_path = "./testing/workspaces"
```

#### Google Provider (Template)

```hcl
provider "google" {
  migration_status = "active"
  
  workspace_id          = env("GOOGLE_WORKSPACE_ID")
  service_account_email = env("GOOGLE_SERVICE_ACCOUNT_EMAIL")
  credentials_path      = env("GOOGLE_CREDENTIALS_PATH")
  
  shared_drive_ids = [
    env("GOOGLE_DRIVE_ID_1"),
  ]
  
  indexing {
    enabled = true
  }
}
```

**⚠️ Security**: Always use `env()` for credentials, never hardcode!

#### Remote Hermes Provider (Template)

```hcl
provider "remote-hermes" {
  migration_status = "active"
  
  hermes_url  = env("REMOTE_HERMES_URL")
  api_version = "v2"
  
  authentication {
    method         = "oidc"
    client_id      = env("FEDERATION_CLIENT_ID")
    client_secret  = env("FEDERATION_CLIENT_SECRET")
    token_endpoint = env("OIDC_TOKEN_ENDPOINT")
  }
  
  sync_mode = "read-only"
  cache_ttl = 3600
}
```

### Migration Scenarios

When migrating documents between providers:

```hcl
project "my-docs" {
  # ...
  
  # SOURCE: Where documents are coming FROM
  provider "google" {
    migration_status   = "source"
    migration_started  = "2025-09-01T00:00:00Z"
    # ... config
  }
  
  # TARGET: Where documents are going TO
  provider "local" {
    migration_status   = "target"
    migration_started  = "2025-09-01T00:00:00Z"
    # ... config
  }
  
  # Migration settings
  migration {
    conflict_detection_enabled = true
    sync_interval              = "1h"
    notify_on_conflict         = true
    notification_email         = env("MIGRATION_NOTIFY_EMAIL")
  }
}
```

**Migration Status Values**:
- `active`: Normal single-provider operation
- `source`: Provider being migrated FROM
- `target`: Provider being migrated TO
- `archived`: Migration complete, no longer active

## Document Identification

### UUID-Based Stable Identity

Every document has a **stable UUID** that persists across providers:

```
hermes://uuid/550e8400-e29b-41d4-a716-446655440000
```

### Short Name in Document IDs

Documents use the project short name in their display ID:

```
RFC-001: API Design Guidelines
PRD-042: New Feature Specification
TEST-123: Integration Test Document
```

### Frontmatter Discovery

Hermes looks for UUIDs in document frontmatter:

```yaml
---
hermes-uuid: 550e8400-e29b-41d4-a716-446655440000
project: testing
short-name: TEST
doc-id: 123
---
```

If missing, Hermes generates a UUID and updates the document.

## Environment Variables

Use environment variables for all sensitive data:

```bash
# Google Workspace
export GOOGLE_WORKSPACE_ID="workspace-12345"
export GOOGLE_SERVICE_ACCOUNT_EMAIL="hermes@project.iam.gserviceaccount.com"
export GOOGLE_CREDENTIALS_PATH="/path/to/credentials.json"
export GOOGLE_RFCS_DRIVE_ID="0ABcDef..."

# Remote Hermes Federation
export REMOTE_HERMES_URL="https://hermes.internal.company.com"
export FEDERATION_CLIENT_ID="hermes-oss-client"
export FEDERATION_CLIENT_SECRET="secret-value"
export OIDC_TOKEN_ENDPOINT="https://auth.company.com/oauth2/token"

# Migration
export MIGRATION_NOTIFY_EMAIL="team@example.com"
export GIT_PLATFORM_DOCS_REPO="https://github.com/org/platform-docs"
```

## Adding a New Project

1. **Create project file**: `projects/my-project.hcl`
2. **Choose short name**: Pick a 2-10 char abbreviation
3. **Configure provider**: Local, Google, or remote Hermes
4. **Add metadata**: Owner, tags, description
5. **Import in main config**: Add `import "projects/my-project.hcl"` to `projects.hcl`

Example:

```hcl
# projects/security.hcl
project "security" {
  title         = "Security Team Documents"
  friendly_name = "Security"
  short_name    = "SEC"
  description   = "Security policies and incident reports"
  status        = "active"
  
  provider "local" {
    migration_status = "active"
    workspace_path   = "./security-docs"
    
    git {
      repository = "https://github.com/example/security-docs"
      branch     = "main"
    }
    
    indexing {
      enabled            = true
      allowed_extensions = ["md"]
      public_read_access = false  # Internal only
    }
  }
  
  metadata {
    created_at = "2025-10-22T00:00:00Z"
    owner      = "security-team"
    tags       = ["security", "confidential"]
  }
}
```

Then in `projects.hcl`:

```hcl
import "projects/testing.hcl"
import "projects/docs.hcl"
import "projects/security.hcl"  # Add this line
```

## Security Best Practices

### ✅ DO

- Use `env()` for all credentials and secrets
- Use separate project files for each team/project
- Prefix templates with `_template-` to prevent accidental loading
- Use `.gitignore` to protect internal project configs
- Document which projects are OSS-safe vs internal-only
- Review commits for accidentally included credentials

### ❌ DON'T

- Never commit real credentials or API keys
- Never hardcode workspace IDs or drive IDs
- Never include internal company domain names
- Never commit `projects.local.hcl` or `projects.production.hcl`
- Never include real email addresses in examples

## Testing

Validate your project configuration:

```bash
# Check HCL syntax
hclconf check testing/projects.hcl

# Validate project configs (when implemented)
./hermes projects validate -config=testing/projects.hcl

# List all projects
./hermes projects list -config=testing/projects.hcl

# Show specific project
./hermes projects show testing -config=testing/projects.hcl
```

## See Also

- [Distributed Projects Architecture](../docs-internal/DISTRIBUTED_PROJECTS_ARCHITECTURE.md)
- [Config.hcl Documentation](../docs-internal/CONFIG_HCL_DOCUMENTATION.md)
- [Local Workspace Provider](../docs-internal/README-local-workspace.md)
- [Google Workspace Provider](../docs-internal/README-google-workspace.md)
