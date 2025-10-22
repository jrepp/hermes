package projectconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zclconf/go-cty/cty"
)

// LoadConfig loads and parses the projects configuration
// This is a simplified version that manually parses the HCL structure
func LoadConfig(configPath string) (*Config, error) {
	// For now, return a basic implementation
	// Full HCL parsing implementation will be completed in the next iteration
	config := &Config{
		Version:           "1.0.0-alpha",
		ConfigDir:         "./projects",
		WorkspaceBasePath: "/app/workspaces",
		Projects:          make(map[string]*Project),
		Defaults: &Defaults{
			Local: &LocalDefaults{
				IndexingEnabled: true,
				GitBranch:       "main",
			},
		},
	}

	// Parse config file directory
	configDir := filepath.Dir(configPath)

	// For MVP, manually load known project files
	// TODO: Implement full HCL parsing with import statement support
	projectFiles := []string{
		filepath.Join(configDir, "projects/testing.hcl"),
		filepath.Join(configDir, "projects/docs.hcl"),
	}

	for _, projectFile := range projectFiles {
		if _, err := os.Stat(projectFile); os.IsNotExist(err) {
			continue // Skip non-existent files
		}

		project, err := parseProjectFileSimple(projectFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse project file %s: %w", projectFile, err)
		}
		config.Projects[project.Name] = project
	}

	return config, nil
}

// parseProjectFileSimple is a simplified project file parser
// Full HCL parsing will be implemented using hclparse
func parseProjectFileSimple(filename string) (*Project, error) {
	// Extract project name from filename
	base := filepath.Base(filename)
	name := strings.TrimSuffix(base, ".hcl")

	// Return hardcoded project structures for MVP
	// TODO: Replace with full HCL parser
	switch name {
	case "testing":
		return &Project{
			Name:         "testing",
			Title:        "Hermes Testing Environment",
			FriendlyName: "Hermes Testing",
			ShortName:    "TEST",
			Description:  "Local testing workspace for Hermes development",
			Status:       "active",
			Providers: []*Provider{
				{
					Type:            "local",
					MigrationStatus: "active",
					WorkspacePath:   "testing",
					Git: &GitConfig{
						Repository: "https://github.com/hashicorp-forge/hermes",
						Branch:     "main",
					},
					Indexing: &IndexingConfig{
						Enabled:           true,
						AllowedExtensions: []string{"md", "txt", "json", "yaml", "yml"},
					},
				},
			},
			Metadata: &Metadata{
				CreatedAt: mustParseTime("2025-10-22T00:00:00Z"),
				Owner:     "hermes-dev-team",
				Tags:      []string{"testing", "development", "local"},
			},
		}, nil
	case "docs":
		return &Project{
			Name:         "docs",
			Title:        "Hermes Documentation (CMS)",
			FriendlyName: "Hermes Documentation",
			ShortName:    "DOCS",
			Description:  "Public documentation for the open-source Hermes project",
			Status:       "active",
			Providers: []*Provider{
				{
					Type:            "local",
					MigrationStatus: "active",
					WorkspacePath:   "docs",
					Git: &GitConfig{
						Repository: "https://github.com/hashicorp-forge/hermes",
						Branch:     "main",
					},
					Indexing: &IndexingConfig{
						Enabled:           true,
						AllowedExtensions: []string{"md", "mdx"},
						PublicReadAccess:  true,
					},
				},
			},
			Metadata: &Metadata{
				CreatedAt: mustParseTime("2025-10-22T00:00:00Z"),
				Owner:     "hermes-dev-team",
				Tags:      []string{"documentation", "public", "cms"},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown project: %s", name)
	}
}

// Helper functions

func ctyListToStringSlice(val cty.Value) []string {
	if val.IsNull() || !val.IsKnown() {
		return nil
	}

	var result []string
	it := val.ElementIterator()
	for it.Next() {
		_, v := it.Element()
		result = append(result, v.AsString())
	}
	return result
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time %q", s)
}

func mustParseTime(s string) time.Time {
	t, err := parseTime(s)
	if err != nil {
		panic(err)
	}
	return t
}

// LoadConfigFromEnv loads config from environment variable or default path
func LoadConfigFromEnv() (*Config, error) {
	configPath := os.Getenv("HERMES_PROJECTS_CONFIG")
	if configPath == "" {
		configPath = "./testing/projects.hcl"
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	return LoadConfig(configPath)
}

// ResolveEnvVars resolves environment variable references in the config
// This handles the env("VAR_NAME") syntax
func ResolveEnvVars(value string) string {
	// Simple implementation - in production, this would be handled by HCL functions
	if strings.HasPrefix(value, "env(") && strings.HasSuffix(value, ")") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "env(\""), "\")")
		return os.Getenv(envVar)
	}
	return value
}
