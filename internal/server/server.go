package server

import (
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/jira"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// Server contains the server configuration.
type Server struct {
	// SearchProvider is the search backend (Algolia, Meilisearch, etc).
	// This is the preferred way to access search functionality.
	SearchProvider search.Provider

	// WorkspaceProvider is the workspace/storage backend (Google Drive, local, etc).
	// Uses RFC-084 WorkspaceProvider interface for multi-provider architecture.
	WorkspaceProvider workspace.WorkspaceProvider

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
