package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// RevisionTrackingProvider interface implementation

// GetRevisionHistory lists all revisions for a document in this backend
func (a *Adapter) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	if !a.versioningEnabled {
		return nil, fmt.Errorf("S3 versioning is not enabled")
	}

	objectKey := a.parseProviderID(providerID)

	// List object versions
	input := &s3.ListObjectVersionsInput{
		Bucket:  aws.String(a.cfg.Bucket),
		Prefix:  aws.String(objectKey),
		MaxKeys: aws.Int32(int32(limit)),
	}

	result, err := a.client.ListObjectVersions(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list object versions: %w", err)
	}

	var revisions []*workspace.BackendRevision
	for _, version := range result.Versions {
		// Only include versions for the exact key (not other objects with same prefix)
		if aws.ToString(version.Key) != objectKey {
			continue
		}

		revision := &workspace.BackendRevision{
			ProviderType: "s3",
			RevisionID:   aws.ToString(version.VersionId),
			ModifiedTime: aws.ToTime(version.LastModified),
			Metadata: map[string]any{
				"etag":      aws.ToString(version.ETag),
				"size":      aws.ToInt64(version.Size),
				"is_latest": aws.ToBool(version.IsLatest),
			},
		}
		revisions = append(revisions, revision)
	}

	return revisions, nil
}

// GetRevision retrieves a specific revision
func (a *Adapter) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	if !a.versioningEnabled {
		return nil, fmt.Errorf("S3 versioning is not enabled")
	}

	objectKey := a.parseProviderID(providerID)

	// Get object metadata for specific version
	input := &s3.HeadObjectInput{
		Bucket:    aws.String(a.cfg.Bucket),
		Key:       aws.String(objectKey),
		VersionId: aws.String(revisionID),
	}

	result, err := a.client.HeadObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}

	return &workspace.BackendRevision{
		ProviderType: "s3",
		RevisionID:   revisionID,
		ModifiedTime: aws.ToTime(result.LastModified),
		Metadata: map[string]any{
			"etag":         aws.ToString(result.ETag),
			"content_type": aws.ToString(result.ContentType),
			"size":         aws.ToInt64(result.ContentLength),
		},
	}, nil
}

// GetRevisionContent retrieves content at a specific revision
func (a *Adapter) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	if !a.versioningEnabled {
		return nil, fmt.Errorf("S3 versioning is not enabled")
	}

	objectKey := a.parseProviderID(providerID)

	// Get object at specific version
	input := &s3.GetObjectInput{
		Bucket:    aws.String(a.cfg.Bucket),
		Key:       aws.String(objectKey),
		VersionId: aws.String(revisionID),
	}

	result, err := a.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get revision content: %w", err)
	}
	defer result.Body.Close()

	// Read content
	contentBytes := make([]byte, aws.ToInt64(result.ContentLength))
	_, err = result.Body.Read(contentBytes)
	if err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("failed to read revision content: %w", err)
	}

	content := string(contentBytes)
	contentHash := computeContentHash(content)

	// Get metadata (best effort)
	metadata, _ := a.metadataStore.Get(ctx, objectKey)
	if metadata == nil {
		// Create minimal metadata if not available
		metadata = &workspace.DocumentMetadata{
			UUID:       docid.NewUUID(),
			ProviderID: providerID,
			Name:       objectKey,
		}
	}

	return &workspace.DocumentContent{
		UUID:       metadata.UUID,
		ProviderID: providerID,
		Title:      metadata.Name,
		Body:       content,
		Format:     "markdown",
		BackendRevision: &workspace.BackendRevision{
			ProviderType: "s3",
			RevisionID:   revisionID,
			ModifiedTime: aws.ToTime(result.LastModified),
			Metadata: map[string]any{
				"etag":         aws.ToString(result.ETag),
				"content_type": aws.ToString(result.ContentType),
			},
		},
		ContentHash:  contentHash,
		LastModified: aws.ToTime(result.LastModified),
	}, nil
}

// KeepRevisionForever marks a revision as permanent (if supported)
// S3 doesn't have a "keep forever" feature like Google Drive, so this is a no-op
func (a *Adapter) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	// S3 versioning is permanent by default (until lifecycle policies delete them)
	// We can tag the version to indicate it should be kept, but that's implementation-specific
	a.logger.Info("KeepRevisionForever called (no-op for S3)",
		"provider_id", providerID,
		"revision_id", revisionID)
	return nil
}

// GetAllDocumentRevisions returns all revisions across all backends for a UUID
// For S3 adapter, this just returns revisions from S3
func (a *Adapter) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	// Find the document by UUID
	doc, err := a.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Get revision history
	revisions, err := a.GetRevisionHistory(ctx, doc.ProviderID, 100) // Get last 100 revisions
	if err != nil {
		return nil, err
	}

	// Convert to RevisionInfo
	var revisionInfos []*workspace.RevisionInfo
	for _, rev := range revisions {
		revisionInfos = append(revisionInfos, &workspace.RevisionInfo{
			UUID:            uuid,
			ProviderType:    "s3",
			ProviderID:      doc.ProviderID,
			BackendRevision: rev,
			ContentHash:     "", // Would need to fetch content to get hash
			SyncStatus:      doc.SyncStatus,
		})
	}

	return revisionInfos, nil
}
