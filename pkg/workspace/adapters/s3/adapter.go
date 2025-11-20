package s3

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

// Adapter provides S3-compatible storage for Hermes documents
type Adapter struct {
	client            *s3.Client
	cfg               *Config
	metadataStore     MetadataStore
	logger            hclog.Logger
	versioningEnabled bool
}

// NewAdapter creates a new S3 storage adapter
func NewAdapter(cfg *Config, logger hclog.Logger) (*Adapter, error) {
	// Validate and set defaults
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid S3 configuration: %w", err)
	}
	cfg.SetDefaults()

	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	// Create AWS SDK config
	awsCfg, err := createAWSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// Custom endpoint for MinIO or other S3-compatible services
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			// Force path-style addressing for MinIO
			o.UsePathStyle = true
		}
	})

	// Create metadata store based on configuration
	var metadataStore MetadataStore
	switch cfg.MetadataStore {
	case "s3-tags":
		metadataStore = NewS3TagsMetadataStore(client, cfg.Bucket, logger)
	case "manifest":
		metadataStore = NewManifestMetadataStore(client, cfg.Bucket, cfg.Prefix, logger)
	case "dynamodb":
		// TODO: Implement DynamoDB metadata store in Phase 2
		return nil, fmt.Errorf("DynamoDB metadata store not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported metadata store: %s", cfg.MetadataStore)
	}

	adapter := &Adapter{
		client:            client,
		cfg:               cfg,
		metadataStore:     metadataStore,
		logger:            logger.Named("s3-adapter"),
		versioningEnabled: cfg.VersioningEnabled,
	}

	// Verify bucket exists and is accessible
	if err := adapter.verifyBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to verify S3 bucket: %w", err)
	}

	logger.Info("S3 adapter initialized",
		"bucket", cfg.Bucket,
		"prefix", cfg.Prefix,
		"versioning", cfg.VersioningEnabled,
		"metadata_store", cfg.MetadataStore)

	return adapter, nil
}

// createAWSConfig creates AWS SDK configuration from S3 config
func createAWSConfig(cfg *Config) (aws.Config, error) {
	// Create custom HTTP client with TLS settings
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.InsecureSkipVerify,
			},
		},
	}

	// Load AWS config with custom settings
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithHTTPClient(httpClient),
	}

	// Add credentials if provided
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	return config.LoadDefaultConfig(context.Background(), opts...)
}

// verifyBucket verifies that the bucket exists and is accessible
func (a *Adapter) verifyBucket(ctx context.Context) error {
	_, err := a.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(a.cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("bucket %s is not accessible: %w", a.cfg.Bucket, err)
	}

	// Check if versioning is enabled (if required)
	if a.cfg.VersioningEnabled {
		versioningResult, err := a.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
			Bucket: aws.String(a.cfg.Bucket),
		})
		if err != nil {
			a.logger.Warn("failed to check bucket versioning status", "error", err)
		} else if versioningResult.Status != types.BucketVersioningStatusEnabled {
			a.logger.Warn("versioning is enabled in config but not on S3 bucket",
				"bucket", a.cfg.Bucket,
				"status", versioningResult.Status)
		}
	}

	return nil
}

// buildObjectKey constructs the S3 object key from document metadata
// Uses the path template from configuration
func (a *Adapter) buildObjectKey(uuid docid.UUID, name string, metadata map[string]any) string {
	// Start with the path template
	key := a.cfg.PathTemplate

	// Replace template variables
	key = strings.ReplaceAll(key, "{uuid}", uuid.String())
	key = strings.ReplaceAll(key, "{name}", sanitizeFilename(name))

	// Replace metadata variables if present
	if metadata != nil {
		if project, ok := metadata["project"].(string); ok {
			key = strings.ReplaceAll(key, "{project}", project)
		}
		if docType, ok := metadata["type"].(string); ok {
			key = strings.ReplaceAll(key, "{type}", docType)
		}
	}

	// Add prefix if configured
	if a.cfg.Prefix != "" {
		key = filepath.Join(a.cfg.Prefix, key)
	}

	return key
}

// parseProviderID extracts the S3 object key from a provider ID
// Provider ID format: "s3:{bucket}/{key}" or just "{bucket}/{key}" or "{key}"
func (a *Adapter) parseProviderID(providerID string) string {
	// Remove "s3:" prefix if present
	key := strings.TrimPrefix(providerID, "s3:")

	// Remove bucket name if present
	key = strings.TrimPrefix(key, a.cfg.Bucket+"/")

	// Remove leading slash
	key = strings.TrimPrefix(key, "/")

	return key
}

// formatProviderID creates a standardized provider ID
func (a *Adapter) formatProviderID(objectKey string) string {
	return fmt.Sprintf("s3:%s/%s", a.cfg.Bucket, objectKey)
}

// computeContentHash computes SHA-256 hash of content
func computeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(hash[:])
}

// sanitizeFilename removes characters that are problematic in S3 keys
func sanitizeFilename(name string) string {
	// Replace spaces with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	// Remove or replace other problematic characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(name)
}

// getObject retrieves an object from S3
func (a *Adapter) getObject(ctx context.Context, key string) ([]byte, *string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(a.cfg.Bucket),
		Key:    aws.String(key),
	}

	result, err := a.client.GetObject(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	// Read content
	content, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read object content: %w", err)
	}

	return content, result.VersionId, nil
}

// putObject stores an object in S3
func (a *Adapter) putObject(ctx context.Context, key string, content []byte, metadata map[string]string) (*string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(a.cfg.Bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(string(content)),
		ContentType: aws.String(a.cfg.DefaultMimeType),
	}

	// Add metadata if provided
	if len(metadata) > 0 {
		input.Metadata = metadata
	}

	result, err := a.client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to put object to S3: %w", err)
	}

	return result.VersionId, nil
}

// deleteObject deletes an object from S3
func (a *Adapter) deleteObject(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(a.cfg.Bucket),
		Key:    aws.String(key),
	}

	_, err := a.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	return nil
}

// ProviderType returns the provider type identifier
func (a *Adapter) ProviderType() string {
	return "s3"
}
