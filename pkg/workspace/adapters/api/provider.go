package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Provider implements workspace.WorkspaceProvider by delegating all operations
// to a remote Hermes instance via REST API.
//
// This provider enables edge-to-central architectures where an edge Hermes
// instance can delegate operations to a central Hermes server.
//
// Example use case:
// - Edge Hermes (developer laptop) with local Git provider + API provider
// - Central Hermes (company server) with Google Workspace provider
// - Edge delegates directory, permissions, notifications to Central
type Provider struct {
	config       *Config
	client       *http.Client
	capabilities *Capabilities
}

// Capabilities discovered from remote Hermes API
type Capabilities struct {
	SupportsContent     bool `json:"supportsContent"`
	SupportsPermissions bool `json:"supportsPermissions"`
	SupportsDirectory   bool `json:"supportsDirectory"`
	SupportsGroups      bool `json:"supportsGroups"`
	SupportsEmail       bool `json:"supportsEmail"`
	SupportsRevisions   bool `json:"supportsRevisions"`
}

// Compile-time checks - API provider implements all RFC-084 interfaces
var (
	_ workspace.WorkspaceProvider        = (*Provider)(nil)
	_ workspace.DocumentProvider         = (*Provider)(nil)
	_ workspace.ContentProvider          = (*Provider)(nil)
	_ workspace.RevisionTrackingProvider = (*Provider)(nil)
	_ workspace.PermissionProvider       = (*Provider)(nil)
	_ workspace.PeopleProvider           = (*Provider)(nil)
	_ workspace.TeamProvider             = (*Provider)(nil)
	_ workspace.NotificationProvider     = (*Provider)(nil)
)

// NewProvider creates a new API workspace provider
func NewProvider(cfg *Config) (*Provider, error) {
	// Apply defaults
	if cfg.TLSVerify == nil {
		defaults := DefaultConfig()
		cfg.TLSVerify = defaults.TLSVerify
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 1 * time.Second
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid API provider config: %w", err)
	}

	// Create HTTP client
	client := cfg.NewHTTPClient()

	p := &Provider{
		config: cfg,
		client: client,
	}

	// Discover remote capabilities
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.discoverCapabilities(ctx); err != nil {
		// Log warning but don't fail - assume full capabilities
		// This allows the provider to work with older Hermes instances
		// that don't have the capabilities endpoint yet
		p.capabilities = &Capabilities{
			SupportsContent:     true,
			SupportsPermissions: true,
			SupportsDirectory:   true,
			SupportsGroups:      true,
			SupportsEmail:       true,
			SupportsRevisions:   true,
		}
	}

	return p, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "api"
}

// ProviderType returns the provider type
func (p *Provider) ProviderType() string {
	return "api"
}

// discoverCapabilities queries remote Hermes for supported features
func (p *Provider) discoverCapabilities(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/api/v2/capabilities", p.config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to discover capabilities: %w", err)
	}
	defer resp.Body.Close()

	// If endpoint doesn't exist (404), return error to trigger default capabilities
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("capabilities endpoint not found, using defaults")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("capabilities returned status %d: %s", resp.StatusCode, string(body))
	}

	var caps Capabilities
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
		return fmt.Errorf("failed to decode capabilities: %w", err)
	}

	p.capabilities = &caps
	return nil
}

// doRequest executes an HTTP request with retry logic and error handling
func (p *Provider) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	endpoint := fmt.Sprintf("%s%s", p.config.BaseURL, path)

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.config.RetryDelay * time.Duration(attempt)):
			}

			// Reset body reader for retry
			if body != nil {
				bodyBytes, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(bodyBytes)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// Handle HTTP errors
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Check if we should retry
			if resp.StatusCode >= 500 && attempt < p.config.MaxRetries {
				lastErr = fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(respBody))
				continue
			}

			// Parse error response
			var apiErr struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error != "" {
				return fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Error)
			}

			return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
		}

		// Decode response if result is provided
		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("request failed after %d attempts: %w", p.config.MaxRetries+1, lastErr)
}

// buildURL constructs a URL with query parameters
func (p *Provider) buildURL(path string, params map[string]string) string {
	u, _ := url.Parse(p.config.BaseURL + path)

	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	return u.String()
}

// checkCapability returns an error if the capability is not supported
func (p *Provider) checkCapability(capability string) error {
	switch strings.ToLower(capability) {
	case "content":
		if !p.capabilities.SupportsContent {
			return fmt.Errorf("remote provider does not support content operations")
		}
	case "permissions":
		if !p.capabilities.SupportsPermissions {
			return fmt.Errorf("remote provider does not support permissions")
		}
	case "directory":
		if !p.capabilities.SupportsDirectory {
			return fmt.Errorf("remote provider does not support directory operations")
		}
	case "groups":
		if !p.capabilities.SupportsGroups {
			return fmt.Errorf("remote provider does not support groups/teams")
		}
	case "email":
		if !p.capabilities.SupportsEmail {
			return fmt.Errorf("remote provider does not support email/notifications")
		}
	case "revisions":
		if !p.capabilities.SupportsRevisions {
			return fmt.Errorf("remote provider does not support revisions")
		}
	}
	return nil
}
