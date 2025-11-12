package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ===================================================================
// DocumentProvider Implementation
// ===================================================================
// All methods delegate to remote Hermes /api/v2/documents/* endpoints

// GetDocument retrieves file metadata from remote Hermes
func (p *Provider) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	path := fmt.Sprintf("/api/v2/documents/%s", url.PathEscape(providerID))

	var doc workspace.DocumentMetadata
	if err := p.doRequest(ctx, "GET", path, nil, &doc); err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return &doc, nil
}

// GetDocumentByUUID retrieves document by UUID from remote Hermes
func (p *Provider) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	path := fmt.Sprintf("/api/v2/documents/uuid/%s", uuid.String())

	var doc workspace.DocumentMetadata
	if err := p.doRequest(ctx, "GET", path, nil, &doc); err != nil {
		return nil, fmt.Errorf("failed to get document by UUID: %w", err)
	}

	return &doc, nil
}

// CreateDocument creates a new document from template on remote Hermes
func (p *Provider) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	path := "/api/v2/documents"

	requestBody := map[string]interface{}{
		"templateID":   templateID,
		"destFolderID": destFolderID,
		"name":         name,
	}

	var doc workspace.DocumentMetadata
	if err := p.doRequest(ctx, "POST", path, requestBody, &doc); err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	return &doc, nil
}

// CreateDocumentWithUUID creates document with explicit UUID (for migration)
func (p *Provider) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	path := "/api/v2/documents"

	requestBody := map[string]interface{}{
		"uuid":         uuid.String(),
		"templateID":   templateID,
		"destFolderID": destFolderID,
		"name":         name,
	}

	var doc workspace.DocumentMetadata
	if err := p.doRequest(ctx, "POST", path, requestBody, &doc); err != nil {
		return nil, fmt.Errorf("failed to create document with UUID: %w", err)
	}

	return &doc, nil
}

// RegisterDocument registers document metadata with remote provider (for tracking)
func (p *Provider) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	path := "/api/v2/documents/register"

	var registered workspace.DocumentMetadata
	if err := p.doRequest(ctx, "POST", path, doc, &registered); err != nil {
		return nil, fmt.Errorf("failed to register document: %w", err)
	}

	return &registered, nil
}

// CopyDocument copies a document on remote Hermes
func (p *Provider) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	path := fmt.Sprintf("/api/v2/documents/%s/copy", url.PathEscape(srcProviderID))

	requestBody := map[string]string{
		"destFolderID": destFolderID,
		"name":         name,
	}

	var doc workspace.DocumentMetadata
	if err := p.doRequest(ctx, "POST", path, requestBody, &doc); err != nil {
		return nil, fmt.Errorf("failed to copy document: %w", err)
	}

	return &doc, nil
}

// MoveDocument moves a document to different folder on remote Hermes
func (p *Provider) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	path := fmt.Sprintf("/api/v2/documents/%s/move", url.PathEscape(providerID))

	requestBody := map[string]string{
		"destFolderID": destFolderID,
	}

	var doc workspace.DocumentMetadata
	if err := p.doRequest(ctx, "PUT", path, requestBody, &doc); err != nil {
		return nil, fmt.Errorf("failed to move document: %w", err)
	}

	return &doc, nil
}

// DeleteDocument deletes a document on remote Hermes
func (p *Provider) DeleteDocument(ctx context.Context, providerID string) error {
	path := fmt.Sprintf("/api/v2/documents/%s", url.PathEscape(providerID))

	if err := p.doRequest(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	return nil
}

// RenameDocument renames a document on remote Hermes
func (p *Provider) RenameDocument(ctx context.Context, providerID, newName string) error {
	path := fmt.Sprintf("/api/v2/documents/%s", url.PathEscape(providerID))

	requestBody := map[string]string{
		"name": newName,
	}

	if err := p.doRequest(ctx, "PATCH", path, requestBody, nil); err != nil {
		return fmt.Errorf("failed to rename document: %w", err)
	}

	return nil
}

// CreateFolder creates a folder/directory on remote Hermes
func (p *Provider) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	path := "/api/v2/folders"

	requestBody := map[string]string{
		"name":     name,
		"parentID": parentID,
	}

	var folder workspace.DocumentMetadata
	if err := p.doRequest(ctx, "POST", path, requestBody, &folder); err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return &folder, nil
}

// GetSubfolder finds a subfolder by name on remote Hermes
func (p *Provider) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	path := fmt.Sprintf("/api/v2/folders/%s/subfolders/%s",
		url.PathEscape(parentID),
		url.PathEscape(name))

	var response struct {
		ID string `json:"id"`
	}

	if err := p.doRequest(ctx, "GET", path, nil, &response); err != nil {
		return "", fmt.Errorf("failed to get subfolder: %w", err)
	}

	return response.ID, nil
}
