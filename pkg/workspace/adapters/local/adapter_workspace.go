package local

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// WorkspaceAdapter implements workspace.WorkspaceProvider for local filesystem storage.
// It wraps the existing Adapter and provides RFC-084 compliant interfaces.
type WorkspaceAdapter struct {
	adapter *Adapter
}

// NewWorkspaceAdapter creates a new WorkspaceProvider-compliant adapter.
func NewWorkspaceAdapter(adapter *Adapter) workspace.WorkspaceProvider {
	return &WorkspaceAdapter{
		adapter: adapter,
	}
}

// GetAdapter returns the underlying Adapter for direct access.
// This is useful for operations that need the low-level adapter interface.
func (w *WorkspaceAdapter) GetAdapter() *Adapter {
	return w.adapter
}

// ===================================================================
// DocumentProvider Implementation
// ===================================================================

// GetDocument retrieves document metadata by backend-specific ID (local path).
func (w *WorkspaceAdapter) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	// Extract local ID from providerID (format: "local:path/to/doc")
	const prefix = "local:"
	localID := providerID
	if len(providerID) > len(prefix) {
		localID = providerID[len(prefix):]
	}

	// Get document using existing adapter
	storage := w.adapter.DocumentStorage()
	doc, err := storage.GetDocument(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Convert to RFC-084 types
	return ConvertToDocumentMetadata(doc)
}

// GetDocumentByUUID retrieves document metadata by UUID.
// For local filesystem, we need to search for documents with this UUID in frontmatter.
func (w *WorkspaceAdapter) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	// Search for document with this UUID
	storage := w.adapter.DocumentStorage()
	docs, err := storage.ListDocuments(ctx, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	// Find document with matching UUID
	for _, doc := range docs {
		if doc.CompositeID != nil && doc.CompositeID.UUID() == uuid {
			return ConvertToDocumentMetadata(doc)
		}
		// Check metadata for hermes_uuid
		if uuidStr, ok := doc.Metadata["hermes_uuid"].(string); ok {
			if parsedUUID, err := docid.ParseUUID(uuidStr); err == nil && parsedUUID == uuid {
				return ConvertToDocumentMetadata(doc)
			}
		}
	}

	return nil, fmt.Errorf("document with UUID %s not found", uuid)
}

// CreateDocument creates a new document from template.
func (w *WorkspaceAdapter) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	// Generate new UUID for the document
	uuid := docid.NewUUID()
	return w.CreateDocumentWithUUID(ctx, uuid, templateID, destFolderID, name)
}

// CreateDocumentWithUUID creates document with explicit UUID (for migration).
func (w *WorkspaceAdapter) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	storage := w.adapter.DocumentStorage()

	// Get template content if provided
	var templateContent string
	var templateMetadata map[string]any
	if templateID != "" {
		template, err := storage.GetDocument(ctx, templateID)
		if err == nil {
			templateContent = template.Content
			// Copy relevant metadata from template
			if tags, ok := template.Metadata["tags"]; ok {
				if templateMetadata == nil {
					templateMetadata = make(map[string]any)
				}
				templateMetadata["tags"] = tags
			}
		}
	}

	// Create document with UUID in metadata
	docCreate := &workspace.DocumentCreate{
		Name:           name,
		ParentFolderID: destFolderID,
		Content:        templateContent,
	}

	// Create the document
	created, err := storage.CreateDocument(ctx, docCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	// Update with UUID and metadata
	if created.Metadata == nil {
		created.Metadata = make(map[string]any)
	}
	created.Metadata["hermes_uuid"] = uuid.String()
	if templateMetadata != nil {
		for k, v := range templateMetadata {
			created.Metadata[k] = v
		}
	}

	// Create CompositeID
	providerID, err := docid.NewProviderID(docid.ProviderTypeLocal, created.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider ID: %w", err)
	}
	compositeID := docid.NewCompositeID(uuid, providerID, "")
	created.CompositeID = &compositeID

	// Update document with metadata and CompositeID
	updates := &workspace.DocumentUpdate{
		Metadata: created.Metadata,
	}
	updated, err := storage.UpdateDocument(ctx, created.ID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update document with UUID: %w", err)
	}
	updated.CompositeID = &compositeID

	return ConvertToDocumentMetadata(updated)
}

// RegisterDocument registers document metadata with provider.
// For local filesystem, this is a no-op as there's no central registry.
func (w *WorkspaceAdapter) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	// For local filesystem, just return the metadata as-is
	// There's no central registry to register with
	return doc, nil
}

// CopyDocument copies a document (preserves UUID if in frontmatter/metadata).
func (w *WorkspaceAdapter) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	// Extract local ID from providerID
	const prefix = "local:"
	srcLocalID := srcProviderID
	if len(srcProviderID) > len(prefix) {
		srcLocalID = srcProviderID[len(prefix):]
	}

	storage := w.adapter.DocumentStorage()

	// Use existing CopyDocument method
	copied, err := storage.CopyDocument(ctx, srcLocalID, destFolderID, name)
	if err != nil {
		return nil, fmt.Errorf("failed to copy document: %w", err)
	}

	return ConvertToDocumentMetadata(copied)
}

// MoveDocument moves a document to different folder.
func (w *WorkspaceAdapter) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	// Extract local ID from providerID
	const prefix = "local:"
	localID := providerID
	if len(providerID) > len(prefix) {
		localID = providerID[len(prefix):]
	}

	storage := w.adapter.DocumentStorage()

	// Use existing MoveDocument method
	if err := storage.MoveDocument(ctx, localID, destFolderID); err != nil {
		return nil, fmt.Errorf("failed to move document: %w", err)
	}

	// Get updated document
	doc, err := storage.GetDocument(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated document: %w", err)
	}

	return ConvertToDocumentMetadata(doc)
}

// DeleteDocument deletes a document.
func (w *WorkspaceAdapter) DeleteDocument(ctx context.Context, providerID string) error {
	// Extract local ID from providerID
	const prefix = "local:"
	localID := providerID
	if len(providerID) > len(prefix) {
		localID = providerID[len(prefix):]
	}

	storage := w.adapter.DocumentStorage()
	return storage.DeleteDocument(ctx, localID)
}

// RenameDocument renames a document.
func (w *WorkspaceAdapter) RenameDocument(ctx context.Context, providerID, newName string) error {
	// Extract local ID from providerID
	const prefix = "local:"
	localID := providerID
	if len(providerID) > len(prefix) {
		localID = providerID[len(prefix):]
	}

	storage := w.adapter.DocumentStorage()

	// Update document name
	namePtr := &newName
	updates := &workspace.DocumentUpdate{
		Name: namePtr,
	}

	_, err := storage.UpdateDocument(ctx, localID, updates)
	return err
}

// CreateFolder creates a folder/directory.
func (w *WorkspaceAdapter) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	storage := w.adapter.DocumentStorage()

	// Create folder using DocumentStorage
	docCreate := &workspace.DocumentCreate{
		Name:           name,
		ParentFolderID: parentID,
		Content:        "",
	}

	created, err := storage.CreateDocument(ctx, docCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return ConvertToDocumentMetadata(created)
}

// GetSubfolder finds a subfolder by name.
func (w *WorkspaceAdapter) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	storage := w.adapter.DocumentStorage()
	docs, err := storage.ListDocuments(ctx, parentID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list documents: %w", err)
	}

	// Find subfolder with matching name
	for _, doc := range docs {
		if doc.Name == name {
			return fmt.Sprintf("local:%s", doc.ID), nil
		}
	}

	return "", fmt.Errorf("subfolder %s not found in parent %s", name, parentID)
}

// ===================================================================
// ContentProvider Implementation
// ===================================================================

// GetContent retrieves document content with backend-specific revision.
func (w *WorkspaceAdapter) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	// Extract local ID from providerID
	const prefix = "local:"
	localID := providerID
	if len(providerID) > len(prefix) {
		localID = providerID[len(prefix):]
	}

	storage := w.adapter.DocumentStorage()
	doc, err := storage.GetDocument(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return ConvertToDocumentContent(doc)
}

// GetContentByUUID retrieves content using UUID.
func (w *WorkspaceAdapter) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	// First get the document metadata to find the providerID
	meta, err := w.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Then get the content using the providerID
	return w.GetContent(ctx, meta.ProviderID)
}

// UpdateContent updates document content.
func (w *WorkspaceAdapter) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	// Extract local ID from providerID
	const prefix = "local:"
	localID := providerID
	if len(providerID) > len(prefix) {
		localID = providerID[len(prefix):]
	}

	storage := w.adapter.DocumentStorage()

	// Update content using UpdateDocumentContent
	if err := storage.UpdateDocumentContent(ctx, localID, content); err != nil {
		return nil, fmt.Errorf("failed to update document content: %w", err)
	}

	// Get updated document to return with new content hash
	doc, err := storage.GetDocument(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated document: %w", err)
	}

	return ConvertToDocumentContent(doc)
}

// GetContentBatch retrieves multiple documents (efficient for migration).
func (w *WorkspaceAdapter) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	contents := make([]*workspace.DocumentContent, 0, len(providerIDs))

	for _, providerID := range providerIDs {
		content, err := w.GetContent(ctx, providerID)
		if err != nil {
			// Log error but continue with other documents
			continue
		}
		contents = append(contents, content)
	}

	return contents, nil
}

// CompareContent compares content between two revisions.
func (w *WorkspaceAdapter) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	// Get both documents
	content1, err := w.GetContent(ctx, providerID1)
	if err != nil {
		return nil, fmt.Errorf("failed to get first document: %w", err)
	}

	content2, err := w.GetContent(ctx, providerID2)
	if err != nil {
		return nil, fmt.Errorf("failed to get second document: %w", err)
	}

	// Compare content hashes
	contentMatch := content1.ContentHash == content2.ContentHash
	hashDiff := "same"
	if !contentMatch {
		hashDiff = "major" // TODO: Implement more sophisticated diff detection
	}

	return &workspace.ContentComparison{
		UUID:           content1.UUID,
		Revision1:      content1.BackendRevision,
		Revision2:      content2.BackendRevision,
		ContentMatch:   contentMatch,
		HashDifference: hashDiff,
	}, nil
}

// ===================================================================
// RevisionTrackingProvider Implementation
// ===================================================================

// GetRevisionHistory lists all revisions for a document in this backend.
// For local filesystem with Git, this would query Git history.
// For simple filesystem, this returns empty list (not yet implemented).
func (w *WorkspaceAdapter) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	// TODO: Implement Git integration for revision tracking
	// For now, return empty list
	return []*workspace.BackendRevision{}, nil
}

// GetRevision retrieves a specific revision.
func (w *WorkspaceAdapter) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	// TODO: Implement Git integration for revision tracking
	return nil, fmt.Errorf("revision tracking not yet implemented for local filesystem")
}

// GetRevisionContent retrieves content at a specific revision.
func (w *WorkspaceAdapter) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	// TODO: Implement Git integration for revision tracking
	return nil, fmt.Errorf("revision tracking not yet implemented for local filesystem")
}

// KeepRevisionForever marks a revision as permanent (if supported).
// For local filesystem, this is a no-op.
func (w *WorkspaceAdapter) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	// Not applicable for local filesystem
	return nil
}

// GetAllDocumentRevisions returns all revisions across all backends for a UUID.
func (w *WorkspaceAdapter) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	// For local filesystem, return empty list for now
	// TODO: Implement Git integration
	return []*workspace.RevisionInfo{}, nil
}

// ===================================================================
// PermissionProvider Implementation
// ===================================================================

// ShareDocument grants access to a user/group.
// For local filesystem, permissions are handled by OS file permissions.
func (w *WorkspaceAdapter) ShareDocument(ctx context.Context, providerID, email, role string) error {
	// Local filesystem doesn't have user-level sharing
	// This would need to be implemented via ACLs or external system
	return fmt.Errorf("document sharing not supported for local filesystem")
}

// ShareDocumentWithDomain grants access to entire domain.
func (w *WorkspaceAdapter) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	return fmt.Errorf("domain sharing not supported for local filesystem")
}

// ListPermissions lists all permissions for a document.
func (w *WorkspaceAdapter) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	// Return empty list for local filesystem
	return []*workspace.FilePermission{}, nil
}

// RemovePermission revokes access.
func (w *WorkspaceAdapter) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	return fmt.Errorf("permission management not supported for local filesystem")
}

// UpdatePermission changes permission role.
func (w *WorkspaceAdapter) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	return fmt.Errorf("permission management not supported for local filesystem")
}

// ===================================================================
// PeopleProvider Implementation
// ===================================================================

// SearchPeople searches for users in the directory.
func (w *WorkspaceAdapter) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	peopleService := w.adapter.PeopleService()
	users, err := peopleService.SearchUsers(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search people: %w", err)
	}

	// Convert to UserIdentity
	identities := make([]*workspace.UserIdentity, len(users))
	for i, user := range users {
		identities[i] = ConvertToUserIdentity(user)
	}

	return identities, nil
}

// GetPerson retrieves a user by email.
func (w *WorkspaceAdapter) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	peopleService := w.adapter.PeopleService()
	user, err := peopleService.GetUser(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get person: %w", err)
	}

	return ConvertToUserIdentity(user), nil
}

// GetPersonByUnifiedID retrieves user by unified ID.
// For local filesystem, unified IDs are the same as email addresses.
func (w *WorkspaceAdapter) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	// For local filesystem, unifiedID is the email
	return w.GetPerson(ctx, unifiedID)
}

// ResolveIdentity resolves alternate identities for a user.
// For local filesystem, there are no alternate identities.
func (w *WorkspaceAdapter) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return w.GetPerson(ctx, email)
}

// ===================================================================
// TeamProvider Implementation
// ===================================================================

// ListTeams lists teams matching query.
// For local filesystem, teams are stored in configuration files.
func (w *WorkspaceAdapter) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	// Local filesystem doesn't have teams by default
	// This would need to be implemented via configuration
	return []*workspace.Team{}, nil
}

// GetTeam retrieves team details.
func (w *WorkspaceAdapter) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	return nil, fmt.Errorf("teams not supported for local filesystem")
}

// GetUserTeams lists all teams a user belongs to.
func (w *WorkspaceAdapter) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	return []*workspace.Team{}, nil
}

// GetTeamMembers lists all members of a team.
func (w *WorkspaceAdapter) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	return []*workspace.UserIdentity{}, nil
}

// ===================================================================
// NotificationProvider Implementation
// ===================================================================

// SendEmail sends an email notification.
func (w *WorkspaceAdapter) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	notificationService := w.adapter.NotificationService()
	return notificationService.SendEmail(ctx, to, from, subject, body)
}

// SendEmailWithTemplate sends email using template.
func (w *WorkspaceAdapter) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	// For now, just send a plain email with template name
	// TODO: Implement proper template rendering
	return w.SendEmail(ctx, to, "", fmt.Sprintf("Template: %s", template), fmt.Sprintf("Data: %v", data))
}
