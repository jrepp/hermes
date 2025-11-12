package mock

import (
	"context"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeAdapter_DocumentLifecycle(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create a document
	doc, err := adapter.CreateDocument(ctx, "", "parent-folder", "Test Document")
	require.NoError(t, err)
	assert.NotEmpty(t, doc.UUID)
	assert.Equal(t, "Test Document", doc.Name)
	assert.Equal(t, "fake", doc.ProviderType)

	// Get document by provider ID
	retrieved, err := adapter.GetDocument(ctx, doc.ProviderID)
	require.NoError(t, err)
	assert.Equal(t, doc.UUID, retrieved.UUID)
	assert.Equal(t, doc.Name, retrieved.Name)

	// Get document by UUID
	retrievedByUUID, err := adapter.GetDocumentByUUID(ctx, doc.UUID)
	require.NoError(t, err)
	assert.Equal(t, doc.UUID, retrievedByUUID.UUID)

	// Rename document
	err = adapter.RenameDocument(ctx, doc.ProviderID, "New Name")
	require.NoError(t, err)

	renamed, err := adapter.GetDocument(ctx, doc.ProviderID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", renamed.Name)

	// Delete document
	err = adapter.DeleteDocument(ctx, doc.ProviderID)
	require.NoError(t, err)

	// Verify deletion
	_, err = adapter.GetDocument(ctx, doc.ProviderID)
	assert.Error(t, err)
}

func TestFakeAdapter_ContentOperations(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create a document
	doc, err := adapter.CreateDocument(ctx, "", "parent-folder", "Test Document")
	require.NoError(t, err)

	// Get initial content
	content, err := adapter.GetContent(ctx, doc.ProviderID)
	require.NoError(t, err)
	assert.Equal(t, doc.UUID, content.UUID)
	assert.Equal(t, "1", content.BackendRevision.RevisionID)

	// Update content
	newContent := "# New Content\n\nThis is updated content."
	updated, err := adapter.UpdateContent(ctx, doc.ProviderID, newContent)
	require.NoError(t, err)
	assert.Equal(t, newContent, updated.Body)
	assert.Equal(t, "2", updated.BackendRevision.RevisionID)

	// Get content by UUID
	contentByUUID, err := adapter.GetContentByUUID(ctx, doc.UUID)
	require.NoError(t, err)
	assert.Equal(t, newContent, contentByUUID.Body)

	// Get content batch
	doc2, err := adapter.CreateDocument(ctx, "", "parent-folder", "Doc 2")
	require.NoError(t, err)

	batch, err := adapter.GetContentBatch(ctx, []string{doc.ProviderID, doc2.ProviderID})
	require.NoError(t, err)
	assert.Len(t, batch, 2)
}

func TestFakeAdapter_RevisionTracking(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create a document
	doc, err := adapter.CreateDocument(ctx, "", "parent-folder", "Test Document")
	require.NoError(t, err)

	// Get revision history
	revisions, err := adapter.GetRevisionHistory(ctx, doc.ProviderID, 0)
	require.NoError(t, err)
	assert.Len(t, revisions, 1)
	assert.Equal(t, "1", revisions[0].RevisionID)

	// Update content to create new revision
	_, err = adapter.UpdateContent(ctx, doc.ProviderID, "Updated content")
	require.NoError(t, err)

	// Get updated revision history
	revisions, err = adapter.GetRevisionHistory(ctx, doc.ProviderID, 0)
	require.NoError(t, err)
	assert.Len(t, revisions, 2)
	assert.Equal(t, "2", revisions[0].RevisionID) // Most recent first

	// Get specific revision
	rev, err := adapter.GetRevision(ctx, doc.ProviderID, "1")
	require.NoError(t, err)
	assert.Equal(t, "1", rev.RevisionID)

	// Keep revision forever
	err = adapter.KeepRevisionForever(ctx, doc.ProviderID, "1")
	require.NoError(t, err)

	rev, err = adapter.GetRevision(ctx, doc.ProviderID, "1")
	require.NoError(t, err)
	assert.True(t, rev.KeepForever)
}

func TestFakeAdapter_Permissions(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create a document
	doc, err := adapter.CreateDocument(ctx, "", "parent-folder", "Test Document")
	require.NoError(t, err)

	// Share document with user
	err = adapter.ShareDocument(ctx, doc.ProviderID, "user@example.com", "writer")
	require.NoError(t, err)

	// List permissions
	perms, err := adapter.ListPermissions(ctx, doc.ProviderID)
	require.NoError(t, err)
	assert.Len(t, perms, 1)
	assert.Equal(t, "user@example.com", perms[0].Email)
	assert.Equal(t, "writer", perms[0].Role)

	// Share with domain
	err = adapter.ShareDocumentWithDomain(ctx, doc.ProviderID, "example.com", "reader")
	require.NoError(t, err)

	perms, err = adapter.ListPermissions(ctx, doc.ProviderID)
	require.NoError(t, err)
	assert.Len(t, perms, 2)

	// Update permission
	err = adapter.UpdatePermission(ctx, doc.ProviderID, perms[0].ID, "owner")
	require.NoError(t, err)

	perms, err = adapter.ListPermissions(ctx, doc.ProviderID)
	require.NoError(t, err)
	assert.Equal(t, "owner", perms[0].Role)

	// Remove permission
	err = adapter.RemovePermission(ctx, doc.ProviderID, perms[0].ID)
	require.NoError(t, err)

	perms, err = adapter.ListPermissions(ctx, doc.ProviderID)
	require.NoError(t, err)
	assert.Len(t, perms, 1)
}

func TestFakeAdapter_People(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Add a person
	person := &workspace.UserIdentity{
		Email:       "test@example.com",
		DisplayName: "Test User",
		PhotoURL:    "https://example.com/photo.jpg",
	}
	adapter.WithPerson(person)

	// Get person by email
	retrieved, err := adapter.GetPerson(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, person.Email, retrieved.Email)
	assert.Equal(t, person.DisplayName, retrieved.DisplayName)

	// Search people
	results, err := adapter.SearchPeople(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, person.Email, results[0].Email)

	// Get person by unified ID
	retrievedByID, err := adapter.GetPersonByUnifiedID(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, person.Email, retrievedByID.Email)
}

func TestFakeAdapter_Teams(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Add a team
	team := &workspace.Team{
		ID:    "team-1",
		Name:  "Engineering",
		Email: "eng@example.com",
	}
	adapter.WithTeam(team)

	// Add a person
	person := &workspace.UserIdentity{
		Email:       "user@example.com",
		DisplayName: "Test User",
	}
	adapter.WithPerson(person)

	// Add user to team
	adapter.WithTeamMember(team.ID, person.Email)

	// Get team
	retrieved, err := adapter.GetTeam(ctx, team.ID)
	require.NoError(t, err)
	assert.Equal(t, team.Name, retrieved.Name)

	// List teams
	teams, err := adapter.ListTeams(ctx, "", "Engineering", 10)
	require.NoError(t, err)
	assert.Len(t, teams, 1)

	// Get user teams
	userTeams, err := adapter.GetUserTeams(ctx, person.Email)
	require.NoError(t, err)
	assert.Len(t, userTeams, 1)
	assert.Equal(t, team.ID, userTeams[0].ID)

	// Get team members
	members, err := adapter.GetTeamMembers(ctx, team.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, person.Email, members[0].Email)
}

func TestFakeAdapter_Notifications(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Send email
	err := adapter.SendEmail(ctx,
		[]string{"user1@example.com", "user2@example.com"},
		"sender@example.com",
		"Test Subject",
		"Test Body",
	)
	require.NoError(t, err)

	// Verify email was recorded
	assert.Len(t, adapter.EmailsSent, 1)
	assert.Equal(t, "Test Subject", adapter.EmailsSent[0].Subject)
	assert.Equal(t, "Test Body", adapter.EmailsSent[0].Body)
	assert.Len(t, adapter.EmailsSent[0].To, 2)

	// Send email with template
	err = adapter.SendEmailWithTemplate(ctx,
		[]string{"user@example.com"},
		"welcome-template",
		map[string]any{"name": "Test User"},
	)
	require.NoError(t, err)

	// Verify template email was recorded
	assert.Len(t, adapter.EmailsSent, 2)
	assert.Equal(t, "welcome-template", adapter.EmailsSent[1].Template)
}

func TestFakeAdapter_Folders(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create a folder
	folder, err := adapter.CreateFolder(ctx, "Test Folder", "parent-123")
	require.NoError(t, err)
	assert.Equal(t, "Test Folder", folder.Name)
	assert.Equal(t, "fake", folder.ProviderType)

	// Get subfolder
	folderID, err := adapter.GetSubfolder(ctx, "parent-123", "Test Folder")
	require.NoError(t, err)
	assert.Equal(t, folder.ProviderID, folderID)
}

func TestFakeAdapter_CopyDocument(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create original document
	original, err := adapter.CreateDocument(ctx, "", "parent-folder", "Original")
	require.NoError(t, err)

	// Add content to original
	_, err = adapter.UpdateContent(ctx, original.ProviderID, "Original content")
	require.NoError(t, err)

	// Copy document
	copied, err := adapter.CopyDocument(ctx, original.ProviderID, "dest-folder", "Copy")
	require.NoError(t, err)
	assert.Equal(t, "Copy", copied.Name)
	assert.NotEqual(t, original.UUID, copied.UUID)
	assert.NotEqual(t, original.ProviderID, copied.ProviderID)

	// Verify content was copied
	copiedContent, err := adapter.GetContent(ctx, copied.ProviderID)
	require.NoError(t, err)
	assert.Equal(t, "Original content", copiedContent.Body)
}

func TestFakeAdapter_MoveDocument(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create document
	doc, err := adapter.CreateDocument(ctx, "", "folder-1", "Test Document")
	require.NoError(t, err)

	// Move document
	moved, err := adapter.MoveDocument(ctx, doc.ProviderID, "folder-2")
	require.NoError(t, err)
	assert.Equal(t, "folder-2", moved.ExtendedMetadata["parent_folder"])
}

func TestFakeAdapter_CreateDocumentWithUUID(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create document with explicit UUID (for migration scenarios)
	uuid := docid.NewUUID()
	doc, err := adapter.CreateDocumentWithUUID(ctx, uuid, "", "parent-folder", "Test Document")
	require.NoError(t, err)
	assert.Equal(t, uuid, doc.UUID)

	// Verify can retrieve by UUID
	retrieved, err := adapter.GetDocumentByUUID(ctx, uuid)
	require.NoError(t, err)
	assert.Equal(t, uuid, retrieved.UUID)
}

func TestFakeAdapter_CompareContent(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create two documents
	doc1, err := adapter.CreateDocument(ctx, "", "parent-folder", "Doc 1")
	require.NoError(t, err)

	doc2, err := adapter.CreateDocument(ctx, "", "parent-folder", "Doc 2")
	require.NoError(t, err)

	// Same content - should match
	_, err = adapter.UpdateContent(ctx, doc1.ProviderID, "Same content")
	require.NoError(t, err)
	_, err = adapter.UpdateContent(ctx, doc2.ProviderID, "Same content")
	require.NoError(t, err)

	// Get content to compare hashes
	content1, _ := adapter.GetContent(ctx, doc1.ProviderID)

	// Update doc2 to have different content
	_, err = adapter.UpdateContent(ctx, doc2.ProviderID, "Different content")
	require.NoError(t, err)

	// Compare content
	comparison, err := adapter.CompareContent(ctx, doc1.ProviderID, doc2.ProviderID)
	require.NoError(t, err)
	assert.False(t, comparison.ContentMatch)
	assert.Equal(t, "major", comparison.HashDifference)
	assert.Equal(t, content1.BackendRevision.RevisionID, comparison.Revision1.RevisionID)
	assert.NotEqual(t, content1.BackendRevision.RevisionID, comparison.Revision2.RevisionID)
}

func TestFakeAdapter_GetAllDocumentRevisions(t *testing.T) {
	ctx := context.Background()
	adapter := NewFakeAdapter()

	// Create document
	doc, err := adapter.CreateDocument(ctx, "", "parent-folder", "Test Document")
	require.NoError(t, err)

	// Create multiple revisions
	_, err = adapter.UpdateContent(ctx, doc.ProviderID, "Content v2")
	require.NoError(t, err)
	_, err = adapter.UpdateContent(ctx, doc.ProviderID, "Content v3")
	require.NoError(t, err)

	// Get all document revisions
	revisions, err := adapter.GetAllDocumentRevisions(ctx, doc.UUID)
	require.NoError(t, err)
	assert.Len(t, revisions, 3)

	// Verify revision info structure
	for _, revInfo := range revisions {
		assert.Equal(t, doc.UUID, revInfo.UUID)
		assert.Equal(t, "fake", revInfo.ProviderType)
		assert.Equal(t, doc.ProviderID, revInfo.ProviderID)
		assert.NotNil(t, revInfo.BackendRevision)
	}
}
