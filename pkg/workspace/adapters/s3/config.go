// Package s3 provides an S3-compatible storage backend for Hermes.
// Implements RFC-089: S3-Compatible Storage Backend and Document Migration System
package s3

import (
	"fmt"
)

// Config contains configuration for the S3 storage adapter
type Config struct {
	// S3 Connection Settings
	Endpoint  string `hcl:"endpoint"`   // S3 endpoint URL (e.g., "https://s3.amazonaws.com" or MinIO endpoint)
	Region    string `hcl:"region"`     // AWS region (e.g., "us-west-2")
	Bucket    string `hcl:"bucket"`     // S3 bucket name
	Prefix    string `hcl:"prefix"`     // Optional namespace prefix (e.g., "docs/")
	AccessKey string `hcl:"access_key"` // Access key ID
	SecretKey string `hcl:"secret_key"` // Secret access key

	// Versioning
	VersioningEnabled bool `hcl:"versioning_enabled"` // Enable S3 versioning for revision tracking

	// Metadata Storage Strategy
	// Options: "s3-tags", "dynamodb", "manifest"
	MetadataStore     string `hcl:"metadata_store"`      // How to store document metadata
	DynamoDBTable     string `hcl:"dynamodb_table"`      // DynamoDB table name (if metadata_store = "dynamodb")
	DynamoDBRegion    string `hcl:"dynamodb_region"`     // DynamoDB region (if different from S3 region)
	DynamoDBAccessKey string `hcl:"dynamodb_access_key"` // DynamoDB access key (if different from S3)
	DynamoDBSecretKey string `hcl:"dynamodb_secret_key"` // DynamoDB secret key (if different from S3)

	// Performance Tuning
	UploadConcurrency        int `hcl:"upload_concurrency"`         // Concurrent uploads (default: 5)
	DownloadConcurrency      int `hcl:"download_concurrency"`       // Concurrent downloads (default: 10)
	MultipartThresholdMB     int `hcl:"multipart_threshold_mb"`     // File size threshold for multipart upload (default: 100)
	RetryMaxAttempts         int `hcl:"retry_max_attempts"`         // Max retry attempts (default: 3)
	RequestTimeoutSeconds    int `hcl:"request_timeout_seconds"`    // Request timeout (default: 30)
	ConnectionTimeoutSeconds int `hcl:"connection_timeout_seconds"` // Connection timeout (default: 10)

	// TLS/SSL Settings
	UseSSL             bool   `hcl:"use_ssl"`              // Use SSL/TLS (default: true)
	InsecureSkipVerify bool   `hcl:"insecure_skip_verify"` // Skip SSL certificate verification (for testing only)
	CACertPath         string `hcl:"ca_cert_path"`         // Path to CA certificate file

	// Path Templates (for organizing documents in S3)
	// Example: "{project}/{type}/{name}.md"
	PathTemplate string `hcl:"path_template"` // Path template for document organization

	// Default values for optional fields
	DefaultMimeType string `hcl:"default_mime_type"` // Default MIME type (default: "text/markdown")
}

// Validate validates the S3 configuration
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}
	if c.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}

	// Validate metadata store option
	validMetadataStores := map[string]bool{
		"s3-tags":  true,
		"dynamodb": true,
		"manifest": true,
	}
	if c.MetadataStore != "" && !validMetadataStores[c.MetadataStore] {
		return fmt.Errorf("invalid metadata_store: %s (must be one of: s3-tags, dynamodb, manifest)", c.MetadataStore)
	}

	// Validate DynamoDB configuration if using DynamoDB metadata store
	if c.MetadataStore == "dynamodb" {
		if c.DynamoDBTable == "" {
			return fmt.Errorf("dynamodb_table is required when metadata_store is 'dynamodb'")
		}
	}

	return nil
}

// SetDefaults sets default values for optional configuration fields
func (c *Config) SetDefaults() {
	if c.MetadataStore == "" {
		c.MetadataStore = "s3-tags" // Default to S3 object tags
	}
	if c.UploadConcurrency == 0 {
		c.UploadConcurrency = 5
	}
	if c.DownloadConcurrency == 0 {
		c.DownloadConcurrency = 10
	}
	if c.MultipartThresholdMB == 0 {
		c.MultipartThresholdMB = 100
	}
	if c.RetryMaxAttempts == 0 {
		c.RetryMaxAttempts = 3
	}
	if c.RequestTimeoutSeconds == 0 {
		c.RequestTimeoutSeconds = 30
	}
	if c.ConnectionTimeoutSeconds == 0 {
		c.ConnectionTimeoutSeconds = 10
	}
	if c.DefaultMimeType == "" {
		c.DefaultMimeType = "text/markdown"
	}
	if c.PathTemplate == "" {
		c.PathTemplate = "{uuid}.md" // Default: flat structure with UUID as filename
	}
	// Enable SSL by default
	if !c.InsecureSkipVerify {
		c.UseSSL = true
	}
	// Use S3 region for DynamoDB if not specified
	if c.DynamoDBRegion == "" {
		c.DynamoDBRegion = c.Region
	}
	// Use S3 credentials for DynamoDB if not specified
	if c.DynamoDBAccessKey == "" {
		c.DynamoDBAccessKey = c.AccessKey
	}
	if c.DynamoDBSecretKey == "" {
		c.DynamoDBSecretKey = c.SecretKey
	}
}
