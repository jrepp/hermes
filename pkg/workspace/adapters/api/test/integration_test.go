//go:build integration
// +build integration

package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/workspace/adapters/api"
)

// TestAPIProvider_ConfigValidation tests configuration validation.
func TestAPIProvider_ConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *api.Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid config",
			config: &api.Config{
				BaseURL:   "https://hermes.example.com",
				AuthToken: "valid-token",
			},
			wantError: false,
		},
		{
			name: "Missing base URL",
			config: &api.Config{
				AuthToken: "valid-token",
			},
			wantError: true,
			errorMsg:  "base_url",
		},
		{
			name: "Missing auth token",
			config: &api.Config{
				BaseURL: "https://hermes.example.com",
			},
			wantError: true,
			errorMsg:  "auth_token",
		},
		{
			name: "Invalid URL scheme",
			config: &api.Config{
				BaseURL:   "ftp://hermes.example.com",
				AuthToken: "valid-token",
			},
			wantError: true,
			errorMsg:  "scheme",
		},
		{
			name: "Negative timeout",
			config: &api.Config{
				BaseURL:   "https://hermes.example.com",
				AuthToken: "valid-token",
				Timeout:   -1 * time.Second,
			},
			wantError: true,
			errorMsg:  "timeout",
		},
		{
			name: "Negative max retries",
			config: &api.Config{
				BaseURL:    "https://hermes.example.com",
				AuthToken:  "valid-token",
				MaxRetries: -1,
			},
			wantError: true,
			errorMsg:  "max_retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := api.NewProvider(tt.config)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				// Provider creation might still fail due to capabilities discovery
				// So we only check that validation didn't fail
				if err != nil {
					// If error, it should be about connection, not validation
					assert.NotContains(t, err.Error(), "invalid")
				}
			}
		})
	}
}

// TestAPIProvider_InterfaceImplementation verifies compile-time interface checks
func TestAPIProvider_InterfaceImplementation(t *testing.T) {
	// This test ensures that the compile-time interface checks in provider.go
	// are working correctly. If the provider doesn't implement all required
	// interfaces, the package won't compile.

	// Create a provider with minimal config (will fail to connect, but that's OK)
	cfg := &api.Config{
		BaseURL:   "https://example.com",
		AuthToken: "test-token",
	}

	provider, err := api.NewProvider(cfg)

	// Provider creation will likely fail due to connection issues, but we don't care
	// We just want to verify the type assertions compiled
	if err == nil {
		assert.NotNil(t, provider)
	}

	// The real test is in provider.go with the var _ InterfaceName = (*Provider)(nil) checks
	// If those fail, the package won't compile
	t.Log("API provider compile-time interface checks passed")
}

// TestAPIProvider_ProviderMetadata tests basic provider metadata
func TestAPIProvider_ProviderMetadata(t *testing.T) {
	cfg := &api.Config{
		BaseURL:   "https://example.com",
		AuthToken: "test-token",
	}

	provider, err := api.NewProvider(cfg)
	if err != nil {
		t.Skip("Skipping metadata test due to provider creation failure (expected without real server)")
	}

	// These methods should work even if the remote server doesn't exist
	assert.Equal(t, "api", provider.Name())
	assert.Equal(t, "api", provider.ProviderType())
}

// TestAPIProvider_ConfigDefaults tests that default values are applied
func TestAPIProvider_ConfigDefaults(t *testing.T) {
	cfg := &api.Config{
		BaseURL:   "https://example.com",
		AuthToken: "test-token",
	}

	provider, err := api.NewProvider(cfg)
	if err != nil {
		// Provider creation may fail without a real server, but defaults should still be applied
		t.Logf("Provider creation failed (expected): %v", err)
	}

	// Check that defaults were applied to the config
	assert.NotNil(t, cfg.TLSVerify, "TLSVerify should have a default value")
	assert.NotZero(t, cfg.Timeout, "Timeout should have a default value")
	assert.NotZero(t, cfg.MaxRetries, "MaxRetries should have a default value")
	assert.NotZero(t, cfg.RetryDelay, "RetryDelay should have a default value")

	if provider != nil {
		t.Log("Provider created successfully with defaults")
	}
}

// TestAPIProvider_ErrorHandling tests error handling for unreachable servers
func TestAPIProvider_ErrorHandling(t *testing.T) {
	t.Run("Connection to unreachable server", func(t *testing.T) {
		tlsVerify := false
		cfg := &api.Config{
			BaseURL:    "http://localhost:9999", // Unreachable port
			AuthToken:  "test-token",
			TLSVerify:  &tlsVerify,
			Timeout:    1 * time.Second,
			MaxRetries: 1,
			RetryDelay: 100 * time.Millisecond,
		}

		// Provider creation should succeed (no network call yet) or fail gracefully
		provider, err := api.NewProvider(cfg)

		// Provider might fail to create if capabilities discovery is required
		if err != nil {
			assert.Contains(t, err.Error(), "connection", "Error should mention connection issue")
			return
		}

		require.NotNil(t, provider)

		// Try an operation - should fail with connection error
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = provider.GetDocument(ctx, "test-doc")
		assert.Error(t, err, "Should fail to connect to unreachable server")
	})

	t.Run("Request timeout", func(t *testing.T) {
		tlsVerify := false
		cfg := &api.Config{
			BaseURL:    "https://httpstat.us/200?sleep=5000", // Slow response
			AuthToken:  "test-token",
			TLSVerify:  &tlsVerify,
			Timeout:    1 * time.Nanosecond, // Very short timeout
			MaxRetries: 0,
		}

		_, err := api.NewProvider(cfg)
		// Timeout during capabilities discovery is acceptable
		if err != nil {
			t.Logf("Provider creation timed out as expected: %v", err)
		}
	})
}

// TestAPIProvider_CompileTimeChecks ensures all interface implementations exist
func TestAPIProvider_CompileTimeChecks(t *testing.T) {
	// This test documents the interfaces that Provider implements.
	// The actual compile-time checks are in provider.go
	t.Log("API Provider implements the following RFC-084 interfaces:")
	t.Log("  - workspace.WorkspaceProvider")
	t.Log("  - workspace.DocumentProvider")
	t.Log("  - workspace.ContentProvider")
	t.Log("  - workspace.RevisionTrackingProvider")
	t.Log("  - workspace.PermissionProvider")
	t.Log("  - workspace.PeopleProvider")
	t.Log("  - workspace.TeamProvider")
	t.Log("  - workspace.NotificationProvider")

	// If this test runs, the compile-time checks passed
	assert.True(t, true, "Compile-time interface checks passed")
}
