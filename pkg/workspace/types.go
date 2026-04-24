package workspace

import (
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

// RFC-084 Types: Provider-agnostic document metadata and content types
// These types support multi-backend document tracking with enhanced metadata

// DocumentMetadata represents provider-agnostic document metadata
// Works with DocID system (UUID + ProviderID)
//
// Design Philosophy:
// - Core attributes: Universal metadata present across all document types
// - Extensible attributes: Document-type-specific metadata in ExtendedMetadata map
type DocumentMetadata struct {
	CreatedTime      time.Time      `json:"createdTime"`
	ModifiedTime     time.Time      `json:"modifiedTime"`
	ExtendedMetadata map[string]any `json:"extendedMetadata,omitempty"`
	Owner            *UserIdentity  `json:"owner,omitempty"`
	OwningTeam       string         `json:"owningTeam,omitempty"`
	MimeType         string         `json:"mimeType"`
	Name             string         `json:"name"`
	ProviderID       string         `json:"providerID"`
	Project          string         `json:"project,omitempty"`
	SyncStatus       string         `json:"syncStatus"`
	WorkflowStatus   string         `json:"workflowStatus,omitempty"`
	ContentHash      string         `json:"contentHash"`
	ProviderType     string         `json:"providerType"`
	Contributors     []UserIdentity `json:"contributors,omitempty"`
	Parents          []string       `json:"parents,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	UUID             docid.UUID     `json:"uuid"`
}

// DocumentContent represents document content with backend-specific revision info
type DocumentContent struct {
	LastModified    time.Time        `json:"lastModified"`
	BackendRevision *BackendRevision `json:"backendRevision"`
	ProviderID      string           `json:"providerID"`
	Title           string           `json:"title"`
	Body            string           `json:"body"`
	Format          string           `json:"format"`
	ContentHash     string           `json:"contentHash"`
	UUID            docid.UUID       `json:"uuid"`
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
	ModifiedTime time.Time      `json:"modifiedTime"`
	ModifiedBy   *UserIdentity  `json:"modifiedBy,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	ProviderType string         `json:"providerType"`
	RevisionID   string         `json:"revisionID"`
	Comment      string         `json:"comment,omitempty"`
	KeepForever  bool           `json:"keepForever,omitempty"`
}

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
	Email          string `json:"email"`                    // e.g., "jrepp@ibm.com", "jacob-repp@users.noreply.github.com"
	Provider       string `json:"provider"`                 // e.g., "ibm-verify", "google-workspace", "github", "okta", "dex"
	ProviderUserID string `json:"providerUserId,omitempty"` // Provider-specific user ID
}

// FilePermission represents file access permissions
type FilePermission struct {
	User  *UserIdentity `json:"user,omitempty"`
	ID    string        `json:"id"`
	Email string        `json:"email"`
	Role  string        `json:"role"`
	Type  string        `json:"type"`
}

// Team represents a group/team (renamed from Group to avoid confusion)
type Team struct {
	ID           string `json:"id"`
	Email        string `json:"email,omitempty"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ProviderType string `json:"providerType"`
	ProviderID   string `json:"providerID"`
	MemberCount  int    `json:"memberCount"`
}

// RevisionInfo represents a document revision for conflict detection
type RevisionInfo struct {
	BackendRevision *BackendRevision `json:"backendRevision"`
	ProviderType    string           `json:"providerType"`
	ProviderID      string           `json:"providerID"`
	ContentHash     string           `json:"contentHash"`
	SyncStatus      string           `json:"syncStatus"`
	UUID            docid.UUID       `json:"uuid"`
}

// ContentComparison represents a content comparison result
type ContentComparison struct {
	Revision1      *BackendRevision
	Revision2      *BackendRevision
	HashDifference string
	UUID           docid.UUID
	ContentMatch   bool
}

// SyncStatus represents document synchronization state
type SyncStatus struct {
	LastSyncTime time.Time  `json:"lastSyncTime"`
	SyncState    string     `json:"syncState"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	UUID         docid.UUID `json:"uuid"`
}

// MergeRequest represents a request to merge two document UUIDs
type MergeRequest struct {
	MergeStrategy  string     `json:"mergeStrategy"`
	InitiatedBy    string     `json:"initiatedBy"`
	SourceUUID     docid.UUID `json:"sourceUUID"`
	TargetUUID     docid.UUID `json:"targetUUID"`
	MergeRevisions bool       `json:"mergeRevisions"`
}

// OAuthFlow represents OAuth flow initiation data
type OAuthFlow struct {
	AuthURL  string `json:"authUrl"`
	State    string `json:"state"`
	Provider string `json:"provider"`
}

// JoinIdentityRequest represents identity join completion request
type JoinIdentityRequest struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	State    string `json:"state"`
}
