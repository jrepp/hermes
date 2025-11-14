package local

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/people/v1"
)

// Compile-time check that ProviderAdapter implements workspace.WorkspaceProvider (RFC-084)
var _ workspace.WorkspaceProvider = (*ProviderAdapter)(nil)

// Compile-time check that ProviderAdapter implements workspace.ProviderCapabilities
var _ workspace.ProviderCapabilities = (*ProviderAdapter)(nil)

// ProviderAdapter adapts the local Adapter (which implements StorageProvider)
// to the workspace.Provider interface used by API handlers.
// This bridges the gap between the comprehensive StorageProvider and the
// simplified Provider interface focused on Drive-like operations.
type ProviderAdapter struct {
	adapter *Adapter
	ctx     context.Context
}

// NewProviderAdapter creates a Provider interface wrapper around a local Adapter.
func NewProviderAdapter(adapter *Adapter) *ProviderAdapter {
	return &ProviderAdapter{
		adapter: adapter,
		ctx:     context.Background(),
	}
}

// NewProviderAdapterWithContext creates a Provider interface wrapper with a specific context.
func NewProviderAdapterWithContext(adapter *Adapter, ctx context.Context) *ProviderAdapter {
	return &ProviderAdapter{
		adapter: adapter,
		ctx:     ctx,
	}
}

// GetAdapter returns the underlying local Adapter for direct access.
// This is useful for operations not exposed through the Provider interface,
// such as document indexing on startup.
func (p *ProviderAdapter) GetAdapter() *Adapter {
	return p.adapter
}

// GetFile retrieves a file by ID and converts it to Google Drive format.
func (p *ProviderAdapter) GetFile(fileID string) (*drive.File, error) {
	doc, err := p.adapter.DocumentStorage().GetDocument(p.ctx, fileID)
	if err != nil {
		return nil, err
	}

	// Load permissions from metadata
	p.loadPermissionsFromMetadata(doc)

	return documentToDriveFile(doc), nil
}

// CopyFile copies a file to a destination folder with a new name.
func (p *ProviderAdapter) CopyFile(srcID, destFolderID, name string) (*drive.File, error) {
	// Copy the document using the correct signature
	newDoc, err := p.adapter.DocumentStorage().CopyDocument(p.ctx, srcID, destFolderID, name)
	if err != nil {
		return nil, fmt.Errorf("failed to copy document: %w", err)
	}

	return documentToDriveFile(newDoc), nil
}

// CreateFileAsUser creates a file by copying a template, with the specified user as owner.
// In the local adapter, this behaves the same as CopyFile.
// In Google Workspace, this would use domain-wide delegation to impersonate the user.
// For the local adapter, we simply copy the template and set owner in metadata.
func (p *ProviderAdapter) CreateFileAsUser(templateID, destFolderID, name, userEmail string) (*drive.File, error) {
	// Copy the template
	newDoc, err := p.adapter.DocumentStorage().CopyDocument(p.ctx, templateID, destFolderID, name)
	if err != nil {
		return nil, fmt.Errorf("failed to copy template: %w", err)
	}

	// Store the owner information in metadata
	if newDoc.Metadata == nil {
		newDoc.Metadata = make(map[string]any)
	}
	newDoc.Metadata["created_as_user"] = userEmail

	// Update metadata to persist the owner info
	_, err = p.adapter.DocumentStorage().UpdateDocument(p.ctx, newDoc.ID, &workspace.DocumentUpdate{
		Metadata: newDoc.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set owner metadata: %w", err)
	}

	// Update the owner field in the returned document
	newDoc.Owner = userEmail

	return documentToDriveFile(newDoc), nil
}

// MoveFile moves a file to a different folder.
func (p *ProviderAdapter) MoveFile(fileID, destFolderID string) (*drive.File, error) {
	err := p.adapter.DocumentStorage().MoveDocument(p.ctx, fileID, destFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to move document: %w", err)
	}

	// Get the updated document to return
	doc, err := p.adapter.DocumentStorage().GetDocument(p.ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get moved document: %w", err)
	}

	return documentToDriveFile(doc), nil
}

// DeleteFile deletes a file by ID.
func (p *ProviderAdapter) DeleteFile(fileID string) error {
	return p.adapter.DocumentStorage().DeleteDocument(p.ctx, fileID)
}

// RenameFile renames a file.
func (p *ProviderAdapter) RenameFile(fileID, newName string) error {
	_, err := p.adapter.DocumentStorage().UpdateDocument(p.ctx, fileID, &workspace.DocumentUpdate{
		Name: &newName,
	})
	return err
}

// ShareFile shares a file with a user by adding permissions.
// Permissions are stored in the document's metadata as they would be
// in Google Drive's permission system.
func (p *ProviderAdapter) ShareFile(fileID, email, role string) error {
	// Get current document
	doc, err := p.adapter.DocumentStorage().GetDocument(p.ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Load existing permissions from metadata
	p.loadPermissionsFromMetadata(doc)

	// Check if permission already exists
	for i, perm := range doc.Permissions {
		if perm.Email == email {
			// Update existing permission
			doc.Permissions[i].Role = role
			return p.updatePermissions(fileID, doc.Permissions)
		}
	}

	// Add new permission
	newPerm := workspace.Permission{
		Email: email,
		Role:  role,
		Type:  "user",
	}

	doc.Permissions = append(doc.Permissions, newPerm)
	return p.updatePermissions(fileID, doc.Permissions)
}

// ListPermissions lists all permissions for a file with RFC-084 signature.
func (p *ProviderAdapter) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	doc, err := p.adapter.DocumentStorage().GetDocument(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Load permissions from metadata
	p.loadPermissionsFromMetadata(doc)

	perms := make([]*workspace.FilePermission, len(doc.Permissions))
	for i, perm := range doc.Permissions {
		perms[i] = &workspace.FilePermission{
			ID:    generatePermissionIDForEmail(perm.Email),
			Type:  perm.Type,
			Email: perm.Email,
			Role:  perm.Role,
		}
	}

	return perms, nil
}

// ListPermissionsLegacy lists all permissions with old Provider interface signature.
// Deprecated: Use ListPermissions with context parameter instead.
func (p *ProviderAdapter) ListPermissionsLegacy(fileID string) ([]*drive.Permission, error) {
	rfc084Perms, err := p.ListPermissions(p.ctx, fileID)
	if err != nil {
		return nil, err
	}

	// Convert RFC-084 FilePermission to drive.Permission
	perms := make([]*drive.Permission, len(rfc084Perms))
	for i, perm := range rfc084Perms {
		perms[i] = &drive.Permission{
			Id:           perm.ID,
			Type:         perm.Type,
			EmailAddress: perm.Email,
			Role:         perm.Role,
		}
	}

	return perms, nil
}

// DeletePermission removes a specific permission from a file.
func (p *ProviderAdapter) DeletePermission(fileID, permissionID string) error {
	doc, err := p.adapter.DocumentStorage().GetDocument(p.ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Load existing permissions from metadata
	p.loadPermissionsFromMetadata(doc)

	// Filter out the permission to delete
	// The permission ID is based on the email, so we need to find by ID
	newPerms := make([]workspace.Permission, 0, len(doc.Permissions))
	found := false
	for _, perm := range doc.Permissions {
		permID := generatePermissionIDForEmail(perm.Email)
		if permID == permissionID {
			found = true
			continue
		}
		newPerms = append(newPerms, perm)
	}

	if !found {
		return fmt.Errorf("permission not found: %s", permissionID)
	}

	return p.updatePermissions(fileID, newPerms)
}

// SearchPeople searches for people in the local people directory with RFC-084 signature.
func (p *ProviderAdapter) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	// SearchUsers expects a query and fields slice
	users, err := p.adapter.PeopleService().SearchUsers(ctx, query, []string{"names", "emailAddresses", "photos"})
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return []*workspace.UserIdentity{}, nil
	}

	// Convert workspace users to RFC-084 UserIdentity format
	identities := make([]*workspace.UserIdentity, len(users))
	for i, user := range users {
		identities[i] = &workspace.UserIdentity{
			Email:       user.Email,
			DisplayName: user.Name,
			PhotoURL:    user.PhotoURL,
		}
	}

	return identities, nil
}

// SearchPeopleLegacy searches for people with old Provider interface signature.
// Deprecated: Use SearchPeople with context parameter instead.
func (p *ProviderAdapter) SearchPeopleLegacy(email string, fields string) ([]*people.Person, error) {
	// SearchUsers expects a query and fields slice
	users, err := p.adapter.PeopleService().SearchUsers(p.ctx, email, []string{"names", "emailAddresses", "photos"})
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return []*people.Person{}, nil
	}

	// Convert workspace users to people.Person format
	persons := make([]*people.Person, len(users))
	for i, user := range users {
		person := &people.Person{
			ResourceName: fmt.Sprintf("people/%s", user.Email),
			Etag:         user.Email,
		}

		// Add name if available
		if user.Name != "" {
			person.Names = []*people.Name{
				{
					DisplayName: user.Name,
					GivenName:   user.GivenName,
					FamilyName:  user.FamilyName,
				},
			}
		}

		// Add email with metadata (required by /me endpoint)
		person.EmailAddresses = []*people.EmailAddress{
			{
				Value: user.Email,
				Type:  "work",
				Metadata: &people.FieldMetadata{
					Primary:  true,
					Verified: true,
					Source: &people.Source{
						Id:   user.Email, // Use email as ID since User struct doesn't have separate ID field
						Type: "DOMAIN_PROFILE",
					},
				},
			},
		}

		// Add photo if available
		if user.PhotoURL != "" {
			person.Photos = []*people.Photo{
				{
					Url: user.PhotoURL,
				},
			}
		}

		persons[i] = person
	}

	return persons, nil
}

// SearchDirectory performs advanced directory search with query strings and filters.
// For the local adapter, this performs a simple text search across user names and emails.
func (p *ProviderAdapter) SearchDirectory(opts workspace.PeopleSearchOptions) ([]*people.Person, error) {
	// For local adapter, we treat Query as a search term across names and emails
	users, err := p.adapter.PeopleService().SearchUsers(p.ctx, opts.Query, []string{"names", "emailAddresses", "photos"})
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return []*people.Person{}, nil
	}

	// Apply max results limit if specified
	if opts.MaxResults > 0 && int64(len(users)) > opts.MaxResults {
		users = users[:opts.MaxResults]
	}

	// Convert workspace users to people.Person format
	persons := make([]*people.Person, len(users))
	for i, user := range users {
		person := &people.Person{
			ResourceName: fmt.Sprintf("people/%s", user.Email),
			Etag:         user.Email,
		}

		// Add name if available
		if user.Name != "" {
			person.Names = []*people.Name{
				{
					DisplayName: user.Name,
					GivenName:   user.GivenName,
					FamilyName:  user.FamilyName,
				},
			}
		}

		// Add email with metadata
		person.EmailAddresses = []*people.EmailAddress{
			{
				Value: user.Email,
				Type:  "work",
				Metadata: &people.FieldMetadata{
					Primary:  true,
					Verified: true,
					Source: &people.Source{
						Id:   user.Email,
						Type: "DOMAIN_PROFILE",
					},
				},
			},
		}

		// Add photo if available
		if user.PhotoURL != "" {
			person.Photos = []*people.Photo{
				{
					Url: user.PhotoURL,
				},
			}
		}

		persons[i] = person
	}

	return persons, nil
}

// GetSubfolder finds a subfolder by name within a parent folder with RFC-084 signature.
func (p *ProviderAdapter) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	folder, err := p.adapter.DocumentStorage().GetSubfolder(ctx, parentID, name)
	if err != nil {
		return "", err
	}
	if folder == nil {
		return "", fmt.Errorf("subfolder not found: %s/%s", parentID, name)
	}
	return folder.ID, nil
}

// GetSubfolderLegacy finds a subfolder by name with old Provider interface signature.
// Deprecated: Use GetSubfolder with context parameter instead.
func (p *ProviderAdapter) GetSubfolderLegacy(parentID, name string) (string, error) {
	return p.GetSubfolder(p.ctx, parentID, name)
}

// documentToDriveFile converts a workspace.Document to a drive.File.
func documentToDriveFile(doc *workspace.Document) *drive.File {
	file := &drive.File{
		Id:           doc.ID,
		Name:         doc.Name,
		MimeType:     doc.MimeType,
		CreatedTime:  doc.CreatedTime.Format(time.RFC3339),
		ModifiedTime: doc.ModifiedTime.Format(time.RFC3339),
	}

	if doc.ParentFolderID != "" {
		file.Parents = []string{doc.ParentFolderID}
	}

	if doc.Owner != "" {
		file.Owners = []*drive.User{
			{EmailAddress: doc.Owner},
		}
	}

	if doc.ThumbnailURL != "" {
		file.ThumbnailLink = doc.ThumbnailURL
	}

	return file
}

// updatePermissions updates the permissions for a document.
func (p *ProviderAdapter) updatePermissions(fileID string, permissions []workspace.Permission) error {
	// Get current document to preserve other fields
	doc, err := p.adapter.DocumentStorage().GetDocument(p.ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Convert permissions to JSON string for storage
	// This is necessary because the YAML frontmatter parser doesn't handle complex types well
	permJSON, err := json.Marshal(permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	// Update permissions in metadata as JSON string
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]any)
	}
	doc.Metadata["permissions_json"] = string(permJSON)

	// Update the document
	_, err = p.adapter.DocumentStorage().UpdateDocument(p.ctx, fileID, &workspace.DocumentUpdate{
		Metadata: doc.Metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to update document metadata: %w", err)
	}

	return nil
}

// generatePermissionIDForEmail generates a consistent permission ID based on email.
// This allows us to map between Permission (which has Email) and drive.Permission (which has Id).
func generatePermissionIDForEmail(email string) string {
	// Use a simple hash of the email for consistency
	return fmt.Sprintf("perm-%s", email)
}

// loadPermissionsFromMetadata extracts permissions from document metadata.
// Permissions are stored in metadata["permissions_json"] as a JSON string.
func (p *ProviderAdapter) loadPermissionsFromMetadata(doc *workspace.Document) {
	if doc.Metadata == nil {
		return
	}

	permJSON, ok := doc.Metadata["permissions_json"].(string)
	if !ok || permJSON == "" {
		return
	}

	var permissions []workspace.Permission
	if err := json.Unmarshal([]byte(permJSON), &permissions); err != nil {
		// Log error but don't fail - just return empty permissions
		return
	}

	doc.Permissions = permissions
}

// ShareFileWithDomain shares a file with all users in a domain.
// In the local adapter, this is a no-op since domain sharing doesn't apply.
func (p *ProviderAdapter) ShareFileWithDomain(fileID, domain, role string) error {
	// Local adapter doesn't support domain-wide sharing
	// This is primarily used in Google Workspace
	return nil
}

// CreateFolder creates a new folder with RFC-084 signature.
func (p *ProviderAdapter) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	folder, err := p.adapter.DocumentStorage().CreateFolder(ctx, name, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	// Convert internal Folder to RFC-084 DocumentMetadata
	parents := []string{}
	if folder.ParentID != "" {
		parents = []string{folder.ParentID}
	}

	return &workspace.DocumentMetadata{
		UUID:         docid.UUID{}, // Folders don't have UUIDs in local adapter yet
		ProviderType: "local",
		ProviderID:   folder.ID,
		Name:         folder.Name,
		MimeType:     "application/vnd.google-apps.folder",
		Parents:      parents,
		CreatedTime:  folder.CreatedTime,
		ModifiedTime: folder.ModifiedTime,
		ContentHash:  "",
	}, nil
}

// CreateFolderLegacy creates a new folder with old Provider interface signature.
// Deprecated: Use CreateFolder with context parameter instead.
func (p *ProviderAdapter) CreateFolderLegacy(name, parentID string) (*drive.File, error) {
	meta, err := p.CreateFolder(p.ctx, name, parentID)
	if err != nil {
		return nil, err
	}

	return &drive.File{
		Id:       meta.ProviderID,
		Name:     meta.Name,
		MimeType: meta.MimeType,
		Parents:  meta.Parents,
	}, nil
}

// CreateShortcut creates a shortcut to a target file.
// In the local adapter, shortcuts are stored as metadata references.
func (p *ProviderAdapter) CreateShortcut(targetID, parentID string) (*drive.File, error) {
	// Get target document to determine its mime type
	target, err := p.adapter.DocumentStorage().GetDocument(p.ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target document: %w", err)
	}

	// Determine target mime type
	targetMimeType := "application/vnd.google-apps.document"
	if target.Metadata != nil {
		if mt, ok := target.Metadata["mime_type"].(string); ok {
			targetMimeType = mt
		}
	}

	// Create a document that acts as a shortcut
	shortcut, err := p.adapter.DocumentStorage().CreateDocument(p.ctx, &workspace.DocumentCreate{
		Name:           "Shortcut",
		ParentFolderID: parentID,
		Content:        fmt.Sprintf("Shortcut to: %s", targetID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create shortcut: %w", err)
	}

	// Store the target ID and mime type in metadata
	if shortcut.Metadata == nil {
		shortcut.Metadata = make(map[string]any)
	}
	shortcut.Metadata["shortcut_target"] = targetID
	shortcut.Metadata["shortcut_target_mime_type"] = targetMimeType

	_, err = p.adapter.DocumentStorage().UpdateDocument(p.ctx, shortcut.ID, &workspace.DocumentUpdate{
		Metadata: shortcut.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set shortcut metadata: %w", err)
	}

	return &drive.File{
		Id:       shortcut.ID,
		Name:     shortcut.Name,
		MimeType: "application/vnd.google-apps.shortcut",
		Parents:  []string{parentID},
		ShortcutDetails: &drive.FileShortcutDetails{
			TargetId:       targetID,
			TargetMimeType: targetMimeType,
		},
	}, nil
}

// GetDoc retrieves document content in Google Docs format.
// For the local adapter, this converts markdown content to a simplified docs structure.
func (p *ProviderAdapter) GetDoc(fileID string) (*docs.Document, error) {
	doc, err := p.adapter.DocumentStorage().GetDocument(p.ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Convert to Google Docs format (simplified)
	return &docs.Document{
		DocumentId: doc.ID,
		Title:      doc.Name,
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{
					Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{
								TextRun: &docs.TextRun{
									Content: doc.Content,
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// UpdateDoc updates document content using Google Docs API requests.
// For the local adapter, this is simplified - we don't support complex formatting.
// The local adapter stores markdown files, not Google Docs, so document header
// replacement operations are skipped. Documents are created with template content
// and headers can be manually updated or handled by the indexer.
func (p *ProviderAdapter) UpdateDoc(fileID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
	// For local adapter, we skip Google Docs API operations
	// Headers in markdown files are managed differently than Google Docs tables
	// Return success response so document creation can proceed
	return &docs.BatchUpdateDocumentResponse{
		DocumentId: fileID,
		WriteControl: &docs.WriteControl{
			RequiredRevisionId: "local-revision-1",
		},
	}, nil
}

// GetLatestRevision retrieves the latest revision of a document.
// The local adapter doesn't support revisions, so this returns a placeholder.
func (p *ProviderAdapter) GetLatestRevision(fileID string) (*drive.Revision, error) {
	doc, err := p.adapter.DocumentStorage().GetDocument(p.ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Return a placeholder revision
	return &drive.Revision{
		Id:           "1",
		ModifiedTime: doc.ModifiedTime.Format(time.RFC3339),
		KeepForever:  false,
	}, nil
}

// KeepRevisionForever marks a revision to be kept forever with RFC-084 signature.
// The local adapter doesn't support revisions, so this is a no-op.
func (p *ProviderAdapter) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	// Local adapter doesn't have revision tracking yet
	return nil
}

// KeepRevisionForeverLegacy marks a revision to be kept forever with old Provider interface signature.
// Deprecated: Use KeepRevisionForever with context parameter instead.
func (p *ProviderAdapter) KeepRevisionForeverLegacy(fileID, revisionID string) (*drive.Revision, error) {
	err := p.KeepRevisionForever(p.ctx, fileID, revisionID)
	if err != nil {
		return nil, err
	}
	return &drive.Revision{
		Id:          revisionID,
		KeepForever: true,
	}, nil
}

// UpdateKeepRevisionForever updates the KeepForever flag on a revision.
// The local adapter doesn't support revisions, so this is a no-op.
func (p *ProviderAdapter) UpdateKeepRevisionForever(fileID, revisionID string, keepForever bool) error {
	return nil
}

// SendEmail sends an email notification with RFC-084 signature.
// This delegates to the adapter's notification service.
func (p *ProviderAdapter) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	return p.adapter.NotificationService().SendEmail(ctx, to, from, subject, body)
}

// SendEmailLegacy sends an email with old Provider interface signature.
// Deprecated: Use SendEmail with context parameter instead.
func (p *ProviderAdapter) SendEmailLegacy(to []string, from, subject, body string) error {
	return p.SendEmail(p.ctx, to, from, subject, body)
}

// ListGroups lists groups matching a query in a domain.
// The local adapter doesn't support groups, so this returns an empty list.
func (p *ProviderAdapter) ListGroups(domain, query string, maxResults int64) ([]*admin.Group, error) {
	// Local adapter doesn't support group management
	return []*admin.Group{}, nil
}

// ListUserGroups lists all groups a user is a member of.
// The local adapter doesn't support groups, so this returns an empty list.
func (p *ProviderAdapter) ListUserGroups(userEmail string) ([]*admin.Group, error) {
	// Local adapter doesn't support group management
	return []*admin.Group{}, nil
}

// SupportsContentEditing returns true as the local workspace provider supports
// direct content editing through GetDocumentContent/UpdateDocumentContent.
func (p *ProviderAdapter) SupportsContentEditing() bool {
	return true
}

// Content operations

// GetDocumentContent retrieves the full text content of a document.
func (p *ProviderAdapter) GetDocumentContent(fileID string) (string, error) {
	return p.adapter.DocumentStorage().GetDocumentContent(p.ctx, fileID)
}

// UpdateDocumentContent updates the text content of a document.
func (p *ProviderAdapter) UpdateDocumentContent(fileID, content string) error {
	return p.adapter.DocumentStorage().UpdateDocumentContent(p.ctx, fileID, content)
}

// CompareContent is a stub for RFC-084 ContentProvider interface.
// TODO: Implement actual content comparison
func (p *ProviderAdapter) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	return p.adapter.CompareContent(ctx, providerID1, providerID2)
}

// ===================================================================
// RFC-084 DocumentProvider stub implementations
// ===================================================================

// CopyDocument creates a copy of a document.
// TODO: Implement actual document copying
func (p *ProviderAdapter) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("CopyDocument not yet implemented for local adapter")
}

// MoveDocument moves a document to a different folder.
// TODO: Implement actual document moving
func (p *ProviderAdapter) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("MoveDocument not yet implemented for local adapter")
}

// DeleteDocument deletes a document.
// TODO: Implement actual document deletion
func (p *ProviderAdapter) DeleteDocument(ctx context.Context, providerID string) error {
	return fmt.Errorf("DeleteDocument not yet implemented for local adapter")
}

// RenameDocument renames a document.
// TODO: Implement actual document renaming
func (p *ProviderAdapter) RenameDocument(ctx context.Context, providerID, newName string) error {
	return fmt.Errorf("RenameDocument not yet implemented for local adapter")
}

// GetDocument retrieves document metadata by provider ID.
// TODO: Implement actual document metadata retrieval
func (p *ProviderAdapter) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("GetDocument not yet implemented for local adapter")
}

// GetDocumentByUUID retrieves document metadata by UUID.
// TODO: Implement actual document metadata retrieval by UUID
func (p *ProviderAdapter) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("GetDocumentByUUID not yet implemented for local adapter")
}

// CreateDocument creates a new document from template.
// TODO: Implement actual document creation
func (p *ProviderAdapter) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("CreateDocument not yet implemented for local adapter")
}

// CreateDocumentWithUUID creates a document with explicit UUID.
// TODO: Implement actual document creation with UUID
func (p *ProviderAdapter) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("CreateDocumentWithUUID not yet implemented for local adapter")
}

// RegisterDocument registers document metadata with provider.
// TODO: Implement actual document registration
func (p *ProviderAdapter) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("RegisterDocument not yet implemented for local adapter")
}

// ===================================================================
// RFC-084 ContentProvider stub implementations
// ===================================================================

// GetContent retrieves document content with revision info.
func (p *ProviderAdapter) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	// Extract document ID from providerID (format: "local:doc-id")
	docID := strings.TrimPrefix(providerID, "local:")

	// Get document metadata
	doc, err := p.adapter.DocumentStorage().GetDocument(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Get document content
	content, err := p.adapter.DocumentStorage().GetDocumentContent(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document content: %w", err)
	}

	// Extract UUID from CompositeID or generate a default one
	var uuid docid.UUID
	if doc.CompositeID != nil {
		uuid = doc.CompositeID.UUID()
	} else {
		uuid = docid.NewUUID()
	}

	// Return RFC-084 DocumentContent
	return &workspace.DocumentContent{
		UUID:         uuid,
		ProviderID:   providerID,
		Title:        doc.Name,
		Body:         content,
		Format:       "markdown",
		ContentHash:  "", // ContentHash calculation not yet implemented
		LastModified: doc.ModifiedTime,
	}, nil
}

// GetContentByUUID retrieves content by UUID.
// TODO: Implement actual content retrieval by UUID
func (p *ProviderAdapter) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("GetContentByUUID not yet implemented for local adapter")
}

// UpdateContent updates document content.
func (p *ProviderAdapter) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	// Extract document ID from providerID (format: "local:doc-id")
	docID := strings.TrimPrefix(providerID, "local:")

	// Update document content
	err := p.adapter.DocumentStorage().UpdateDocumentContent(ctx, docID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to update document content: %w", err)
	}

	// Get updated document metadata
	doc, err := p.adapter.DocumentStorage().GetDocument(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document after update: %w", err)
	}

	// Extract UUID from CompositeID or generate a default one
	var uuid docid.UUID
	if doc.CompositeID != nil {
		uuid = doc.CompositeID.UUID()
	} else {
		uuid = docid.NewUUID()
	}

	// Return updated DocumentContent
	return &workspace.DocumentContent{
		UUID:         uuid,
		ProviderID:   providerID,
		Title:        doc.Name,
		Body:         content,
		Format:       "markdown",
		ContentHash:  "", // ContentHash calculation not yet implemented
		LastModified: doc.ModifiedTime,
	}, nil
}

// GetContentBatch retrieves multiple documents efficiently.
// TODO: Implement actual batch content retrieval
func (p *ProviderAdapter) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("GetContentBatch not yet implemented for local adapter")
}

// ===================================================================
// RFC-084 RevisionTrackingProvider stub implementations
// ===================================================================

// GetRevisionHistory lists all revisions for a document.
// TODO: Implement actual revision history retrieval
func (p *ProviderAdapter) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("GetRevisionHistory not yet implemented for local adapter")
}

// GetRevision retrieves a specific revision.
// TODO: Implement actual revision retrieval
func (p *ProviderAdapter) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("GetRevision not yet implemented for local adapter")
}

// GetRevisionContent retrieves content at a specific revision.
// TODO: Implement actual revision content retrieval
func (p *ProviderAdapter) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("GetRevisionContent not yet implemented for local adapter")
}

// GetAllDocumentRevisions returns all revisions across all backends for a UUID.
// TODO: Implement actual multi-backend revision retrieval
func (p *ProviderAdapter) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	return nil, fmt.Errorf("GetAllDocumentRevisions not yet implemented for local adapter")
}

// ===================================================================
// RFC-084 PermissionProvider stub implementations
// ===================================================================

// ShareDocument grants access to a user/group.
// TODO: Implement actual permission granting
func (p *ProviderAdapter) ShareDocument(ctx context.Context, providerID, email, role string) error {
	return fmt.Errorf("ShareDocument not yet implemented for local adapter")
}

// ShareDocumentWithDomain grants access to entire domain.
// TODO: Implement actual domain-wide permission granting
func (p *ProviderAdapter) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	// Local adapter doesn't support domain-wide sharing
	return nil
}

// RemovePermission revokes access.
// TODO: Implement actual permission revocation
func (p *ProviderAdapter) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	return fmt.Errorf("RemovePermission not yet implemented for local adapter")
}

// UpdatePermission changes permission role.
// TODO: Implement actual permission update
func (p *ProviderAdapter) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	return fmt.Errorf("UpdatePermission not yet implemented for local adapter")
}

// ===================================================================
// RFC-084 PeopleProvider stub implementations
// NOTE: SearchPeople has old signature - RFC-084 version not added to avoid conflict
// ===================================================================

// GetPerson retrieves a user by email.
// TODO: Implement actual user retrieval
func (p *ProviderAdapter) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("GetPerson not yet implemented for local adapter")
}

// GetPersonByUnifiedID retrieves user by unified ID.
// TODO: Implement actual unified ID lookup
func (p *ProviderAdapter) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("GetPersonByUnifiedID not yet implemented for local adapter")
}

// ResolveIdentity resolves alternate identities for a user.
// TODO: Implement actual identity resolution
func (p *ProviderAdapter) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("ResolveIdentity not yet implemented for local adapter")
}

// ===================================================================
// RFC-084 TeamProvider stub implementations
// ===================================================================

// ListTeams lists teams matching query.
// TODO: Implement actual team listing
func (p *ProviderAdapter) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	// Local adapter doesn't support team management
	return []*workspace.Team{}, nil
}

// GetTeam retrieves team details.
// TODO: Implement actual team retrieval
func (p *ProviderAdapter) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	return nil, fmt.Errorf("GetTeam not yet implemented for local adapter")
}

// GetUserTeams lists all teams a user belongs to.
// TODO: Implement actual user team listing
func (p *ProviderAdapter) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	// Local adapter doesn't support team management
	return []*workspace.Team{}, nil
}

// GetTeamMembers lists all members of a team.
// TODO: Implement actual team member listing
func (p *ProviderAdapter) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("GetTeamMembers not yet implemented for local adapter")
}

// ===================================================================
// RFC-084 NotificationProvider stub implementations
// ===================================================================

// SendEmailWithTemplate sends email using template.
// TODO: Implement actual template-based email sending
func (p *ProviderAdapter) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	return fmt.Errorf("SendEmailWithTemplate not yet implemented for local adapter")
}
