package projectconfig

import (
	"strings"
	"testing"
)

func TestValidator_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Version:           "1.0.0-alpha",
				WorkspaceBasePath: "/app/workspaces",
				Projects: map[string]*Project{
					"test": {
						Name:      "test",
						Title:     "Test Project",
						ShortName: "TEST",
						Status:    "active",
						Providers: []*Provider{
							{Type: "local", WorkspacePath: "test"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			config: &Config{
				Version:           "",
				WorkspaceBasePath: "/app/workspaces",
				Projects:          map[string]*Project{},
			},
			wantErr: true,
			errMsg:  "version is required",
		},
		{
			name: "invalid version format",
			config: &Config{
				Version:           "invalid",
				WorkspaceBasePath: "/app/workspaces",
				Projects:          map[string]*Project{},
			},
			wantErr: true,
			errMsg:  "version must be in semver format",
		},
		{
			name: "missing workspace_base_path",
			config: &Config{
				Version:           "1.0.0",
				WorkspaceBasePath: "",
				Projects:          map[string]*Project{},
			},
			wantErr: true,
			errMsg:  "workspace_base_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error message = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidator_ValidateProject(t *testing.T) {
	config := &Config{
		Version:           "1.0.0",
		WorkspaceBasePath: "/app/workspaces",
	}

	tests := []struct {
		name    string
		project *Project
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid project",
			project: &Project{
				Name:      "test-project",
				Title:     "Test Project",
				ShortName: "TEST",
				Status:    "active",
				Providers: []*Provider{
					{Type: "local", WorkspacePath: "test"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid project name (not kebab-case)",
			project: &Project{
				Name:      "TestProject",
				Title:     "Test Project",
				ShortName: "TEST",
				Status:    "active",
				Providers: []*Provider{
					{Type: "local"},
				},
			},
			wantErr: true,
			errMsg:  "name must be lowercase alphanumeric with hyphens",
		},
		{
			name: "missing title",
			project: &Project{
				Name:      "test",
				Title:     "",
				ShortName: "TEST",
				Status:    "active",
				Providers: []*Provider{
					{Type: "local"},
				},
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "invalid short name (lowercase)",
			project: &Project{
				Name:      "test",
				Title:     "Test",
				ShortName: "test",
				Status:    "active",
				Providers: []*Provider{
					{Type: "local"},
				},
			},
			wantErr: true,
			errMsg:  "short_name must be 2-10 uppercase characters",
		},
		{
			name: "invalid short name (too long)",
			project: &Project{
				Name:      "test",
				Title:     "Test",
				ShortName: "VERYLONGNAME",
				Status:    "active",
				Providers: []*Provider{
					{Type: "local"},
				},
			},
			wantErr: true,
			errMsg:  "short_name must be 2-10 uppercase characters",
		},
		{
			name: "invalid status",
			project: &Project{
				Name:      "test",
				Title:     "Test",
				ShortName: "TEST",
				Status:    "invalid",
				Providers: []*Provider{
					{Type: "local"},
				},
			},
			wantErr: true,
			errMsg:  "status must be one of: active, completed, archived",
		},
		{
			name: "no providers",
			project: &Project{
				Name:      "test",
				Title:     "Test",
				ShortName: "TEST",
				Status:    "active",
				Providers: []*Provider{},
			},
			wantErr: true,
			errMsg:  "at least one provider is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			validator.validateProject(tt.project, config)

			hasErrors := len(validator.errors) > 0
			if hasErrors != tt.wantErr {
				t.Errorf("validateProject() hasErrors = %v, wantErr %v", hasErrors, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				found := false
				for _, err := range validator.errors {
					if strings.Contains(err.Message, tt.errMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateProject() errors = %v, want to contain %v", validator.errors, tt.errMsg)
				}
			}
		})
	}
}

func TestValidator_ValidateProvider(t *testing.T) {
	config := &Config{
		WorkspaceBasePath: "/app/workspaces",
	}
	project := &Project{
		Name: "test",
	}

	tests := []struct {
		name     string
		provider *Provider
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid local provider",
			provider: &Provider{
				Type:            "local",
				MigrationStatus: "active",
				WorkspacePath:   "test",
				Git: &GitConfig{
					Repository: "https://github.com/example/repo",
					Branch:     "main",
				},
				Indexing: &IndexingConfig{
					Enabled:           true,
					AllowedExtensions: []string{"md", "txt"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid provider type",
			provider: &Provider{
				Type: "invalid",
			},
			wantErr: true,
			errMsg:  "type must be one of: local, google, remote-hermes",
		},
		{
			name: "local provider missing workspace_path",
			provider: &Provider{
				Type:          "local",
				WorkspacePath: "",
			},
			wantErr: true,
			errMsg:  "workspace_path is required for local provider",
		},
		{
			name: "local provider invalid git URL",
			provider: &Provider{
				Type:          "local",
				WorkspacePath: "test",
				Git: &GitConfig{
					Repository: "not-a-url",
				},
			},
			wantErr: true,
			errMsg:  "repository must be a valid URL",
		},
		{
			name: "local provider invalid file extension",
			provider: &Provider{
				Type:          "local",
				WorkspacePath: "test",
				Indexing: &IndexingConfig{
					AllowedExtensions: []string{".md", "txt"},
				},
			},
			wantErr: true,
			errMsg:  "invalid extension",
		},
		{
			name: "google provider missing workspace_id",
			provider: &Provider{
				Type:                "google",
				WorkspaceID:         "",
				ServiceAccountEmail: "test@example.com",
				CredentialsPath:     "/path/to/creds",
			},
			wantErr: true,
			errMsg:  "workspace_id is required for Google provider",
		},
		{
			name: "remote-hermes provider invalid URL",
			provider: &Provider{
				Type:       "remote-hermes",
				HermesURL:  "not-a-url",
				APIVersion: "v2",
			},
			wantErr: true,
			errMsg:  "hermes_url must be a valid URL",
		},
		{
			name: "remote-hermes provider invalid API version",
			provider: &Provider{
				Type:       "remote-hermes",
				HermesURL:  "https://hermes.example.com",
				APIVersion: "v3",
			},
			wantErr: true,
			errMsg:  "api_version must be v1 or v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			validator.validateProvider(tt.provider, project, config, 0)

			hasErrors := len(validator.errors) > 0
			if hasErrors != tt.wantErr {
				t.Errorf("validateProvider() hasErrors = %v, wantErr %v, errors = %v", hasErrors, tt.wantErr, validator.errors)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				found := false
				for _, err := range validator.errors {
					if strings.Contains(err.Message, tt.errMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateProvider() errors = %v, want to contain %v", validator.errors, tt.errMsg)
				}
			}
		})
	}
}

func TestValidator_ValidateMigration(t *testing.T) {
	tests := []struct {
		name    string
		project *Project
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid migration with source and target",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: "google", MigrationStatus: "source"},
					{Type: "local", MigrationStatus: "target"},
				},
			},
			wantErr: false,
		},
		{
			name: "source without target",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: "google", MigrationStatus: "source"},
				},
			},
			wantErr: true,
			errMsg:  "migration has source provider but no target provider",
		},
		{
			name: "target without source",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: "local", MigrationStatus: "target"},
				},
			},
			wantErr: true,
			errMsg:  "migration has target provider but no source provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			validator.validateMigration(tt.project)

			hasErrors := len(validator.errors) > 0
			if hasErrors != tt.wantErr {
				t.Errorf("validateMigration() hasErrors = %v, wantErr %v", hasErrors, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				found := false
				for _, err := range validator.errors {
					if strings.Contains(err.Message, tt.errMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateMigration() errors = %v, want to contain %v", validator.errors, tt.errMsg)
				}
			}
		})
	}
}

func TestValidationHelpers(t *testing.T) {
	t.Run("isValidVersion", func(t *testing.T) {
		tests := []struct {
			version string
			want    bool
		}{
			{"1.0.0", true},
			{"1.0.0-alpha", true},
			{"1.2.3-beta", true},
			{"invalid", false},
			{"1.0", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidVersion(tt.version)
			if got != tt.want {
				t.Errorf("isValidVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		}
	})

	t.Run("isValidProjectName", func(t *testing.T) {
		tests := []struct {
			name string
			want bool
		}{
			{"test-project", true},
			{"test", true},
			{"test-project-123", true},
			{"TestProject", false},
			{"test_project", false},
			{"-test", false},
			{"test-", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidProjectName(tt.name)
			if got != tt.want {
				t.Errorf("isValidProjectName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		}
	})

	t.Run("isValidShortName", func(t *testing.T) {
		tests := []struct {
			name string
			want bool
		}{
			{"TEST", true},
			{"RFC", true},
			{"DOCS", true},
			{"VERYLONGNAME", false},
			{"T", false},
			{"test", false},
			{"Test", false},
			{"123", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidShortName(tt.name)
			if got != tt.want {
				t.Errorf("isValidShortName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		}
	})

	t.Run("isValidStatus", func(t *testing.T) {
		tests := []struct {
			status string
			want   bool
		}{
			{"active", true},
			{"completed", true},
			{"archived", true},
			{"invalid", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidStatus(tt.status)
			if got != tt.want {
				t.Errorf("isValidStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		}
	})

	t.Run("isValidProviderType", func(t *testing.T) {
		tests := []struct {
			providerType string
			want         bool
		}{
			{"local", true},
			{"google", true},
			{"remote-hermes", true},
			{"invalid", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidProviderType(tt.providerType)
			if got != tt.want {
				t.Errorf("isValidProviderType(%q) = %v, want %v", tt.providerType, got, tt.want)
			}
		}
	})

	t.Run("isValidMigrationStatus", func(t *testing.T) {
		tests := []struct {
			status string
			want   bool
		}{
			{"active", true},
			{"source", true},
			{"target", true},
			{"archived", true},
			{"invalid", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidMigrationStatus(tt.status)
			if got != tt.want {
				t.Errorf("isValidMigrationStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		}
	})

	t.Run("isValidURL", func(t *testing.T) {
		tests := []struct {
			url  string
			want bool
		}{
			{"https://example.com", true},
			{"http://localhost:8000", true},
			{"not-a-url", false},
			{"ftp://example.com", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidURL(tt.url)
			if got != tt.want {
				t.Errorf("isValidURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		}
	})

	t.Run("isValidFileExtension", func(t *testing.T) {
		tests := []struct {
			ext  string
			want bool
		}{
			{"md", true},
			{"txt", true},
			{"json", true},
			{".md", false},
			{"MD", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidFileExtension(tt.ext)
			if got != tt.want {
				t.Errorf("isValidFileExtension(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		}
	})

	t.Run("isValidAuthMethod", func(t *testing.T) {
		tests := []struct {
			method string
			want   bool
		}{
			{"oidc", true},
			{"bearer", true},
			{"api-key", true},
			{"invalid", false},
			{"", false},
		}

		for _, tt := range tests {
			got := isValidAuthMethod(tt.method)
			if got != tt.want {
				t.Errorf("isValidAuthMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		}
	})
}

func TestValidationErrors(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		err := &ValidationError{
			Field:   "test.field",
			Message: "test message",
		}

		want := "test.field: test message"
		if got := err.Error(); got != want {
			t.Errorf("ValidationError.Error() = %v, want %v", got, want)
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		errors := ValidationErrors{
			{Field: "field1", Message: "error1"},
			{Field: "field2", Message: "error2"},
		}

		errStr := errors.Error()
		if !strings.Contains(errStr, "field1: error1") {
			t.Errorf("ValidationErrors.Error() should contain first error")
		}
		if !strings.Contains(errStr, "field2: error2") {
			t.Errorf("ValidationErrors.Error() should contain second error")
		}
	})

	t.Run("empty errors", func(t *testing.T) {
		errors := ValidationErrors{}
		if got := errors.Error(); got != "" {
			t.Errorf("ValidationErrors.Error() = %v, want empty string", got)
		}
	})
}

// Integration test with actual config
func TestValidateActualConfig(t *testing.T) {
	config, err := LoadConfig("../../testing/projects.hcl")
	if err != nil {
		t.Skipf("Skipping integration test, config not found: %v", err)
		return
	}

	err = ValidateConfig(config)
	if err != nil {
		t.Errorf("ValidateConfig() failed for actual config: %v", err)
	}
}

// Benchmark tests

func BenchmarkValidator_Validate(b *testing.B) {
	config := &Config{
		Version:           "1.0.0",
		WorkspaceBasePath: "/app/workspaces",
		Projects: map[string]*Project{
			"test": {
				Name:      "test",
				Title:     "Test",
				ShortName: "TEST",
				Status:    "active",
				Providers: []*Provider{
					{Type: "local", WorkspacePath: "test"},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateConfig(config)
	}
}
