package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
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
	OllamaURL     string `json:"ollama_url,omitempty"`   // e.g., "http://localhost:11434"
	OllamaModel   string `json:"ollama_model,omitempty"` // e.g., "llama2"
}

// OllamaValidationRequest contains Ollama connection details to validate
type OllamaValidationRequest struct {
	URL   string `json:"url"`   // e.g., "http://localhost:11434"
	Model string `json:"model"` // e.g., "llama2"
}

// OllamaValidationResponse indicates if Ollama is accessible
type OllamaValidationResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
	Version string `json:"version,omitempty"`
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
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error("error encoding response", "error", err)
		}
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
		if err := generateConfigFile(configPath, workspacePath, req.UpstreamURL, req.OllamaURL, req.OllamaModel); err != nil {
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
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error("error encoding response", "error", err)
		}
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
		if err := os.WriteFile(readmePath, []byte(readme), 0o600); err != nil {
			return fmt.Errorf("error creating README: %w", err)
		}
	}

	return nil
}

// generateConfigFile creates a config.hcl file with the specified settings
func generateConfigFile(configPath, workspacePath, upstreamURL, ollamaURL, ollamaModel string) error {
	cfg := config.GenerateSimplifiedConfig(workspacePath)

	// Add Ollama configuration if provided
	if ollamaURL != "" {
		cfg.Ollama = &config.Ollama{
			URL:            ollamaURL,
			SummarizeModel: ollamaModel,
		}
	}

	// If upstream URL is provided, add it to the config
	// (This would be for syncing with a central Hermes server - future feature)
	if upstreamURL != "" {
		// For now, just add it as a comment in the config
		// Future: implement sync functionality
	}

	return config.WriteConfig(cfg, configPath)
}

// OllamaValidateHandler validates that Ollama is accessible and has the requested model
func OllamaValidateHandler(log hclog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req OllamaValidationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error("error decoding ollama validation request", "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate URL is provided
		if req.URL == "" {
			response := OllamaValidationResponse{
				Valid:   false,
				Message: "Ollama URL is required",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Try to connect to Ollama and get version
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		// Check if Ollama is running by hitting the /api/version endpoint
		versionResp, err := client.Get(req.URL + "/api/version")
		if err != nil {
			log.Warn("ollama connection failed", "url", req.URL, "error", err)
			response := OllamaValidationResponse{
				Valid:   false,
				Message: fmt.Sprintf("Could not connect to Ollama at %s: %v", req.URL, err),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		defer versionResp.Body.Close()

		if versionResp.StatusCode != http.StatusOK {
			response := OllamaValidationResponse{
				Valid:   false,
				Message: fmt.Sprintf("Ollama returned status %d", versionResp.StatusCode),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Parse version response
		var versionData struct {
			Version string `json:"version"`
		}
		if err := json.NewDecoder(versionResp.Body).Decode(&versionData); err != nil {
			log.Warn("failed to parse ollama version", "error", err)
		}

		// If model is specified, check if it's available
		if req.Model != "" {
			tagsResp, err := client.Get(req.URL + "/api/tags")
			if err != nil {
				log.Warn("failed to get ollama tags", "error", err)
				response := OllamaValidationResponse{
					Valid:   true,
					Message: fmt.Sprintf("Connected to Ollama %s (could not verify model)", versionData.Version),
					Version: versionData.Version,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
				return
			}
			defer tagsResp.Body.Close()

			var tagsData struct {
				Models []struct {
					Name string `json:"name"`
				} `json:"models"`
			}
			if err := json.NewDecoder(tagsResp.Body).Decode(&tagsData); err != nil {
				log.Warn("failed to parse ollama tags", "error", err)
			} else {
				// Check if the requested model is in the list
				modelFound := false
				for _, model := range tagsData.Models {
					if strings.Contains(model.Name, req.Model) {
						modelFound = true
						break
					}
				}

				if !modelFound {
					response := OllamaValidationResponse{
						Valid:   false,
						Message: fmt.Sprintf("Model '%s' not found. Run: ollama pull %s", req.Model, req.Model),
						Version: versionData.Version,
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
					return
				}
			}
		}

		// Success!
		response := OllamaValidationResponse{
			Valid:   true,
			Message: fmt.Sprintf("Connected to Ollama %s", versionData.Version),
			Version: versionData.Version,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error("error encoding response", "error", err)
		}
	})
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
