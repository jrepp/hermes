# Setup Wizard - Zero-Config to Guided Configuration

## Overview

Hermes now provides a web-based setup wizard for initial configuration. Instead of auto-creating a workspace, users are greeted with a friendly configuration page that guides them through the setup process.

## User Experience Flow

### First Run

```bash
# Download and run - that's it!
./hermes

# Or explicitly (since serve is now default):
./hermes serve
```

**What happens:**

1. **No Config Detected** → Server starts in setup mode
2. **Browser Opens** → Auto-navigates to `http://localhost:8000/setup`
3. **Setup Wizard** → User configures:
   - Workspace directory path (default: `docs-cms`)
   - Upstream server URL (optional)
4. **Config Created** → `config.hcl` generated in working directory
5. **Auto-Reload** → Page redirects to `/`, server ready!

### Subsequent Runs

```bash
./hermes
```

**What happens:**

1. **Config Exists** → Loads `config.hcl` from working directory
2. **Server Starts** → Normal operation with configured workspace
3. **Browser Opens** → Auto-navigates to `http://localhost:8000`

## Setup Wizard UI

The setup page provides:

### Workspace Configuration
- **Path Input**: Defaults to `docs-cms`
- **Validation**: Must be within working directory (no `../` traversal)
- **Preview**: Shows full resolved path
- **Auto-Creation**: Directory structure created automatically

### Optional Features
- **Upstream Server**: URL to central Hermes instance (for future sync)
- **Info Boxes**: Explains what you get (SQLite, Bleve, local storage)
- **Help Links**: Points to advanced configuration docs

### Visual Design
- Clean Tailwind UI
- Loading states
- Error/success messaging
- Auto-redirect on completion

## Architecture

### Backend Components

**1. Setup API (`internal/api/v2/setup.go`)**

```go
// Check if configured
GET /api/v2/setup/status
→ { "is_configured": false, "working_dir": "/path/to/cwd" }

// Submit configuration
POST /api/v2/setup/configure
{
  "workspace_path": "docs-cms",
  "upstream_url": "https://hermes.example.com" // optional
}
→ { "success": true, "config_path": "/path/to/config.hcl" }
```

**2. Serve Command (`internal/cmd/commands/serve/serve.go`)**

```go
// Flow:
1. Check for explicit -config flag → use it
2. Check for config.hcl in CWD → use it
3. No config found → setup mode:
   - Create minimal temp config (just web server)
   - Launch browser to /setup
   - Wait for user to configure
```

**3. Path Validation**

```go
func validateWorkspacePath(userPath, workingDir string) (string, error) {
  // Ensures:
  // - Path is within working directory
  // - No .. traversal
  // - Defaults to docs-cms if empty
}
```

### Frontend Components

**1. Setup Route (`web/app/routes/setup.ts`)**

```typescript
async model() {
  const status = await fetch('/api/v2/setup/status');
  // If already configured, redirect to home
  if (status.is_configured) {
    this.router.transitionTo('authenticated');
  }
}
```

**2. Setup Wizard Component (`web/app/components/setup-wizard.ts`)**

```typescript
@action
async submitSetup(event) {
  const response = await fetch('/api/v2/setup/configure', {
    method: 'POST',
    body: JSON.stringify({
      workspace_path: this.workspacePath,
      upstream_url: this.upstreamURL,
    }),
  });
  
  // On success, redirect to reload with new config
  if (response.ok) {
    setTimeout(() => window.location.href = '/', 2000);
  }
}
```

**3. Application Route Check (`web/app/routes/application.ts`)**

```typescript
async beforeModel(transition) {
  // Skip check if already going to setup
  if (transitionTo !== 'setup') {
    const setupStatus = await fetch('/api/v2/setup/status');
    if (!setupStatus.is_configured) {
      this.router.transitionTo('setup');
      return;
    }
  }
  // Continue with normal config/session loading
}
```

## Configuration Generated

When user submits the setup form, `config.hcl` is created:

```hcl
# Auto-generated Hermes configuration

server {
  addr = "127.0.0.1:8000"
}

providers {
  workspace = "local"
  search    = "bleve"
}

bleve {
  index_path = "/path/to/docs-cms/data/fts.index"
}

local_workspace {
  base_path   = "/path/to/docs-cms"
  docs_path   = "/path/to/docs-cms/documents"
  drafts_path = "/path/to/docs-cms/drafts"
  # ... etc
}

# Disable external auth
okta {
  disabled = true
}

dex {
  disabled = true
}
```

## Workspace Structure Created

```
working-directory/
├── config.hcl                    # Generated config
└── docs-cms/                     # User-configured path
    ├── README.md                 # Auto-generated guide
    ├── documents/                # Published docs
    ├── drafts/                   # Work in progress
    ├── attachments/              # Binary files
    ├── templates/                # Document templates
    └── data/                     # Auto-managed
        ├── hermes.db            # SQLite database
        └── fts.index/           # Bleve search index
```

## Security

### Path Validation
- **Working Directory Restriction**: Workspace must be within CWD
- **No Traversal**: Rejects paths with `../`
- **Absolute Path Resolution**: Prevents symlink attacks

### Unauthenticated Endpoints
Setup endpoints are intentionally unauthenticated because:
1. First-run scenario has no auth configured yet
2. Setup only works when no `config.hcl` exists
3. Once configured, setup endpoints return "already configured"

## Command Line Options

```bash
# Default (runs serve)
./hermes

# Explicit serve
./hermes serve

# With config file (skips setup check)
./hermes serve -config=custom.hcl
./hermes -config=custom.hcl  # also works

# Disable browser launch
./hermes --browser=false

# Other commands still work
./hermes server -config=config.hcl
./hermes indexer -config=config.hcl
./hermes version
```

## Comparison: Before vs After

### Before (RFC-083 Original)

```bash
./hermes serve
# → Auto-creates ./docs-cms/
# → Generates temp config
# → Starts immediately
# → Browser opens to working app
```

**Pros**: Truly zero-config  
**Cons**: No user control, assumes defaults

### After (Current with Setup Wizard)

```bash
./hermes
# → Detects no config
# → Starts web server in setup mode
# → Browser opens to /setup wizard
# → User configures workspace path
# → config.hcl created
# → Restart required (auto page reload)
```

**Pros**: User control, explicit configuration, teaches users about structure  
**Cons**: Requires one extra step (but still very simple)

## Advanced Configuration

The setup wizard creates a **local mode configuration**. For advanced setups:

### Google Workspace Integration
Users can manually edit `config.hcl` or see docs:
```hcl
google_workspace {
  # ... OAuth, service accounts, etc
}
```

### PostgreSQL Database
```hcl
postgres {
  host     = "localhost"
  port     = 5432
  user     = "hermes"
  password = env("POSTGRES_PASSWORD")
  dbname   = "hermes"
}
```

### Algolia Search
```hcl
algolia {
  app_id     = env("ALGOLIA_APP_ID")
  search_key = env("ALGOLIA_SEARCH_KEY")
}
```

The setup wizard mentions these options with a link to the configuration guide.

## Future Enhancements

Potential additions to the setup wizard:

1. **Database Choice**: SQLite (default) vs PostgreSQL
2. **Search Choice**: Bleve (default) vs Algolia vs Meilisearch
3. **Auth Provider**: None (default) vs Google vs Okta vs Dex
4. **Upstream Sync**: Actually implement syncing with central server
5. **Import Existing**: Detect and import from existing workspace
6. **Multi-Step Wizard**: Break into stages (basic → advanced → review)

## Implementation Files

**Backend**:
- `internal/cmd/main.go` - Default to serve command
- `internal/api/v2/setup.go` - Setup API handlers (210 lines)
- `internal/cmd/commands/serve/serve.go` - Setup mode detection
- `internal/cmd/commands/server/server.go` - Endpoint registration

**Frontend**:
- `web/app/router.js` - Setup route registration
- `web/app/routes/setup.ts` - Setup route logic
- `web/app/routes/application.ts` - Setup status check
- `web/app/components/setup-wizard.ts` - Form component
- `web/app/components/setup-wizard.hbs` - Tailwind UI template
- `web/app/templates/setup.hbs` - Route template

**Total**: 10 files, ~600 lines added

## Testing the Setup Flow

```bash
# Build
make bin

# Clean slate (no config)
cd /tmp/test-setup
rm -f config.hcl

# Run
/path/to/hermes

# Expected:
# 1. Terminal shows: "No configuration found. Starting setup wizard..."
# 2. Browser opens to http://localhost:8000/setup
# 3. Fill in:
#    - Workspace: docs-cms (or custom)
#    - Upstream: (leave blank or enter URL)
# 4. Click "Create Configuration"
# 5. Success message shown
# 6. Page redirects to /
# 7. App loads normally

# Verify config created
cat config.hcl

# Run again - should skip setup
/path/to/hermes
# → Loads existing config, starts normally
```

## Key Benefits

1. **User Control**: Explicit choice of workspace location
2. **Discoverable**: Web UI teaches users about Hermes structure
3. **Safe Defaults**: Validates paths, prevents mistakes
4. **Progressive**: Start simple, add advanced config later
5. **Familiar**: Uses existing Ember app, no separate HTML pages
6. **Flexible**: Link to docs for Google Workspace, PostgreSQL, etc.

The setup wizard strikes a balance between **simplicity** (still just `./hermes`) and **control** (explicit configuration with validation).
