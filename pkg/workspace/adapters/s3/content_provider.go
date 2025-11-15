package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ContentProvider interface implementation

// GetContent retrieves document content with backend-specific revision
func (a *Adapter) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	objectKey := a.parseProviderID(providerID)

	// Get object from S3
	contentBytes, versionID, err := a.getObject(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get content from S3: %w", err)
	}

	// Get metadata
	metadata, err := a.metadataStore.Get(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get document metadata: %w", err)
	}

	content := string(contentBytes)
	contentHash := computeContentHash(content)

	// Build backend revision info
	backendRevision := &workspace.BackendRevision{
		ProviderType: "s3",
		RevisionID:   stringOrEmpty(versionID),
		ModifiedTime: metadata.ModifiedTime,
		Metadata: map[string]any{
			"version_id":   stringOrEmpty(versionID),
			"content_type": metadata.MimeType,
		},
	}

	return &workspace.DocumentContent{
		UUID:            metadata.UUID,
		ProviderID:      providerID,
		Title:           metadata.Name,
		Body:            content,
		Format:          "markdown",
		BackendRevision: backendRevision,
		ContentHash:     contentHash,
		LastModified:    metadata.ModifiedTime,
	}, nil
}

// GetContentByUUID retrieves content using UUID (looks up providerID)
func (a *Adapter) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	// First find the document by UUID
	doc, err := a.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Then get its content
	return a.GetContent(ctx, doc.ProviderID)
}

// UpdateContent updates document content
func (a *Adapter) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	objectKey := a.parseProviderID(providerID)

	// Get current metadata
	metadata, err := a.metadataStore.Get(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get document metadata: %w", err)
	}

	// Write updated content to S3
	s3Metadata := map[string]string{
		"hermes-uuid": metadata.UUID.String(),
		"hermes-name": metadata.Name,
	}
	versionID, err := a.putObject(ctx, objectKey, []byte(content), s3Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to update content in S3: %w", err)
	}

	// Update metadata
	now := time.Now()
	metadata.ModifiedTime = now
	metadata.ContentHash = computeContentHash(content)
	if err := a.metadataStore.Set(ctx, objectKey, metadata); err != nil {
		a.logger.Warn("failed to update metadata after content update", "error", err)
	}

	// Build backend revision info
	backendRevision := &workspace.BackendRevision{
		ProviderType: "s3",
		RevisionID:   stringOrEmpty(versionID),
		ModifiedTime: now,
		Metadata: map[string]any{
			"version_id":   stringOrEmpty(versionID),
			"content_type": metadata.MimeType,
		},
	}

	a.logger.Info("content updated",
		"uuid", metadata.UUID.String(),
		"version_id", stringOrEmpty(versionID))

	return &workspace.DocumentContent{
		UUID:            metadata.UUID,
		ProviderID:      providerID,
		Title:           metadata.Name,
		Body:            content,
		Format:          "markdown",
		BackendRevision: backendRevision,
		ContentHash:     metadata.ContentHash,
		LastModified:    now,
	}, nil
}

// GetContentBatch retrieves multiple documents (efficient for migration)
func (a *Adapter) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	var contents []*workspace.DocumentContent

	// S3 doesn't have a batch get API, so we need to fetch individually
	// TODO: Optimize with goroutines for parallel fetching
	for _, providerID := range providerIDs {
		content, err := a.GetContent(ctx, providerID)
		if err != nil {
			a.logger.Warn("failed to get content in batch", "provider_id", providerID, "error", err)
			continue // Skip documents with errors
		}
		contents = append(contents, content)
	}

	return contents, nil
}

// CompareContent compares content between two revisions
func (a *Adapter) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	// Get content for both documents
	content1, err := a.GetContent(ctx, providerID1)
	if err != nil {
		return nil, fmt.Errorf("failed to get first document: %w", err)
	}

	content2, err := a.GetContent(ctx, providerID2)
	if err != nil {
		return nil, fmt.Errorf("failed to get second document: %w", err)
	}

	// Compare content hashes
	contentMatch := content1.ContentHash == content2.ContentHash

	// Determine hash difference level
	hashDifference := "major"
	if contentMatch {
		hashDifference = "same"
	} else {
		// Simple heuristic: if content length is similar, it's a minor change
		lenDiff := abs(len(content1.Body) - len(content2.Body))
		totalLen := max(len(content1.Body), len(content2.Body))
		if float64(lenDiff)/float64(totalLen) < 0.1 {
			hashDifference = "minor"
		}
	}

	return &workspace.ContentComparison{
		UUID:           content1.UUID,
		Revision1:      content1.BackendRevision,
		Revision2:      content2.BackendRevision,
		ContentMatch:   contentMatch,
		HashDifference: hashDifference,
	}, nil
}

// Helper functions

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
