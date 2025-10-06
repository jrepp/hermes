package server

import (
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/email"
	"github.com/hashicorp-forge/hermes/internal/jira"
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
	// This abstracts document storage and collaboration operations like file management
	// and permissions.
	WorkspaceProvider workspace.Provider

	// Config is the config for the server.
	Config *config.Config

	// DB is the database for the server.
	DB *gorm.DB

	// Jira is the Jira service for the server.
	Jira *jira.Service

	// Logger is the logger for the server.
	Logger hclog.Logger

	// MSGraphService is the Microsoft Graph service for the server.
	// MSGraphService *microsoftgraph.Service

	//Sharepoint
	SharePoint *sp.Service
}

// GetEmailSender returns the appropriate email.EmailSender based on which
// backend is configured (SharePoint or Google Workspace).
func (s Server) GetEmailSender() email.EmailSender {
	if s.SharePoint != nil {
		return s.SharePoint
	}
	return &gw.EmailSenderAdapter{Svc: s.GWService}
}

// IsSharePoint returns true when the server is configured for a SharePoint
// backend, false when it is configured for Google Workspace.
func (s Server) IsSharePoint() bool {
	return s.SharePoint != nil
}

// NewDocumentByFileID returns a models.Document with the correct file-ID
// field populated based on the configured backend.
func (s Server) NewDocumentByFileID(fileID string) models.Document {
	return models.NewDocumentByFileID(fileID, s.IsSharePoint())
}
