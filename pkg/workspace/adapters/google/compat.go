package google

import (
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/people/v1"
)

// CompatAdapter wraps the Service to implement the old workspace.Provider interface.
// This provides backward compatibility for code that hasn't been migrated to WorkspaceProvider yet.
//
// DEPRECATED: Use Adapter (which implements WorkspaceProvider) instead.
type CompatAdapter struct {
	service *Service
}

// NewCompatAdapter creates a compatibility adapter for the old Provider interface.
//
// DEPRECATED: Use NewAdapter instead.
func NewCompatAdapter(service *Service) workspace.Provider {
	return &CompatAdapter{
		service: service,
	}
}

// File operations

func (a *CompatAdapter) GetFile(fileID string) (*drive.File, error) {
	return a.service.GetFile(fileID)
}

func (a *CompatAdapter) CopyFile(srcID, destFolderID, name string) (*drive.File, error) {
	return a.service.CopyFile(srcID, destFolderID, name)
}

func (a *CompatAdapter) MoveFile(fileID, destFolderID string) (*drive.File, error) {
	return a.service.MoveFile(fileID, destFolderID)
}

func (a *CompatAdapter) DeleteFile(fileID string) error {
	return a.service.DeleteFile(fileID)
}

func (a *CompatAdapter) RenameFile(fileID, newName string) error {
	return a.service.RenameFile(fileID, newName)
}

func (a *CompatAdapter) ShareFile(fileID, email, role string) error {
	return a.service.ShareFile(fileID, email, role)
}

func (a *CompatAdapter) ShareFileWithDomain(fileID, domain, role string) error {
	return a.service.ShareFileWithDomain(fileID, domain, role)
}

func (a *CompatAdapter) ListPermissions(fileID string) ([]*drive.Permission, error) {
	return a.service.ListPermissions(fileID)
}

func (a *CompatAdapter) DeletePermission(fileID, permissionID string) error {
	return a.service.DeletePermission(fileID, permissionID)
}

func (a *CompatAdapter) CreateFileAsUser(templateID, destFolderID, name, userEmail string) (*drive.File, error) {
	return a.service.CreateFileAsUser(templateID, destFolderID, name, userEmail)
}

// People operations

func (a *CompatAdapter) SearchPeople(email string, fields string) ([]*people.Person, error) {
	return a.service.SearchPeople(email, fields)
}

func (a *CompatAdapter) SearchDirectory(opts workspace.PeopleSearchOptions) ([]*people.Person, error) {
	return a.service.SearchDirectory(opts)
}

// Folder operations

func (a *CompatAdapter) GetSubfolder(parentID, name string) (string, error) {
	subfolder, err := a.service.GetSubfolder(parentID, name)
	if err != nil {
		return "", err
	}
	if subfolder == nil {
		return "", nil
	}
	return subfolder.Id, nil
}

func (a *CompatAdapter) CreateFolder(name, parentID string) (*drive.File, error) {
	return a.service.CreateFolder(name, parentID)
}

func (a *CompatAdapter) CreateShortcut(targetID, parentID string) (*drive.File, error) {
	return a.service.CreateShortcut(targetID, parentID)
}

// Document content operations (Google Docs)

func (a *CompatAdapter) GetDoc(fileID string) (*docs.Document, error) {
	return a.service.GetDoc(fileID)
}

func (a *CompatAdapter) UpdateDoc(fileID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
	// Use the Docs API directly since Service doesn't wrap this
	req := &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}
	return a.service.Docs.Documents.BatchUpdate(fileID, req).Do()
}

// Revision management

func (a *CompatAdapter) GetLatestRevision(fileID string) (*drive.Revision, error) {
	return a.service.GetLatestRevision(fileID)
}

func (a *CompatAdapter) KeepRevisionForever(fileID, revisionID string) (*drive.Revision, error) {
	return a.service.KeepRevisionForever(fileID, revisionID)
}

func (a *CompatAdapter) UpdateKeepRevisionForever(fileID, revisionID string, keepForever bool) error {
	return a.service.UpdateKeepRevisionForever(fileID, revisionID, keepForever)
}

// Email operations

func (a *CompatAdapter) SendEmail(to []string, from, subject, body string) error {
	return a.service.SendEmail(to, from, subject, body)
}

// Group operations (Google Admin Directory)

func (a *CompatAdapter) ListGroups(domain, query string, maxResults int64) ([]*admin.Group, error) {
	// Use Admin Directory API directly
	call := a.service.AdminDirectory.Groups.List().
		Domain(domain).
		MaxResults(maxResults)

	if query != "" {
		call = call.Query(query)
	}

	groups, err := call.Do()
	if err != nil {
		return nil, err
	}
	return groups.Groups, nil
}

func (a *CompatAdapter) ListUserGroups(userEmail string) ([]*admin.Group, error) {
	// Use Admin Directory API directly
	groups, err := a.service.AdminDirectory.Groups.List().
		UserKey(userEmail).
		Do()
	if err != nil {
		return nil, err
	}
	return groups.Groups, nil
}

// Content operations

func (a *CompatAdapter) GetDocumentContent(fileID string) (string, error) {
	// Get Google Doc and extract text
	doc, err := a.service.GetDoc(fileID)
	if err != nil {
		return "", fmt.Errorf("failed to get document: %w", err)
	}

	// Extract plain text from document
	return ExtractDocText(doc), nil
}

func (a *CompatAdapter) UpdateDocumentContent(fileID, content string) error {
	// Google Docs content updates are complex - not yet implemented
	return fmt.Errorf("UpdateDocumentContent not implemented for Google Workspace")
}

// SupportsContentEditing implements workspace.ProviderCapabilities.
func (a *CompatAdapter) SupportsContentEditing() bool {
	// Google adapter supports content editing
	return true
}
