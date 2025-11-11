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
	Parents []string `json:"parents,omitempty"` // Parent folder/directory IDs
	Project string   `json:"project,omitempty"` // Project association (e.g., "agf-iac-remediation-poc")
	Tags    []string `json:"tags,omitempty"`    // Universal tags for categorization and search

	// Document Lifecycle
	SyncStatus     string `json:"syncStatus"`                // Multi-backend sync: "canonical", "mirror", "conflict", "archived"
	WorkflowStatus string `json:"workflowStatus,omitempty"` // Document workflow: "Draft", "In Review", "Published", "Deprecated"

	// Multi-backend tracking
	ContentHash string `json:"contentHash"` // SHA-256 for drift detection

	// Extensible metadata for document-type-specific fields
	// Examples: RFC id ("rfc-010"), sidebar_position (10), rfc_type ("Architecture")
	ExtendedMetadata map[string]any `json:"extendedMetadata,omitempty"`
}

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
	ModifiedTime time.Time     `json:"modifiedTime"`
	ModifiedBy   *UserIdentity `json:"modifiedBy,omitempty"`
	Comment      string        `json:"comment,omitempty"`    // Git commit message, Drive comment, etc.
	KeepForever  bool          `json:"keepForever,omitempty"` // Google Drive feature

	// Backend-specific metadata (flexible for different systems)
	// Examples:
	//   - Google: {"published": true, "size": 12345}
	//   - Git: {"tree": "abc123", "parent": "def456", "author": "..."}
	//   - O365: {"versionLabel": "Major", "size": 12345, "comment": "..."}
	Metadata map[string]any `json:"metadata,omitempty"`
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

// SyncStatus represents document synchronization state
type SyncStatus struct {
	UUID         docid.UUID `json:"uuid"`
	LastSyncTime time.Time  `json:"lastSyncTime"`
	SyncState    string     `json:"syncState"` // "synced", "pending", "failed"
	ErrorMessage string     `json:"errorMessage,omitempty"`
}

// MergeRequest represents a request to merge two document UUIDs
type MergeRequest struct {
	SourceUUID     docid.UUID `json:"sourceUUID"`      // UUID to be merged (deprecated)
	TargetUUID     docid.UUID `json:"targetUUID"`      // UUID to keep (canonical)
	MergeRevisions bool       `json:"mergeRevisions"`  // Merge revision histories
	MergeStrategy  string     `json:"mergeStrategy"`   // "keep-target", "keep-source", "merge-all"
	InitiatedBy    string     `json:"initiatedBy"`     // User email
}

// MergeRecord represents a historical UUID merge operation
type MergeRecord struct {
	MergeID       string     `json:"mergeId"`
	SourceUUID    docid.UUID `json:"sourceUuid"`
	TargetUUID    docid.UUID `json:"targetUuid"`
	MergedAt      time.Time  `json:"mergedAt"`
	InitiatedBy   string     `json:"initiatedBy"`
	RevisionCount int        `json:"revisionCount"`
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
