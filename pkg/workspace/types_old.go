package workspace

import (
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

// Document represents a storage-agnostic document.
type Document struct {
	CreatedTime    time.Time
	ModifiedTime   time.Time
	CompositeID    *docid.CompositeID
	Metadata       map[string]any
	Content        string
	ParentFolderID string
	MimeType       string
	ID             string
	Owner          string
	ThumbnailURL   string
	Name           string
	Permissions    []Permission
	Trashed        bool
}

// DocumentCreate contains fields for creating a new document.
type DocumentCreate struct {
	Metadata       map[string]any
	Name           string
	ParentFolderID string
	TemplateID     string
	Content        string
	Owner          string
}

// DocumentUpdate contains fields that can be updated.
type DocumentUpdate struct {
	// Name updates the document name if non-nil.
	Name *string

	// Content updates the document content if non-nil.
	Content *string

	// ParentFolderID moves the document if non-nil.
	ParentFolderID *string

	// Metadata updates metadata fields.
	Metadata map[string]any
}

// Folder represents a storage folder/directory.
type Folder struct {
	CreatedTime  time.Time
	ModifiedTime time.Time
	Metadata     map[string]any
	ID           string
	Name         string
	ParentID     string
}

// Revision represents a document revision/version.
type Revision struct {
	// ID is the unique identifier for the revision.
	ID string

	// DocumentID is the document this revision belongs to.
	DocumentID string

	// ModifiedTime is when this revision was created.
	ModifiedTime time.Time

	// ModifiedBy is the email of who created this revision.
	ModifiedBy string

	// Name is an optional custom name for this revision.
	Name string

	// Content is the content at this revision (may be empty).
	Content string
}

// User represents a user/person.
type User struct {
	Metadata   map[string]any `json:"metadata,omitempty"`
	Email      string         `json:"email"`
	Name       string         `json:"name"`
	GivenName  string         `json:"given_name"`
	FamilyName string         `json:"family_name"`
	PhotoURL   string         `json:"photo_url"`
}

// Permission represents a document permission.
type Permission struct {
	// Email is the email of the user/group with this permission.
	Email string

	// Role is the permission role (e.g., "owner", "writer", "reader").
	Role string

	// Type is the permission type (e.g., "user", "group", "domain").
	Type string
}

// AuthInfo contains authentication information.
type AuthInfo struct {
	ExpiresAt time.Time
	Email     string
	Valid     bool
}

// UserInfo contains user information from authentication.
type UserInfo struct {
	// ID is the user's unique identifier.
	ID string

	// Email is the user's email address.
	Email string

	// Name is the user's full name.
	Name string

	// GivenName is the user's first name.
	GivenName string

	// FamilyName is the user's last name.
	FamilyName string

	// Picture is the URL to the user's profile picture.
	Picture string

	// Locale is the user's locale (e.g., "en").
	Locale string

	// HD is the hosted domain (e.g., "hashicorp.com").
	HD string

	// VerifiedEmail indicates if the email is verified.
	VerifiedEmail bool
}
