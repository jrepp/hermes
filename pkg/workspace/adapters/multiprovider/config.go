package multiprovider

import (
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Config configures the multi-provider manager
type Config struct {
	// Primary provider - used for document authoring and content operations
	Primary workspace.WorkspaceProvider

	// Secondary provider - used for directory, permissions, notifications
	Secondary workspace.WorkspaceProvider

	// Sync configuration
	Sync *SyncConfig
}

// SyncConfig configures document synchronization to central
type SyncConfig struct {
	// Enabled controls whether automatic sync is enabled
	Enabled bool

	// Mode controls sync timing
	// "immediate" - sync on every document operation
	// "batch" - batch sync operations periodically
	// "manual" - only sync when explicitly requested
	Mode SyncMode

	// EdgeInstance identifier for this edge instance
	EdgeInstance string

	// BatchInterval for batch mode (default: 30s)
	BatchInterval time.Duration

	// RetryAttempts for failed sync operations (default: 3)
	RetryAttempts int

	// RetryDelay between retry attempts (default: 5s)
	RetryDelay time.Duration
}

// SyncMode represents sync timing strategy
type SyncMode string

const (
	// SyncModeImmediate syncs on every document operation
	SyncModeImmediate SyncMode = "immediate"

	// SyncModeBatch batches sync operations periodically
	SyncModeBatch SyncMode = "batch"

	// SyncModeManual only syncs when explicitly requested
	SyncModeManual SyncMode = "manual"
)

// DefaultSyncConfig returns default sync configuration
func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		Enabled:       false,
		Mode:          SyncModeManual,
		BatchInterval: 30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    5 * time.Second,
	}
}

// Validate checks configuration validity
func (c *Config) Validate() error {
	if c.Primary == nil {
		return fmt.Errorf("primary provider is required")
	}

	// Secondary is optional - multiprovider can work with just primary
	// This allows gradual adoption

	// Validate sync config if provided
	if c.Sync != nil {
		if c.Sync.Enabled && c.Sync.EdgeInstance == "" {
			return fmt.Errorf("edge_instance is required when sync is enabled")
		}

		if c.Sync.Mode != SyncModeImmediate &&
			c.Sync.Mode != SyncModeBatch &&
			c.Sync.Mode != SyncModeManual {
			return fmt.Errorf("invalid sync mode: %s", c.Sync.Mode)
		}

		if c.Sync.BatchInterval <= 0 {
			c.Sync.BatchInterval = 30 * time.Second
		}

		if c.Sync.RetryAttempts <= 0 {
			c.Sync.RetryAttempts = 3
		}

		if c.Sync.RetryDelay <= 0 {
			c.Sync.RetryDelay = 5 * time.Second
		}
	} else {
		// Apply defaults if not provided
		c.Sync = DefaultSyncConfig()
	}

	return nil
}

// RoutingStrategy defines how operations are routed between providers
type RoutingStrategy struct {
	// UseSecondaryForDirectory routes directory operations to secondary
	UseSecondaryForDirectory bool

	// UseSecondaryForPermissions routes permission operations to secondary
	UseSecondaryForPermissions bool

	// UseSecondaryForNotifications routes notification operations to secondary
	UseSecondaryForNotifications bool

	// UseSecondaryForTeams routes team operations to secondary
	UseSecondaryForTeams bool

	// FallbackToPrimary enables fallback to primary if secondary fails
	FallbackToPrimary bool
}

// DefaultRoutingStrategy returns the recommended routing strategy
// for edge-to-central architecture
func DefaultRoutingStrategy() *RoutingStrategy {
	return &RoutingStrategy{
		UseSecondaryForDirectory:     true,  // Delegate directory to central
		UseSecondaryForPermissions:   true,  // Delegate permissions to central
		UseSecondaryForNotifications: true,  // Delegate notifications to central
		UseSecondaryForTeams:         true,  // Delegate teams to central
		FallbackToPrimary:            false, // Don't fallback (fail explicitly)
	}
}

// LocalOnlyStrategy returns a strategy that only uses primary provider
// Useful for offline operation or testing
func LocalOnlyStrategy() *RoutingStrategy {
	return &RoutingStrategy{
		UseSecondaryForDirectory:     false,
		UseSecondaryForPermissions:   false,
		UseSecondaryForNotifications: false,
		UseSecondaryForTeams:         false,
		FallbackToPrimary:            false,
	}
}
