# RFC-084 Split Proposal

**Current State**: RFC-084 is 2,192 lines and covers three major topics
**Proposed State**: Split into 3 focused RFCs

## Proposed RFC Structure

### RFC-084: Provider Interface Refactoring (Core Architecture)
**Status**: Keep and refine
**Size**: ~800-1000 lines
**Focus**: Provider interface design and multi-backend document model

**Contents**:
1. **Context**: Current document model (RFC-082 foundation), multi-backend reality
2. **Type Definitions**:
   - DocumentMetadata (UUID + ProviderID + ContentHash)
   - BackendRevision (provider-specific revision tracking)
   - DocumentContent (with backend revision info)
   - UserIdentity (unified identity across providers)
   - Team, FilePermission, RevisionInfo
3. **7 Focused Provider Interfaces**:
   - DocumentProvider (CRUD with UUID support)
   - ContentProvider (content with revision tracking)
   - RevisionTrackingProvider (backend-specific + cross-backend)
   - PermissionProvider (file sharing)
   - PeopleProvider (user directory + identity resolution)
   - TeamProvider (group operations)
   - NotificationProvider (email/notifications)
4. **Required Interfaces Architecture**:
   - All interfaces REQUIRED
   - Satisfied locally or via delegation
   - Implementation patterns (full local, hybrid, full delegation)
5. **Provider Implementation Matrix**
6. **Design Decisions**:
   - Interface naming rationale
   - Required vs optional decision
   - Type safety over Google types
   - Multi-backend tracking

**Dependencies**: RFC-082 (DocID system)
**Timeline**: Phase 1 implementation (Weeks 1-3)

---

### RFC-085: API Provider and Remote Delegation (NEW)
**Status**: Extract from RFC-084
**Size**: ~600-800 lines
**Focus**: Implementing providers that delegate to remote Hermes

**Contents**:
1. **API Provider Architecture**:
   - Remote delegation patterns
   - Capability discovery (`/api/v2/capabilities`)
   - HTTP client implementation
2. **Implementation Patterns**:
   - Pattern 1: Full local (Google Workspace)
   - Pattern 2: Hybrid (Local + delegation)
   - Pattern 3: Full delegation (API provider)
   - Pattern 4: Mostly local (GitHub + one delegated)
3. **Configuration Examples**:
   - Full local configuration
   - Hybrid with delegation block
   - Full delegation (edge/thin client)
   - GitHub with notification delegation
4. **API Contract Requirements**:
   - Document endpoints
   - Content endpoints
   - Permission endpoints
   - People endpoints
   - Team endpoints
   - Notification endpoints
   - Response format (Hermes-native types)
5. **Error Handling**:
   - Network failures
   - Remote timeout handling
   - Graceful degradation
6. **Performance Considerations**:
   - Caching strategies
   - Batch operations
   - Connection pooling

**Dependencies**: RFC-084 (provider interfaces)
**Timeline**: Phase 2 implementation (Weeks 4-6)

---

### RFC-086: Authentication and Bearer Token Management (NEW)
**Status**: Extract from RFC-084
**Size**: ~500-700 lines
**Focus**: Authentication for delegated operations

**Contents**:
1. **Authentication Challenge**:
   - Local Hermes needs authenticated access to remote
   - User authentication flow
   - Token validation requirements
2. **Authentication Strategies**:
   - Strategy 1: Shared OIDC Provider (recommended)
   - Strategy 2: Remote OIDC Discovery
   - Strategy 3: Machine-to-Machine API Key
3. **Discovery Flow**:
   - `GET /api/v2/auth/config` endpoint
   - AuthConfigResponse format
   - Dynamic configuration on startup
4. **Token Proxying Implementation**:
   - Handler extracts user bearer token
   - Context-based token passing
   - RemoteAPIClient token forwarding
   - Fallback to API key for background ops
5. **Security Considerations**:
   - Token validation (OIDC provider)
   - Token scope requirements
   - CORS configuration
   - Token storage (HttpOnly cookies)
   - Refresh token management
   - Token expiration enforcement
6. **Configuration Examples**:
   - Shared OIDC configuration
   - Discovery-based configuration
   - M2M API key configuration
7. **Authentication Endpoints**:
   - `GET /api/v2/auth/config` - Discover OIDC config
   - `POST /api/v2/auth/validate` - Validate token (debugging)

**Dependencies**: RFC-084, RFC-007 (Multi-Provider Auth)
**Timeline**: Phase 2 implementation (Week 5-6)

---

## Migration Plan

### Step 1: Refine RFC-084
- Remove API provider implementation details → RFC-085
- Remove authentication section → RFC-086
- Keep provider interface design, types, and architecture
- Add references to RFC-085 and RFC-086

### Step 2: Create RFC-085
- Extract API provider implementation from RFC-084
- Add API contract details
- Add configuration patterns
- Add error handling and performance sections

### Step 3: Create RFC-086
- Extract authentication section from RFC-084
- Expand security considerations
- Add more detailed discovery flow
- Add troubleshooting section

### Step 4: Update Cross-References
- RFC-084: Add "Related: RFC-085, RFC-086" in header
- RFC-085: Add "Related: RFC-084 (interfaces), RFC-086 (auth)" in header
- RFC-086: Add "Related: RFC-084 (interfaces), RFC-085 (API provider), RFC-007 (auth architecture)" in header

---

## Benefits of Split

1. **Focused Review**: Each RFC covers one major topic
2. **Independent Implementation**: Can implement provider refactoring without API provider
3. **Easier Navigation**: Find specific information faster
4. **Clearer Dependencies**: Explicit dependencies between RFCs
5. **Incremental Adoption**: Teams can adopt provider refactoring first, API provider later
6. **Better Documentation**: Each RFC is comprehensive for its topic
7. **Simpler Updates**: Update auth strategy without touching interface design

---

## Implementation Order

**Phase 1** (Weeks 1-3): RFC-084 - Provider Interface Refactoring
- Define Hermes-native types
- Create 7 focused interfaces
- Update existing providers (Google, Local)
- Migrate API handlers to new interfaces

**Phase 2** (Weeks 4-6): RFC-085 + RFC-086 - API Provider + Auth
- Implement API provider
- Implement authentication discovery
- Implement token proxying
- Create REST API endpoints

**Phase 3** (Week 7): Integration Testing
- Test hybrid deployments (local + delegation)
- Test full delegation (edge nodes)
- Test authentication flows
- Performance testing

---

## Proposed File Structure

```
docs-internal/rfc/
├── RFC-084-provider-interface-refactoring.md   (Core: interfaces, types, architecture)
├── RFC-085-api-provider-remote-delegation.md   (Implementation: API provider)
└── RFC-086-authentication-bearer-tokens.md     (Auth: OIDC, token proxying)
```

## Next Steps

1. Review and approve split proposal
2. Refactor RFC-084 (remove API provider and auth sections)
3. Create RFC-085 from extracted API provider content
4. Create RFC-086 from extracted authentication content
5. Update cross-references
6. Update implementation plan in each RFC
