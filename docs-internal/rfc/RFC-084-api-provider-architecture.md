---
id: RFC-084
title: API Provider Architecture - Remote Hermes Backend Support
date: 2025-11-11
type: RFC
subtype: Architecture
status: Proposed
tags: [providers, architecture, remote-api, federation, multi-tier]
related:
  - RFC-082
  - RFC-007
  - MEMO-019
---

# API Provider Architecture - Remote Hermes Backend Support

## Executive Summary

This RFC proposes a refactoring of the Hermes provider architecture to support a new "API provider" that can delegate workspace and search operations to a remote Hermes instance via its REST API. This enables multi-tier deployments, federated document management, and hybrid cloud/on-premise architectures.

**Key Benefits**:
- Enable multi-tier architectures (edge nodes → central Hermes)
- Support federated document management across multiple Hermes instances
- Allow thin clients that delegate storage to remote Hermes servers
- Maintain consistent API contracts across all deployment models

## Context

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

1. **No Remote Delegation**: Providers must directly access their backends (Google APIs, local filesystem). There's no way for one Hermes instance to use another Hermes instance as its backend.

2. **Tight Coupling to Backend Types**: The `workspace.Provider` interface returns Google-specific types (`*drive.File`, `*people.Person`), making it difficult to proxy operations through a different API layer.

3. **No Federation Support**: Cannot build multi-tier architectures where edge nodes delegate to central Hermes instances.

4. **Limited Deployment Flexibility**: Cannot deploy a thin Hermes frontend that delegates storage operations to a remote Hermes cluster.

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

Introduce a new **API Provider** that implements the `workspace.Provider` interface by delegating operations to a remote Hermes instance via REST API calls.

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

### Provider Refactoring Strategy

The refactoring will occur in **two phases**:

#### Phase 1: Type Abstraction (Foundation)

**Problem**: Current `workspace.Provider` interface returns Google-specific types:
```go
type Provider interface {
    GetFile(fileID string) (*drive.File, error)
    GetDoc(fileID string) (*docs.Document, error)
    SearchPeople(email string, fields string) ([]*people.Person, error)
    // ... 27 more methods
}
```

**Solution**: Introduce Hermes-native types and create adapters:

1. **Define Hermes Types** (`pkg/workspace/types.go`):
```go
// FileMetadata represents provider-agnostic file metadata
type FileMetadata struct {
    ID          string
    Name        string
    MimeType    string
    CreatedTime time.Time
    ModifiedTime time.Time
    Owner       string
    Parents     []string
    Permissions []Permission
    // ... other common fields
}

// DocumentContent represents document content
type DocumentContent struct {
    ID      string
    Title   string
    Body    string
    Format  string // "markdown", "html", "plain"
}

// Person represents a workspace user
type Person struct {
    Email       string
    DisplayName string
    PhotoURL    string
}

// Permission represents file access permissions
type Permission struct {
    ID           string
    Email        string
    Role         string // "owner", "writer", "reader"
    Type         string // "user", "group", "domain"
}

// Group represents a workspace group (for enterprise providers)
type Group struct {
    ID          string
    Email       string
    Name        string
    Description string
    MemberCount int
}

// Revision represents a document revision
type Revision struct {
    ID           string
    ModifiedTime time.Time
    KeepForever  bool
}
```

2. **Split Provider into Focused Interfaces**:

Instead of a single monolithic interface, create focused provider interfaces that can be composed:

```go
// Provider is the legacy interface (Google types) - DEPRECATED
type Provider interface {
    GetFile(fileID string) (*drive.File, error)
    // ... existing ~30 methods
}

// FileProvider handles file operations (CRUD on files/folders)
type FileProvider interface {
    GetFile(ctx context.Context, fileID string) (*FileMetadata, error)
    CopyFile(ctx context.Context, srcID, destFolderID, name string) (*FileMetadata, error)
    MoveFile(ctx context.Context, fileID, destFolderID string) (*FileMetadata, error)
    DeleteFile(ctx context.Context, fileID string) error
    RenameFile(ctx context.Context, fileID, newName string) error

    // Folder operations
    GetSubfolder(ctx context.Context, parentID, name string) (string, error)
    CreateFolder(ctx context.Context, name, parentID string) (*FileMetadata, error)
    CreateShortcut(ctx context.Context, targetID, parentID string) (*FileMetadata, error)
}

// ContentProvider handles document content operations
type ContentProvider interface {
    GetDocumentContent(ctx context.Context, fileID string) (*DocumentContent, error)
    UpdateDocumentContent(ctx context.Context, fileID, content string) error

    // Batch operations for efficiency
    GetDocumentContentBatch(ctx context.Context, fileIDs []string) ([]*DocumentContent, error)
}

// PermissionProvider handles file sharing and permissions
type PermissionProvider interface {
    ShareFile(ctx context.Context, fileID, email, role string) error
    ShareFileWithDomain(ctx context.Context, fileID, domain, role string) error
    ListPermissions(ctx context.Context, fileID string) ([]Permission, error)
    DeletePermission(ctx context.Context, fileID, permissionID string) error
}

// DirectoryProvider handles people/user directory operations
type DirectoryProvider interface {
    SearchPeople(ctx context.Context, query string) ([]Person, error)
    GetUser(ctx context.Context, email string) (*Person, error)
}

// GroupProvider handles group operations (optional, for enterprise providers)
type GroupProvider interface {
    ListGroups(ctx context.Context, domain, query string, maxResults int64) ([]Group, error)
    ListUserGroups(ctx context.Context, userEmail string) ([]Group, error)
    GetGroup(ctx context.Context, groupID string) (*Group, error)
}

// EmailProvider handles email notifications (optional)
type EmailProvider interface {
    SendEmail(ctx context.Context, to []string, from, subject, body string) error
}

// RevisionProvider handles document revision management (optional)
type RevisionProvider interface {
    GetLatestRevision(ctx context.Context, fileID string) (*Revision, error)
    KeepRevisionForever(ctx context.Context, fileID, revisionID string) (*Revision, error)
    UpdateKeepRevisionForever(ctx context.Context, fileID, revisionID string, keepForever bool) error
}

// WorkspaceProvider is a composite that may implement multiple focused interfaces
// This allows providers to opt-in to capabilities they support
type WorkspaceProvider interface {
    FileProvider
    // Optional interfaces checked via type assertion:
    // - ContentProvider (if provider supports direct content access)
    // - PermissionProvider (if provider supports permissions)
    // - DirectoryProvider (if provider has user directory)
    // - GroupProvider (if provider supports groups)
    // - EmailProvider (if provider can send emails)
    // - RevisionProvider (if provider has revision history)

    // Name returns the provider name for logging/debugging
    Name() string
}

// Capability checking pattern
func SupportsContent(provider WorkspaceProvider) bool {
    _, ok := provider.(ContentProvider)
    return ok
}

func SupportsPermissions(provider WorkspaceProvider) bool {
    _, ok := provider.(PermissionProvider)
    return ok
}

func SupportsDirectory(provider WorkspaceProvider) bool {
    _, ok := provider.(DirectoryProvider)
    return ok
}
```

**Benefits of Interface Splitting**:

1. **Separation of Concerns**: Each interface has a single, focused responsibility
2. **Capability Detection**: Check if provider supports feature via type assertion
3. **Easier Testing**: Mock only the interfaces you need for a test
4. **Incremental Implementation**: API provider can implement core FileProvider first, add others later
5. **Provider Flexibility**: Local provider might not support GroupProvider, and that's OK
6. **Clearer Documentation**: Each interface documents its own contract

3. **Adapter Pattern for Existing Providers**:

Existing providers (Google, Local) implement all focused interfaces:

```go
// GoogleWorkspaceProvider adapts Google provider to new interface design
type GoogleWorkspaceProvider struct {
    adapter *google.Adapter
}

// Compile-time checks that GoogleWorkspaceProvider implements all interfaces
var _ workspace.WorkspaceProvider = (*GoogleWorkspaceProvider)(nil)
var _ workspace.FileProvider = (*GoogleWorkspaceProvider)(nil)
var _ workspace.ContentProvider = (*GoogleWorkspaceProvider)(nil)
var _ workspace.PermissionProvider = (*GoogleWorkspaceProvider)(nil)
var _ workspace.DirectoryProvider = (*GoogleWorkspaceProvider)(nil)
var _ workspace.GroupProvider = (*GoogleWorkspaceProvider)(nil)
var _ workspace.EmailProvider = (*GoogleWorkspaceProvider)(nil)
var _ workspace.RevisionProvider = (*GoogleWorkspaceProvider)(nil)

// Name returns the provider name
func (p *GoogleWorkspaceProvider) Name() string {
    return "google"
}

// FileProvider implementation
func (p *GoogleWorkspaceProvider) GetFile(ctx context.Context, fileID string) (*workspace.FileMetadata, error) {
    driveFile, err := p.adapter.GetFile(fileID)
    if err != nil {
        return nil, err
    }
    return driveFileToMetadata(driveFile), nil
}

func (p *GoogleWorkspaceProvider) CopyFile(ctx context.Context, srcID, destFolderID, name string) (*workspace.FileMetadata, error) {
    driveFile, err := p.adapter.CopyFile(srcID, destFolderID, name)
    if err != nil {
        return nil, err
    }
    return driveFileToMetadata(driveFile), nil
}

// ContentProvider implementation
func (p *GoogleWorkspaceProvider) GetDocumentContent(ctx context.Context, fileID string) (*workspace.DocumentContent, error) {
    doc, err := p.adapter.GetDoc(fileID)
    if err != nil {
        return nil, err
    }
    return docsDocumentToContent(doc), nil
}

// PermissionProvider implementation
func (p *GoogleWorkspaceProvider) ShareFile(ctx context.Context, fileID, email, role string) error {
    return p.adapter.ShareFile(fileID, email, role)
}

// DirectoryProvider implementation
func (p *GoogleWorkspaceProvider) SearchPeople(ctx context.Context, query string) ([]workspace.Person, error) {
    people, err := p.adapter.SearchPeople(query, "emailAddresses,names,photos")
    if err != nil {
        return nil, err
    }
    return convertPeople(people), nil
}

// GroupProvider implementation (Google supports this)
func (p *GoogleWorkspaceProvider) ListGroups(ctx context.Context, domain, query string, maxResults int64) ([]workspace.Group, error) {
    groups, err := p.adapter.ListGroups(domain, query, maxResults)
    if err != nil {
        return nil, err
    }
    return convertGroups(groups), nil
}

// EmailProvider implementation (Google supports this)
func (p *GoogleWorkspaceProvider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
    return p.adapter.SendEmail(to, from, subject, body)
}

// RevisionProvider implementation (Google supports this)
func (p *GoogleWorkspaceProvider) GetLatestRevision(ctx context.Context, fileID string) (*workspace.Revision, error) {
    rev, err := p.adapter.GetLatestRevision(fileID)
    if err != nil {
        return nil, err
    }
    return driveRevisionToRevision(rev), nil
}

// Conversion helpers
func driveFileToMetadata(df *drive.File) *workspace.FileMetadata {
    return &workspace.FileMetadata{
        ID:           df.Id,
        Name:         df.Name,
        MimeType:     df.MimeType,
        CreatedTime:  parseTime(df.CreatedTime),
        ModifiedTime: parseTime(df.ModifiedTime),
        Owner:        getOwnerEmail(df.Owners),
        Parents:      df.Parents,
    }
}
```

**Local Provider** (implements subset of interfaces):

```go
// LocalWorkspaceProvider adapts local filesystem provider
type LocalWorkspaceProvider struct {
    adapter *local.Adapter
}

// Compile-time checks - Local only implements core interfaces
var _ workspace.WorkspaceProvider = (*LocalWorkspaceProvider)(nil)
var _ workspace.FileProvider = (*LocalWorkspaceProvider)(nil)
var _ workspace.ContentProvider = (*LocalWorkspaceProvider)(nil)
var _ workspace.PermissionProvider = (*LocalWorkspaceProvider)(nil)
// Note: Local does NOT implement GroupProvider, EmailProvider, or RevisionProvider

func (p *LocalWorkspaceProvider) Name() string {
    return "local"
}

// FileProvider implementation (same as Google)
func (p *LocalWorkspaceProvider) GetFile(ctx context.Context, fileID string) (*workspace.FileMetadata, error) {
    doc, err := p.adapter.DocumentStorage().GetDocument(ctx, fileID)
    if err != nil {
        return nil, err
    }
    return localDocToMetadata(doc), nil
}

// ContentProvider implementation (local supports markdown content)
func (p *LocalWorkspaceProvider) GetDocumentContent(ctx context.Context, fileID string) (*workspace.DocumentContent, error) {
    content, err := p.adapter.DocumentStorage().GetDocumentContent(ctx, fileID)
    if err != nil {
        return nil, err
    }
    return &workspace.DocumentContent{
        ID:     fileID,
        Body:   content,
        Format: "markdown",
    }, nil
}

// PermissionProvider implementation (local has basic permission model)
func (p *LocalWorkspaceProvider) ShareFile(ctx context.Context, fileID, email, role string) error {
    // Store permission in metadata
    return p.adapter.ShareFile(fileID, email, role)
}

// DirectoryProvider - Local does NOT implement this
// Type assertion will fail: _, ok := provider.(DirectoryProvider) // ok == false

// GroupProvider - Local does NOT implement this
// EmailProvider - Local does NOT implement this
// RevisionProvider - Local does NOT implement this (could add git-based later)
```

4. **Update API Handlers with Capability Detection**:

Handlers check if provider supports a capability before using it:

```go
// Old pattern (using Google types, assumes all providers support everything)
func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
    file, err := s.workspace.GetFile(docID)  // returns *drive.File
    // ...
}

// New pattern (using Hermes types with capability detection)
func (s *Server) handleGetDocumentV2(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // All providers implement FileProvider
    file, err := s.workspace.GetFile(ctx, docID)  // returns *FileMetadata
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(file)
}

// Handler that requires optional capability
func (s *Server) handleShareDocument(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Check if provider supports permissions
    permProvider, ok := s.workspace.(workspace.PermissionProvider)
    if !ok {
        http.Error(w, "provider does not support sharing", http.StatusNotImplemented)
        return
    }

    var req ShareRequest
    json.NewDecoder(r.Body).Decode(&req)

    err := permProvider.ShareFile(ctx, req.FileID, req.Email, req.Role)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}

// Handler with graceful degradation
func (s *Server) handleGetDocumentWithContent(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Get file metadata (always works)
    file, err := s.workspace.GetFile(ctx, docID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    response := map[string]any{
        "file": file,
    }

    // Try to get content if provider supports it
    if contentProvider, ok := s.workspace.(workspace.ContentProvider); ok {
        if content, err := contentProvider.GetDocumentContent(ctx, docID); err == nil {
            response["content"] = content
        }
    }

    json.NewEncoder(w).Encode(response)
}

// Handler with multiple capability checks
func (s *Server) handleSearchPeopleAndGroups(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    query := r.URL.Query().Get("q")

    response := map[string]any{}

    // Search people (if supported)
    if dirProvider, ok := s.workspace.(workspace.DirectoryProvider); ok {
        if people, err := dirProvider.SearchPeople(ctx, query); err == nil {
            response["people"] = people
        }
    }

    // Search groups (if supported)
    if groupProvider, ok := s.workspace.(workspace.GroupProvider); ok {
        if groups, err := groupProvider.ListGroups(ctx, "", query, 10); err == nil {
            response["groups"] = groups
        }
    }

    // Return what we found
    if len(response) == 0 {
        http.Error(w, "provider does not support directory search", http.StatusNotImplemented)
        return
    }

    json.NewEncoder(w).Encode(response)
}

// Helper functions for common capability checks
func (s *Server) requiresPermissions(provider workspace.WorkspaceProvider) error {
    if _, ok := provider.(workspace.PermissionProvider); !ok {
        return fmt.Errorf("provider %s does not support permissions", provider.Name())
    }
    return nil
}

func (s *Server) requiresContent(provider workspace.WorkspaceProvider) error {
    if _, ok := provider.(workspace.ContentProvider); !ok {
        return fmt.Errorf("provider %s does not support content operations", provider.Name())
    }
    return nil
}
```

**Benefits of This Pattern**:
- Handlers degrade gracefully when capabilities are missing
- Clear error messages when operations are not supported
- Providers can implement only what makes sense for their backend
- Easy to add new optional capabilities without breaking existing providers

**Migration Path**:
- Existing providers implement both `Provider` (legacy) and `ProviderV2` (new)
- API handlers gradually migrate from `Provider` to `ProviderV2`
- Once all handlers migrated, deprecate `Provider` interface
- Remove Google type dependencies from provider interface

#### Phase 2: API Provider Implementation

Once Phase 1 is complete (Hermes-native types established), implement the API provider:

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

// Provider implements focused workspace interfaces by delegating to remote Hermes API
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
var _ workspace.FileProvider = (*Provider)(nil)
var _ workspace.ContentProvider = (*Provider)(nil)
var _ workspace.PermissionProvider = (*Provider)(nil)
var _ workspace.DirectoryProvider = (*Provider)(nil)
// GroupProvider, EmailProvider, RevisionProvider implemented conditionally

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
            SupportsGroups:      false, // Conservative default
            SupportsEmail:       false,
            SupportsRevisions:   false,
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

// FileProvider implementation

// GetFile retrieves file metadata from remote Hermes
func (p *Provider) GetFile(ctx context.Context, fileID string) (*workspace.FileMetadata, error) {
    url := fmt.Sprintf("%s/api/v2/documents/%s", p.config.BaseURL, fileID)

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

    var file workspace.FileMetadata
    if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &file, nil
}

// CopyFile delegates to remote Hermes API
func (p *Provider) CopyFile(ctx context.Context, srcID, destFolderID, name string) (*workspace.FileMetadata, error) {
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

    var file workspace.FileMetadata
    json.NewDecoder(resp.Body).Decode(&file)
    return &file, nil
}

// ... other FileProvider methods (Move, Delete, Rename, CreateFolder, etc.)

// ContentProvider implementation

func (p *Provider) GetDocumentContent(ctx context.Context, fileID string) (*workspace.DocumentContent, error) {
    if !p.capabilities.SupportsContent {
        return nil, fmt.Errorf("remote provider does not support content operations")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/content", p.config.BaseURL, fileID)

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

func (p *Provider) UpdateDocumentContent(ctx context.Context, fileID, content string) error {
    if !p.capabilities.SupportsContent {
        return fmt.Errorf("remote provider does not support content operations")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/content", p.config.BaseURL, fileID)

    body, _ := json.Marshal(map[string]string{"content": content})
    req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("API returned status %d", resp.StatusCode)
    }

    return nil
}

// PermissionProvider implementation

func (p *Provider) ShareFile(ctx context.Context, fileID, email, role string) error {
    if !p.capabilities.SupportsPermissions {
        return fmt.Errorf("remote provider does not support permissions")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/permissions", p.config.BaseURL, fileID)

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

func (p *Provider) ListPermissions(ctx context.Context, fileID string) ([]workspace.Permission, error) {
    if !p.capabilities.SupportsPermissions {
        return nil, fmt.Errorf("remote provider does not support permissions")
    }

    url := fmt.Sprintf("%s/api/v2/documents/%s/permissions", p.config.BaseURL, fileID)

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var perms []workspace.Permission
    json.NewDecoder(resp.Body).Decode(&perms)
    return perms, nil
}

// DirectoryProvider implementation

func (p *Provider) SearchPeople(ctx context.Context, query string) ([]workspace.Person, error) {
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

    var people []workspace.Person
    json.NewDecoder(resp.Body).Decode(&people)
    return people, nil
}

// ... implement other interfaces (GroupProvider, EmailProvider, RevisionProvider)
// Each checks p.capabilities before making API call
```

### Configuration

Add API provider configuration to `internal/config/config.go`:

```hcl
# Example: Edge Hermes that delegates to central instance
providers {
  workspace = "api"
  search    = "api"
}

# API workspace provider config
api_workspace {
  base_url  = "https://central.hermes.example.com"
  auth_token = env("HERMES_API_TOKEN")
  tls_verify = true
  timeout    = "30s"
}

# API search provider config (optional, if search also proxied)
api_search {
  base_url  = "https://central.hermes.example.com"
  auth_token = env("HERMES_API_TOKEN")
}
```

### API Contract Requirements

For the API provider to work, the remote Hermes instance must expose consistent REST APIs:

**Required Endpoints**:
- `GET /api/v2/documents/:id` - Get file metadata
- `POST /api/v2/documents/:id/copy` - Copy document
- `PUT /api/v2/documents/:id/move` - Move document
- `DELETE /api/v2/documents/:id` - Delete document
- `PATCH /api/v2/documents/:id` - Rename/update metadata
- `GET /api/v2/documents/:id/content` - Get document content
- `PUT /api/v2/documents/:id/content` - Update document content
- `GET /api/v2/documents/:id/permissions` - List permissions
- `POST /api/v2/documents/:id/permissions` - Add permission
- `DELETE /api/v2/documents/:id/permissions/:permId` - Remove permission
- `GET /api/v2/people/search?q=:query` - Search directory
- `POST /api/v2/folders` - Create folder
- `GET /api/v2/folders/:id/subfolders/:name` - Get subfolder

**Authentication**:
- Support Bearer token authentication
- Token-based API keys for machine-to-machine communication

**Response Format**:
All endpoints return Hermes-native types (not Google types):
```json
{
  "id": "uuid:550e8400-...:provider:api:id:remote-123",
  "name": "RFC-001.md",
  "mimeType": "text/markdown",
  "createdTime": "2025-01-15T10:30:00Z",
  "modifiedTime": "2025-01-16T14:20:00Z",
  "owner": "user@example.com",
  "parents": ["folder-uuid"],
  "permissions": [...]
}
```

## Implementation Plan

### Phase 1: Type Abstraction (Weeks 1-3)

**Week 1: Define Hermes Types**
- [ ] Create `pkg/workspace/types.go` with native types
- [ ] Define `FileMetadata`, `DocumentContent`, `Person`, `Permission`
- [ ] Add JSON/HCL serialization tags
- [ ] Write comprehensive tests for type conversions

**Week 2: Create Focused Provider Interfaces**
- [ ] Define focused interfaces: `FileProvider`, `ContentProvider`, `PermissionProvider`
- [ ] Define optional interfaces: `DirectoryProvider`, `GroupProvider`, `EmailProvider`, `RevisionProvider`
- [ ] Define composite `WorkspaceProvider` interface
- [ ] Add capability detection helpers (`SupportsContent()`, `SupportsPermissions()`, etc.)
- [ ] Create adapter pattern for Google provider (implements all interfaces)
- [ ] Create adapter pattern for Local provider (implements core interfaces)
- [ ] Ensure backward compatibility with existing `Provider` interface

**Week 3: Migrate Core API Handlers**
- [ ] Update document CRUD handlers to use `ProviderV2`
- [ ] Update permission handlers
- [ ] Update content handlers
- [ ] Add feature flag for gradual rollout

**Testing Phase 1**:
- Run full E2E test suite with both Google and Local providers
- Verify no regressions in existing functionality
- Performance benchmarks (should be equivalent)

### Phase 2: API Provider Implementation (Weeks 4-6)

**Week 4: API Provider Foundation**
- [ ] Create `pkg/workspace/adapters/api/` package
- [ ] Implement configuration (`api_workspace` HCL block)
- [ ] Implement HTTP client with authentication
- [ ] Add retry logic and circuit breakers
- [ ] Implement basic file operations (Get, Copy, Move, Delete)

**Week 5: Complete API Provider**
- [ ] Implement content operations
- [ ] Implement permission operations
- [ ] Implement directory/people operations
- [ ] Implement folder operations
- [ ] Add comprehensive error handling

**Week 6: REST API Endpoints**
- [ ] Create new API v2 endpoints for all operations
- [ ] Add authentication middleware
- [ ] Add rate limiting
- [ ] Comprehensive API documentation

**Testing Phase 2**:
- Integration tests with local Hermes → local Hermes
- Integration tests with API provider → Google provider
- Performance testing (latency, throughput)
- Failure mode testing (network errors, auth failures)

### Phase 3: Documentation & Deployment (Week 7)

- [ ] Write ADR documenting the architectural decision
- [ ] Create MEMO with deployment patterns
- [ ] Update configuration documentation
- [ ] Create example configurations for common patterns
- [ ] Migration guide for existing deployments

## Design Decisions

### Decision 1: Two-Phase Approach

**Options Considered**:
1. **Big Bang**: Refactor everything at once
2. **Two-Phase**: Types first, then API provider
3. **Parallel Development**: Build API provider alongside type refactoring

**Decision**: Two-Phase Approach

**Rationale**:
- **Risk Mitigation**: Phase 1 can be tested independently with existing providers
- **Clear Milestones**: Each phase delivers value and can be validated
- **Team Velocity**: Allows parallel work (type refactoring + API design)
- **Rollback Path**: Can abandon Phase 2 if Phase 1 reveals fundamental issues

### Decision 2: Hermes-Native Types vs. Generic Interface

**Options Considered**:
1. **Keep Google Types**: Maintain `*drive.File` in interface
2. **Hermes-Native Types**: Define `FileMetadata` struct
3. **Generic Interface**: Use `map[string]any` for flexibility

**Decision**: Hermes-Native Types

**Rationale**:
- **Type Safety**: Compile-time validation of field access
- **Clear Contracts**: Explicit schema for API responses
- **Maintainability**: Easier to evolve and document
- **Performance**: No runtime type assertions needed
- **Trade-off**: Requires mapping code, but worth it for clarity

### Decision 3: Interface Splitting vs. Monolithic Interface

**Options Considered**:
1. **Monolithic Interface**: Single `WorkspaceProvider` with all ~30 methods
2. **Focused Interfaces**: Split into `FileProvider`, `ContentProvider`, `PermissionProvider`, etc.
3. **Plugin Architecture**: Dynamic capability registration system

**Decision**: Focused Interfaces

**Rationale**:
- **Separation of Concerns**: Each interface has single responsibility (files, content, permissions, etc.)
- **Optional Capabilities**: Providers opt-in to features they support via interface implementation
- **Easier Testing**: Mock only what's needed (e.g., just `FileProvider` for file operation tests)
- **Incremental Implementation**: API provider can start with core `FileProvider`, add others later
- **Clear Documentation**: Each interface's contract is self-contained and focused
- **Compile-Time Safety**: Type assertions catch capability mismatches at runtime
- **Provider Flexibility**:
  - Google provider implements all 7 interfaces (full-featured)
  - Local provider implements 4 interfaces (no groups, email, revisions)
  - API provider implements what remote backend supports

**Benefits Over Monolithic**:
- Providers not forced to implement stub methods for unsupported features
- API handlers can gracefully degrade when capabilities missing
- New capabilities can be added without breaking existing providers
- Clearer intent: "Does this provider support X?" → type assertion check

**Trade-offs**:
- More interfaces to maintain (7 instead of 1)
- API handlers need capability checks (type assertions)
- Slightly more boilerplate in provider implementations

**Accepted Trade-off**: The flexibility and clarity gained from focused interfaces far outweighs the minor increase in interface definitions. Go's interface composition makes this pattern natural and idiomatic.

### Decision 4: REST API vs. gRPC

**Options Considered**:
1. **REST API**: HTTP/JSON endpoints
2. **gRPC**: Protocol Buffers + HTTP/2
3. **GraphQL**: Flexible query language

**Decision**: REST API (Initially)

**Rationale**:
- **Existing Pattern**: Hermes already has REST API infrastructure
- **Simplicity**: Easier to debug, test, and document
- **Broad Compatibility**: Any HTTP client can integrate
- **Future-Proof**: Can add gRPC later as optimization
- **Trade-off**: Slightly higher latency than gRPC, but acceptable for MVP

### Decision 5: Synchronous vs. Asynchronous Operations

**Options Considered**:
1. **Synchronous**: Direct HTTP calls, wait for response
2. **Asynchronous**: Queue-based with callbacks
3. **Hybrid**: Sync for reads, async for writes

**Decision**: Synchronous (Initially)

**Rationale**:
- **Simplicity**: Matches existing provider interface contract
- **Predictable**: Easier to reason about state and errors
- **Sufficient**: Network latency acceptable for most operations
- **Future Extension**: Can add async for bulk operations later
- **Trade-off**: Higher latency for bulk operations, but rare in practice

## Benefits

### 1. Architectural Flexibility

**Multi-Tier Deployments**:
```
Edge Hermes (simplified mode) → Central Hermes (full provider)
```
- Edge nodes can run simplified Hermes (SQLite, no Google creds)
- All document storage delegated to central instance
- Central instance handles all provider complexity

**Federated Document Management**:
```
Team A Hermes ──┐
Team B Hermes ──┼──> Aggregator Hermes (consolidated view)
Team C Hermes ──┘
```
- Each team runs independent Hermes instance
- Aggregator provides unified search/view across all teams
- Teams maintain autonomy over their own documents

### 2. Simplified Deployment

**Before** (Every instance needs full provider setup):
```
Hermes Instance
├── Google Workspace credentials
├── Algolia API keys
├── OIDC configuration
├── Full database
└── All provider code
```

**After** (Edge nodes are lightweight):
```
Edge Hermes                Central Hermes
├── API token      →       ├── Google credentials
└── SQLite                 ├── Algolia keys
                           ├── PostgreSQL
                           └── Full provider stack
```

### 3. Improved Testing

**Integration Testing**:
- Can test API provider with mock Hermes backend
- No need for Google credentials in CI/CD
- Faster test execution (no external API calls)

**Development Workflow**:
- Developers can run thin Hermes locally
- Delegate to shared dev Hermes instance
- Reduces local setup complexity

### 4. Future Capabilities

**Progressive Enhancement**:
- Start with basic API provider
- Add caching layer for performance
- Add intelligent routing (multi-backend)
- Add conflict resolution for distributed writes

**Hybrid Deployments**:
```
Hermes Instance
├── Google provider (primary storage)
├── API provider (backup/archive to remote)
└── Local provider (offline cache)
```

## Risks & Mitigations

### Risk 1: Performance Degradation

**Risk**: Additional network hop adds latency

**Mitigation**:
- Implement response caching for read-heavy operations
- Add batch API endpoints for bulk operations
- Use HTTP/2 for connection multiplexing
- Add performance monitoring and alerting

**Acceptable Trade-off**: Slightly higher latency acceptable for deployment flexibility gained

### Risk 2: Increased Complexity

**Risk**: Two provider implementations to maintain (native + API)

**Mitigation**:
- Comprehensive integration tests
- Clear documentation and examples
- Feature parity matrix between providers
- Dedicated ownership and on-call rotation

### Risk 3: API Versioning Challenges

**Risk**: API contract changes break compatibility

**Mitigation**:
- Semantic versioning for API endpoints
- Support multiple API versions concurrently
- Graceful degradation for missing features
- Automated compatibility testing

### Risk 4: Authentication Complexity

**Risk**: Secure API token management and rotation

**Mitigation**:
- Support multiple auth methods (Bearer, mTLS)
- Token expiration and automatic renewal
- Audit logging for all API access
- Integration with secret management systems (Vault)

## Success Metrics

### Phase 1 Success Criteria
- [ ] All existing E2E tests pass with ProviderV2
- [ ] No performance regression (< 5% latency increase)
- [ ] 100% feature parity with existing Provider interface
- [ ] Zero production incidents during rollout

### Phase 2 Success Criteria
- [ ] API provider successfully delegates to remote Hermes
- [ ] End-to-end document lifecycle works through API provider
- [ ] < 100ms additional latency for API provider operations
- [ ] Successful multi-tier deployment in staging environment

### Overall Success Criteria
- [ ] Production deployment of edge → central architecture
- [ ] 50% reduction in edge node setup complexity
- [ ] Positive feedback from operations team
- [ ] Documentation and runbooks completed

## Open Questions

1. **API Versioning Strategy**: Should we use `/api/v2/`, `/api/v3/`, or date-based versioning?

2. **Batch Operations**: Should we add batch endpoints (e.g., `POST /api/v2/documents/batch/copy`) in Phase 2 or later?

3. **Streaming Support**: Do we need streaming for large document content? Or is chunked transfer encoding sufficient?

4. **Caching Strategy**: Should API provider implement its own cache, or rely on HTTP caching headers?

5. **Error Propagation**: How should provider-specific errors (Google quota, filesystem permissions) be represented in API responses?

6. **Capabilities Discovery**: Should remote Hermes expose a `/capabilities` endpoint so API provider can adapt to backend features?

## Alternatives Considered

### Alternative 1: GraphQL Instead of REST

**Pros**:
- Flexible queries, reduce over-fetching
- Strong typing with schema
- Built-in introspection

**Cons**:
- Higher learning curve
- More complex implementation
- Overkill for simple CRUD operations

**Decision**: REST is simpler and sufficient for MVP. Can add GraphQL later if needed.

### Alternative 2: Keep Google Types, Add Conversion Layer

**Pros**:
- Less code change in existing handlers
- Faster initial implementation

**Cons**:
- API tied to Google semantics forever
- Hard to evolve independently
- Confusing for non-Google providers

**Decision**: Clean break with Hermes-native types is better long-term investment.

### Alternative 3: Microservices Architecture

**Pros**:
- Each provider as separate service
- Independent scaling and deployment

**Cons**:
- Massive increase in operational complexity
- Network calls for every operation
- Harder to reason about transactions

**Decision**: Too much complexity for current needs. Modular monolith with provider abstraction is sufficient.

## References

- **RFC-082**: Document Identification System - UUID-based document identity
- **RFC-007**: Multi-Provider Auth Architecture - Provider pattern precedent
- **MEMO-019**: Development Velocity Analysis - 10-15x speedup with grounded development
- **Provider Implementations**:
  - `pkg/workspace/adapters/google/` - Google Workspace provider
  - `pkg/workspace/adapters/local/` - Local filesystem provider
- **API Handlers**: `internal/api/v2/` - Existing REST API patterns

## Timeline

- **Week 1-3**: Phase 1 (Type Abstraction)
- **Week 4-6**: Phase 2 (API Provider Implementation)
- **Week 7**: Documentation & Deployment
- **Total**: 7 weeks to production-ready API provider

## Conclusion

This RFC proposes a two-phase refactoring of Hermes provider architecture to support remote API delegation. Phase 1 establishes Hermes-native types and ProviderV2 interface, enabling Phase 2 implementation of the API provider.

**Key Benefits**:
- Multi-tier and federated deployment architectures
- Simplified edge node setup (API token vs. full provider credentials)
- Improved testability (mock remote Hermes instead of Google APIs)
- Foundation for future enhancements (caching, routing, hybrid backends)

**Risks are manageable** through incremental rollout, comprehensive testing, and clear documentation.

**Recommendation**: Approve RFC and proceed with Phase 1 implementation.
