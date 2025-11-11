---
id: RFC-084
title: Provider Interface Refactoring - Multi-Backend Document Model
date: 2025-11-11
type: RFC
subtype: Architecture
status: Proposed
tags: [providers, architecture, interfaces, multi-backend, types]
related:
  - RFC-082
  - RFC-085
  - RFC-086
  - MEMO-091
---

# Provider Interface Refactoring - Multi-Backend Document Model

## Executive Summary

This RFC proposes a refactoring of the Hermes provider architecture to support multi-backend document tracking through focused provider interfaces and Hermes-native types. This is the foundation for enabling API providers (RFC-085) and authentication delegation (RFC-086).

**Key Benefits**:
- Type-safe provider interfaces using Hermes-native types (not Google types)
- 7 focused interfaces supporting multi-backend document tracking
- All interfaces REQUIRED (satisfied locally or via delegation)
- UUID-based document identification across multiple backends
- Backend-specific revision tracking (Google revs, Git commits, O365 versions)

**Related RFCs**:
- **RFC-085**: API Provider and Remote Delegation (implementation patterns)
- **RFC-086**: Authentication and Bearer Token Management (auth strategy)

## Context

### Current Document Model (RFC-082 Foundation)

Hermes uses a UUID-based document identification system where documents can exist across multiple backends:

**Core Concepts**:
- **UUID**: Stable global identifier (`550e8400-e29b-41d4-a716-446655440000`)
- **ProviderID**: Backend-specific identifier (`google:1a2b3c4d`, `local:docs/rfc.md`, `github:owner/repo/path@commit`)
- **Multi-Backend Tracking**: Same document UUID can have multiple active revisions across different backends

**Example - Document Across Multiple Backends**:
```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
Title: "RFC-001: API Gateway Design"

┌─────────────────────────────────────────────────────────────┐
│ Revision 1: Google Workspace (Source of Truth)             │
├─────────────────────────────────────────────────────────────┤
│ ProviderID: google:1a2b3c4d5e6f7890                         │
│ Backend Revision: Google Doc revision v123                  │
│ Content Hash: sha256:abc123...                              │
│ Last Modified: 2025-10-15T14:30:00Z                         │
│ Status: canonical                                            │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Revision 2: Local Git (Migrated Copy)                       │
├─────────────────────────────────────────────────────────────┤
│ ProviderID: local:docs/rfc-001.md                           │
│ Backend Revision: Git commit a1b2c3d4e5f6                   │
│ Content Hash: sha256:abc123...  ✅ matches Google           │
│ Last Modified: 2025-10-01T09:00:00Z                         │
│ Status: target                                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Revision 3: Office 365 (Mirror for Collaboration)          │
├─────────────────────────────────────────────────────────────┤
│ ProviderID: office365:ABC-DEF-123-456                       │
│ Backend Revision: O365 version 2.1                          │
│ Content Hash: sha256:def456...  ⚠️ drift detected          │
│ Last Modified: 2025-10-20T11:15:00Z                         │
│ Status: conflict                                             │
└─────────────────────────────────────────────────────────────┘
```

**Provider Interface Implications**:
1. Providers must work with DocID (UUID + ProviderID) system
2. Each backend has its own revision tracking mechanism
3. Content operations need to return backend-specific revision info
4. Providers need to support conflict detection across backends

### Current Provider Architecture

Hermes currently supports three types of providers, each with direct backend integrations:

```
┌─────────────────────────────────────────────────────────┐
│                   Hermes Server                         │
├─────────────────────────────────────────────────────────┤
│  Provider System (internal/config/config.go)            │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Auth         │  │ Workspace    │  │ Search       │  │
│  │ Providers    │  │ Providers    │  │ Providers    │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  │
│         │                  │                  │          │
└─────────┼──────────────────┼──────────────────┼──────────┘
          │                  │                  │
    ┌─────┴─────┐      ┌────┴─────┐      ┌─────┴──────┐
    │ - Dex     │      │ - Google │      │ - Algolia  │
    │ - Okta    │      │ - Local  │      │ - Meili    │
    │ - Google  │      │          │      │            │
    └───────────┘      └──────────┘      └────────────┘
       (OIDC)          (Direct)          (Direct)
```

**Current Workspace Providers** (`pkg/workspace/provider.go`):
- **Google Workspace**: Direct integration via Google Drive/Docs APIs
- **Local**: Direct filesystem access for markdown-based documents

**Provider Interface Characteristics**:
- ~30 methods covering file operations, permissions, content, email, groups
- Returns Google Drive/Docs types (`*drive.File`, `*docs.Document`)
- Assumes direct backend access (no network proxy/delegation pattern)

### The Problem

The current architecture has several limitations:

1. **Tight Coupling to Backend Types**: The `workspace.Provider` interface returns Google-specific types (`*drive.File`, `*people.Person`), making it difficult to support non-Google backends or proxy operations through a different API layer.

2. **No Multi-Backend Tracking**: Cannot track document revisions across multiple providers (Google + Git + O365) for migration and conflict detection.

3. **No Remote Delegation**: Providers must directly access their backends. There's no way for one Hermes instance to delegate operations to another Hermes instance.

4. **Limited Identity Unification**: No support for linking user identities across providers (jacob.repp@hashicorp.com = jrepp@ibm.com).

## Proposed Solution

### Type Abstraction - Hermes-Native Types

Introduce Hermes-native types that support multi-backend document tracking:

#### 1. DocumentMetadata

```go
// DocumentMetadata represents provider-agnostic document metadata
// Works with DocID system (UUID + ProviderID)
type DocumentMetadata struct {
    // Global identifier (RFC-082)
    UUID docid.UUID `json:"uuid"`

    // Backend-specific identifier
    ProviderType string `json:"providerType"` // "google", "local", "office365", "github"
    ProviderID   string `json:"providerID"`   // Backend-specific ID

    // Metadata
    Name         string    `json:"name"`
    MimeType     string    `json:"mimeType"`
    CreatedTime  time.Time `json:"createdTime"`
    ModifiedTime time.Time `json:"modifiedTime"`

    // Ownership (unified identity aware)
    Owner        *UserIdentity   `json:"owner"`
    Contributors []UserIdentity  `json:"contributors,omitempty"`

    // Hierarchy
    Parents      []string `json:"parents,omitempty"`
    Project      string   `json:"project,omitempty"`

    // Multi-backend tracking
    ContentHash  string `json:"contentHash"` // SHA-256 for drift detection
    Status       string `json:"status"`      // "canonical", "mirror", "conflict", "archived"
}
```

#### 2. DocumentContent with Backend Revision

```go
// DocumentContent represents document content with backend-specific revision info
type DocumentContent struct {
    // Document identification
    UUID       docid.UUID `json:"uuid"`
    ProviderID string     `json:"providerID"`

    // Content
    Title  string `json:"title"`
    Body   string `json:"body"`
    Format string `json:"format"` // "markdown", "html", "plain", "richtext"

    // Backend-specific revision information
    BackendRevision *BackendRevision `json:"backendRevision"`

    // Content tracking
    ContentHash  string    `json:"contentHash"` // SHA-256
    LastModified time.Time `json:"lastModified"`
}

// BackendRevision captures backend-specific revision metadata
type BackendRevision struct {
    ProviderType string `json:"providerType"`

    // Backend-specific revision ID (varies by provider)
    RevisionID string `json:"revisionID"` // Examples:
    //   Google: "123" (Drive revision number)
    //   Git: "a1b2c3d4e5f6" (commit SHA)
    //   Office365: "2.1" (version number)
    //   GitHub: "e7f8g9h0i1j2" (commit SHA)

    // Revision metadata
    ModifiedTime time.Time      `json:"modifiedTime"`
    ModifiedBy   *UserIdentity  `json:"modifiedBy,omitempty"`
    Comment      string         `json:"comment,omitempty"`
    KeepForever  bool           `json:"keepForever,omitempty"`

    // Backend-specific metadata (flexible for different systems)
    Metadata     map[string]any `json:"metadata,omitempty"`
}
```

#### 3. UserIdentity (Unified Identity)

```go
// UserIdentity represents a unified user identity across multiple auth providers
// Addresses the requirement: jacob.repp@hashicorp.com = jrepp@ibm.com (same person)
type UserIdentity struct {
    // Primary identifier (canonical email)
    Email       string `json:"email"`
    DisplayName string `json:"displayName"`
    PhotoURL    string `json:"photoURL,omitempty"`

    // Unified identity tracking
    UnifiedUserID string `json:"unifiedUserId,omitempty"` // Links identities across providers

    // Provider-specific identities (same person, multiple providers)
    AlternateEmails []AlternateIdentity `json:"alternateEmails,omitempty"`
}

// AlternateIdentity represents the same user in a different identity provider
type AlternateIdentity struct {
    Email          string `json:"email"`        // e.g., "jrepp@ibm.com"
    Provider       string `json:"provider"`     // e.g., "ibm-verify", "google-workspace"
    ProviderUserID string `json:"providerUserId,omitempty"`
}
```

#### 4. Supporting Types

```go
// FilePermission represents file access permissions
type FilePermission struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    Role  string `json:"role"` // "owner", "writer", "reader"
    Type  string `json:"type"` // "user", "group", "domain", "anyone"

    // Identity tracking
    User *UserIdentity `json:"user,omitempty"`
}

// Team represents a group/team (renamed from Group to avoid confusion)
type Team struct {
    ID          string `json:"id"`
    Email       string `json:"email,omitempty"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    MemberCount int    `json:"memberCount"`

    // Provider-specific
    ProviderType string `json:"providerType"`
    ProviderID   string `json:"providerID"`
}

// RevisionInfo represents a document revision for conflict detection
type RevisionInfo struct {
    UUID            docid.UUID       `json:"uuid"`
    ProviderType    string           `json:"providerType"`
    ProviderID      string           `json:"providerID"`
    BackendRevision *BackendRevision `json:"backendRevision"`
    ContentHash     string           `json:"contentHash"`
    Status          string           `json:"status"` // "canonical", "mirror", "conflict"
}

// ContentComparison represents a content comparison result
type ContentComparison struct {
    UUID           docid.UUID
    Revision1      *BackendRevision
    Revision2      *BackendRevision
    ContentMatch   bool   // True if content hashes match
    HashDifference string // "same", "minor", "major"
}
```

### Focused Provider Interfaces

Split the monolithic `Provider` interface into **7 focused interfaces** that can be composed:

```go
// ===================================================================
// CORE INTERFACE: DocumentProvider
// ===================================================================
// DocumentProvider handles document metadata operations (CRUD)
// Works with DocID system (UUID + ProviderID)
//
// NOTE: Renamed from "FileProvider" to avoid confusion with file system directories
type DocumentProvider interface {
    // GetDocument retrieves document metadata by backend-specific ID
    // Returns: DocumentMetadata with UUID, ProviderID, status, content hash
    GetDocument(ctx context.Context, providerID string) (*DocumentMetadata, error)

    // GetDocumentByUUID retrieves document metadata by UUID
    // Useful when UUID is known but provider ID is not
    GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*DocumentMetadata, error)

    // CreateDocument creates a new document from template
    // Returns: DocumentMetadata with newly generated UUID
    CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*DocumentMetadata, error)

    // CreateDocumentWithUUID creates document with explicit UUID (for migration)
    CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*DocumentMetadata, error)

    // CopyDocument copies a document (preserves UUID if in frontmatter/metadata)
    CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*DocumentMetadata, error)

    // MoveDocument moves a document to different folder
    MoveDocument(ctx context.Context, providerID, destFolderID string) (*DocumentMetadata, error)

    // DeleteDocument deletes a document
    DeleteDocument(ctx context.Context, providerID string) error

    // RenameDocument renames a document
    RenameDocument(ctx context.Context, providerID, newName string) error

    // CreateFolder creates a folder/directory
    CreateFolder(ctx context.Context, name, parentID string) (*DocumentMetadata, error)

    // GetSubfolder finds a subfolder by name
    GetSubfolder(ctx context.Context, parentID, name string) (string, error)
}

// ===================================================================
// CORE INTERFACE: ContentProvider
// ===================================================================
// ContentProvider handles document content operations with revision tracking
//
// CRITICAL: Content operations must return BackendRevision info for
// multi-backend conflict detection (e.g., Google Doc v123 vs Git commit abc)
type ContentProvider interface {
    // GetContent retrieves document content with backend-specific revision
    // Returns: DocumentContent with BackendRevision (Google rev, Git commit, etc.)
    GetContent(ctx context.Context, providerID string) (*DocumentContent, error)

    // GetContentByUUID retrieves content using UUID (looks up providerID)
    GetContentByUUID(ctx context.Context, uuid docid.UUID) (*DocumentContent, error)

    // UpdateContent updates document content
    // Returns: Updated DocumentContent with new BackendRevision and content hash
    UpdateContent(ctx context.Context, providerID string, content string) (*DocumentContent, error)

    // GetContentBatch retrieves multiple documents (efficient for migration)
    GetContentBatch(ctx context.Context, providerIDs []string) ([]*DocumentContent, error)

    // CompareContent compares content between two revisions
    // Used for conflict detection during migration
    CompareContent(ctx context.Context, providerID1, providerID2 string) (*ContentComparison, error)
}

// ===================================================================
// CORE INTERFACE: RevisionTrackingProvider
// ===================================================================
// RevisionTrackingProvider handles backend-specific revision operations
//
// NOTE: Renamed from "RevisionProvider" to emphasize backend-specific tracking
// Each backend (Google, Git, O365, GitHub) has its own revision system
type RevisionTrackingProvider interface {
    // GetRevisionHistory lists all revisions for a document in this backend
    // Returns: List of BackendRevision ordered by time (newest first)
    GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*BackendRevision, error)

    // GetRevision retrieves a specific revision
    GetRevision(ctx context.Context, providerID, revisionID string) (*BackendRevision, error)

    // GetRevisionContent retrieves content at a specific revision
    GetRevisionContent(ctx context.Context, providerID, revisionID string) (*DocumentContent, error)

    // KeepRevisionForever marks a revision as permanent (if supported)
    KeepRevisionForever(ctx context.Context, providerID, revisionID string) error

    // GetAllDocumentRevisions returns all revisions across all backends for a UUID
    // This is CRITICAL for multi-backend tracking:
    //   - Returns Google Doc revisions (if exists in Google)
    //   - Returns Git commits (if exists in Git)
    //   - Returns O365 versions (if exists in O365)
    // Used for conflict detection and migration status
    GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*RevisionInfo, error)
}

// ===================================================================
// REQUIRED INTERFACE: PermissionProvider
// ===================================================================
// PermissionProvider handles file sharing and access control
// NOTE: ALL providers must implement this - either locally or via delegation
type PermissionProvider interface {
    // ShareDocument grants access to a user/group
    ShareDocument(ctx context.Context, providerID, email, role string) error

    // ShareDocumentWithDomain grants access to entire domain
    ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error

    // ListPermissions lists all permissions for a document
    ListPermissions(ctx context.Context, providerID string) ([]*FilePermission, error)

    // RemovePermission revokes access
    RemovePermission(ctx context.Context, providerID, permissionID string) error

    // UpdatePermission changes permission role
    UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error
}

// ===================================================================
// REQUIRED INTERFACE: PeopleProvider
// ===================================================================
// PeopleProvider handles user directory operations
// NOTE: ALL providers must implement this - either locally or via delegation
//
// Renamed from "DirectoryProvider" to avoid confusion with file directories
// This is about PEOPLE/USERS, not file system directories
type PeopleProvider interface {
    // SearchPeople searches for users in the directory
    SearchPeople(ctx context.Context, query string) ([]*UserIdentity, error)

    // GetPerson retrieves a user by email
    GetPerson(ctx context.Context, email string) (*UserIdentity, error)

    // GetPersonByUnifiedID retrieves user by unified ID (cross-provider lookup)
    // Example: Look up person by unified ID, returns all their email addresses
    //   Input: unifiedID = "user-12345"
    //   Output: UserIdentity{
    //     Email: "jacob.repp@hashicorp.com",
    //     AlternateEmails: [{Email: "jrepp@ibm.com", Provider: "ibm-verify"}]
    //   }
    GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*UserIdentity, error)

    // ResolveIdentity resolves alternate identities for a user
    // Used for identity unification: jacob.repp@hashicorp.com = jrepp@ibm.com
    ResolveIdentity(ctx context.Context, email string) (*UserIdentity, error)
}

// ===================================================================
// REQUIRED INTERFACE: TeamProvider
// ===================================================================
// TeamProvider handles group/team operations
// NOTE: ALL providers must implement this - either locally or via delegation
//
// Renamed from "GroupProvider" to avoid generic term confusion
type TeamProvider interface {
    // ListTeams lists teams matching query
    ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*Team, error)

    // GetTeam retrieves team details
    GetTeam(ctx context.Context, teamID string) (*Team, error)

    // GetUserTeams lists all teams a user belongs to
    GetUserTeams(ctx context.Context, userEmail string) ([]*Team, error)

    // GetTeamMembers lists all members of a team
    GetTeamMembers(ctx context.Context, teamID string) ([]*UserIdentity, error)
}

// ===================================================================
// REQUIRED INTERFACE: NotificationProvider
// ===================================================================
// NotificationProvider handles email/notification sending
// NOTE: ALL providers must implement this - either locally or via delegation
//
// Renamed from "EmailProvider" to be more generic
type NotificationProvider interface {
    // SendEmail sends an email notification
    SendEmail(ctx context.Context, to []string, from, subject, body string) error

    // SendEmailWithTemplate sends email using template
    SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error
}

// ===================================================================
// COMPOSITE INTERFACE: WorkspaceProvider
// ===================================================================
// WorkspaceProvider is the main provider interface that composes ALL focused interfaces
//
// CRITICAL DESIGN PRINCIPLE: All interfaces are REQUIRED.
// Providers must implement all 7 interfaces either:
//   1. Locally (e.g., Google implements PeopleProvider via Google Directory API)
//   2. Via delegation to remote API (e.g., Local delegates PeopleProvider to remote Hermes)
//
// This ensures consistent API surface - handlers never need capability checks.
type WorkspaceProvider interface {
    // ALL INTERFACES REQUIRED
    DocumentProvider         // Document CRUD
    ContentProvider          // Content operations with revision tracking
    RevisionTrackingProvider // Backend-specific revision management
    PermissionProvider       // File sharing and access control
    PeopleProvider           // User directory and identity resolution
    TeamProvider             // Team/group operations
    NotificationProvider     // Email/notification sending

    // Metadata
    Name() string         // Provider name for logging
    ProviderType() string // "google", "local", "office365", "github", "api"
}
```

### Interface Naming Rationale

| Old Name | New Name | Reason |
|----------|----------|--------|
| `FileProvider` | **`DocumentProvider`** | Avoids confusion with file system directories; we manage documents, not files |
| `RevisionProvider` | **`RevisionTrackingProvider`** | Emphasizes backend-specific revision tracking (Google revs, Git commits, O365 versions) |
| `DirectoryProvider` | **`PeopleProvider`** | "Directory" overloaded (file dirs vs user dir); "People" is unambiguous |
| `GroupProvider` | **`TeamProvider`** | "Group" too generic; "Team" more specific for user groups/teams |
| `EmailProvider` | **`NotificationProvider`** | More generic, allows future expansion to Slack/webhooks/etc. |

### Provider Implementation Patterns

**Pattern 1: Fully Local Implementation** (Google Workspace):
```go
type GoogleWorkspaceProvider struct {
    driveService     *drive.Service
    docsService      *docs.Service
    directoryService *admin.Service
    gmailService     *gmail.Service
}

// Implements ALL 7 interfaces locally using Google APIs
func (p *GoogleWorkspaceProvider) SearchPeople(ctx context.Context, query string) ([]*UserIdentity, error) {
    // Use Google Directory API
    people, err := p.directoryService.Users.List().Query(query).Do()
    // Convert to UserIdentity with alternate emails support
    // ...
}
```

**Pattern 2: Hybrid Implementation** (Local with Delegation):
```go
type LocalWorkspaceProvider struct {
    storage       *LocalStorage      // Implements Document, Content, RevisionTracking locally
    remoteAPI     *RemoteAPIClient   // Delegates People, Team, Notification to remote
    permissionMgr *LocalPermissions  // Simple metadata-based permissions
}

// Implements DocumentProvider locally (Git)
func (p *LocalWorkspaceProvider) GetDocument(ctx context.Context, providerID string) (*DocumentMetadata, error) {
    // Read from local filesystem
    return p.storage.GetDocument(ctx, providerID)
}

// Delegates PeopleProvider to remote API
func (p *LocalWorkspaceProvider) SearchPeople(ctx context.Context, query string) ([]*UserIdentity, error) {
    // Delegate to remote Hermes instance
    return p.remoteAPI.SearchPeople(ctx, query)
}

// Delegates NotificationProvider to remote API
func (p *LocalWorkspaceProvider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
    // Delegate to remote Hermes instance
    return p.remoteAPI.SendEmail(ctx, to, from, subject, body)
}
```

**Pattern 3: Full Delegation** (API Provider - see RFC-085):
```go
type APIProvider struct {
    client    *http.Client
    baseURL   string
    authToken string
}

// Delegates ALL interfaces to remote Hermes
func (p *APIProvider) GetDocument(ctx context.Context, providerID string) (*DocumentMetadata, error) {
    url := fmt.Sprintf("%s/api/v2/documents/%s", p.baseURL, providerID)
    // HTTP GET request
    // ...
}
```

### Provider Implementation Matrix

All interfaces are REQUIRED. The matrix shows how each provider satisfies them:

| Provider | Document | Content | Revision | Permission | People | Team | Notification |
|----------|----------|---------|----------|------------|--------|------|--------------|
| **Google Workspace** | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local |
| **Local (Git)** | 🟢 Local | 🟢 Local | 🟢 Local | 🟡 Basic | 🔵 Delegated | 🔵 Delegated | 🔵 Delegated |
| **Office 365** | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local |
| **GitHub** | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🔵 Delegated |
| **API (Remote)** | 🔵 Delegated | 🔵 Delegated | 🔵 Delegated | 🔵 Delegated | 🔵 Delegated | 🔵 Delegated | 🔵 Delegated |

**Legend**:
- 🟢 **Local**: Implemented directly using provider's native APIs
- 🔵 **Delegated**: Delegated to remote Hermes API (see RFC-085)
- 🟡 **Basic**: Simple local implementation (e.g., metadata-based permissions)

### Benefits of Required Interfaces with Delegation

1. **Consistent API Surface**: Handlers never need capability checks - all providers implement all interfaces
2. **Simplified Handler Logic**:
   ```go
   // NO capability checking needed!
   func (s *Server) handleShareDocument(w http.ResponseWriter, r *http.Request) {
       // Always works - either local or delegated
       err := s.workspace.ShareDocument(ctx, docID, email, role)
   }
   ```
3. **Flexible Implementation**: Providers choose local vs delegated based on backend capabilities
4. **Separation of Concerns**: Each interface has single responsibility
5. **Easier Testing**: Mock only needed interfaces
6. **Incremental Development**: Start with full delegation, gradually move to local implementation
7. **Deployment Flexibility**: Development can delegate to staging, production implements locally
8. **Multi-Backend Ready**: All interfaces designed for UUID-based multi-backend tracking
9. **Graceful Degradation**: Delegated operations fail gracefully with clear error messages

## Implementation Plan

### Phase 1: Type Definitions (Weeks 1-2)

**Week 1: Define Hermes Types**
- [ ] Create `pkg/workspace/types.go` with native types
- [ ] Define `DocumentMetadata`, `DocumentContent`, `BackendRevision`
- [ ] Define `UserIdentity`, `AlternateIdentity`
- [ ] Define `FilePermission`, `Team`, `RevisionInfo`
- [ ] Add JSON/GORM serialization tags
- [ ] Write comprehensive tests for type conversions

**Week 2: Create Focused Provider Interfaces**
- [ ] Define focused interfaces: `DocumentProvider`, `ContentProvider`, `RevisionTrackingProvider`
- [ ] Define required interfaces: `PermissionProvider`, `PeopleProvider`, `TeamProvider`, `NotificationProvider`
- [ ] Define composite `WorkspaceProvider` interface
- [ ] Document interface contracts and multi-backend requirements
- [ ] Create adapter pattern for Google provider (implements all interfaces)
- [ ] Create adapter pattern for Local provider (hybrid implementation)

### Phase 2: Adapter Implementation (Weeks 3-4)

**Week 3: Google Workspace Adapter**
- [ ] Implement all 7 interfaces using Google APIs
- [ ] Add BackendRevision support (Google Doc revision numbers)
- [ ] Add UserIdentity support with alternate emails
- [ ] Write conversion helpers (Google types → Hermes types)
- [ ] Comprehensive unit tests

**Week 4: Local Workspace Adapter**
- [ ] Implement Document/Content/RevisionTracking locally (Git)
- [ ] Add BackendRevision support (Git commit SHAs)
- [ ] Implement basic PermissionProvider (metadata-based)
- [ ] Create RemoteAPIClient for delegated interfaces (People, Team, Notification)
- [ ] Integration tests with delegation

### Phase 3: API Handler Migration (Weeks 5-6)

**Week 5: Core API Handlers**
- [ ] Update document CRUD handlers to use new types
- [ ] Update permission handlers
- [ ] Update content handlers
- [ ] Add feature flag for gradual rollout

**Week 6: Testing and Validation**
- [ ] Run full E2E test suite with both Google and Local providers
- [ ] Verify no regressions in existing functionality
- [ ] Performance benchmarks (should be equivalent)
- [ ] Document migration guide

## Design Decisions

### Decision 1: Hermes-Native Types vs. Google Types

**Options Considered**:
1. **Keep Google Types**: Maintain `*drive.File` in interface
2. **Hermes-Native Types**: Define `DocumentMetadata` struct
3. **Generic Interface**: Use `map[string]any` for flexibility

**Decision**: Hermes-Native Types

**Rationale**:
- **Type Safety**: Compile-time validation of field access
- **Clear Contracts**: Explicit schema for API responses
- **Multi-Backend Support**: Not tied to Google semantics
- **Maintainability**: Easier to evolve and document
- **Performance**: No runtime type assertions needed

### Decision 2: All Interfaces Required

**Options Considered**:
1. **Optional Interfaces**: Providers implement subset, handlers check capabilities
2. **Required Interfaces**: All providers implement all interfaces
3. **Required with Delegation**: All required, satisfied locally or via delegation

**Decision**: Required with Delegation

**Rationale**:
- **Consistent API**: No capability checks in handlers
- **Flexible Implementation**: Providers choose local vs remote
- **Clear Contracts**: Known interface surface for all providers
- **Graceful Degradation**: Remote delegation handles missing local capabilities
- **Incremental Development**: Start with delegation, move to local over time

### Decision 3: Interface Naming

**Decision**: Rename interfaces for clarity

**Rationale**:
- `DocumentProvider` (not FileProvider): Emphasizes document management, not file system
- `PeopleProvider` (not DirectoryProvider): Unambiguous - about users, not directories
- `TeamProvider` (not GroupProvider): Specific to user teams/groups
- `NotificationProvider` (not EmailProvider): Generic for future expansion
- `RevisionTrackingProvider` (not RevisionProvider): Emphasizes backend-specific tracking

## Success Metrics

### Phase 1-3 Success Criteria
- [ ] All existing E2E tests pass with new interfaces
- [ ] No performance regression (< 5% latency increase)
- [ ] 100% feature parity with existing Provider interface
- [ ] Zero production incidents during rollout
- [ ] All 7 interfaces implemented by Google and Local providers

### Overall Success Criteria
- [ ] Type-safe provider interfaces in production
- [ ] Multi-backend document tracking operational
- [ ] Local provider successfully delegates to remote Hermes
- [ ] Positive feedback from development team
- [ ] Documentation and examples completed

## Risks & Mitigations

### Risk 1: Increased Complexity

**Risk**: More interfaces and types to maintain

**Mitigation**:
- Comprehensive documentation with examples
- Clear interface contracts
- Extensive unit and integration tests
- Phased rollout with feature flags

### Risk 2: Migration Effort

**Risk**: Updating existing handlers to use new types

**Mitigation**:
- Backward compatibility during transition
- Gradual handler migration
- Conversion helpers for Google types
- Feature flag for rollback if needed

### Risk 3: Performance Impact

**Risk**: Type conversions add latency

**Mitigation**:
- Benchmark critical paths
- Optimize conversion functions
- Monitor production metrics
- Accept <5% overhead for type safety

## References

- **RFC-082**: Document Identification System (UUID + ProviderID)
- **RFC-085**: API Provider and Remote Delegation (implementation patterns)
- **RFC-086**: Authentication and Bearer Token Management (auth strategy)
- **MEMO-091**: Document Revisions and Migration Tracking
- **Provider Implementations**:
  - `pkg/workspace/adapters/google/` - Google Workspace provider
  - `pkg/workspace/adapters/local/` - Local filesystem provider

## Timeline

- **Week 1-2**: Type definitions and interface design
- **Week 3-4**: Adapter implementation (Google and Local)
- **Week 5-6**: API handler migration and testing
- **Total**: 6 weeks to production-ready provider interfaces

## Next Steps

1. Review and approve RFC-084
2. Review RFC-085 (API Provider implementation patterns)
3. Review RFC-086 (Authentication strategy)
4. Begin Phase 1 implementation (type definitions)

---

**Status**: Proposed
**Implementation**: See RFC-085 for API provider patterns and RFC-086 for authentication strategy
