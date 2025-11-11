---
id: RFC-085
title: Multi-Provider Architecture with Automatic Pass-Through and Document Synchronization
date: 2025-11-11
type: RFC
subtype: Implementation
status: Proposed
tags: [multi-provider, pass-through, federation, synchronization, uuid-merging, identity-joining]
related:
  - RFC-084
  - RFC-086
  - RFC-082
---

# Multi-Provider Architecture with Automatic Pass-Through and Document Synchronization

## Executive Summary

This RFC proposes a multi-provider architecture for Hermes that enables simultaneous operation with multiple workspace providers (local + remote), automatic pass-through routing, document UUID merging for drift scenarios, cross-provider identity joining, and replicated notifications between edge and central instances.

**Key Benefits**:
- **Multi-Provider Support**: Run multiple providers simultaneously (e.g., local Git + API pass-through to central)
- **Automatic Pass-Through**: Edge Hermes automatically routes requests to appropriate provider
- **Document Synchronization**: Documents authored locally can be tracked centrally with revision state management
- **UUID Merging**: Handle document drift by merging two document UUIDs and their revision histories
- **Identity Joining**: Join identities from multiple authentication providers through UI
- **Replicated Notifications**: Both edge and central Hermes can send notifications
- **Federated Document Management**: Track documents across multiple Hermes instances

**Related RFCs**:
- **RFC-084**: Provider Interface Refactoring (defines the 7 required interfaces)
- **RFC-086**: Authentication and Bearer Token Management (auth strategy)
- **RFC-082**: Document Identification System (UUID + ProviderID)

## Context

### Use Cases for Multi-Provider Architecture

**Use Case 1: Local Authoring with Central Tracking**
```
┌────────────────────────────────────────────┐
│ Edge Hermes (Developer Laptop)             │
├────────────────────────────────────────────┤
│ Multi-Provider Configuration:              │
│  1. Local Git Provider (primary)           │
│  2. API Provider → Central Hermes          │
│                                             │
│ Flow:                                       │
│  • Author document in local Git repo       │
│  • Hermes tracks UUID locally              │
│  • Auto-sync metadata to Central           │
│  • Central tracks document + revisions     │
│  • Notifications sent by both instances    │
└────────────────────────────────────────────┘
         │ (auto pass-through)
         ▼
┌────────────────────────────────────────────┐
│ Central Hermes (Company Server)            │
├────────────────────────────────────────────┤
│  • Tracks all documents globally           │
│  • Maintains UUID registry                 │
│  • Manages document revision states        │
│  • Central identity provider               │
└────────────────────────────────────────────┘
```

**Use Case 2: Document UUID Merging (Drift Resolution)**
```
Scenario: Same document authored independently in two locations

Edge Hermes:
  UUID: 550e8400-e29b-41d4-a716-446655440000
  ProviderID: local:docs/rfc-010.md
  Revisions: Git commits a1b2c3, d4e5f6

Central Hermes (independently created):
  UUID: 7e8f4a2c-9d5b-4c1e-a8f7-3b2d1e6c9a4f
  ProviderID: google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs
  Revisions: Google Doc rev 1, 2, 3

Resolution via UUID Merging:
  1. User identifies documents are the same
  2. Initiates merge via UI
  3. System merges UUIDs → keeps 550e8400... (canonical)
  4. All revisions combined under single UUID
  5. Both providers tracked as different backends
  6. Sync status: canonical (central), mirror (edge)
```

**Use Case 3: Cross-Provider Identity Joining**
```
Developer has multiple identities:
  • jacob.repp@hashicorp.com (Google OAuth)
  • jrepp@ibm.com (IBM Verify)
  • jacob-repp (GitHub)

Flow:
  1. Developer logs in to Edge Hermes with local identity
  2. Via UI, clicks "Join Identity" to connect to Central
  3. Central Hermes prompts for authentication
  4. Upon success, identities linked via UnifiedUserID
  5. Documents authored on edge attributed correctly
  6. Permissions propagate across identities
```

**Use Case 4: Replicated Notifications**
```
Document workflow requiring notifications:
  • Document authored on Edge Hermes (local Git)
  • Review requested via Edge UI
  • Edge Hermes sends notification: "Review requested"
  • Central Hermes also notified of state change
  • Central sends notification: "Document awaiting review"
  • Both notifications reach reviewers
  • Ensures delivery even if Edge offline
```

## Proposed Solution

### Multi-Provider Architecture Overview

**Edge Hermes with Multiple Providers**:
```
┌─────────────────────────────────────────────────────────────────┐
│                   Edge Hermes (Developer)                       │
├─────────────────────────────────────────────────────────────────┤
│  Multi-Provider Manager (NEW)                                   │
│  - Routes requests to appropriate provider(s)                   │
│  - Handles automatic pass-through                               │
│  - Manages document synchronization                             │
│                                                                  │
│  ┌──────────────────┐          ┌──────────────────────────┐    │
│  │ Local Provider   │          │ API Provider (Central)   │    │
│  │ (Primary)        │          │ (Pass-through)           │    │
│  ├──────────────────┤          ├──────────────────────────┤    │
│  │ • DocumentProv   │          │ • All interfaces         │    │
│  │ • ContentProv    │          │   delegate to Central    │    │
│  │ • RevisionProv   │          │                          │    │
│  └────────┬─────────┘          └───────────┬──────────────┘    │
│           │                                 │                    │
└───────────┼─────────────────────────────────┼────────────────────┘
            │                                 │ REST API
            │ Local Git                       │ /api/v2/*
            ▼                                 ▼
   ┌────────────────┐              ┌─────────────────────────────┐
   │ Local Git Repo │              │ Central Hermes              │
   │ docs/          │              ├─────────────────────────────┤
   └────────────────┘              │ • UUID Registry             │
                                   │ • Document Metadata DB      │
                                   │ • Revision Tracking         │
                                   │ • Identity Management       │
                                   │ • Notification Hub          │
                                   │                             │
                                   │ Workspace Providers:        │
                                   │  - Google Workspace         │
                                   │  - Local                    │
                                   │  - GitHub                   │
                                   └─────────────────────────────┘
```

### Automatic Pass-Through Routing

The Multi-Provider Manager intelligently routes operations:

```go
// Multi-Provider Manager routes requests to appropriate provider(s)
type MultiProviderManager struct {
    primary   workspace.WorkspaceProvider  // Local provider
    secondary workspace.WorkspaceProvider  // API provider (central)
    registry  *DocumentRegistry            // UUID → Provider mapping
}

// GetDocument with automatic routing
func (m *MultiProviderManager) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
    // Try primary provider first
    doc, err := m.primary.GetDocument(ctx, providerID)
    if err == nil {
        return doc, nil
    }

    // If not found locally, try secondary (central)
    doc, err = m.secondary.GetDocument(ctx, providerID)
    if err == nil {
        // Document exists in central, cache locally
        m.registry.Register(doc.UUID, "secondary")
        return doc, nil
    }

    return nil, fmt.Errorf("document not found in any provider")
}

// CreateDocument decides where to create based on policy
func (m *MultiProviderManager) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
    // New documents created in primary (local) provider
    doc, err := m.primary.CreateDocument(ctx, templateID, destFolderID, name)
    if err != nil {
        return nil, err
    }

    // Automatically register with central for tracking
    go m.syncToSecondary(context.Background(), doc)

    return doc, nil
}

// syncToSecondary replicates document metadata to central
func (m *MultiProviderManager) syncToSecondary(ctx context.Context, doc *workspace.DocumentMetadata) {
    // Send document metadata to central via API
    _, err := m.secondary.RegisterDocument(ctx, doc)
    if err != nil {
        log.Error("failed to sync document to central", "uuid", doc.UUID, "error", err)
    }
}
```

### Document UUID Merging for Drift Resolution

When the same document is independently created in multiple locations, Hermes provides UUID merging to combine them:

```go
// DocumentMergeService handles UUID merging for drift scenarios
type DocumentMergeService struct {
    db       *sql.DB
    registry *DocumentRegistry
}

// MergeRequest represents a request to merge two document UUIDs
type MergeRequest struct {
    SourceUUID      docid.UUID `json:"sourceUUID"`      // UUID to be merged (deprecated)
    TargetUUID      docid.UUID `json:"targetUUID"`      // UUID to keep (canonical)
    MergeRevisions  bool       `json:"mergeRevisions"`  // Merge revision histories
    MergeStrategy   string     `json:"mergeStrategy"`   // "keep-target", "keep-source", "merge-all"
    InitiatedBy     string     `json:"initiatedBy"`     // User email
}

// MergeDocuments merges two document UUIDs
func (s *DocumentMergeService) MergeDocuments(ctx context.Context, req *MergeRequest) error {
    // 1. Validate both documents exist
    sourceDoc, err := s.registry.GetDocument(ctx, req.SourceUUID)
    if err != nil {
        return fmt.Errorf("source document not found: %w", err)
    }

    targetDoc, err := s.registry.GetDocument(ctx, req.TargetUUID)
    if err != nil {
        return fmt.Errorf("target document not found: %w", err)
    }

    // 2. Merge revision histories
    if req.MergeRevisions {
        if err := s.mergeRevisions(ctx, req.SourceUUID, req.TargetUUID); err != nil {
            return fmt.Errorf("failed to merge revisions: %w", err)
        }
    }

    // 3. Update all references to source UUID → target UUID
    if err := s.updateUUIDReferences(ctx, req.SourceUUID, req.TargetUUID); err != nil {
        return fmt.Errorf("failed to update references: %w", err)
    }

    // 4. Mark source UUID as merged (soft delete)
    if err := s.markAsMerged(ctx, req.SourceUUID, req.TargetUUID); err != nil {
        return fmt.Errorf("failed to mark as merged: %w", err)
    }

    // 5. Update sync status for merged document
    if err := s.updateSyncStatus(ctx, targetDoc, sourceDoc); err != nil {
        return fmt.Errorf("failed to update sync status: %w", err)
    }

    log.Info("documents merged successfully",
        "sourceUUID", req.SourceUUID,
        "targetUUID", req.TargetUUID,
        "initiatedBy", req.InitiatedBy)

    return nil
}

// mergeRevisions combines revision histories from both documents
func (s *DocumentMergeService) mergeRevisions(ctx context.Context, sourceUUID, targetUUID docid.UUID) error {
    // Get all revisions for source document
    sourceRevs, err := s.registry.GetAllDocumentRevisions(ctx, sourceUUID)
    if err != nil {
        return err
    }

    // Re-assign all revisions to target UUID
    for _, rev := range sourceRevs {
        rev.UUID = targetUUID
        if err := s.registry.SaveRevision(ctx, rev); err != nil {
            return fmt.Errorf("failed to save revision: %w", err)
        }
    }

    return nil
}

// updateUUIDReferences updates all database references
func (s *DocumentMergeService) updateUUIDReferences(ctx context.Context, oldUUID, newUUID docid.UUID) error {
    queries := []string{
        `UPDATE document_metadata SET uuid = $1 WHERE uuid = $2`,
        `UPDATE document_revisions SET document_uuid = $1 WHERE document_uuid = $2`,
        `UPDATE document_permissions SET document_uuid = $1 WHERE document_uuid = $2`,
        `UPDATE document_comments SET document_uuid = $1 WHERE document_uuid = $2`,
    }

    for _, query := range queries {
        if _, err := s.db.ExecContext(ctx, query, newUUID, oldUUID); err != nil {
            return err
        }
    }

    return nil
}
```

**UUID Merge API Endpoint**:
```
POST /api/v2/documents/merge
Authorization: Bearer <token>

{
  "sourceUUID": "7e8f4a2c-9d5b-4c1e-a8f7-3b2d1e6c9a4f",
  "targetUUID": "550e8400-e29b-41d4-a716-446655440000",
  "mergeRevisions": true,
  "mergeStrategy": "merge-all"
}

Response:
{
  "success": true,
  "targetUUID": "550e8400-e29b-41d4-a716-446655440000",
  "mergedRevisionCount": 8,
  "syncStatus": "canonical"
}
```

### Cross-Provider Identity Joining

Allow users to join multiple authentication provider identities through the UI:

```go
// IdentityJoinService manages cross-provider identity linking
type IdentityJoinService struct {
    db           *sql.DB
    authProvider auth.Provider
}

// JoinIdentityRequest represents a request to join identities
type JoinIdentityRequest struct {
    PrimaryEmail    string `json:"primaryEmail"`    // Current user email
    ProviderToJoin  string `json:"providerToJoin"`  // "google", "ibm-verify", "github", etc.
    AuthToken       string `json:"authToken"`       // OAuth token for provider
}

// JoinIdentity links a new provider identity to existing user
func (s *IdentityJoinService) JoinIdentity(ctx context.Context, req *JoinIdentityRequest) (*workspace.UserIdentity, error) {
    // 1. Get current user identity
    currentUser, err := s.getUserIdentity(ctx, req.PrimaryEmail)
    if err != nil {
        return nil, fmt.Errorf("current user not found: %w", err)
    }

    // 2. Validate auth token with provider
    providerIdentity, err := s.authProvider.ValidateToken(ctx, req.ProviderToJoin, req.AuthToken)
    if err != nil {
        return nil, fmt.Errorf("failed to validate provider token: %w", err)
    }

    // 3. Check if identity already linked to another user
    existingUser, err := s.findUserByProviderEmail(ctx, providerIdentity.Email)
    if err == nil && existingUser.UnifiedUserID != currentUser.UnifiedUserID {
        return nil, fmt.Errorf("identity already linked to different user")
    }

    // 4. Create alternate identity record
    altIdentity := workspace.AlternateIdentity{
        Email:          providerIdentity.Email,
        Provider:       req.ProviderToJoin,
        ProviderUserID: providerIdentity.ID,
    }

    // 5. Add to user's alternate identities
    if err := s.addAlternateIdentity(ctx, currentUser.UnifiedUserID, altIdentity); err != nil {
        return nil, fmt.Errorf("failed to add alternate identity: %w", err)
    }

    // 6. Update user identity
    currentUser.AlternateEmails = append(currentUser.AlternateEmails, altIdentity)

    log.Info("identity joined successfully",
        "primaryEmail", req.PrimaryEmail,
        "provider", req.ProviderToJoin,
        "providerEmail", providerIdentity.Email)

    return currentUser, nil
}
```

**Identity Join API Flow**:
```
1. UI: User clicks "Join Identity" button
   GET /api/v2/identity/join/initiate?provider=github

   Response:
   {
     "authURL": "https://github.com/login/oauth/authorize?...",
     "state": "random-state-token"
   }

2. User authenticates with provider (OAuth flow)

3. Provider redirects back with code
   POST /api/v2/identity/join/complete
   {
     "provider": "github",
     "code": "oauth-code",
     "state": "random-state-token"
   }

   Response:
   {
     "success": true,
     "userIdentity": {
       "email": "jacob.repp@hashicorp.com",
       "unifiedUserId": "user-12345",
       "alternateEmails": [
         {"email": "jrepp@ibm.com", "provider": "ibm-verify"},
         {"email": "jacob-repp", "provider": "github", "providerUserId": "87654321"}
       ]
     }
   }
```

### Replicated Notifications

Both edge and central Hermes can send notifications:

```go
// NotificationReplicator handles notification replication across instances
type NotificationReplicator struct {
    localNotifier  workspace.NotificationProvider  // Local notification provider
    centralClient  *APIProvider                    // Central Hermes API
}

// SendNotificationWithReplication sends via both local and central
func (r *NotificationReplicator) SendNotification(ctx context.Context, notification *Notification) error {
    var wg sync.WaitGroup
    var localErr, centralErr error

    // Send via local provider
    wg.Add(1)
    go func() {
        defer wg.Done()
        localErr = r.localNotifier.SendEmail(ctx, notification.To, notification.From, notification.Subject, notification.Body)
        if localErr != nil {
            log.Warn("local notification failed", "error", localErr)
        }
    }()

    // Send via central Hermes (for replication)
    wg.Add(1)
    go func() {
        defer wg.Done()
        centralErr = r.centralClient.SendEmail(ctx, notification.To, notification.From, notification.Subject, notification.Body)
        if centralErr != nil {
            log.Warn("central notification failed", "error", centralErr)
        }
    }()

    wg.Wait()

    // Success if at least one succeeds
    if localErr != nil && centralErr != nil {
        return fmt.Errorf("both local and central notifications failed: local=%v, central=%v", localErr, centralErr)
    }

    return nil
}
```

**Notification Replication Flow**:
```
Edge Hermes:
  1. User requests document review
  2. NotificationReplicator.SendNotification() called
  3. Sends email via local SMTP (if configured)
  4. Also sends via API to Central: POST /api/v2/notifications/email
  5. Both notifications tracked for delivery confirmation

Central Hermes:
  1. Receives notification request from Edge
  2. Sends email via its NotificationProvider (Google, SendGrid, etc.)
  3. Logs delivery for audit trail
  4. Returns success/failure to Edge

Benefits:
  • Redundant delivery ensures notifications reach recipients
  • Central has audit log of all notifications
  • Edge can operate offline (queues to Central when reconnected)
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

### Provider Feature Matrix

The multi-provider architecture supports these key capabilities:

| Feature | Edge (Local + API) | Central | Notes |
|---------|-------------------|---------|-------|
| **Document Authoring** | ✅ Local Primary | ✅ All Providers | Documents authored against connected Hermes instance |
| **Automatic Pass-Through** | ✅ Yes | N/A | Routes requests to appropriate provider automatically |
| **Document Synchronization** | ✅ Yes | ✅ Yes | Edge → Central metadata sync for tracking |
| **UUID Merging** | ✅ Via API to Central | ✅ Yes | Merge duplicate UUIDs, combine revision histories |
| **Revision Tracking** | ✅ Local (Git) | ✅ Multi-Backend | Tracks revisions across all backends |
| **Identity Joining** | ✅ Via UI | ✅ Yes | Join provider identities through OAuth flow |
| **Notification Replication** | ✅ Yes | ✅ Yes | Both instances can send notifications |
| **Offline Operation** | ✅ Yes (primary only) | N/A | Edge works offline, syncs when reconnected |
| **Multi-Backend Documents** | ✅ Via Pass-Through | ✅ Yes | Single doc tracked across multiple providers |

**Key Scenarios Supported**:

| Scenario | Edge Configuration | Central Configuration | Flow |
|----------|-------------------|----------------------|------|
| **Local Authoring + Central Tracking** | Local (primary) + API (secondary) | Google + Local | Author in Edge Git → Auto-sync to Central → Central tracks globally |
| **UUID Drift Resolution** | Any | Any | User initiates merge → Central combines UUIDs → All revisions under single UUID |
| **Identity Joining** | Local auth + OAuth | Multiple auth providers | User logs in Edge → UI "Join Identity" → OAuth to Central → Identities linked |
| **Notification Redundancy** | SMTP (optional) + API pass-through | Google/SendGrid | Edge sends → Also POST to Central → Central sends → Both logged |
| **Offline Authoring** | Local only (queue sync) | N/A | Work offline → Queue metadata → Sync when online |

**Provider Implementation Matrix**:

| Provider | Document | Content | Revision | Permission | People | Team | Notification | UUID Merging | Identity Join |
|----------|----------|---------|----------|------------|--------|------|--------------|--------------|---------------|
| **Local (Edge)** | 🟢 Local | 🟢 Local | 🟢 Git | 🟡 Basic | 🔵 API | 🔵 API | 🔵 API | 🔵 API | 🔵 API |
| **API (Edge)** | 🔵 API | 🔵 API | 🔵 API | 🔵 API | 🔵 API | 🔵 API | 🔵 API | 🔵 API | 🔵 API |
| **Google (Central)** | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Local | 🟢 Service | 🟢 Service |
| **Multi-Provider (Edge)** | 🟣 Multi | 🟣 Multi | 🟣 Multi | 🟣 Multi | 🔵 API | 🔵 API | 🟣 Replicated | 🔵 API | 🔵 API |

**Legend**:
- 🟢 **Local**: Implemented directly
- 🔵 **API**: Delegated to Central via REST API
- 🟡 **Basic**: Simple local implementation
- 🟣 **Multi**: Routes to multiple providers
- 🟣 **Replicated**: Executes on both Edge and Central

### Configuration Patterns

**Pattern 1: Multi-Provider Edge with Automatic Pass-Through**:
```hcl
# Edge Hermes with local Git + API pass-through to Central
providers {
  workspace = "multi"  # NEW: Multi-provider manager
  search    = "meilisearch"
}

multi_workspace {
  # Primary provider (local authoring)
  primary = "local"

  # Secondary provider (central tracking)
  secondary = "api"

  # Routing policy
  routing_policy = "primary_first"  # Try primary, fall back to secondary

  # Auto-sync configuration
  auto_sync {
    enabled  = true
    metadata = true  # Sync document metadata to central
    content  = false # Keep content local-only
  }
}

# Local provider configuration
local_workspace {
  base_path = "/Users/dev/hermes-docs"
  docs_path = "docs"
}

# API provider configuration (delegates to Central)
api_workspace {
  base_url   = "https://central.hermes.company.com"
  auth_token = env("HERMES_API_TOKEN")
  timeout    = "30s"
}

# Notification replication
notification_replication {
  enabled = true
  send_via_both = true  # Send notifications via both local and central
}
```

**Pattern 2: Central Hermes with Multiple Providers**:
```hcl
# Central Hermes tracking documents across Google + Local + GitHub
providers {
  workspace = "google"  # Primary provider
  search    = "algolia"
}

google_workspace {
  credentials_file = "credentials.json"
  domain          = "company.com"
}

# Track documents from additional providers
document_tracking {
  track_providers = ["local", "github"]

  local {
    base_path = "/var/hermes/git-docs"
  }

  github {
    token        = env("GITHUB_TOKEN")
    organization = "company"
    repositories = ["rfcs", "design-docs"]
  }
}

# UUID merging service
uuid_merge {
  enabled         = true
  require_approval = true  # User must approve merges
}

# Identity management
identity_management {
  unified_identity = true

  auth_providers = ["google", "github", "ibm-verify"]

  # Allow users to join identities via UI
  allow_identity_joining = true
}
```

**Pattern 3: Offline-Capable Edge**:
```hcl
# Edge Hermes that works offline, syncs when online
providers {
  workspace = "multi"
  search    = "meilisearch"
}

multi_workspace {
  primary   = "local"
  secondary = "api"

  # Offline support
  offline_mode {
    enabled = true
    queue_sync = true  # Queue operations when offline

    # Retry configuration
    retry {
      max_attempts = 5
      backoff      = "exponential"
    }
  }
}

local_workspace {
  base_path = "/Users/dev/docs"
  docs_path = "docs"
}

api_workspace {
  base_url   = "https://central.hermes.company.com"
  auth_token = env("HERMES_API_TOKEN")

  # Circuit breaker for network resilience
  circuit_breaker {
    enabled           = true
    failure_threshold = 5
    timeout           = "30s"
  }
}
```

**Pattern 4: Development Setup with Identity Joining**:
```hcl
# Developer laptop configuration
providers {
  workspace = "multi"
  search    = "local"
}

multi_workspace {
  primary   = "local"
  secondary = "api"

  # Developer-specific settings
  developer_mode = true
}

local_workspace {
  base_path = "/Users/dev/projects/docs"
  docs_path = "."

  # Local identity (for offline work)
  local_identity {
    email       = "dev@localhost"
    displayName = "Local Developer"
  }
}

api_workspace {
  base_url = "https://hermes-dev.company.com"

  # OAuth configuration for identity joining
  oauth {
    provider     = "google"
    client_id    = env("OAUTH_CLIENT_ID")
    redirect_url = "http://localhost:8080/auth/callback"
  }
}

# Identity joining enabled
identity_joining {
  enabled = true
  primary_email = env("USER_EMAIL")  # jacob.repp@company.com

  # Additional providers to join
  join_providers = ["github", "ibm-verify"]
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

### Document Synchronization Endpoints (NEW)

- `POST /api/v2/documents/register` - Register document with central (for tracking)
- `PUT /api/v2/documents/:uuid/sync-metadata` - Sync metadata from edge to central
- `GET /api/v2/documents/sync-status` - Get sync status for all documents
- `POST /api/v2/documents/:uuid/sync-revision` - Sync revision information

### UUID Merging Endpoints (NEW)

- `POST /api/v2/documents/merge` - Merge two document UUIDs
- `GET /api/v2/documents/:uuid/merge-candidates` - Find potential duplicate documents
- `GET /api/v2/documents/merge-history` - Get merge history
- `POST /api/v2/documents/merge/:mergeId/rollback` - Rollback a merge (if needed)

### Identity Joining Endpoints (NEW)

- `GET /api/v2/identity/join/initiate` - Initiate identity join OAuth flow
- `POST /api/v2/identity/join/complete` - Complete identity join with OAuth code
- `GET /api/v2/identity/current` - Get current user's unified identity
- `DELETE /api/v2/identity/alternate/:id` - Remove alternate identity
- `GET /api/v2/identity/:unifiedUserId/all` - Get all identities for unified user

### Notification Replication Endpoints (NEW)

- `POST /api/v2/notifications/replicate` - Replicate notification from edge to central
- `GET /api/v2/notifications/:id/status` - Get delivery status of notification
- `GET /api/v2/notifications/audit-log` - Get audit log of all notifications

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
