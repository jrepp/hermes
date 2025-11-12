package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Config contains configuration for the API workspace provider.
// This provider delegates all operations to a remote Hermes instance via REST API.
//
// Example configuration (HCL):
//
//	api_workspace {
//	  base_url   = "https://central.hermes.company.com"
//	  auth_token = env("HERMES_API_TOKEN")
//	  timeout    = "30s"
//	  tls_verify = true
//	}
type Config struct {
	// BaseURL is the base URL of the remote Hermes instance
	// Example: "https://hermes.example.com"
	BaseURL string `hcl:"base_url" json:"baseUrl"`

	// AuthToken is the API token for authentication (Bearer token)
	// Should be kept in environment variable for security
	AuthToken string `hcl:"auth_token" json:"-"` // Don't marshal auth token to JSON

	// TLSVerify controls TLS certificate verification
	// Set to false only for development/testing with self-signed certs
	TLSVerify *bool `hcl:"tls_verify,optional" json:"tlsVerify,omitempty"`

	// Timeout for API requests
	// Default: 30 seconds
	Timeout time.Duration `hcl:"timeout,optional" json:"timeout,omitempty"`

	// MaxRetries for failed requests
	// Default: 3
	MaxRetries int `hcl:"max_retries,optional" json:"maxRetries,omitempty"`

	// RetryDelay between retries
	// Default: 1 second
	RetryDelay time.Duration `hcl:"retry_delay,optional" json:"retryDelay,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	tlsVerify := true
	return &Config{
		TLSVerify:  &tlsVerify,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("base_url must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	if c.AuthToken == "" {
		return fmt.Errorf("auth_token is required")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", c.Timeout)
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative, got: %d", c.MaxRetries)
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("retry_delay must be non-negative, got: %v", c.RetryDelay)
	}

	return nil
}

// NewHTTPClient creates a configured HTTP client for this provider
func (c *Config) NewHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Configure TLS verification
	if c.TLSVerify != nil && !*c.TLSVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	return &http.Client{
		Timeout:   c.Timeout,
		Transport: transport,
	}
}
