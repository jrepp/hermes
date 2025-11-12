package workspace

import (
	"context"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

// ===================================================================
// CORE PROVIDER INTERFACES (RFC-084)
// ===================================================================
//
// This file defines the 7 core provider interfaces from RFC-084.
//
// Required Interfaces (all providers must implement):
// 1. DocumentProvider - Document metadata CRUD
// 2. ContentProvider - Document content with revision tracking
// 3. RevisionTrackingProvider - Backend-specific revision operations
// 4. PermissionProvider - File sharing and access control
// 5. PeopleProvider - User directory operations
// 6. TeamProvider - Group/team operations
// 7. NotificationProvider - Email/notification sending
//
// Optional Interfaces (edge/central architectures):
// 8. DocumentSyncProvider - Edge→Central synchronization
// 9. DocumentMergeProvider - UUID merging for drift resolution
// 10. IdentityJoinProvider - Cross-provider identity linking

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
	// SyncMetadata synchronizes document metadata from edge to central
	SyncMetadata(ctx context.Context, uuid docid.UUID, metadata *DocumentMetadata) error

	// GetSyncStatus gets synchronization status for documents
	GetSyncStatus(ctx context.Context, uuids []docid.UUID) ([]*SyncStatus, error)

	// SyncRevision synchronizes revision information to central
	SyncRevision(ctx context.Context, uuid docid.UUID, revision *BackendRevision) error
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
	MergeID       string     `json:"mergeId"`
	SourceUUID    docid.UUID `json:"sourceUuid"`
	TargetUUID    docid.UUID `json:"targetUuid"`
	MergedAt      string     `json:"mergedAt"`
	InitiatedBy   string     `json:"initiatedBy"`
	RevisionCount int        `json:"revisionCount"`
}

// ===================================================================
// OPTIONAL INTERFACE: IdentityJoinProvider
// ===================================================================
// IdentityJoinProvider handles cross-provider identity linking
// This interface is OPTIONAL - only needed for central instances with multi-provider auth
type IdentityJoinProvider interface {
	// InitiateIdentityJoin starts OAuth flow to join identity from another provider
	InitiateIdentityJoin(ctx context.Context, provider string) (*OAuthFlow, error)

	// CompleteIdentityJoin completes OAuth flow and links identities
	CompleteIdentityJoin(ctx context.Context, req *JoinIdentityRequest) (*UserIdentity, error)

	// UnlinkIdentity removes a linked identity
	UnlinkIdentity(ctx context.Context, userEmail, providerToUnlink string) error

	// ListLinkedIdentities lists all linked identities for a user
	ListLinkedIdentities(ctx context.Context, userEmail string) ([]*AlternateIdentity, error)
}

// ===================================================================
// COMPOSITE INTERFACE: WorkspaceProvider
// ===================================================================
// WorkspaceProvider composes all 7 required interfaces
// This is the main interface that provider adapters implement
type WorkspaceProvider interface {
	DocumentProvider
	ContentProvider
	RevisionTrackingProvider
	PermissionProvider
	PeopleProvider
	TeamProvider
	NotificationProvider
}
