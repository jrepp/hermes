---
id: RFC-085
title: API Provider and Remote Delegation
date: 2025-11-11
type: RFC
subtype: Implementation
status: Proposed
tags: [api-provider, remote-delegation, federation, multi-tier]
related:
  - RFC-084
  - RFC-086
  - RFC-082
---

# API Provider and Remote Delegation

## Executive Summary

This RFC proposes implementing an API provider that delegates workspace and search operations to a remote Hermes instance via REST API. This enables multi-tier deployments, federated document management, and hybrid cloud/on-premise architectures.

**Key Benefits**:
- Enable multi-tier architectures (edge nodes → central Hermes)
- Support federated document management across multiple Hermes instances
- Allow thin clients that delegate storage to remote Hermes servers
- Maintain consistent API contracts across all deployment models

**Related RFCs**:
- **RFC-084**: Provider Interface Refactoring (defines the 7 required interfaces)
- **RFC-086**: Authentication and Bearer Token Management (auth strategy)

## Context

### Use Cases for Remote API Provider

**Use Case 1: Multi-Tier Architecture**
```
┌──────────────┐           ┌──────────────┐
│ Edge Hermes  │  ─REST─>  │ Central      │
│ (thin client)│  <─API──  │ Hermes       │
│              │           │ (full stack) │
└──────────────┘           └──────┬───────┘
                                  │
                          ┌───────┴────────┐
                          │ Google/Local   │
                          │ Backend        │
                          └────────────────┘
```

**Use Case 2: Federated Documents**
```
┌──────────────┐           ┌──────────────┐
│ Team A       │  ─┐       │ Central      │
│ Hermes       │   │       │ Hermes       │
└──────────────┘   │       │ (aggregator) │
                   ├─REST─>│              │
┌──────────────┐   │       └──────────────┘
│ Team B       │   │
│ Hermes       │  ─┘
└──────────────┘
```

**Use Case 3: Hybrid Cloud/On-Premise**
```
┌──────────────┐           ┌──────────────┐
│ On-Premise   │  ─────>   │ Cloud        │
│ Hermes       │  <─REST─  │ Hermes       │
│ (air-gapped) │           │ (internet)   │
└──────────────┘           └──────────────┘
      │                           │
  Local Docs              Google Workspace
```

## Proposed Solution

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                   Hermes Server                         │
├─────────────────────────────────────────────────────────┤
│  Workspace Providers (pkg/workspace/provider.go)        │
│                                                          │
│  ┌────────────┐  ┌────────────┐  ┌──────────────────┐  │
│  │  Google    │  │   Local    │  │   API Provider   │  │
│  │  Provider  │  │  Provider  │  │   (NEW)          │  │
│  └────────────┘  └────────────┘  └────────┬─────────┘  │
│       │               │                    │             │
└───────┼───────────────┼────────────────────┼─────────────┘
        │               │                    │
        │               │                    │ REST API
        │               │                    │ (/api/v2/...)
        │               │                    ▼
        │               │         ┌─────────────────────┐
        │               │         │ Remote Hermes       │
        │               │         │ Instance            │
        │               │         └─────────────────────┘
        │               │
   Google APIs    Local Filesystem
```

### API Provider Implementation

```go
// Package apiworkspace provides a workspace provider that delegates
// to a remote Hermes instance via REST API.
package apiworkspace

import (
    "context"
    "github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Config contains configuration for the API provider
type Config struct {
    // BaseURL is the base URL of the remote Hermes instance
    // Example: "https://hermes.example.com"
    BaseURL string `hcl:"base_url"`

    // AuthToken is the API token for authentication
    AuthToken string `hcl:"auth_token"`

    // TLSVerify controls TLS certificate verification
    TLSVerify bool `hcl:"tls_verify,optional"`

    // Timeout for API requests
    Timeout time.Duration `hcl:"timeout,optional"`
}

// Provider implements all 7 workspace interfaces by delegating to remote Hermes API
type Provider struct {
    config       *Config
    client       *http.Client
    capabilities *Capabilities
}

// Capabilities discovered from remote API
type Capabilities struct {
    SupportsContent     bool
    SupportsPermissions bool
    SupportsDirectory   bool
    SupportsGroups      bool
    SupportsEmail       bool
    SupportsRevisions   bool
}

// Compile-time checks - API provider implements all interfaces
var _ workspace.WorkspaceProvider = (*Provider)(nil)
var _ workspace.DocumentProvider = (*Provider)(nil)
var _ workspace.ContentProvider = (*Provider)(nil)
var _ workspace.PermissionProvider = (*Provider)(nil)
var _ workspace.PeopleProvider = (*Provider)(nil)
var _ workspace.TeamProvider = (*Provider)(nil)
var _ workspace.NotificationProvider = (*Provider)(nil)
var _ workspace.RevisionTrackingProvider = (*Provider)(nil)

func NewProvider(cfg *Config) (*Provider, error) {
    p := &Provider{
        config: cfg,
        client: &http.Client{
            Timeout: cfg.Timeout,
        },
    }

    // Discover remote capabilities
    if err := p.discoverCapabilities(context.Background()); err != nil {
        return nil, fmt.Errorf("failed to discover capabilities: %w", err)
    }

    return p, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
    return "api"
}

// ProviderType returns the provider type
func (p *Provider) ProviderType() string {
    return "api"
}

// discoverCapabilities queries remote Hermes for supported features
func (p *Provider) discoverCapabilities(ctx context.Context) error {
    url := fmt.Sprintf("%s/api/v2/capabilities", p.config.BaseURL)

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

    resp, err := p.client.Do(req)
    if err != nil {
        // If capabilities endpoint doesn't exist, assume full support
        p.capabilities = &Capabilities{
            SupportsContent:     true,
            SupportsPermissions: true,
            SupportsDirectory:   true,
            SupportsGroups:      true,
            SupportsEmail:       true,
            SupportsRevisions:   true,
        }
        return nil
    }
    defer resp.Body.Close()

    var caps Capabilities
    if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
        return err
    }

    p.capabilities = &caps
    return nil
}

// ===================================================================
// DocumentProvider Implementation
// ===================================================================

// GetDocument retrieves file metadata from remote Hermes
func (p *Provider) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
    url := fmt.Sprintf("%s/api/v2/documents/%s", p.config.BaseURL, providerID)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("API request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
    }

    var doc workspace.DocumentMetadata
    if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &doc, nil
}

// GetDocumentByUUID retrieves document by UUID
func (p *Provider) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
    url := fmt.Sprintf("%s/api/v2/documents/uuid/%s", p.config.BaseURL, uuid.String())

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var doc workspace.DocumentMetadata
    json.NewDecoder(resp.Body).Decode(&doc)
    return &doc, nil
}

// CopyDocument delegates to remote Hermes API
func (p *Provider) CopyDocument(ctx context.Context, srcID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
    url := fmt.Sprintf("%s/api/v2/documents/%s/copy", p.config.BaseURL, srcID)

    body, _ := json.Marshal(map[string]string{
        "destFolderID": destFolderID,
        "name":         name,
    })

    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var doc workspace.DocumentMetadata
    json.NewDecoder(resp.Body).Decode(&doc)
    return &doc, nil
}

// ... other DocumentProvider methods (Move, Delete, Rename, CreateFolder, etc.)

// ===================================================================
// ContentProvider Implementation
// ===================================================================

func (p *Provider) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
    if !p.capabilities.SupportsContent {
        return nil, fmt.Errorf("remote provider does not support content operations")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/content", p.config.BaseURL, providerID)

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var content workspace.DocumentContent
    json.NewDecoder(resp.Body).Decode(&content)
    return &content, nil
}

func (p *Provider) UpdateContent(ctx context.Context, providerID, content string) (*workspace.DocumentContent, error) {
    if !p.capabilities.SupportsContent {
        return nil, fmt.Errorf("remote provider does not support content operations")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/content", p.config.BaseURL, providerID)

    body, _ := json.Marshal(map[string]string{"content": content})
    req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var updatedContent workspace.DocumentContent
    json.NewDecoder(resp.Body).Decode(&updatedContent)
    return &updatedContent, nil
}

// ... other ContentProvider methods

// ===================================================================
// PermissionProvider Implementation
// ===================================================================

func (p *Provider) ShareDocument(ctx context.Context, providerID, email, role string) error {
    if !p.capabilities.SupportsPermissions {
        return fmt.Errorf("remote provider does not support permissions")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/permissions", p.config.BaseURL, providerID)

    body, _ := json.Marshal(map[string]string{
        "email": email,
        "role":  role,
    })

    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    return nil
}

func (p *Provider) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
    if !p.capabilities.SupportsPermissions {
        return nil, fmt.Errorf("remote provider does not support permissions")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/permissions", p.config.BaseURL, providerID)

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var perms []*workspace.FilePermission
    json.NewDecoder(resp.Body).Decode(&perms)
    return perms, nil
}

// ... other PermissionProvider methods

// ===================================================================
// PeopleProvider Implementation
// ===================================================================

func (p *Provider) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
    if !p.capabilities.SupportsDirectory {
        return nil, fmt.Errorf("remote provider does not support directory")
    }

    url := fmt.Sprintf("%s/api/v2/people/search?q=%s", p.config.BaseURL, url.QueryEscape(query))

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var people []*workspace.UserIdentity
    json.NewDecoder(resp.Body).Decode(&people)
    return people, nil
}

// ... other PeopleProvider, TeamProvider, NotificationProvider, RevisionTrackingProvider methods
```

### Configuration Patterns

**Pattern 1: Full Local Implementation** (Google Workspace):
```hcl
# Google implements all interfaces locally
providers {
  workspace = "google"
  search    = "algolia"
}

google_workspace {
  credentials_file = "credentials.json"
  domain          = "example.com"
  # All interfaces satisfied via Google APIs
}
```

**Pattern 2: Hybrid (Local + Delegation)**:
```hcl
# Local provider with delegation to remote for missing capabilities
providers {
  workspace = "local"
  search    = "meilisearch"
}

local_workspace {
  base_path = "/var/hermes/docs"
  docs_path = "docs"

  # Delegation configuration for interfaces not implemented locally
  delegate {
    # Delegate People, Team, Notification to remote Hermes
    people_provider       = "remote_api"
    team_provider         = "remote_api"
    notification_provider = "remote_api"

    # Remote API configuration
    remote_api {
      base_url   = "https://central.hermes.example.com"
      auth_token = env("HERMES_DELEGATION_TOKEN")
      timeout    = "30s"
    }
  }
}
```

**Pattern 3: Full Delegation** (Edge/Thin Client):
```hcl
# Edge Hermes that delegates everything to central instance
providers {
  workspace = "api"
  search    = "api"
}

# API workspace provider config
api_workspace {
  base_url   = "https://central.hermes.example.com"
  auth_token = env("HERMES_API_TOKEN")
  tls_verify = true
  timeout    = "30s"

  # All 7 interfaces delegated to remote
}

# API search provider config
api_search {
  base_url   = "https://central.hermes.example.com"
  auth_token = env("HERMES_API_TOKEN")
}
```

**Pattern 4: GitHub with Notification Delegation**:
```hcl
providers {
  workspace = "github"
  search    = "meilisearch"
}

github_workspace {
  token        = env("GITHUB_TOKEN")
  organization = "hashicorp"
  repository   = "rfcs"

  # GitHub can't send arbitrary emails, delegate to remote
  delegate {
    notification_provider = "remote_api"

    remote_api {
      base_url   = "https://hermes.example.com"
      auth_token = env("HERMES_DELEGATION_TOKEN")
    }
  }
}
```

## API Contract Requirements

For the API provider to work, the remote Hermes instance must expose consistent REST APIs:

### Document Endpoints

- `GET /api/v2/documents/:id` - Get file metadata
- `GET /api/v2/documents/uuid/:uuid` - Get document by UUID
- `POST /api/v2/documents/:id/copy` - Copy document
- `PUT /api/v2/documents/:id/move` - Move document
- `DELETE /api/v2/documents/:id` - Delete document
- `PATCH /api/v2/documents/:id` - Rename/update metadata
- `POST /api/v2/folders` - Create folder
- `GET /api/v2/folders/:id/subfolders/:name` - Get subfolder

### Content Endpoints

- `GET /api/v2/documents/:id/content` - Get document content
- `PUT /api/v2/documents/:id/content` - Update document content
- `GET /api/v2/documents/batch/content` - Get multiple documents (batch)
- `POST /api/v2/documents/compare` - Compare content between revisions

### Permission Endpoints

- `GET /api/v2/documents/:id/permissions` - List permissions
- `POST /api/v2/documents/:id/permissions` - Add permission
- `DELETE /api/v2/documents/:id/permissions/:permId` - Remove permission
- `PATCH /api/v2/documents/:id/permissions/:permId` - Update permission
- `POST /api/v2/documents/:id/permissions/domain` - Share with domain

### People Endpoints

- `GET /api/v2/people/search?q=:query` - Search directory
- `GET /api/v2/people/:email` - Get person by email
- `GET /api/v2/people/unified/:id` - Get person by unified ID
- `POST /api/v2/people/resolve` - Resolve identity (alternate emails)

### Team Endpoints

- `GET /api/v2/teams?domain=:domain&q=:query` - List teams
- `GET /api/v2/teams/:id` - Get team details
- `GET /api/v2/teams/user/:email` - Get user's teams
- `GET /api/v2/teams/:id/members` - Get team members

### Notification Endpoints

- `POST /api/v2/notifications/email` - Send email
- `POST /api/v2/notifications/email/template` - Send email with template

### Revision Endpoints

- `GET /api/v2/documents/:id/revisions` - Get revision history
- `GET /api/v2/documents/:id/revisions/:revId` - Get specific revision
- `GET /api/v2/documents/:id/revisions/:revId/content` - Get revision content
- `POST /api/v2/documents/:id/revisions/:revId/keep` - Keep revision forever
- `GET /api/v2/documents/uuid/:uuid/revisions/all` - Get all backend revisions

### Capabilities Endpoint

- `GET /api/v2/capabilities` - Discover remote capabilities

**Response Format**:
All endpoints return Hermes-native types (not Google types):
```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "providerType": "api",
  "providerID": "remote-123",
  "name": "RFC-001.md",
  "mimeType": "text/markdown",
  "createdTime": "2025-01-15T10:30:00Z",
  "modifiedTime": "2025-01-16T14:20:00Z",
  "owner": {
    "email": "user@example.com",
    "displayName": "User Name"
  },
  "contentHash": "sha256:abc123...",
  "status": "canonical"
}
```

## Error Handling

### Network Failures

```go
func (p *Provider) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
    resp, err := p.client.Do(req)
    if err != nil {
        // Network error - log and return with context
        return nil, fmt.Errorf("failed to reach remote Hermes at %s: %w", p.config.BaseURL, err)
    }
    defer resp.Body.Close()

    // HTTP error - include status code
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("remote API returned status %d: %s", resp.StatusCode, string(body))
    }

    return &doc, nil
}
```

### Remote Timeout Handling

```go
// Configure timeout in client
p.client = &http.Client{
    Timeout: cfg.Timeout, // e.g., 30s
}

// Context-aware requests
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
```

### Graceful Degradation

```go
// If remote API is down, return clear error
func (p *Provider) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
    resp, err := p.client.Do(req)
    if err != nil {
        // Log error for monitoring
        log.Error("remote API unavailable", "error", err, "remote", p.config.BaseURL)

        // Return user-friendly error
        return nil, fmt.Errorf("directory search unavailable: remote Hermes instance is not responding")
    }
    // ...
}
```

## Performance Considerations

### Caching Strategies

```go
// Cache frequently accessed documents
type CachedAPIProvider struct {
    *Provider
    cache *ttlcache.Cache
}

func (p *CachedAPIProvider) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
    // Check cache first
    if cached, ok := p.cache.Get(providerID); ok {
        return cached.(*workspace.DocumentMetadata), nil
    }

    // Fetch from remote
    doc, err := p.Provider.GetDocument(ctx, providerID)
    if err != nil {
        return nil, err
    }

    // Cache for 5 minutes
    p.cache.Set(providerID, doc, 5*time.Minute)
    return doc, nil
}
```

### Batch Operations

```go
// Batch endpoint for efficient multi-document fetch
func (p *Provider) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
    url := fmt.Sprintf("%s/api/v2/documents/batch/content", p.config.BaseURL)

    body, _ := json.Marshal(map[string][]string{
        "providerIDs": providerIDs,
    })

    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var contents []*workspace.DocumentContent
    json.NewDecoder(resp.Body).Decode(&contents)
    return contents, nil
}
```

### Connection Pooling

```go
// Configure HTTP client with connection pooling
func NewProvider(cfg *Config) (*Provider, error) {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    }

    p := &Provider{
        config: cfg,
        client: &http.Client{
            Timeout:   cfg.Timeout,
            Transport: transport,
        },
    }

    return p, nil
}
```

## Implementation Plan

### Phase 1: API Provider Foundation (Week 4)

- [ ] Create `pkg/workspace/adapters/api/` package
- [ ] Implement configuration (`api_workspace` HCL block)
- [ ] Implement HTTP client with authentication
- [ ] Add retry logic and circuit breakers
- [ ] Implement basic file operations (Get, Copy, Move, Delete)

### Phase 2: Complete API Provider (Week 5)

- [ ] Implement content operations
- [ ] Implement permission operations
- [ ] Implement directory/people operations
- [ ] Implement folder operations
- [ ] Add comprehensive error handling

### Phase 3: REST API Endpoints (Week 6)

- [ ] Create new API v2 endpoints for all operations
- [ ] Add authentication middleware (see RFC-086)
- [ ] Add rate limiting
- [ ] Comprehensive API documentation

### Testing

**Integration Tests**:
- Integration tests with local Hermes → local Hermes
- Integration tests with API provider → Google provider
- Performance testing (latency, throughput)
- Failure mode testing (network errors, auth failures)

## Success Metrics

- [ ] API provider successfully delegates to remote Hermes
- [ ] End-to-end document lifecycle works through API provider
- [ ] < 100ms additional latency for API provider operations
- [ ] Successful multi-tier deployment in staging environment
- [ ] 100% feature parity with local providers

## Risks & Mitigations

### Risk 1: Performance Degradation

**Risk**: Additional network hop adds latency

**Mitigation**:
- Implement response caching for read-heavy operations
- Add batch API endpoints for bulk operations
- Use HTTP/2 for connection multiplexing
- Add performance monitoring and alerting

### Risk 2: Network Reliability

**Risk**: Network failures disrupt operations

**Mitigation**:
- Implement retry logic with exponential backoff
- Add circuit breaker pattern
- Graceful degradation with clear error messages
- Monitor remote API health

### Risk 3: API Versioning

**Risk**: API contract changes break compatibility

**Mitigation**:
- Semantic versioning for API endpoints (`/api/v2/`, `/api/v3/`)
- Support multiple API versions concurrently
- Capabilities endpoint for feature discovery
- Automated compatibility testing

## References

- **RFC-084**: Provider Interface Refactoring (defines interfaces)
- **RFC-086**: Authentication and Bearer Token Management (auth strategy)
- **RFC-082**: Document Identification System (UUID + ProviderID)
- **Implementation**: `pkg/workspace/adapters/api/`

## Timeline

- **Week 4**: API provider foundation
- **Week 5**: Complete all 7 interfaces
- **Week 6**: REST API endpoints
- **Total**: 3 weeks to production-ready API provider

---

**Status**: Proposed
**Dependencies**: RFC-084 (interfaces), RFC-086 (authentication)
**Next Steps**: Begin Phase 1 implementation after RFC-084 and RFC-086 are approved
