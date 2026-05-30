// Package server provides server functionality.
package server

import (
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/jira"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/sharepointhelper"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	gw "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/google"
)

// Server contains the server configuration.
type Server struct {
	// SearchProvider is the search backend (Algolia, Meilisearch, etc).
	// This is the preferred way to access search functionality.
	SearchProvider search.Provider

	// WorkspaceProvider is the workspace/storage backend (Google Drive, local, etc).
	// Uses RFC-084 WorkspaceProvider interface for multi-provider architecture.
	WorkspaceProvider workspace.WorkspaceProvider

	// GWService is retained as a temporary compatibility escape hatch for V2
	// handlers not yet migrated to workspace.Provider methods.
	GWService *gw.Service

	// SharePoint is retained as a temporary compatibility escape hatch for V2
	// handlers not yet migrated to workspace.Provider methods.
	SharePoint *sharepointhelper.Service

	// Config is the config for the server.
	Config *config.Config

	// DB is the database for the server.
	DB *gorm.DB

	// Jira is the Jira service for the server.
	Jira *jira.Service

	// Logger is the logger for the server.
	Logger hclog.Logger

	// ProjectConfig contains workspace project configurations (multi-tenant).
	// This enables different projects to use different workspace providers
	// (local, Google Workspace, remote Hermes) and supports migration scenarios.
	ProjectConfig *projectconfig.Config

	// SemanticSearch provides semantic/vector search capabilities (RFC-088).
	// Uses OpenAI embeddings and pgvector for similarity search.
	SemanticSearch *search.SemanticSearch

	// HybridSearch combines keyword and semantic search (RFC-088).
	// Provides weighted combination of Meilisearch and pgvector results.
	HybridSearch *search.HybridSearch
}

// GetEmailSender returns the configured workspace notification provider.
func (s Server) GetEmailSender() workspace.NotificationProvider {
	return s.WorkspaceProvider
}

// IsSharePoint returns true when the active workspace provider is SharePoint.
func (s Server) IsSharePoint() bool {
	return s.Config != nil && s.Config.Providers != nil && s.Config.Providers.Workspace == "sharepoint"
}

// NewDocumentByFileID returns a document keyed for the active workspace backend.
func (s Server) NewDocumentByFileID(fileID string) models.Document {
	return models.NewDocumentByFileID(fileID, s.IsSharePoint())
}
