---
title: "RFC-001: Multi-Provider Architecture"
document_type: "RFC"
document_number: "RFC-001"
status: "approved"
summary: "Proposes a modular provider architecture enabling swappable backends for authentication, workspace storage, and search functionality."
authors:
  - name: "Engineering Team"
    email: "eng@hermes.local"
approvers:
  - name: "Architecture Lead"
    email: "arch@hermes.local"
    approved_at: "2025-11-10T12:00:00Z"
created_at: "2025-11-01T10:00:00Z"
modified_at: "2025-11-10T12:00:00Z"
tags:
  - architecture
  - providers
  - flexibility
related_documents:
  - "ADR-073: Provider Abstraction Architecture"
  - "RFC-080: Outbox Pattern Document Sync"
---

# RFC-001: Multi-Provider Architecture

## Summary

This RFC proposes implementing a modular provider architecture that decouples Hermes' core functionality from specific backend implementations. This enables organizations to:

1. **Swap authentication providers** (Dex, Google OAuth, Okta) via configuration
2. **Choose workspace backends** (local filesystem, Google Workspace, future: Office365)
3. **Select search engines** (Meilisearch self-hosted, Algolia managed)

All without changing application code.

## Background

### Current Pain Points

1. **Cloud Dependency**: Initial versions required Google Workspace for all operations
2. **Testing Friction**: E2E tests needed production credentials, slowing development
3. **Vendor Lock-in**: Single workspace provider limited deployment options
4. **Cost**: Cloud API calls during every development cycle

### Business Impact

- **Developer velocity**: Hours spent on OAuth setup for local testing
- **Testing costs**: $50+/month per developer for cloud dev accounts
- **Deployment restrictions**: Cannot deploy to air-gapped environments
- **Feature development**: Blocked on external service availability

## Proposal

### Architecture

Introduce provider abstraction layer:

```
┌─────────────────────────────────────┐
│     Core Application Logic          │
│  (Document lifecycle, workflows)    │
└──────────┬──────────┬───────────────┘
           │          │
    ┌──────▼────┐  ┌──▼──────┐  ┌────▼────┐
    │Auth       │  │Workspace│  │Search   │
    │Interface  │  │Interface│  │Interface│
    └──────┬────┘  └──┬──────┘  └────┬────┘
           │          │              │
       ┌───┴───┬──────┴─────┬────────┴────┐
       │  Dex  │  Local     │ Meilisearch │
       │Google │  Google WS │  Algolia    │
       │ Okta  │  Office365 │             │
       └───────┴────────────┴─────────────┘
```

### Provider Interfaces

**Authentication Provider**:
```go
type AuthProvider interface {
    ValidateToken(ctx context.Context, token string) (*UserInfo, error)
    GetAuthURL(state string) string
    ExchangeCode(ctx context.Context, code string) (*Tokens, error)
}
```

**Workspace Provider**:
```go
type WorkspaceProvider interface {
    CreateDocument(ctx context.Context, doc *Document) error
    GetDocument(ctx context.Context, id string) (*Document, error)
    UpdateDocument(ctx context.Context, doc *Document) error
    DeleteDocument(ctx context.Context, id string) error
    ListDocuments(ctx context.Context, opts ListOptions) ([]*Document, error)
}
```

**Search Provider**:
```go
type SearchProvider interface {
    IndexDocument(ctx context.Context, doc *Document) error
    Search(ctx context.Context, query SearchQuery) (*SearchResults, error)
    DeleteDocument(ctx context.Context, id string) error
    UpdateDocument(ctx context.Context, doc *Document) error
}
```

### Configuration

Runtime provider selection via HCL configuration:

```hcl
# Local development
providers {
  auth      = "dex"
  workspace = "local"
  search    = "meilisearch"
}

# Production
providers {
  auth      = "google"
  workspace = "google"
  search    = "algolia"
}
```

## Implementation

### Phase 1: Core Abstraction (✅ Complete)

1. Define provider interfaces
2. Implement local providers (Dex, local workspace, Meilisearch)
3. Update core application to use interfaces
4. Add provider factory/registry

**Deliverables**:
- `pkg/auth/provider.go` - Auth interface
- `pkg/workspace/provider.go` - Workspace interface
- `pkg/search/provider.go` - Search interface
- `pkg/providers/registry.go` - Provider factory

### Phase 2: Google Providers (✅ Complete)

1. Implement Google OAuth adapter
2. Implement Google Workspace adapter
3. Implement Algolia adapter
4. Add provider-specific configuration

**Deliverables**:
- `pkg/auth/adapters/google/` - Google OAuth implementation
- `pkg/workspace/adapters/google/` - Google Workspace implementation
- `pkg/search/adapters/algolia/` - Algolia implementation

### Phase 3: Testing Infrastructure (✅ Complete)

1. Docker Compose for local services
2. Playwright E2E test suite
3. Provider-agnostic test fixtures
4. Performance benchmarks

**Deliverables**:
- `testing/docker-compose.yml` - Local environment
- `tests/e2e-playwright/` - E2E test suite
- `testing/README.md` - Testing documentation

### Phase 4: Migration Pipeline (✅ Complete)

1. Document UUID system (provider-agnostic identity)
2. Cross-provider migration tool
3. Metadata preservation logic
4. Idempotent sync operations

**Deliverables**:
- `cmd/hermes-migrate/` - Migration CLI tool
- `pkg/migration/` - Migration library
- RFC-080 - Outbox pattern design

### Phase 5: Office365 Provider (🚧 Planned - Q1 2025)

1. SharePoint adapter for workspace
2. Azure AD adapter for auth
3. Parity testing with Google providers
4. Documentation and examples

**Deliverables**:
- `pkg/workspace/adapters/office365/` - SharePoint implementation
- `pkg/auth/adapters/azuread/` - Azure AD implementation
- Updated documentation

## Benefits

### Development Velocity

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| **Local Setup Time** | 2-3 hours | 5 minutes | **96% faster** |
| **Test Execution** | 2-3 minutes | ~30 seconds | **85% faster** |
| **Cost/Developer** | $50+/month | $0/month | **100% savings** |
| **Offline Support** | ❌ None | ✅ Full | **New capability** |

### Production Flexibility

1. **Multi-cloud**: Deploy with Google or Microsoft backends
2. **Self-hosted**: Run entirely on-premise with local + Meilisearch
3. **Hybrid**: Mix providers (e.g., Okta auth + Google Workspace)
4. **Air-gapped**: Local provider for secure/classified deployments

### Risk Mitigation

1. **Vendor lock-in**: Not dependent on single cloud provider
2. **Cost control**: Switch to self-hosted when needed
3. **Compliance**: Deploy in restricted environments
4. **Continuity**: Operate during cloud service outages

## Drawbacks & Mitigation

### Performance Overhead

**Concern**: Abstraction layer adds latency
**Reality**: <5ms overhead (interface method calls)
**Mitigation**: Negligible compared to network latency (50-200ms for cloud APIs)

### Implementation Complexity

**Concern**: More code to maintain
**Reality**: Well-defined interfaces, isolated adapters
**Mitigation**: Each provider ~500-1000 LOC, heavily tested

### Feature Parity

**Concern**: Providers may have different capabilities
**Reality**: True, but manageable
**Mitigation**:
- Capability negotiation (query provider features)
- Graceful degradation (fallback for missing features)
- Clear documentation of provider differences

## Alternatives Considered

### 1. Single Provider (Status Quo)

**Pros**: Simpler implementation
**Cons**: Vendor lock-in, no local testing, cloud dependency
**Verdict**: Rejected - too limiting

### 2. Plugin Architecture

**Pros**: More flexible, third-party extensions
**Cons**: Complex loading, security risks, runtime instability
**Verdict**: Rejected - overkill for our needs

### 3. Microservices per Provider

**Pros**: Isolation, independent deployment
**Cons**: Operational complexity, higher latency
**Verdict**: Rejected - too heavyweight

## Success Metrics

### Adoption

- ✅ 100% of developers using local environment
- ✅ Zero cloud credentials needed for development
- ✅ E2E test suite runs in CI with local providers

### Performance

- ✅ <5ms provider abstraction overhead
- ✅ <30 seconds for full E2E test suite
- ✅ 100+ documents indexed in <1 second

### Flexibility

- ✅ 3 auth providers (Dex, Google, Okta)
- ✅ 2 workspace providers (local, Google)
- ✅ 2 search providers (Meilisearch, Algolia)
- 🚧 Office365 workspace provider (Q1 2025)

## Timeline

- **Q4 2024**: ✅ Core abstraction, local providers, testing infrastructure
- **Q1 2025**: 🚧 Office365 provider, enhanced migration tools
- **Q2 2025**: 🔮 Additional providers as needed (Azure AD, Confluence, etc.)

## References

- [ADR-071: Local File Workspace System](../../docs-internal/adr/ADR-071-local-file-workspace-system.md)
- [ADR-072: Dex OIDC Authentication](../../docs-internal/adr/ADR-072-dex-oidc-authentication-for-development.md)
- [ADR-073: Provider Abstraction Architecture](../../docs-internal/adr/ADR-073-provider-abstraction-architecture.md)
- [ADR-075: Meilisearch as Local Search Solution](../../docs-internal/adr/ADR-075-meilisearch-as-local-search-solution.md)
- [RFC-080: Outbox Pattern Document Sync](../../docs-internal/rfc/RFC-080-outbox-pattern-document-sync.md)

## Appendix A: Provider Comparison

| Provider Type | Local | Google | Office365 (Planned) |
|---------------|-------|--------|---------------------|
| **Auth: Dex** | ✅ Dev only | ❌ | ❌ |
| **Auth: Google** | ❌ | ✅ Production | ❌ |
| **Auth: Okta** | ✅ Enterprise | ✅ Enterprise | ✅ Enterprise |
| **Auth: Azure AD** | ❌ | ❌ | ✅ Production |
| **Workspace: Local** | ✅ Markdown | ❌ | ❌ |
| **Workspace: Google** | ❌ | ✅ Google Docs | ❌ |
| **Workspace: Office365** | ❌ | ❌ | ✅ SharePoint |
| **Search: Meilisearch** | ✅ Self-hosted | ✅ Self-hosted | ✅ Self-hosted |
| **Search: Algolia** | ✅ Managed | ✅ Managed | ✅ Managed |

## Appendix B: Migration Example

Migrate documents from Google Workspace to local filesystem:

```bash
# Export from Google
./hermes migrate export \
  --source google \
  --output ./export/

# Import to local
./hermes migrate import \
  --target local \
  --input ./export/

# Or direct migration
./hermes migrate \
  --source google \
  --target local \
  --preserve-metadata
```

Result:
- All documents copied to `workspace_data/`
- Metadata preserved (authors, approvers, status)
- UUIDs maintained (same document identity)
- Versions tracked (full revision history)
