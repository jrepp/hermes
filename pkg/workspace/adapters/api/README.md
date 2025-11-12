# API Workspace Provider

The API workspace provider implements all RFC-084 WorkspaceProvider interfaces by delegating operations to a remote Hermes instance via REST API.

## Overview

This provider enables edge-to-central architectures where an edge Hermes instance can delegate operations to a central Hermes server.

### Use Cases

1. **Local Authoring with Central Tracking**:
   - Developer runs edge Hermes with local Git provider + API provider
   - Local documents authored in Git
   - Directory, permissions, notifications delegated to central Hermes

2. **Multi-Tier Hermes Deployment**:
   - Edge Hermes instances in different regions/offices
   - All delegate to central Hermes for coordination
   - Central Hermes manages Google Workspace, identity, etc.

3. **Offline-Capable Edge**:
   - Edge Hermes works offline with local provider
   - When online, syncs metadata to central via API provider
   - Central tracks all documents globally

## Configuration

```hcl
api_workspace {
  base_url   = "https://central.hermes.company.com"
  auth_token = env("HERMES_API_TOKEN")
  timeout    = "30s"
  tls_verify = true
  max_retries = 3
}
```

## Implemented Interfaces

The API provider implements all 8 RFC-084 interfaces:

- `workspace.WorkspaceProvider` - Core provider metadata
- `workspace.DocumentProvider` - Document CRUD operations
- `workspace.ContentProvider` - Document content operations
- `workspace.RevisionTrackingProvider` - Revision history
- `workspace.PermissionProvider` - Access control
- `workspace.PeopleProvider` - Directory search
- `workspace.TeamProvider` - Group management
- `workspace.NotificationProvider` - Email notifications

Compile-time interface checks ensure all methods are implemented correctly (see `provider.go`).

## Testing

### Integration Tests

Run integration tests with:

```bash
go test -tags=integration -v ./pkg/workspace/adapters/api/test/...
```

The integration test suite covers:

- Configuration validation
- Interface implementation verification
- Provider metadata (Name, ProviderType)
- Default value application
- Error handling (unreachable servers, timeouts)
- Compile-time interface checks

### Test Coverage

All tests pass without requiring a live Hermes server. Tests validate:

- ✅ Configuration validation (missing fields, invalid URLs, negative values)
- ✅ All 8 RFC-084 interfaces implemented
- ✅ Provider metadata correctly set
- ✅ Defaults applied (TLSVerify, Timeout, MaxRetries, RetryDelay)
- ✅ Graceful error handling for connection failures
- ✅ Timeout handling during operations

## Architecture

```
┌─────────────────────────────────────────┐
│ Edge Hermes (Developer Laptop)          │
├─────────────────────────────────────────┤
│  Local Git Provider (primary)           │
│  API Provider (delegates to central)    │
└─────────────────┬───────────────────────┘
                  │ REST API
                  │ /api/v2/*
                  ▼
┌─────────────────────────────────────────┐
│ Central Hermes (Company Server)         │
├─────────────────────────────────────────┤
│  Google Workspace Provider              │
│  - Documents                             │
│  - Directory (People)                    │
│  - Groups (Teams)                        │
│  - Gmail (Notifications)                 │
└─────────────────────────────────────────┘
```

## API Endpoints Required

The remote Hermes instance must expose these REST API endpoints. See `doc.go` for the complete list of required endpoints.

## Error Handling

The provider includes:

- Automatic retry with exponential backoff
- Circuit breaker for network resilience
- Clear error messages with context
- Capability checking before operations

## Performance Considerations

- HTTP/2 with connection pooling
- Configurable timeouts and retries
- Batch operations for bulk data transfer
- Capability discovery to avoid unnecessary requests

## Security

- Bearer token authentication
- TLS with certificate verification
- Auth token not logged or serialized to JSON
- Configurable TLS verification for dev/test environments

## Development

### Recent Changes

**2024-11-12**: Fixed syntax error in provider.go
- Issue: File corruption causing parse error at line 53
- Resolution: Recreated provider.go from clean source
- Added validation tooling to prevent similar issues

### Validation Tooling

To prevent syntax errors and maintain code quality, use the comprehensive validation tooling:

**Quick commands:**
```bash
# One-time setup
make install-hooks
make complexity-install

# Before committing
make validate

# Check complexity
make complexity
```

**Available tools:**

1. **Pre-commit hooks** - Automatic validation before commits
   - Code formatting, static analysis, build checks
   - Module tidiness, test compilation
   - Complexity analysis on push

2. **CI/CD validation** - Automated checks on PRs
   - All pre-commit checks
   - golangci-lint, complexity reports

3. **Make targets** - Convenient development commands
   - `make fmt` - Format code
   - `make lint` - Run linters
   - `make complexity` - Check complexity
   - `make validate` - Full validation
   - `make help` - See all commands

**Documentation:**
- Quick Start: [VALIDATION-QUICK-START.md](/VALIDATION-QUICK-START.md)
- Complete Guide: [docs/development/validation-tools.md](/docs/development/validation-tools.md)

## Files

- `provider.go` - Core provider implementation and interface checks
- `config.go` - Configuration structure and validation
- `document_provider.go` - Document CRUD operations
- `content_provider.go` - Content operations and comparison
- `revision_provider.go` - Revision history management
- `permission_provider.go` - Permission management
- `people_provider.go` - Directory search operations
- `team_provider.go` - Group/team operations
- `notification_provider.go` - Email notifications
- `doc.go` - Package documentation
- `test/integration_test.go` - Integration test suite
