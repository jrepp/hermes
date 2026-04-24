// Package s3 provides an S3-compatible storage backend for Hermes.
// Implements RFC-089: S3-Compatible Storage Backend and Document Migration System
package s3

import (
	"fmt"
)

// Config contains configuration for the S3 storage adapter
type Config struct {
	DynamoDBAccessKey        string `hcl:"dynamodb_access_key"`
	PathTemplate             string `hcl:"path_template"`
	Bucket                   string `hcl:"bucket"`
	Prefix                   string `hcl:"prefix"`
	AccessKey                string `hcl:"access_key"`
	SecretKey                string `hcl:"secret_key"`
	DefaultMimeType          string `hcl:"default_mime_type"`
	Endpoint                 string `hcl:"endpoint"`
	DynamoDBSecretKey        string `hcl:"dynamodb_secret_key"`
	DynamoDBRegion           string `hcl:"dynamodb_region"`
	Region                   string `hcl:"region"`
	DynamoDBTable            string `hcl:"dynamodb_table"`
	MetadataStore            string `hcl:"metadata_store"`
	CACertPath               string `hcl:"ca_cert_path"`
	RequestTimeoutSeconds    int    `hcl:"request_timeout_seconds"`
	RetryMaxAttempts         int    `hcl:"retry_max_attempts"`
	MultipartThresholdMB     int    `hcl:"multipart_threshold_mb"`
	ConnectionTimeoutSeconds int    `hcl:"connection_timeout_seconds"`
	DownloadConcurrency      int    `hcl:"download_concurrency"`
	UploadConcurrency        int    `hcl:"upload_concurrency"`
	UseSSL                   bool   `hcl:"use_ssl"`
	InsecureSkipVerify       bool   `hcl:"insecure_skip_verify"`
	VersioningEnabled        bool   `hcl:"versioning_enabled"`
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
