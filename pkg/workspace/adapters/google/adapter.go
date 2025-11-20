package google

import (
	"context"
	"fmt"

	"google.golang.org/api/drive/v3"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Adapter implements WorkspaceProvider interface for Google Workspace.
// This adapter uses Hermes-native types instead of Google-specific types.
//
// Implements:
// - DocumentProvider
// - ContentProvider
// - RevisionTrackingProvider
// - PermissionProvider
// - PeopleProvider
// - TeamProvider
// - NotificationProvider
type Adapter struct {
	service *Service
}

// Compile-time interface checks - ensures Adapter implements all RFC-084 interfaces
var (
	_ workspace.WorkspaceProvider        = (*Adapter)(nil)
	_ workspace.DocumentProvider         = (*Adapter)(nil)
	_ workspace.ContentProvider          = (*Adapter)(nil)
	_ workspace.RevisionTrackingProvider = (*Adapter)(nil)
	_ workspace.PermissionProvider       = (*Adapter)(nil)
	_ workspace.PeopleProvider           = (*Adapter)(nil)
	_ workspace.TeamProvider             = (*Adapter)(nil)
	_ workspace.NotificationProvider     = (*Adapter)(nil)
)

// NewAdapter creates a new Google Workspace adapter.
func NewAdapter(service *Service) workspace.WorkspaceProvider {
	return &Adapter{
		service: service,
	}
}

// GetService returns the underlying Service for Google-specific operations.
// This is useful for operations that need direct Google API access (e.g., GetDoc for suggestions).
func (a *Adapter) GetService() *Service {
	return a.service
}

// ===================================================================
// DocumentProvider Implementation
// ===================================================================

// GetDocument retrieves document metadata by backend-specific ID.
// For Google: providerID = "google:{fileId}"
func (a *Adapter) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	// Extract Google file ID from providerID
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return nil, err
	}

	// Get file from Google Drive
	file, err := a.service.GetFile(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Google Drive file: %w", err)
	}

	// Convert to RFC-084 DocumentMetadata
	return ConvertToDocumentMetadata(file)
}

// GetDocumentByUUID retrieves document metadata by UUID.
// This requires searching Google Drive for files with hermesUuid custom property.
func (a *Adapter) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	// Search for file with hermesUuid property
	query := fmt.Sprintf("properties has { key='hermesUuid' and value='%s' } and trashed=false", uuid.String())

	files, err := a.service.Drive.Files.List().
		Q(query).
		Fields("files(id,name,mimeType,createdTime,modifiedTime,owners,parents,properties,version,webViewLink,thumbnailLink)").
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to search for document by UUID: %w", err)
	}

	if len(files.Files) == 0 {
		return nil, fmt.Errorf("document with UUID %s not found", uuid.String())
	}

	if len(files.Files) > 1 {
		return nil, fmt.Errorf("multiple documents found with UUID %s", uuid.String())
	}

	return ConvertToDocumentMetadata(files.Files[0])
}

// CreateDocument creates a new document from template.
func (a *Adapter) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	// Generate new UUID
	uuid := docid.NewUUID()

	return a.CreateDocumentWithUUID(ctx, uuid, templateID, destFolderID, name)
}

// CreateDocumentWithUUID creates document with explicit UUID (for migration).
func (a *Adapter) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	// Copy file from template
	file, err := a.service.CopyFile(templateID, destFolderID, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create document from template: %w", err)
	}

	// Update file with UUID in custom properties
	if err := UpdateFileWithUUID(a.service.Drive, file.Id, uuid); err != nil {
		return nil, fmt.Errorf("failed to set UUID on document: %w", err)
	}

	// Get updated file
	file, err = a.service.GetFile(file.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve created document: %w", err)
	}

	return ConvertToDocumentMetadata(file)
}

// RegisterDocument registers document metadata with provider (for tracking).
// For Google, this means ensuring the UUID is stored in custom properties.
func (a *Adapter) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	// Extract Google file ID from providerID
	fileID, err := extractGoogleFileID(doc.ProviderID)
	if err != nil {
		return nil, err
	}

	// Update file with UUID in custom properties
	if err := UpdateFileWithUUID(a.service.Drive, fileID, doc.UUID); err != nil {
		return nil, fmt.Errorf("failed to register document UUID: %w", err)
	}

	// Return updated metadata
	return a.GetDocument(ctx, doc.ProviderID)
}

// CopyDocument copies a document (preserves UUID if in frontmatter/metadata).
func (a *Adapter) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	srcFileID, err := extractGoogleFileID(srcProviderID)
	if err != nil {
		return nil, err
	}

	// Copy file
	file, err := a.service.CopyFile(srcFileID, destFolderID, name)
	if err != nil {
		return nil, fmt.Errorf("failed to copy document: %w", err)
	}

	return ConvertToDocumentMetadata(file)
}

// MoveDocument moves a document to different folder.
func (a *Adapter) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return nil, err
	}

	file, err := a.service.MoveFile(fileID, destFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to move document: %w", err)
	}

	return ConvertToDocumentMetadata(file)
}

// DeleteDocument deletes a document.
func (a *Adapter) DeleteDocument(ctx context.Context, providerID string) error {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return err
	}

	return a.service.DeleteFile(fileID)
}

// RenameDocument renames a document.
func (a *Adapter) RenameDocument(ctx context.Context, providerID, newName string) error {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return err
	}

	return a.service.RenameFile(fileID, newName)
}

// CreateFolder creates a folder/directory.
func (a *Adapter) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	file, err := a.service.CreateFolder(name, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return ConvertToDocumentMetadata(file)
}

// GetSubfolder finds a subfolder by name.
func (a *Adapter) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	subfolder, err := a.service.GetSubfolder(parentID, name)
	if err != nil {
		return "", err
	}
	if subfolder == nil {
		return "", nil
	}
	return subfolder.Id, nil
}

// ===================================================================
// ContentProvider Implementation
// ===================================================================

// GetContent retrieves document content with backend-specific revision.
func (a *Adapter) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return nil, err
	}

	// Get file metadata
	file, err := a.service.GetFile(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	// Get document content (for Google Docs)
	doc, err := a.service.GetDoc(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document content: %w", err)
	}

	return ConvertToDocumentContent(doc, file)
}

// GetContentByUUID retrieves content using UUID (looks up providerID).
func (a *Adapter) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	// First get document metadata to find providerID
	meta, err := a.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	return a.GetContent(ctx, meta.ProviderID)
}

// UpdateContent updates document content.
func (a *Adapter) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	// Note: For Google Docs, this would require using the Docs API to update content
	// This is a complex operation that requires converting markdown to Google Docs format
	// For now, return not implemented
	return nil, fmt.Errorf("UpdateContent not yet implemented for Google Workspace")
}

// GetContentBatch retrieves multiple documents (efficient for migration).
func (a *Adapter) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	results := make([]*workspace.DocumentContent, 0, len(providerIDs))

	for _, providerID := range providerIDs {
		content, err := a.GetContent(ctx, providerID)
		if err != nil {
			// Log error but continue with other documents
			continue
		}
		results = append(results, content)
	}

	return results, nil
}

// CompareContent compares content between two revisions.
func (a *Adapter) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	content1, err := a.GetContent(ctx, providerID1)
	if err != nil {
		return nil, fmt.Errorf("failed to get content1: %w", err)
	}

	content2, err := a.GetContent(ctx, providerID2)
	if err != nil {
		return nil, fmt.Errorf("failed to get content2: %w", err)
	}

	comparison := &workspace.ContentComparison{
		UUID:         content1.UUID,
		Revision1:    content1.BackendRevision,
		Revision2:    content2.BackendRevision,
		ContentMatch: content1.ContentHash == content2.ContentHash,
	}

	if comparison.ContentMatch {
		comparison.HashDifference = "same"
	} else {
		// Could implement more sophisticated diff analysis here
		comparison.HashDifference = "major"
	}

	return comparison, nil
}

// ===================================================================
// RevisionTrackingProvider Implementation
// ===================================================================

// GetRevisionHistory lists all revisions for a document in this backend.
func (a *Adapter) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return nil, err
	}

	// Get revisions from Google Drive
	listCall := a.service.Drive.Revisions.List(fileID).
		Fields("revisions(id,modifiedTime,keepForever,size,md5Checksum,mimeType,lastModifyingUser)").
		Context(ctx)

	if limit > 0 {
		listCall = listCall.PageSize(int64(limit))
	}

	revisions, err := listCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list revisions: %w", err)
	}

	// Convert to BackendRevision
	results := make([]*workspace.BackendRevision, 0, len(revisions.Revisions))
	for _, rev := range revisions.Revisions {
		results = append(results, ConvertDriveRevisionToBackendRevision(rev))
	}

	return results, nil
}

// GetRevision retrieves a specific revision.
func (a *Adapter) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return nil, err
	}

	rev, err := a.service.Drive.Revisions.Get(fileID, revisionID).
		Fields("id,modifiedTime,keepForever,size,md5Checksum,mimeType,lastModifyingUser").
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}

	return ConvertDriveRevisionToBackendRevision(rev), nil
}

// GetRevisionContent retrieves content at a specific revision.
func (a *Adapter) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	_, err := extractGoogleFileID(providerID)
	if err != nil {
		return nil, err
	}

	// Get revision metadata
	rev, err := a.GetRevision(ctx, providerID, revisionID)
	if err != nil {
		return nil, err
	}

	// Export revision content
	// Note: This would require downloading the specific revision
	// For now, return not fully implemented
	_ = rev

	return nil, fmt.Errorf("GetRevisionContent not yet fully implemented for Google Workspace")
}

// KeepRevisionForever marks a revision as permanent (if supported).
func (a *Adapter) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return err
	}

	rev := &drive.Revision{
		KeepForever: true,
	}

	_, err = a.service.Drive.Revisions.Update(fileID, revisionID, rev).
		Context(ctx).
		Do()

	return err
}

// GetAllDocumentRevisions returns all revisions across all backends for a UUID.
// For Google adapter, this only returns Google revisions.
func (a *Adapter) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	// Get document metadata to find providerID
	meta, err := a.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Get revision history
	backendRevisions, err := a.GetRevisionHistory(ctx, meta.ProviderID, 0)
	if err != nil {
		return nil, err
	}

	// Convert to RevisionInfo
	results := make([]*workspace.RevisionInfo, 0, len(backendRevisions))
	for _, backendRev := range backendRevisions {
		revInfo := &workspace.RevisionInfo{
			UUID:            uuid,
			ProviderType:    "google",
			ProviderID:      meta.ProviderID,
			BackendRevision: backendRev,
			ContentHash:     meta.ContentHash,
			SyncStatus:      "canonical",
		}
		results = append(results, revInfo)
	}

	return results, nil
}

// ===================================================================
// Helper Functions
// ===================================================================

// extractGoogleFileID extracts the file ID from a Google providerID.
// Format: "google:{fileId}"
func extractGoogleFileID(providerID string) (string, error) {
	const prefix = "google:"
	if len(providerID) <= len(prefix) {
		return "", fmt.Errorf("invalid Google providerID: %s", providerID)
	}
	if providerID[:len(prefix)] != prefix {
		return "", fmt.Errorf("providerID is not a Google ID: %s", providerID)
	}
	return providerID[len(prefix):], nil
}
