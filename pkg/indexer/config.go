package indexer

import (
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Config holds the configuration for the indexer.
type Config struct {
	// Database connection
	Database *gorm.DB

	// Logger
	Logger hclog.Logger

	// Providers
	WorkspaceProvider workspace.StorageProvider
	SearchProvider    search.Provider

	// Execution settings
	MaxParallelDocs int
	DryRun          bool
}
