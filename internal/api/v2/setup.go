package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp/go-hclog"
)

// SetupStatusResponse indicates whether Hermes is configured
type SetupStatusResponse struct {
	IsConfigured bool   `json:"is_configured"`
	ConfigPath   string `json:"config_path,omitempty"`
	WorkingDir   string `json:"working_dir"`
}

// SetupConfigRequest contains the setup wizard configuration
type SetupConfigRequest struct {
	WorkspacePath string `json:"workspace_path"` // Relative to working directory
	UpstreamURL   string `json:"upstream_url,omitempty"`
}

// SetupConfigResponse is returned after successful configuration
type SetupConfigResponse struct {
	Success      bool   `json:"success"`
	ConfigPath   string `json:"config_path"`
	WorkspaceDir string `json:"workspace_dir"`
	Message      string `json:"message"`
}

// SetupStatusHandler checks if Hermes is configured
func SetupStatusHandler(configPath string, log hclog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		workingDir, err := os.Getwd()
		if err != nil {
			log.Error("error getting working directory", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Check if config file exists
		isConfigured := configPath != "" && fileExists(configPath)

		response := SetupStatusResponse{
			IsConfigured: isConfigured,
			ConfigPath:   configPath,
			WorkingDir:   workingDir,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})
}

// SetupConfigureHandler handles the initial configuration setup
func SetupConfigureHandler(log hclog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req SetupConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error("error decoding setup request", "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Get working directory
		workingDir, err := os.Getwd()
		if err != nil {
			log.Error("error getting working directory", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Validate and resolve workspace path
		workspacePath, err := validateWorkspacePath(req.WorkspacePath, workingDir)
		if err != nil {
			log.Error("invalid workspace path", "error", err, "path", req.WorkspacePath)
			http.Error(w, fmt.Sprintf("Invalid workspace path: %v", err), http.StatusBadRequest)
			return
		}

		// Create workspace directory structure if it doesn't exist
		if err := ensureWorkspaceExists(workspacePath); err != nil {
			log.Error("error creating workspace", "error", err)
			http.Error(w, fmt.Sprintf("Error creating workspace: %v", err), http.StatusInternalServerError)
			return
		}

		// Generate config file in the working directory
		configPath := filepath.Join(workingDir, "config.hcl")
		if err := generateConfigFile(configPath, workspacePath, req.UpstreamURL); err != nil {
			log.Error("error generating config file", "error", err)
			http.Error(w, fmt.Sprintf("Error generating config: %v", err), http.StatusInternalServerError)
			return
		}

		log.Info("hermes configured successfully", "config", configPath, "workspace", workspacePath)

		response := SetupConfigResponse{
			Success:      true,
			ConfigPath:   configPath,
			WorkspaceDir: workspacePath,
			Message:      "Configuration created successfully. Please restart Hermes.",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})
}

// validateWorkspacePath ensures the path is within the working directory
func validateWorkspacePath(userPath, workingDir string) (string, error) {
	// If empty, default to ./docs-cms
	if userPath == "" {
		userPath = "docs-cms"
	}

	// Clean the path
	userPath = filepath.Clean(userPath)

	// Convert to absolute path
	var absPath string
	if filepath.IsAbs(userPath) {
		absPath = userPath
	} else {
		absPath = filepath.Join(workingDir, userPath)
	}

	// Ensure it's within or equal to working directory
	relPath, err := filepath.Rel(workingDir, absPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check for path traversal
	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("workspace path must be within the working directory")
	}

	return absPath, nil
}

// ensureWorkspaceExists creates the workspace directory structure
func ensureWorkspaceExists(workspacePath string) error {
	dirs := []string{
		workspacePath,
		filepath.Join(workspacePath, "documents"),
		filepath.Join(workspacePath, "drafts"),
		filepath.Join(workspacePath, "attachments"),
		filepath.Join(workspacePath, "templates"),
		filepath.Join(workspacePath, "data"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	// Create README if it doesn't exist
	readmePath := filepath.Join(workspacePath, "README.md")
	if !fileExists(readmePath) {
		readme := `# Hermes Workspace

This directory contains your Hermes document management system data.

## Structure

- **documents/** - Published documents
- **drafts/** - Draft documents
- **attachments/** - Binary attachments (images, PDFs)
- **templates/** - Document templates
- **data/** - Database and search index

## Getting Started

Create your first document using the web interface at http://localhost:8000
`
		if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
			return fmt.Errorf("error creating README: %w", err)
		}
	}

	return nil
}

// generateConfigFile creates a config.hcl file with the specified settings
func generateConfigFile(configPath, workspacePath, upstreamURL string) error {
	cfg := config.GenerateSimplifiedConfig(workspacePath)

	// If upstream URL is provided, add it to the config
	// (This would be for syncing with a central Hermes server - future feature)
	if upstreamURL != "" {
		// For now, just add it as a comment in the config
		// Future: implement sync functionality
	}

	return config.WriteConfig(cfg, configPath)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
