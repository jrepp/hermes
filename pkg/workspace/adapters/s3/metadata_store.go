package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// MetadataStore defines the interface for storing and retrieving document metadata
type MetadataStore interface {
	// Get retrieves document metadata by object key
	Get(ctx context.Context, key string) (*workspace.DocumentMetadata, error)

	// Set stores document metadata
	Set(ctx context.Context, key string, metadata *workspace.DocumentMetadata) error

	// Delete removes document metadata
	Delete(ctx context.Context, key string) error

	// List lists all documents (returns provider IDs)
	List(ctx context.Context, prefix string) ([]string, error)
}

// =================================================================
// S3 Tags Metadata Store
// =================================================================
// Stores metadata as S3 object tags
// Limitations: S3 tags have a 10-tag limit and 256-character value limit
// Best for: Simple metadata, low cost, no extra infrastructure

type S3TagsMetadataStore struct {
	client *s3.Client
	bucket string
	logger hclog.Logger
}

func NewS3TagsMetadataStore(client *s3.Client, bucket string, logger hclog.Logger) *S3TagsMetadataStore {
	return &S3TagsMetadataStore{
		client: client,
		bucket: bucket,
		logger: logger.Named("s3-tags-store"),
	}
}

func (s *S3TagsMetadataStore) Get(ctx context.Context, key string) (*workspace.DocumentMetadata, error) {
	// Get object metadata (head request)
	headResult, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Get object tags
	tagsResult, err := s.client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object tags: %w", err)
	}

	// Build metadata from tags and object metadata
	metadata := &workspace.DocumentMetadata{
		ProviderType: "s3",
		ProviderID:   fmt.Sprintf("s3:%s/%s", s.bucket, key),
		MimeType:     aws.ToString(headResult.ContentType),
		ModifiedTime: aws.ToTime(headResult.LastModified),
	}

	// Parse tags into metadata
	tagMap := make(map[string]string)
	for _, tag := range tagsResult.TagSet {
		tagMap[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	// Extract core metadata from tags
	if uuidStr, ok := tagMap["hermes-uuid"]; ok {
		uuid, err := docid.ParseUUID(uuidStr)
		if err == nil {
			metadata.UUID = uuid
		}
	}
	if name, ok := tagMap["hermes-name"]; ok {
		metadata.Name = name
	}
	if createdTime, ok := tagMap["hermes-created"]; ok {
		if t, err := time.Parse(time.RFC3339, createdTime); err == nil {
			metadata.CreatedTime = t
		}
	}
	if contentHash, ok := tagMap["hermes-hash"]; ok {
		metadata.ContentHash = contentHash
	}
	if syncStatus, ok := tagMap["hermes-sync-status"]; ok {
		metadata.SyncStatus = syncStatus
	}
	if project, ok := tagMap["hermes-project"]; ok {
		metadata.Project = project
	}

	// Parse extended metadata from JSON tag (if present)
	if extMetaJSON, ok := tagMap["hermes-ext-metadata"]; ok {
		var extMeta map[string]any
		if err := json.Unmarshal([]byte(extMetaJSON), &extMeta); err == nil {
			metadata.ExtendedMetadata = extMeta
		}
	}

	return metadata, nil
}

func (s *S3TagsMetadataStore) Set(ctx context.Context, key string, metadata *workspace.DocumentMetadata) error {
	// Build tag set from metadata
	tagSet := []struct {
		Key   *string
		Value *string
	}{
		{Key: aws.String("hermes-uuid"), Value: aws.String(metadata.UUID.String())},
		{Key: aws.String("hermes-name"), Value: aws.String(metadata.Name)},
		{Key: aws.String("hermes-created"), Value: aws.String(metadata.CreatedTime.Format(time.RFC3339))},
		{Key: aws.String("hermes-hash"), Value: aws.String(metadata.ContentHash)},
		{Key: aws.String("hermes-sync-status"), Value: aws.String(metadata.SyncStatus)},
	}

	// Add optional fields
	if metadata.Project != "" {
		tagSet = append(tagSet, struct {
			Key   *string
			Value *string
		}{Key: aws.String("hermes-project"), Value: aws.String(metadata.Project)})
	}

	// Add extended metadata as JSON (if present and fits in tag size limit)
	if len(metadata.ExtendedMetadata) > 0 {
		extMetaJSON, err := json.Marshal(metadata.ExtendedMetadata)
		if err == nil && len(extMetaJSON) < 256 {
			tagSet = append(tagSet, struct {
				Key   *string
				Value *string
			}{Key: aws.String("hermes-ext-metadata"), Value: aws.String(string(extMetaJSON))})
		}
	}

	// Convert to S3 types
	var s3Tags []types.Tag
	for _, tag := range tagSet {
		s3Tags = append(s3Tags, types.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}

	// Put object tagging
	_, err := s.client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Tagging: &types.Tagging{
			TagSet: s3Tags,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set object tags: %w", err)
	}

	return nil
}

func (s *S3TagsMetadataStore) Delete(ctx context.Context, key string) error {
	// S3 tags are automatically deleted when the object is deleted
	// No action needed
	return nil
}

func (s *S3TagsMetadataStore) List(ctx context.Context, prefix string) ([]string, error) {
	var providerIDs []string

	// List objects with prefix
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			providerID := fmt.Sprintf("s3:%s/%s", s.bucket, key)
			providerIDs = append(providerIDs, providerID)
		}
	}

	return providerIDs, nil
}

// =================================================================
// Manifest Metadata Store
// =================================================================
// Stores metadata in a separate manifest file (.metadata.json)
// Best for: Rich metadata, no tag limits, simple architecture

type ManifestMetadataStore struct {
	client *s3.Client
	bucket string
	prefix string
	logger hclog.Logger
}

func NewManifestMetadataStore(client *s3.Client, bucket, prefix string, logger hclog.Logger) *ManifestMetadataStore {
	return &ManifestMetadataStore{
		client: client,
		bucket: bucket,
		prefix: prefix,
		logger: logger.Named("manifest-store"),
	}
}

func (m *ManifestMetadataStore) Get(ctx context.Context, key string) (*workspace.DocumentMetadata, error) {
	// Get manifest file
	manifestKey := m.getManifestKey(key)
	result, err := m.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(manifestKey),
	})
	if err != nil {
		// If manifest doesn't exist, try to build metadata from object
		return m.buildMetadataFromObject(ctx, key)
	}
	defer result.Body.Close()

	// Parse manifest JSON
	var metadata workspace.DocumentMetadata
	if err := json.NewDecoder(result.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &metadata, nil
}

func (m *ManifestMetadataStore) Set(ctx context.Context, key string, metadata *workspace.DocumentMetadata) error {
	// Serialize metadata to JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

	// Store manifest file
	manifestKey := m.getManifestKey(key)
	_, err = m.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(m.bucket),
		Key:         aws.String(manifestKey),
		Body:        strings.NewReader(string(metadataJSON)),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("failed to store manifest: %w", err)
	}

	return nil
}

func (m *ManifestMetadataStore) Delete(ctx context.Context, key string) error {
	// Delete manifest file
	manifestKey := m.getManifestKey(key)
	_, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(manifestKey),
	})
	if err != nil {
		m.logger.Warn("failed to delete manifest", "key", manifestKey, "error", err)
		// Don't return error if manifest doesn't exist
	}
	return nil
}

func (m *ManifestMetadataStore) List(ctx context.Context, prefix string) ([]string, error) {
	var providerIDs []string

	// List objects with prefix, excluding metadata files
	paginator := s3.NewListObjectsV2Paginator(m.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(m.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			// Skip metadata files
			if strings.HasSuffix(key, ".metadata.json") {
				continue
			}
			providerID := fmt.Sprintf("s3:%s/%s", m.bucket, key)
			providerIDs = append(providerIDs, providerID)
		}
	}

	return providerIDs, nil
}

func (m *ManifestMetadataStore) getManifestKey(docKey string) string {
	return docKey + ".metadata.json"
}

func (m *ManifestMetadataStore) buildMetadataFromObject(ctx context.Context, key string) (*workspace.DocumentMetadata, error) {
	// Get object metadata
	headResult, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	// Try to extract UUID from filename
	filename := key
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		filename = key[idx+1:]
	}
	filename = strings.TrimSuffix(filename, ".md")

	uuid, err := docid.ParseUUID(filename)
	if err != nil {
		// Generate new UUID if can't parse from filename
		uuid = docid.NewUUID()
	}

	return &workspace.DocumentMetadata{
		UUID:         uuid,
		ProviderType: "s3",
		ProviderID:   fmt.Sprintf("s3:%s/%s", m.bucket, key),
		Name:         filename,
		MimeType:     aws.ToString(headResult.ContentType),
		ModifiedTime: aws.ToTime(headResult.LastModified),
		CreatedTime:  aws.ToTime(headResult.LastModified), // Use modified time as created time
		SyncStatus:   "canonical",
	}, nil
}

// =================================================================
// Helper Functions
// =================================================================

// encodeTag URL-encodes a tag value to fit S3 tag constraints
func encodeTag(value string) string {
	return url.QueryEscape(value)
}

// decodeTag URL-decodes a tag value
func decodeTag(value string) string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		// Return original value if unescape fails
		return value
	}
	return decoded
}
