# Demo 1: Local-First Development Example

This directory contains example configurations and scripts demonstrating Hermes' local-first capabilities.

## Overview

Demonstrates:
1. **Zero Cloud Dependencies**: Complete environment runs locally
2. **Configuration-Driven Providers**: Swap backends via config file
3. **Rapid Local Testing**: Create, edit, and search documents offline

## Files

- `config-local.hcl` - Local development configuration (Dex + local workspace + Meilisearch)
- `config-production.hcl` - Production configuration example (Google + Google Workspace + Algolia)
- `example-document.md` - Sample Markdown document in local workspace format
- `test-local-environment.sh` - Quick verification script

## Quick Start

### 1. Start Local Environment

```bash
# From Hermes root
cd testing && docker compose up -d
```

### 2. Verify Services

```bash
# Check all services are running
docker compose ps

# Should show:
# - postgres (database)
# - meilisearch (search)
# - dex (auth provider)
# - hermes-backend (API server)
# - hermes-frontend (web UI)
```

### 3. Access Hermes

```bash
# Open in browser
open http://localhost:4201

# Login credentials
# Email: test@hermes.local
# Password: password
```

## Configuration Examples

### Local Development (config-local.hcl)

```hcl
providers {
  auth      = "dex"          # Local OIDC provider (no Google OAuth)
  workspace = "local"        # Filesystem-based storage
  search    = "meilisearch"  # Self-hosted search engine
}

dex {
  addr = "http://localhost:5556"
}

workspace_local {
  base_path = "./testing/workspace_data"
}

meilisearch {
  addr = "http://localhost:7700"
  api_key = "masterKey"
}
```

**Benefits**:
- ✅ No cloud credentials required
- ✅ Fast iteration (no network latency)
- ✅ Cost: $0 for development
- ✅ Works offline after initial setup

### Production (config-production.hcl)

```hcl
providers {
  auth      = "google"       # Google OAuth
  workspace = "google"       # Google Workspace (Docs)
  search    = "algolia"      # Managed search service
}

google_oauth {
  client_id     = env("GOOGLE_CLIENT_ID")
  client_secret = env("GOOGLE_CLIENT_SECRET")
}

google_workspace {
  credentials_file = env("GOOGLE_CREDENTIALS_PATH")
  folder_id        = env("GOOGLE_FOLDER_ID")
}

algolia {
  app_id  = env("ALGOLIA_APP_ID")
  api_key = env("ALGOLIA_API_KEY")
}
```

**Benefits**:
- ✅ Google Workspace integration
- ✅ Managed search (Algolia)
- ✅ Enterprise SSO (Google OAuth)
- ✅ Production-grade infrastructure

## Local Workspace Structure

Documents stored as Markdown files with frontmatter:

```
testing/workspace_data/
├── drafts/
│   ├── RFC-001-example-proposal.md
│   └── ADR-002-architecture-decision.md
├── in-review/
│   └── MEMO-003-project-update.md
└── approved/
    └── PRD-004-feature-specification.md
```

### Example Document Format

See `example-document.md` for a complete example:

```markdown
---
title: "Example RFC: Multi-Provider Architecture"
document_type: "RFC"
status: "draft"
authors:
  - email: "author@example.com"
approvers:
  - email: "approver@example.com"
created_at: "2025-11-10T10:00:00Z"
modified_at: "2025-11-10T12:00:00Z"
---

# RFC-001: Multi-Provider Architecture

## Summary
This RFC proposes a modular provider architecture...

## Background
...
```

## Testing the Local Environment

### Manual Testing

1. **Create a Document**:
   - Navigate to http://localhost:4201
   - Click "New Document"
   - Fill in title, select type (RFC, ADR, MEMO, etc.)
   - Save

2. **Verify Local Storage**:
   ```bash
   # Document should appear in workspace
   ls testing/workspace_data/drafts/
   cat testing/workspace_data/drafts/RFC-XXX-your-title.md
   ```

3. **Test Search**:
   - Use search bar in UI
   - Query powered by Meilisearch (not cloud service)
   - Check Meilisearch directly:
     ```bash
     curl http://localhost:7700/indexes/documents/search \
       -H "Authorization: Bearer masterKey" \
       -d '{"q": "your search term"}'
     ```

4. **Test Authentication**:
   - Logout from UI
   - Login with: test@hermes.local / password
   - Auth handled by local Dex (OIDC) provider

### Automated Testing

```bash
# Run E2E tests against local environment
cd tests/e2e-playwright
npx playwright test --reporter=line

# Tests cover:
# - Document creation
# - Search functionality
# - Approval workflows
# - User authentication
```

## Performance Comparison

| Metric | Local (Dex + Local + Meilisearch) | Production (Google + Google + Algolia) |
|--------|-----------------------------------|----------------------------------------|
| **Setup Time** | 5 minutes | 2-3 hours (OAuth credentials, API keys) |
| **API Latency** | <5ms (localhost) | 50-200ms (cloud services) |
| **Cost/Month** | $0 (self-hosted) | $50+ (Algolia, Google API calls) |
| **Offline Support** | ✅ Yes (after initial setup) | ❌ No (requires internet) |
| **Test Execution** | ~30 seconds | ~2-3 minutes (network latency) |

## Migration Between Configurations

### Switch from Local to Production

1. **Update config.hcl**:
   ```hcl
   # Change providers section
   providers {
     auth      = "google"    # was "dex"
     workspace = "google"    # was "local"
     search    = "algolia"   # was "meilisearch"
   }
   ```

2. **Set environment variables**:
   ```bash
   export GOOGLE_CLIENT_ID="your-client-id"
   export GOOGLE_CLIENT_SECRET="your-client-secret"
   export GOOGLE_CREDENTIALS_PATH="/path/to/credentials.json"
   export ALGOLIA_APP_ID="your-app-id"
   export ALGOLIA_API_KEY="your-api-key"
   ```

3. **Restart server**:
   ```bash
   ./hermes server -config=config.hcl
   ```

4. **Migrate documents** (optional):
   ```bash
   # Copy documents from local to Google Workspace
   ./hermes migrate --source local --target google
   ```

### Switch from Production to Local

Reverse the process above. Useful for:
- Offline development
- Cost reduction in dev/test environments
- Air-gapped deployments
- Troubleshooting without affecting production

## Troubleshooting

### Services Won't Start

```bash
# Check logs
cd testing && docker compose logs

# Common issues:
# - Port conflicts (8001, 4201, 5432, 7700, 5556)
# - Docker daemon not running
# - Insufficient memory (<4GB available)
```

### Documents Not Appearing

```bash
# Verify workspace directory exists and is writable
ls -la testing/workspace_data/

# Check backend logs
docker compose logs hermes-backend | grep workspace

# Verify configuration
cat config.hcl | grep workspace
```

### Search Not Working

```bash
# Check Meilisearch health
curl http://localhost:7700/health

# View indexes
curl http://localhost:7700/indexes \
  -H "Authorization: Bearer masterKey"

# Check documents are indexed
curl http://localhost:7700/indexes/documents/documents \
  -H "Authorization: Bearer masterKey"
```

### Authentication Issues

```bash
# Verify Dex is running
curl http://localhost:5556/.well-known/openid-configuration

# Check backend Dex configuration
cat config.hcl | grep -A 5 dex

# View backend logs for auth errors
docker compose logs hermes-backend | grep -i auth
```

## Next Steps

1. **Explore UI**: Create different document types (RFC, ADR, MEMO, PRD)
2. **Test Search**: Use search bar, observe Meilisearch results
3. **Inspect Files**: Examine Markdown files in `testing/workspace_data/`
4. **Run Tests**: Execute Playwright E2E suite
5. **Try Migration**: Experiment with provider switching

## Related Documentation

- [Testing Environment Guide](../../testing/README.md)
- [Local Workspace Documentation](../../docs-internal/README-local-workspace.md)
- [Dex Authentication Setup](../../docs-internal/README-dex.md)
- [Meilisearch Configuration](../../docs-internal/README-meilisearch.md)
- [ADR-071: Local File Workspace](../../docs-internal/adr/ADR-071-local-file-workspace-system.md)
- [ADR-073: Provider Abstraction](../../docs-internal/adr/ADR-073-provider-abstraction-architecture.md)

## Key Takeaways

✅ **Local-first**: Complete development environment without cloud dependencies
✅ **Configuration-driven**: Swap providers by editing config.hcl
✅ **Migration-ready**: Move documents between providers safely
✅ **Cost-effective**: $0/month for local development
✅ **Fast iteration**: <5ms latency for all operations
✅ **Production-flexible**: Same code runs with different backends
