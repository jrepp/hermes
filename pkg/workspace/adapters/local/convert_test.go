package local

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToDocumentMetadata(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	// Create ProviderID and CompositeID properly
	providerID, err := docid.NewProviderID(docid.ProviderTypeLocal, "rfc-084")
	require.NoError(t, err)
	compositeID := docid.NewCompositeID(uuid, providerID, "")

	doc := &workspace.Document{
		ID:             "rfc-084",
		Name:           "RFC-084: Provider Interface Refactoring",
		MimeType:       "text/markdown",
		ParentFolderID: "rfcs",
		CreatedTime:    now,
		ModifiedTime:   now,
		Owner:          "jacob.repp@hashicorp.com",
		CompositeID:    &compositeID,
		Metadata: map[string]any{
			"tags":             []string{"rfc", "provider", "interface"},
			"project":          "hermes-refactor",
			"owning_team":      "Platform Team",
			"workflow_status":  "Draft",
			"sidebar_position": 84,
		},
	}

	meta, err := ConvertToDocumentMetadata(doc)
	require.NoError(t, err)

	// Verify UUID
	assert.Equal(t, uuid, meta.UUID)

	// Verify provider info
	assert.Equal(t, "local", meta.ProviderType)
	assert.Equal(t, "local:rfc-084", meta.ProviderID)

	// Verify core metadata
	assert.Equal(t, "RFC-084: Provider Interface Refactoring", meta.Name)
	assert.Equal(t, "text/markdown", meta.MimeType)

	// Verify owner
	require.NotNil(t, meta.Owner)
	assert.Equal(t, "jacob.repp@hashicorp.com", meta.Owner.Email)

	// Verify parents
	assert.Equal(t, []string{"rfcs"}, meta.Parents)

	// Verify tags
	assert.Equal(t, []string{"rfc", "provider", "interface"}, meta.Tags)

	// Verify project and team
	assert.Equal(t, "hermes-refactor", meta.Project)
	assert.Equal(t, "Platform Team", meta.OwningTeam)
	assert.Equal(t, "Draft", meta.WorkflowStatus)

	// Verify sync status
	assert.Equal(t, "canonical", meta.SyncStatus)

	// Verify extended metadata
	assert.Equal(t, 84, meta.ExtendedMetadata["sidebar_position"])
}

func TestConvertToDocumentMetadata_NoUUID(t *testing.T) {
	doc := &workspace.Document{
		ID:           "test-doc",
		Name:         "Test Document",
		MimeType:     "text/markdown",
		CreatedTime:  time.Now(),
		ModifiedTime: time.Now(),
	}

	meta, err := ConvertToDocumentMetadata(doc)
	require.NoError(t, err)

	// UUID should be auto-generated
	assert.False(t, meta.UUID.IsZero())
}

func TestConvertToDocumentContent(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	// Create ProviderID and CompositeID properly
	providerID, err := docid.NewProviderID(docid.ProviderTypeLocal, "rfc-084")
	require.NoError(t, err)
	compositeID := docid.NewCompositeID(uuid, providerID, "")

	doc := &workspace.Document{
		ID:           "rfc-084",
		Name:         "RFC-084",
		MimeType:     "text/markdown",
		Content:      "# RFC-084\n\n## Summary\n\nThis RFC proposes...",
		CreatedTime:  now,
		ModifiedTime: now,
		CompositeID:  &compositeID,
		Metadata: map[string]any{
			"git_commit":      "abc123def456",
			"revision_number": 42,
		},
	}

	content, err := ConvertToDocumentContent(doc)
	require.NoError(t, err)

	// Verify UUID
	assert.Equal(t, uuid, content.UUID)

	// Verify provider ID
	assert.Equal(t, "local:rfc-084", content.ProviderID)

	// Verify content
	assert.Equal(t, "RFC-084", content.Title)
	assert.Equal(t, "markdown", content.Format)
	assert.Contains(t, content.Body, "# RFC-084")
	assert.Contains(t, content.Body, "## Summary")

	// Verify content hash
	assert.True(t, strings.HasPrefix(content.ContentHash, "sha256:"))

	// Verify backend revision
	require.NotNil(t, content.BackendRevision)
	assert.Equal(t, "local", content.BackendRevision.ProviderType)
	assert.Equal(t, "abc123def456", content.BackendRevision.RevisionID)
	assert.Equal(t, 42, content.BackendRevision.Metadata["revision_number"])

	// Verify last modified
	assert.Equal(t, now.Year(), content.LastModified.Year())
	assert.Equal(t, now.Month(), content.LastModified.Month())
}

func TestConvertToUserIdentity(t *testing.T) {
	user := &workspace.User{
		Email:    "user@example.com",
		Name:     "Test User",
		PhotoURL: "https://example.com/photo.jpg",
	}

	identity := ConvertToUserIdentity(user)
	require.NotNil(t, identity)

	assert.Equal(t, "user@example.com", identity.Email)
	assert.Equal(t, "Test User", identity.DisplayName)
	assert.Equal(t, "https://example.com/photo.jpg", identity.PhotoURL)
}

func TestConvertToFilePermission(t *testing.T) {
	perm := workspace.Permission{
		Email: "user@example.com",
		Role:  "writer",
		Type:  "user",
	}

	fp := ConvertToFilePermission(perm)
	require.NotNil(t, fp)

	assert.Equal(t, "user@example.com", fp.ID) // Uses email as ID
	assert.Equal(t, "user@example.com", fp.Email)
	assert.Equal(t, "writer", fp.Role)
	assert.Equal(t, "user", fp.Type)

	require.NotNil(t, fp.User)
	assert.Equal(t, "user@example.com", fp.User.Email)
	assert.Equal(t, "user@example.com", fp.User.DisplayName) // Uses email as display name
}

func TestConvertToBackendRevision(t *testing.T) {
	now := time.Now().UTC()

	rev := &workspace.Revision{
		ID:           "rev123",
		DocumentID:   "doc123",
		ModifiedTime: now,
		ModifiedBy:   "user@example.com",
		Name:         "Initial version",
	}

	backendRev := ConvertToBackendRevision(rev)
	require.NotNil(t, backendRev)

	assert.Equal(t, "local", backendRev.ProviderType)
	assert.Equal(t, "rev123", backendRev.RevisionID)
	assert.Equal(t, now.Year(), backendRev.ModifiedTime.Year())

	// Verify ModifiedBy
	require.NotNil(t, backendRev.ModifiedBy)
	assert.Equal(t, "user@example.com", backendRev.ModifiedBy.Email)

	// Verify Name in metadata
	assert.Equal(t, "Initial version", backendRev.Metadata["name"])
}

func TestConvertFromDocumentMetadata(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	meta := &workspace.DocumentMetadata{
		UUID:         uuid,
		ProviderType: "local",
		ProviderID:   "local:rfc-084",
		Name:         "RFC-084",
		MimeType:     "text/markdown",
		CreatedTime:  now,
		ModifiedTime: now,
		Owner: &workspace.UserIdentity{
			Email:       "test@example.com",
			DisplayName: "Test User",
		},
		Parents:        []string{"rfcs"},
		Tags:           []string{"rfc", "test"},
		Project:        "test-project",
		OwningTeam:     "Test Team",
		WorkflowStatus: "Draft",
		ExtendedMetadata: map[string]any{
			"sidebar_position": 84,
		},
	}

	doc := ConvertFromDocumentMetadata(meta)
	require.NotNil(t, doc)

	// Verify ID extracted from providerID
	assert.Equal(t, "rfc-084", doc.ID)

	// Verify core fields
	assert.Equal(t, "RFC-084", doc.Name)
	assert.Equal(t, "text/markdown", doc.MimeType)
	assert.Equal(t, "test@example.com", doc.Owner)
	assert.Equal(t, "rfcs", doc.ParentFolderID)

	// Verify CompositeID
	require.NotNil(t, doc.CompositeID)
	assert.Equal(t, uuid, doc.CompositeID.UUID())
	assert.Equal(t, docid.ProviderTypeLocal, doc.CompositeID.ProviderID().Provider())

	// Verify metadata
	assert.Equal(t, uuid.String(), doc.Metadata["hermes_uuid"])
	assert.Equal(t, []string{"rfc", "test"}, doc.Metadata["tags"])
	assert.Equal(t, "test-project", doc.Metadata["project"])
	assert.Equal(t, "Test Team", doc.Metadata["owning_team"])
	assert.Equal(t, "Draft", doc.Metadata["workflow_status"])
	assert.Equal(t, 84, doc.Metadata["sidebar_position"])
}

func TestConvertToDocumentMetadata_NilDoc(t *testing.T) {
	_, err := ConvertToDocumentMetadata(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document cannot be nil")
}

func TestConvertToDocumentContent_NilDoc(t *testing.T) {
	_, err := ConvertToDocumentContent(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document cannot be nil")
}
