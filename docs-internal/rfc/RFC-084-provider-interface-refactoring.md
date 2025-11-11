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
- Enhanced metadata with core attributes (tags, ownership, workflow status) and extensible fields

**Related RFCs**:
- **RFC-085**: API Provider and Remote Delegation (implementation patterns)
- **RFC-086**: Authentication and Bearer Token Management (auth strategy)

## Context

### Current Document Model (RFC-082 Foundation)

Hermes uses a UUID-based document identification system where documents can exist across multiple backends:

**Core Concepts**:
- **UUID**: Stable global identifier (`550e8400-e29b-41d4-a716-446655440000`)
- **ProviderID**: Backend-specific identifier with real-world formats:
  - Google Drive: `google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs` (33-44 char alphanumeric)
  - Office 365: `office365:01CYZLFJGUJ7JHBSZDFZFL25KSZGQTVAUN` (Base32-encoded ID)
  - Local Git: `local:docs/rfc-001.md` (filesystem path)
  - GitHub: `github:owner/repo/path/file.md@a1b2c3d4` (repo path + commit ref)
- **Multi-Backend Tracking**: Same document UUID can have multiple active revisions across different backends

**Example - Document Across Multiple Backends**:
```
Document UUID: 550e8400-e29b-41d4-a716-446655440000
Title: "RFC-001: API Gateway Design"
Tags: [rfc, architecture, api-gateway, infrastructure]
Project: platform-engineering
Workflow Status: Published

┌─────────────────────────────────────────────────────────────┐
│ Revision 1: Google Workspace (Source of Truth)             │
├─────────────────────────────────────────────────────────────┤
│ ProviderID: google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs        │
│ Backend Revision: 123                                        │
│   (Google Drive revision ID - numeric string)               │
│ Content Hash: sha256:abc123...                              │
│ Last Modified: 2025-10-15T14:30:00Z                         │
│ Sync Status: canonical                                       │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Revision 2: Local Git (Migrated Copy)                       │
├─────────────────────────────────────────────────────────────┤
│ ProviderID: local:docs/rfc-001.md                           │
│ Backend Revision: a1b2c3d4e5f67890abcdef1234567890abcdef12  │
│   (Git commit SHA - 40 character hex string)                │
│ Content Hash: sha256:abc123...  ✅ matches Google           │
│ Last Modified: 2025-10-01T09:00:00Z                         │
│ Sync Status: mirror                                          │
│ Extended Metadata: {id: "rfc-001", sidebar_position: 1}     │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Revision 3: Office 365 (Mirror for Collaboration)          │
├─────────────────────────────────────────────────────────────┤
│ ProviderID: office365:01CYZLFJGUJ7JHBSZDFZFL25KSZGQTVAUN    │
│ Backend Revision: 2.0                                        │
│   (O365 version ID - semantic version or timestamp string)  │
│ Content Hash: sha256:def456...  ⚠️ drift detected          │
│ Last Modified: 2025-10-20T11:15:00Z                         │
│ Sync Status: conflict                                        │
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
    │ - GitHub  │      │          │      │            │
    │ - IBM     │      │          │      │            │
    │   Verify  │      │          │      │            │
    └───────────┘      └──────────┘      └────────────┘
       (OIDC)          (Direct)          (Direct)
```

**Current Authentication Providers**:
- **Dex**: Generic OIDC/OAuth2 provider with support for multiple identity backends
- **Okta**: Enterprise identity management and SSO platform
- **Google**: Google OAuth2/OIDC authentication
- **GitHub**: GitHub OAuth2 authentication for developers
- **IBM Verify**: IBM's cloud identity and access management platform (formerly IBM Cloud Identity)

**Authentication Provider Characteristics**:
- All support OIDC/OAuth2 protocols
- Provide user identity, email, and profile information
- Enable unified identity across multiple providers (e.g., jacob.repp@hashicorp.com = jrepp@ibm.com)
- GitHub provides GitHub username and user ID for repository access
- IBM Verify enables enterprise SSO with IBM Cloud and on-premise systems

**Current Workspace Providers** (`pkg/workspace/provider.go`):
- **Google Workspace**: Direct integration via Google Drive/Docs APIs
- **Local**: Direct filesystem access for markdown-based documents

**Workspace Provider Interface Characteristics**:
- ~30 methods covering file operations, permissions, content, email, groups
- Returns Google Drive/Docs types (`*drive.File`, `*docs.Document`)
- Assumes direct backend access (no network proxy/delegation pattern)

### Real-World ID Format Reference

This section documents the actual ID and revision formats used by each backend system to ensure transparent handling.

#### Google Drive/Docs Document IDs

**Format**: 33-44 character alphanumeric string (Base64-like encoding)
**Character Set**: `[A-Za-z0-9_-]`
**Examples**:
- `1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms` (Google Docs)
- `1a2b3c4d5e6f7g8h9i0jklmnopqrstuv` (Google Drive file)
- `0B1234567890ABCDEFGHIJKLMNOPQRST` (Folder ID)

**Hermes ProviderID Format**: `google:{fileId}`
- Example: `google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms`

**Revision IDs**: Numeric strings that increment sequentially
- Format: `[0-9]+`
- Examples: `"1"`, `"42"`, `"123"`, `"9876"`
- First revision is always `"1"`
- Auto-pruned after 30 days (unless marked `keepForever`)

**API References**:
- Files API: `https://www.googleapis.com/drive/v3/files/{fileId}`
- Revisions API: `https://www.googleapis.com/drive/v3/files/{fileId}/revisions/{revisionId}`

#### Office 365 (OneDrive/SharePoint) Document IDs

**Format**: Base32-encoded string (uppercase, no padding)
**Character Set**: `[A-Z0-9]`
**Length**: Typically 27-50+ characters
**Examples**:
- `01CYZLFJGUJ7JHBSZDFZFL25KSZGQTVAUN` (OneDrive item)
- `01BYE5RZ6QN3ZWBTUFOFD3GSPGOHDJD36K` (SharePoint item)
- `b!8oWRTQj8eE-C_VEzRgKvI8qPqm...` (Drive item with full path)

**Hermes ProviderID Format**: `office365:{itemId}`
- Example: `office365:01CYZLFJGUJ7JHBSZDFZFL25KSZGQTVAUN`

**Version IDs**: Multiple formats depending on API
1. **Semantic versions**: `"1.0"`, `"2.0"`, `"3.1"`
2. **Version labels**: `"Major"`, `"Minor"`, `"Current"`
3. **Encoded version IDs**: `"AWQiRU9kdkNVVnVjM1Z5WTJWelBIQStQSFJoWW14bFBqeHdjajQ4ZEdRKw"` (Base64)
4. **Timestamp-based**: ISO 8601 format `"2025-10-15T14:30:00Z"`

**API References**:
- Item API: `https://graph.microsoft.com/v1.0/me/drive/items/{itemId}`
- Versions API: `https://graph.microsoft.com/v1.0/me/drive/items/{itemId}/versions`

#### Git Commit SHAs (Local/GitHub)

**Format**: 40-character hexadecimal string (SHA-1 hash)
**Character Set**: `[0-9a-f]`
**Examples**:
- `a1b2c3d4e5f67890abcdef1234567890abcdef12` (full SHA)
- `e7f8g9h0` (short form, 7-8 characters for display)
- `48b8e291b2c4e7c4d1234567890abcdef1234567` (another commit)

**Hermes ProviderID Format**:
- Local: `local:{relative/path/to/file.md}`
  - Example: `local:docs/rfc/RFC-084.md`
- GitHub: `github:{owner}/{repo}/{path}@{ref}`
  - Example: `github:hashicorp/hermes/docs/RFC-084.md@main`

**Revision IDs**: Full 40-character commit SHA-1 hash
- Always use full SHA for storage and API operations
- Can display short form (7-8 chars) in UI for readability
- Example storage: `"a1b2c3d4e5f67890abcdef1234567890abcdef12"`
- Example display: `"a1b2c3d"`

**Git Notes for Revision History**:
- Git commits form an immutable directed acyclic graph (DAG)
- Each commit has 0-N parent commits (merges can have multiple parents)
- Revision history requires walking the commit tree from HEAD
- Use `git log` or `git rev-list` to enumerate revisions in chronological order

**API References**:
- GitHub Commits API: `https://api.github.com/repos/{owner}/{repo}/commits/{sha}`
- Git CLI: `git show {sha}`, `git log {sha}`

#### Handling ID Format Variations

The system must handle these ID format variations transparently:

1. **Validation**: Each provider adapter should validate IDs match expected format
2. **Storage**: Store IDs as opaque strings (no parsing/modification)
3. **Comparison**: Use exact string equality (case-sensitive for Git, case-insensitive for O365)
4. **Display**: Can truncate or format for UI, but always use full ID in API calls
5. **Serialization**: JSON/YAML safe (all formats use safe character sets)

**Implementation Considerations**:
```go
// Example validation functions
func IsValidGoogleDriveID(id string) bool {
    // 33-44 chars, alphanumeric + underscore + hyphen
    match, _ := regexp.MatchString(`^[A-Za-z0-9_-]{33,44}$`, id)
    return match
}

func IsValidGitCommitSHA(sha string) bool {
    // 40 hex characters (full SHA-1)
    match, _ := regexp.MatchString(`^[0-9a-f]{40}$`, sha)
    return match
}

func IsValidOffice365ID(id string) bool {
    // Flexible - various formats, typically 20+ chars
    return len(id) >= 20 && len(id) <= 200
}
```

### The Problem

The current architecture has several limitations:

1. **Tight Coupling to Backend Types**: The `workspace.Provider` interface returns Google-specific types (`*drive.File`, `*people.Person`), making it difficult to support non-Google backends or proxy operations through a different API layer.

2. **No Multi-Backend Tracking**: Cannot track document revisions across multiple providers (Google + Git + O365) for migration and conflict detection.

3. **No Remote Delegation**: Providers must directly access their backends. There's no way for one Hermes instance to delegate operations to another Hermes instance.

4. **Limited Identity Unification**: No support for linking user identities across multiple authentication providers (jacob.repp@hashicorp.com = jrepp@ibm.com = jacob-repp on GitHub).

## Proposed Solution

### Type Abstraction - Hermes-Native Types

Introduce Hermes-native types that support multi-backend document tracking:

#### 1. DocumentMetadata

```go
// DocumentMetadata represents provider-agnostic document metadata
// Works with DocID system (UUID + ProviderID)
//
// Design Philosophy:
// - Core attributes: Universal metadata present across all document types
// - Extensible attributes: Document-type-specific metadata in ExtendedMetadata map
type DocumentMetadata struct {
    // Global identifier (RFC-082)
    UUID docid.UUID `json:"uuid"`

    // Backend-specific identifier
    ProviderType string `json:"providerType"` // "google", "local", "office365", "github"
    ProviderID   string `json:"providerID"`   // Backend-specific ID

    // Core Metadata
    Name         string    `json:"name"`         // Document title
    MimeType     string    `json:"mimeType"`     // MIME type (e.g., "text/markdown", "application/vnd.google-apps.document")
    CreatedTime  time.Time `json:"createdTime"`  // When document was created
    ModifiedTime time.Time `json:"modifiedTime"` // Last modification timestamp

    // Ownership (unified identity aware)
    Owner        *UserIdentity   `json:"owner,omitempty"`        // Individual owner (can be nil if team-owned)
    OwningTeam   string          `json:"owningTeam,omitempty"`   // Team/group ownership (e.g., "Engineering Team")
    Contributors []UserIdentity  `json:"contributors,omitempty"` // Document contributors/collaborators

    // Hierarchy and Organization
    Parents      []string `json:"parents,omitempty"` // Parent folder/directory IDs
    Project      string   `json:"project,omitempty"` // Project association (e.g., "agf-iac-remediation-poc")
    Tags         []string `json:"tags,omitempty"`    // Universal tags for categorization and search

    // Document Lifecycle
    SyncStatus     string `json:"syncStatus"`                // Multi-backend sync: "canonical", "mirror", "conflict", "archived"
    WorkflowStatus string `json:"workflowStatus,omitempty"` // Document workflow: "Draft", "In Review", "Published", "Deprecated"

    // Multi-backend tracking
    ContentHash  string `json:"contentHash"` // SHA-256 for drift detection

    // Extensible metadata for document-type-specific fields
    // Examples: RFC id ("rfc-010"), sidebar_position (10), rfc_type ("Architecture")
    ExtendedMetadata map[string]any `json:"extendedMetadata,omitempty"`
}
```

**Core vs Extensible Attributes**:

The `DocumentMetadata` design separates universal attributes from document-type-specific metadata:

| Attribute Category | Location | Examples | Rationale |
|-------------------|----------|----------|-----------|
| **Core Attributes** | Type-safe struct fields | `UUID`, `Name`, `Tags`, `CreatedTime`, `Project` | Universal across all document types, searchable, type-safe validation |
| **Extensible Attributes** | `ExtendedMetadata` map | `id: "rfc-010"`, `sidebar_position: 10`, `rfc_type: "Architecture"` | Document-type-specific, flexible schema, no type safety needed |

**Frontmatter Mapping Example**:

```yaml
# Document frontmatter (YAML)
---
id: rfc-010
title: "RFC-010: Diff Classification and Correlation System"
status: Draft
author: Engineering Team
created: 2025-11-08
updated: 2025-11-08
tags: [rfc, classification, diff, correlation, observability]
project_id: agf-iac-remediation-poc
doc_uuid: 7e8f4a2c-9d5b-4c1e-a8f7-3b2d1e6c9a4f
sidebar_position: 10
---
```

```go
// Maps to DocumentMetadata
metadata := DocumentMetadata{
    // Core: Identity
    UUID:         "7e8f4a2c-9d5b-4c1e-a8f7-3b2d1e6c9a4f",
    ProviderType: "local",
    ProviderID:   "local:docs/rfc/rfc-010.md",

    // Core: Metadata
    Name:         "RFC-010: Diff Classification and Correlation System",
    CreatedTime:  parseTime("2025-11-08"),
    ModifiedTime: parseTime("2025-11-08"),

    // Core: Ownership
    OwningTeam:   "Engineering Team",

    // Core: Organization
    Project:      "agf-iac-remediation-poc",
    Tags:         []string{"rfc", "classification", "diff", "correlation", "observability"},

    // Core: Lifecycle
    WorkflowStatus: "Draft",
    SyncStatus:     "canonical",

    // Extensible: Document-type-specific
    ExtendedMetadata: map[string]any{
        "id":               "rfc-010",          // RFC-specific ID format
        "sidebar_position": 10,                 // UI/presentation metadata
        "document_type":    "rfc",              // Type classification
    },
}
```

**Benefits of This Design**:
1. **Type Safety**: Core attributes have compile-time validation and autocomplete
2. **Searchability**: Core attributes can be efficiently indexed and queried
3. **Flexibility**: New document types can add custom metadata without schema changes
4. **Backward Compatibility**: Adding new core attributes doesn't break existing code
5. **Clear Contracts**: API consumers know which fields are always present

**When to Use Core vs Extensible**:
- **Use Core** for: Universal metadata, search/filter criteria, identity, timestamps, ownership
- **Use Extensible** for: Document-type-specific IDs, UI preferences, type-specific classifications

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
//
// Real-world revision ID formats by provider:
//
// Google Drive/Docs:
//   - RevisionID format: Numeric string (e.g., "123", "456")
//   - Increments with each revision
//   - Can be marked "keepForever" to prevent auto-pruning
//   - Example: "123" for the 123rd revision of the document
//
// Git (Local/GitHub):
//   - RevisionID format: 40-character SHA-1 hash (e.g., "a1b2c3d4e5f67890abcdef1234567890abcdef12")
//   - Full commit hash, can use short form (7-8 chars) for display
//   - Immutable and globally unique within repository
//   - Example: "a1b2c3d4e5f67890abcdef1234567890abcdef12"
//
// Office 365 (OneDrive/SharePoint):
//   - RevisionID format: Version string or timestamp (e.g., "2.0", "1.1", "2023-10-15T14:30:00Z")
//   - Can be semantic version (major.minor) or complex version identifiers
//   - May include version labels like "1.0", "2.0" or timestamp-based IDs
//   - Example: "2.0" or "AWQiRU9kdkNVVnVjM1Z5WTJWelBIQStQSFJoWW14bFBqeHdjajQ4ZEdRKw"
//
// GitHub API:
//   - RevisionID format: 40-character SHA-1 hash (same as Git)
//   - Retrieved via GitHub API with additional metadata
//   - Example: "e7f8g9h0i1j2k3l4m5n6o7p8q9r0s1t2u3v4w5x6"
type BackendRevision struct {
    ProviderType string `json:"providerType"` // "google", "git", "office365", "github"

    // Backend-specific revision ID (varies by provider - see format docs above)
    RevisionID string `json:"revisionID"`

    // Revision metadata
    ModifiedTime time.Time      `json:"modifiedTime"`
    ModifiedBy   *UserIdentity  `json:"modifiedBy,omitempty"`
    Comment      string         `json:"comment,omitempty"` // Git commit message, Drive comment, etc.
    KeepForever  bool           `json:"keepForever,omitempty"` // Google Drive feature

    // Backend-specific metadata (flexible for different systems)
    // Examples:
    //   - Google: {"published": true, "size": 12345}
    //   - Git: {"tree": "abc123", "parent": "def456", "author": "..."}
    //   - O365: {"versionLabel": "Major", "size": 12345, "comment": "..."}
    Metadata     map[string]any `json:"metadata,omitempty"`
}
```

#### 3. UserIdentity (Unified Identity)

```go
// UserIdentity represents a unified user identity across multiple auth providers
// Addresses the requirement: jacob.repp@hashicorp.com = jrepp@ibm.com = jacob-repp on GitHub (same person)
// Supports multiple authentication providers: Google, GitHub, IBM Verify, Okta, Dex
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
    Email          string `json:"email"`        // e.g., "jrepp@ibm.com", "jacob-repp@users.noreply.github.com"
    Provider       string `json:"provider"`     // e.g., "ibm-verify", "google-workspace", "github", "okta", "dex"
    ProviderUserID string `json:"providerUserId,omitempty"` // Provider-specific user ID
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
    SyncStatus      string           `json:"syncStatus"` // "canonical", "mirror", "conflict"
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

    // RegisterDocument registers document metadata with provider (for tracking)
    // Used by edge instances to register documents with central tracking system
    RegisterDocument(ctx context.Context, doc *DocumentMetadata) (*DocumentMetadata, error)

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
    //     AlternateEmails: [
    //       {Email: "jrepp@ibm.com", Provider: "ibm-verify"},
    //       {Email: "jacob-repp", Provider: "github", ProviderUserID: "12345678"},
    //     ]
    //   }
    GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*UserIdentity, error)

    // ResolveIdentity resolves alternate identities for a user
    // Used for identity unification: jacob.repp@hashicorp.com = jrepp@ibm.com = jacob-repp on GitHub
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
// OPTIONAL INTERFACE: DocumentSyncProvider
// ===================================================================
// DocumentSyncProvider handles document synchronization between edge and central instances
// This interface is OPTIONAL - only needed for multi-provider edge/central architectures
type DocumentSyncProvider interface {
    // RegisterDocument registers document metadata with central for tracking
    RegisterDocument(ctx context.Context, doc *DocumentMetadata) (*DocumentMetadata, error)

    // SyncMetadata synchronizes document metadata from edge to central
    SyncMetadata(ctx context.Context, uuid docid.UUID, metadata *DocumentMetadata) error

    // GetSyncStatus gets synchronization status for documents
    GetSyncStatus(ctx context.Context, uuids []docid.UUID) ([]*SyncStatus, error)

    // SyncRevision synchronizes revision information to central
    SyncRevision(ctx context.Context, uuid docid.UUID, revision *BackendRevision) error
}

// SyncStatus represents document synchronization state
type SyncStatus struct {
    UUID          docid.UUID `json:"uuid"`
    LastSyncTime  time.Time  `json:"lastSyncTime"`
    SyncState     string     `json:"syncState"` // "synced", "pending", "failed"
    ErrorMessage  string     `json:"errorMessage,omitempty"`
}

// ===================================================================
// OPTIONAL INTERFACE: DocumentMergeProvider
// ===================================================================
// DocumentMergeProvider handles UUID merging for drift resolution
// This interface is OPTIONAL - only needed for central instances managing multi-backend documents
type DocumentMergeProvider interface {
    // MergeDocuments merges two document UUIDs (combine revision histories)
    MergeDocuments(ctx context.Context, req *MergeRequest) error

    // FindMergeCandidates finds potential duplicate documents for given UUID
    FindMergeCandidates(ctx context.Context, uuid docid.UUID) ([]*DocumentMetadata, error)

    // GetMergeHistory retrieves history of UUID merges
    GetMergeHistory(ctx context.Context, limit int) ([]*MergeRecord, error)

    // RollbackMerge rolls back a previous merge operation
    RollbackMerge(ctx context.Context, mergeID string) error
}

// MergeRecord represents a historical UUID merge operation
type MergeRecord struct {
    MergeID        string     `json:"mergeId"`
    SourceUUID     docid.UUID `json:"sourceUuid"`
    TargetUUID     docid.UUID `json:"targetUuid"`
    MergedAt       time.Time  `json:"mergedAt"`
    InitiatedBy    string     `json:"initiatedBy"`
    RevisionCount  int        `json:"revisionCount"`
}

// ===================================================================
// OPTIONAL INTERFACE: IdentityJoinProvider
// ===================================================================
// IdentityJoinProvider handles cross-provider identity linking
// This interface is OPTIONAL - only needed for central instances with multi-provider auth
type IdentityJoinProvider interface {
    // InitiateIdentityJoin starts OAuth flow to join identity from another provider
    InitiateIdentityJoin(ctx context.Context, provider string) (*OAuthFlow, error)

    // CompleteIdentityJoin completes identity join after OAuth callback
    CompleteIdentityJoin(ctx context.Context, req *JoinIdentityRequest) (*UserIdentity, error)

    // GetCurrentUserIdentity retrieves current user's unified identity
    GetCurrentUserIdentity(ctx context.Context) (*UserIdentity, error)

    // RemoveAlternateIdentity unlinks an alternate identity
    RemoveAlternateIdentity(ctx context.Context, identityID string) error

    // GetAllIdentities retrieves all identities for a unified user
    GetAllIdentities(ctx context.Context, unifiedUserID string) ([]*UserIdentity, error)
}

// OAuthFlow represents OAuth flow initiation data
type OAuthFlow struct {
    AuthURL      string `json:"authUrl"`
    State        string `json:"state"`
    Provider     string `json:"provider"`
}

// JoinIdentityRequest represents identity join completion request
type JoinIdentityRequest struct {
    Provider string `json:"provider"`
    Code     string `json:"code"`
    State    string `json:"state"`
}

// ===================================================================
// COMPOSITE INTERFACE: WorkspaceProvider
// ===================================================================
// WorkspaceProvider is the main provider interface that composes core focused interfaces
//
// CRITICAL DESIGN PRINCIPLE: Core interfaces (7) are REQUIRED.
// Optional interfaces (3) are for advanced multi-provider scenarios.
//
// Providers must implement all 7 CORE interfaces either:
//   1. Locally (e.g., Google implements PeopleProvider via Google Directory API)
//   2. Via delegation to remote API (e.g., Local delegates PeopleProvider to remote Hermes)
//
// This ensures consistent API surface - handlers never need capability checks for core operations.
type WorkspaceProvider interface {
    // CORE INTERFACES (REQUIRED)
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

// ===================================================================
// COMPOSITE INTERFACE: ExtendedWorkspaceProvider
// ===================================================================
// ExtendedWorkspaceProvider adds optional interfaces for multi-provider architectures
// Used by central Hermes instances and edge instances with advanced capabilities
type ExtendedWorkspaceProvider interface {
    WorkspaceProvider        // All core interfaces

    // OPTIONAL INTERFACES (for multi-provider scenarios)
    DocumentSyncProvider     // Document synchronization (edge ↔ central)
    DocumentMergeProvider    // UUID merging (drift resolution)
    IdentityJoinProvider     // Cross-provider identity linking
}
```

### Supporting Infrastructure

#### DocumentRegistry

The DocumentRegistry tracks UUID to provider mappings for multi-backend document tracking:

```go
// DocumentRegistry manages document UUID to provider mappings
// Required for multi-provider architectures where documents exist across multiple backends
type DocumentRegistry interface {
    // Register registers a document UUID with its provider
    Register(ctx context.Context, uuid docid.UUID, providerType string) error

    // GetDocument retrieves document metadata by UUID (any provider)
    GetDocument(ctx context.Context, uuid docid.UUID) (*DocumentMetadata, error)

    // GetProviderForUUID returns which provider(s) have this UUID
    GetProviderForUUID(ctx context.Context, uuid docid.UUID) ([]string, error)

    // GetAllDocumentRevisions gets all revisions across all backends for a UUID
    GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*RevisionInfo, error)

    // SaveRevision stores revision information
    SaveRevision(ctx context.Context, rev *RevisionInfo) error

    // UpdateSyncStatus updates the synchronization status for a document
    UpdateSyncStatus(ctx context.Context, uuid docid.UUID, syncStatus string) error
}
```

#### MultiProviderManager

The MultiProviderManager enables running multiple providers simultaneously with automatic routing:

```go
// MultiProviderManager composes multiple WorkspaceProviders for edge/central architectures
// Implements WorkspaceProvider by intelligently routing to appropriate provider
type MultiProviderManager struct {
    primary   WorkspaceProvider  // Primary provider (e.g., local Git)
    secondary WorkspaceProvider  // Secondary provider (e.g., API to central)
    registry  DocumentRegistry   // UUID tracking across providers
    config    *MultiProviderConfig
}

// MultiProviderConfig configures multi-provider behavior
type MultiProviderConfig struct {
    RoutingPolicy string // "primary_first", "secondary_first", "both"
    AutoSync      bool   // Automatically sync metadata to secondary
    SyncContent   bool   // Sync content or just metadata
}

// Ensure MultiProviderManager implements WorkspaceProvider
var _ WorkspaceProvider = (*MultiProviderManager)(nil)

// GetDocument with automatic routing
func (m *MultiProviderManager) GetDocument(ctx context.Context, providerID string) (*DocumentMetadata, error) {
    // Try primary provider first
    doc, err := m.primary.GetDocument(ctx, providerID)
    if err == nil {
        return doc, nil
    }

    // If not found locally, try secondary (central)
    doc, err = m.secondary.GetDocument(ctx, providerID)
    if err == nil {
        // Document exists in central, cache locally
        m.registry.Register(ctx, doc.UUID, "secondary")
        return doc, nil
    }

    return nil, fmt.Errorf("document not found in any provider")
}

// CreateDocument creates in primary and optionally syncs to secondary
func (m *MultiProviderManager) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*DocumentMetadata, error) {
    // New documents created in primary (local) provider
    doc, err := m.primary.CreateDocument(ctx, templateID, destFolderID, name)
    if err != nil {
        return nil, err
    }

    // Register with local registry
    if err := m.registry.Register(ctx, doc.UUID, "primary"); err != nil {
        log.Warn("failed to register document", "uuid", doc.UUID, "error", err)
    }

    // Automatically register with central for tracking (if configured)
    if m.config.AutoSync {
        go m.syncToSecondary(context.Background(), doc)
    }

    return doc, nil
}

// syncToSecondary replicates document metadata to central
func (m *MultiProviderManager) syncToSecondary(ctx context.Context, doc *DocumentMetadata) {
    // Check if secondary implements DocumentSyncProvider
    if syncer, ok := m.secondary.(DocumentSyncProvider); ok {
        if _, err := syncer.RegisterDocument(ctx, doc); err != nil {
            log.Error("failed to sync document to central", "uuid", doc.UUID, "error", err)
        }
    }
}
```

**Usage Example**:
```go
// Create multi-provider manager
manager := &MultiProviderManager{
    primary:   localProvider,
    secondary: apiProvider,
    registry:  documentRegistry,
    config: &MultiProviderConfig{
        RoutingPolicy: "primary_first",
        AutoSync:      true,
        SyncContent:   false, // Only sync metadata
    },
}

// Use as normal WorkspaceProvider
doc, err := manager.GetDocument(ctx, "local:docs/rfc-084.md")
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
- [ ] Define `DocumentMetadata` with core and extensible attributes:
  - [ ] Core fields: UUID, Name, Tags, CreatedTime, ModifiedTime
  - [ ] Ownership: Owner, OwningTeam, Contributors
  - [ ] Lifecycle: SyncStatus, WorkflowStatus
  - [ ] Extensible: ExtendedMetadata map
- [ ] Define `DocumentContent`, `BackendRevision`
- [ ] Define `UserIdentity`, `AlternateIdentity`
- [ ] Define `FilePermission`, `Team`, `RevisionInfo`
- [ ] Add JSON/GORM serialization tags
- [ ] Write comprehensive tests for type conversions
- [ ] Create frontmatter parsing utilities for ExtendedMetadata

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
- [ ] Map Google Drive metadata to core DocumentMetadata fields (tags, ownership)
- [ ] Handle ExtendedMetadata for Google-specific attributes
- [ ] Write conversion helpers (Google types → Hermes types)
- [ ] Comprehensive unit tests

**Week 4: Local Workspace Adapter**
- [ ] Implement Document/Content/RevisionTracking locally (Git)
- [ ] Add BackendRevision support (Git commit SHAs)
- [ ] Parse frontmatter (YAML/TOML) to populate DocumentMetadata
- [ ] Map frontmatter to core fields and ExtendedMetadata
- [ ] Support tags, workflow status, and project association
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
