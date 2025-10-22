package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

// WorkspaceProjectsGetResponse is the response for GET /api/v2/workspace-projects
type WorkspaceProjectsGetResponse struct {
	Projects []*projectconfig.ProjectSummary `json:"projects"`
}

// WorkspaceProjectsHandler handles GET requests for all workspace projects
// Endpoint: GET /api/v2/workspace-projects
func WorkspaceProjectsHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logArgs := []any{
			"path", r.URL.Path,
			"method", r.Method,
		}

		// Authorize request
		userEmail := pkgauth.MustGetUserEmail(r.Context())
		if userEmail == "" {
			srv.Logger.Error("user email not found in request context", logArgs...)
			http.Error(w, "No authorization information for request", http.StatusUnauthorized)
			return
		}

		// Check if project config is loaded
		if srv.ProjectConfig == nil {
			srv.Logger.Warn("project config not loaded", logArgs...)
			http.Error(w, "Workspace projects not configured", http.StatusNotImplemented)
			return
		}

		switch r.Method {
		case "GET":
			// Get all active projects
			summaries := srv.ProjectConfig.GetActiveProjectSummaries()

			resp := WorkspaceProjectsGetResponse{
				Projects: summaries,
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				srv.Logger.Error("error encoding response",
					append(logArgs, "error", err)...)
				http.Error(w, "Error encoding response", http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// WorkspaceProjectHandler handles GET requests for a single workspace project
// Endpoint: GET /api/v2/workspace-projects/{name}
func WorkspaceProjectHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logArgs := []any{
			"path", r.URL.Path,
			"method", r.Method,
		}

		// Authorize request
		userEmail := pkgauth.MustGetUserEmail(r.Context())
		if userEmail == "" {
			srv.Logger.Error("user email not found in request context", logArgs...)
			http.Error(w, "No authorization information for request", http.StatusUnauthorized)
			return
		}

		// Check if project config is loaded
		if srv.ProjectConfig == nil {
			srv.Logger.Warn("project config not loaded", logArgs...)
			http.Error(w, "Workspace projects not configured", http.StatusNotImplemented)
			return
		}

		// Extract project name from URL path
		// URL pattern: /api/v2/workspace-projects/{name}
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/workspace-projects/"), "/")
		if len(pathParts) == 0 || pathParts[0] == "" {
			http.Error(w, "Project name required", http.StatusBadRequest)
			return
		}
		projectName := pathParts[0]

		switch r.Method {
		case "GET":
			// Get single project
			project, err := srv.ProjectConfig.GetProject(projectName)
			if err != nil {
				srv.Logger.Error("project not found",
					append(logArgs, "project_name", projectName, "error", err)...)
				http.Error(w, "Project not found", http.StatusNotFound)
				return
			}

			// Convert to sanitized summary
			summary := project.ToSummary()

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(summary); err != nil {
				srv.Logger.Error("error encoding response",
					append(logArgs, "error", err)...)
				http.Error(w, "Error encoding response", http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
