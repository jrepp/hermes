package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// DocumentProvider interface implementation

// GetDocument retrieves document metadata by backend-specific ID
func (a *Adapter) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	objectKey := a.parseProviderID(providerID)

	// Get metadata from store
	metadata, err := a.metadataStore.Get(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get document metadata: %w", err)
	}

	return metadata, nil
}

// GetDocumentByUUID retrieves document metadata by UUID
// This requires searching through the metadata store
func (a *Adapter) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	// List all documents and find the one with matching UUID
	// TODO: Optimize this with an index (DynamoDB metadata store would be better for this)
	providerIDs, err := a.metadataStore.List(ctx, a.cfg.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	for _, providerID := range providerIDs {
		objectKey := a.parseProviderID(providerID)
		metadata, err := a.metadataStore.Get(ctx, objectKey)
		if err != nil {
			continue // Skip documents with errors
		}
		if metadata.UUID == uuid {
			return metadata, nil
		}
	}

	return nil, workspace.NotFoundError("document", uuid.String())
}

// CreateDocument creates a new document from template
func (a *Adapter) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	uuid := docid.NewUUID()
	return a.CreateDocumentWithUUID(ctx, uuid, templateID, destFolderID, name)
}

// CreateDocumentWithUUID creates document with explicit UUID (for migration)
func (a *Adapter) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	// Fetch template content if specified
	content := ""
	if templateID != "" {
		templateDoc, err := a.GetDocument(ctx, templateID)
		if err != nil {
			return nil, fmt.Errorf("failed to get template: %w", err)
		}
		templateContent, err := a.GetContent(ctx, templateDoc.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("failed to get template content: %w", err)
		}
		content = templateContent.Body
	}

	// Build object key using path template
	metadata := make(map[string]any)
	if destFolderID != "" {
		metadata["folder"] = destFolderID
	}
	objectKey := a.buildObjectKey(uuid, name, metadata)

	// Create document metadata
	now := time.Now()
	doc := &workspace.DocumentMetadata{
		UUID:         uuid,
		ProviderType: "s3",
		ProviderID:   a.formatProviderID(objectKey),
		Name:         name,
		MimeType:     a.cfg.DefaultMimeType,
		CreatedTime:  now,
		ModifiedTime: now,
		SyncStatus:   "canonical",
		ContentHash:  computeContentHash(content),
	}

	// Write content to S3
	s3Metadata := map[string]string{
		"hermes-uuid": uuid.String(),
		"hermes-name": name,
	}
	_, err := a.putObject(ctx, objectKey, []byte(content), s3Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create document in S3: %w", err)
	}

	// Store metadata
	if err := a.metadataStore.Set(ctx, objectKey, doc); err != nil {
		// Try to clean up the S3 object if metadata storage fails
		if delErr := a.deleteObject(ctx, objectKey); delErr != nil {
			a.logger.Warn("failed to clean up S3 object after metadata error",
				"error", delErr, "key", objectKey)
		}
		return nil, fmt.Errorf("failed to store metadata: %w", err)
	}

	a.logger.Info("document created",
		"uuid", uuid.String(),
		"name", name,
		"key", objectKey)

	return doc, nil
}

// RegisterDocument registers document metadata with provider (for tracking)
func (a *Adapter) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	objectKey := a.parseProviderID(doc.ProviderID)

	// Store metadata
	if err := a.metadataStore.Set(ctx, objectKey, doc); err != nil {
		return nil, fmt.Errorf("failed to register document: %w", err)
	}

	a.logger.Info("document registered",
		"uuid", doc.UUID.String(),
		"provider_id", doc.ProviderID)

	return doc, nil
}

// CopyDocument copies a document (preserves UUID if in frontmatter/metadata)
func (a *Adapter) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	// Get source document
	srcDoc, err := a.GetDocument(ctx, srcProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source document: %w", err)
	}

	// Get source content
	srcContent, err := a.GetContent(ctx, srcProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source content: %w", err)
	}

	// Create new document with new UUID (standard copy behavior)
	newUUID := docid.NewUUID()
	metadata := make(map[string]any)
	if destFolderID != "" {
		metadata["folder"] = destFolderID
	}
	objectKey := a.buildObjectKey(newUUID, name, metadata)

	now := time.Now()
	destDoc := &workspace.DocumentMetadata{
		UUID:             newUUID,
		ProviderType:     "s3",
		ProviderID:       a.formatProviderID(objectKey),
		Name:             name,
		MimeType:         srcDoc.MimeType,
		CreatedTime:      now,
		ModifiedTime:     now,
		SyncStatus:       "canonical",
		ContentHash:      computeContentHash(srcContent.Body),
		ExtendedMetadata: srcDoc.ExtendedMetadata, // Preserve extended metadata
	}

	// Write content to S3
	s3Metadata := map[string]string{
		"hermes-uuid": newUUID.String(),
		"hermes-name": name,
	}
	_, err = a.putObject(ctx, objectKey, []byte(srcContent.Body), s3Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to copy document to S3: %w", err)
	}

	// Store metadata
	if err := a.metadataStore.Set(ctx, objectKey, destDoc); err != nil {
		if delErr := a.deleteObject(ctx, objectKey); delErr != nil {
			a.logger.Warn("failed to clean up S3 object after metadata error",
				"error", delErr, "key", objectKey)
		}
		return nil, fmt.Errorf("failed to store metadata: %w", err)
	}

	a.logger.Info("document copied",
		"src_uuid", srcDoc.UUID.String(),
		"dest_uuid", newUUID.String(),
		"name", name)

	return destDoc, nil
}

// MoveDocument moves a document to different folder
func (a *Adapter) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	// Get current document
	doc, err := a.GetDocument(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Get current content
	content, err := a.GetContent(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	// Build new object key with different folder
	metadata := make(map[string]any)
	metadata["folder"] = destFolderID
	newObjectKey := a.buildObjectKey(doc.UUID, doc.Name, metadata)
	oldObjectKey := a.parseProviderID(providerID)

	// Copy to new location
	s3Metadata := map[string]string{
		"hermes-uuid": doc.UUID.String(),
		"hermes-name": doc.Name,
	}
	_, err = a.putObject(ctx, newObjectKey, []byte(content.Body), s3Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to copy to new location: %w", err)
	}

	// Update metadata with new provider ID
	doc.ProviderID = a.formatProviderID(newObjectKey)
	doc.ModifiedTime = time.Now()
	if err := a.metadataStore.Set(ctx, newObjectKey, doc); err != nil {
		if delErr := a.deleteObject(ctx, newObjectKey); delErr != nil {
			a.logger.Warn("failed to clean up S3 object after metadata error",
				"error", delErr, "key", newObjectKey)
		}
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}

	// Delete old location
	if err := a.deleteObject(ctx, oldObjectKey); err != nil {
		a.logger.Warn("failed to delete old S3 object after move",
			"error", err, "key", oldObjectKey)
	}
	if err := a.metadataStore.Delete(ctx, oldObjectKey); err != nil {
		a.logger.Warn("failed to delete old metadata after move",
			"error", err, "key", oldObjectKey)
	}

	a.logger.Info("document moved",
		"uuid", doc.UUID.String(),
		"old_key", oldObjectKey,
		"new_key", newObjectKey)

	return doc, nil
}

// DeleteDocument deletes a document
func (a *Adapter) DeleteDocument(ctx context.Context, providerID string) error {
	objectKey := a.parseProviderID(providerID)

	// Delete from S3
	if err := a.deleteObject(ctx, objectKey); err != nil {
		return fmt.Errorf("failed to delete document from S3: %w", err)
	}

	// Delete metadata
	if err := a.metadataStore.Delete(ctx, objectKey); err != nil {
		a.logger.Warn("failed to delete metadata", "key", objectKey, "error", err)
		// Don't return error, document is already deleted
	}

	a.logger.Info("document deleted", "provider_id", providerID, "key", objectKey)

	return nil
}

// RenameDocument renames a document
func (a *Adapter) RenameDocument(ctx context.Context, providerID, newName string) error {
	// Get current document
	doc, err := a.GetDocument(ctx, providerID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Get current content
	content, err := a.GetContent(ctx, providerID)
	if err != nil {
		return fmt.Errorf("failed to get content: %w", err)
	}

	// Build new object key with new name
	newObjectKey := a.buildObjectKey(doc.UUID, newName, doc.ExtendedMetadata)
	oldObjectKey := a.parseProviderID(providerID)

	// If the key hasn't changed (e.g., path template doesn't use {name}), just update metadata
	if newObjectKey == oldObjectKey {
		doc.Name = newName
		doc.ModifiedTime = time.Now()
		return a.metadataStore.Set(ctx, oldObjectKey, doc)
	}

	// Copy to new location with new name
	s3Metadata := map[string]string{
		"hermes-uuid": doc.UUID.String(),
		"hermes-name": newName,
	}
	_, err = a.putObject(ctx, newObjectKey, []byte(content.Body), s3Metadata)
	if err != nil {
		return fmt.Errorf("failed to copy to new location: %w", err)
	}

	// Update metadata
	doc.Name = newName
	doc.ProviderID = a.formatProviderID(newObjectKey)
	doc.ModifiedTime = time.Now()
	if err := a.metadataStore.Set(ctx, newObjectKey, doc); err != nil {
		if delErr := a.deleteObject(ctx, newObjectKey); delErr != nil {
			a.logger.Warn("failed to clean up S3 object after metadata error",
				"error", delErr, "key", newObjectKey)
		}
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Delete old location
	if err := a.deleteObject(ctx, oldObjectKey); err != nil {
		a.logger.Warn("failed to delete old S3 object after rename",
			"error", err, "key", oldObjectKey)
	}
	if err := a.metadataStore.Delete(ctx, oldObjectKey); err != nil {
		a.logger.Warn("failed to delete old metadata after rename",
			"error", err, "key", oldObjectKey)
	}

	a.logger.Info("document renamed",
		"uuid", doc.UUID.String(),
		"old_name", doc.Name,
		"new_name", newName,
		"old_key", oldObjectKey,
		"new_key", newObjectKey)

	return nil
}

// CreateFolder creates a folder/directory
// Note: S3 doesn't have real folders, but we can simulate them with prefixes
func (a *Adapter) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	// S3 doesn't have folders, but we can create a marker object
	uuid := docid.NewUUID()
	folderKey := name + "/"
	if parentID != "" {
		folderKey = parentID + "/" + folderKey
	}
	if a.cfg.Prefix != "" {
		folderKey = a.cfg.Prefix + "/" + folderKey
	}

	now := time.Now()
	folder := &workspace.DocumentMetadata{
		UUID:         uuid,
		ProviderType: "s3",
		ProviderID:   a.formatProviderID(folderKey),
		Name:         name,
		MimeType:     "application/x-directory",
		CreatedTime:  now,
		ModifiedTime: now,
		SyncStatus:   "canonical",
	}

	// Create folder marker object (empty file)
	s3Metadata := map[string]string{
		"hermes-uuid": uuid.String(),
		"hermes-name": name,
		"hermes-type": "folder",
	}
	_, err := a.putObject(ctx, folderKey, []byte{}, s3Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder marker: %w", err)
	}

	// Store metadata
	if err := a.metadataStore.Set(ctx, folderKey, folder); err != nil {
		if delErr := a.deleteObject(ctx, folderKey); delErr != nil {
			a.logger.Warn("failed to clean up folder marker after metadata error",
				"error", delErr, "key", folderKey)
		}
		return nil, fmt.Errorf("failed to store folder metadata: %w", err)
	}

	a.logger.Info("folder created", "name", name, "key", folderKey)

	return folder, nil
}

// GetSubfolder finds a subfolder by name
func (a *Adapter) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	prefix := parentID + "/" + name + "/"
	if a.cfg.Prefix != "" {
		prefix = a.cfg.Prefix + "/" + prefix
	}

	// List objects with this prefix
	providerIDs, err := a.metadataStore.List(ctx, prefix)
	if err != nil {
		return "", fmt.Errorf("failed to list folder contents: %w", err)
	}

	if len(providerIDs) > 0 {
		// Found folder (has contents)
		return a.formatProviderID(prefix), nil
	}

	return "", workspace.NotFoundError("folder", name)
}
