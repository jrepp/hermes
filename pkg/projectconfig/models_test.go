package projectconfig

import (
	"testing"
)

func TestConfig_GetProject(t *testing.T) {
	config := &Config{
		Projects: map[string]*Project{
			"test-project": {
				Name:  "test-project",
				Title: "Test Project",
			},
		},
	}

	tests := []struct {
		name        string
		projectName string
		wantErr     bool
	}{
		{
			name:        "existing project",
			projectName: "test-project",
			wantErr:     false,
		},
		{
			name:        "non-existent project",
			projectName: "missing",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, err := config.GetProject(tt.projectName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && project == nil {
				t.Error("GetProject() returned nil project")
			}
			if !tt.wantErr && project.Name != tt.projectName {
				t.Errorf("GetProject() returned wrong project: got %s, want %s", project.Name, tt.projectName)
			}
		})
	}
}

func TestConfig_ListProjects(t *testing.T) {
	config := &Config{
		Projects: map[string]*Project{
			"project-1": {Name: "project-1"},
			"project-2": {Name: "project-2"},
			"project-3": {Name: "project-3"},
		},
	}

	projects := config.ListProjects()
	if len(projects) != 3 {
		t.Errorf("ListProjects() returned %d projects, want 3", len(projects))
	}

	// Check all projects are in the list
	projectMap := make(map[string]bool)
	for _, name := range projects {
		projectMap[name] = true
	}

	for name := range config.Projects {
		if !projectMap[name] {
			t.Errorf("ListProjects() missing project: %s", name)
		}
	}
}

func TestConfig_GetActiveProjects(t *testing.T) {
	config := &Config{
		Projects: map[string]*Project{
			"active-1": {
				Name:   "active-1",
				Status: "active",
			},
			"active-2": {
				Name:   "active-2",
				Status: "active",
			},
			"archived": {
				Name:   "archived",
				Status: "archived",
			},
		},
	}

	active := config.GetActiveProjects()
	if len(active) != 2 {
		t.Errorf("GetActiveProjects() returned %d projects, want 2", len(active))
	}

	for _, project := range active {
		if project.Status != "active" {
			t.Errorf("GetActiveProjects() returned non-active project: %s", project.Name)
		}
	}
}

func TestProject_GetProvider(t *testing.T) {
	project := &Project{
		Name: "test",
		Providers: []*Provider{
			{Type: "local"},
			{Type: "google"},
		},
	}

	tests := []struct {
		name         string
		providerType string
		wantErr      bool
	}{
		{
			name:         "existing provider",
			providerType: "local",
			wantErr:      false,
		},
		{
			name:         "another existing provider",
			providerType: "google",
			wantErr:      false,
		},
		{
			name:         "non-existent provider",
			providerType: "remote-hermes",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := project.GetProvider(tt.providerType)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider.Type != tt.providerType {
				t.Errorf("GetProvider() returned wrong provider type: got %s, want %s", provider.Type, tt.providerType)
			}
		})
	}
}

func TestProject_GetActiveProvider(t *testing.T) {
	tests := []struct {
		name      string
		providers []*Provider
		wantErr   bool
		wantType  string
	}{
		{
			name: "single active provider",
			providers: []*Provider{
				{Type: "local", MigrationStatus: "active"},
			},
			wantErr:  false,
			wantType: "local",
		},
		{
			name: "provider with empty migration status (defaults to active)",
			providers: []*Provider{
				{Type: "local", MigrationStatus: ""},
			},
			wantErr:  false,
			wantType: "local",
		},
		{
			name: "multiple providers with one active",
			providers: []*Provider{
				{Type: "google", MigrationStatus: "source"},
				{Type: "local", MigrationStatus: "active"},
			},
			wantErr:  false,
			wantType: "local",
		},
		{
			name: "no active provider",
			providers: []*Provider{
				{Type: "google", MigrationStatus: "archived"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &Project{
				Name:      "test",
				Providers: tt.providers,
			}

			provider, err := project.GetActiveProvider()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetActiveProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider.Type != tt.wantType {
				t.Errorf("GetActiveProvider() returned wrong provider type: got %s, want %s", provider.Type, tt.wantType)
			}
		})
	}
}

func TestProject_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{
			name:   "active status",
			status: "active",
			want:   true,
		},
		{
			name:   "archived status",
			status: "archived",
			want:   false,
		},
		{
			name:   "completed status",
			status: "completed",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &Project{
				Name:   "test",
				Status: tt.status,
			}

			if got := project.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_ResolveWorkspacePath(t *testing.T) {
	tests := []struct {
		name          string
		workspacePath string
		basePath      string
		want          string
	}{
		{
			name:          "relative path",
			workspacePath: "testing",
			basePath:      "/app/workspaces",
			want:          "/app/workspaces/testing",
		},
		{
			name:          "absolute path",
			workspacePath: "/absolute/path",
			basePath:      "/app/workspaces",
			want:          "/absolute/path",
		},
		{
			name:          "empty workspace path",
			workspacePath: "",
			basePath:      "/app/workspaces",
			want:          "/app/workspaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{
				WorkspacePath: tt.workspacePath,
			}

			got := provider.ResolveWorkspacePath(tt.basePath)
			if got != tt.want {
				t.Errorf("ResolveWorkspacePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_TypeCheckers(t *testing.T) {
	tests := []struct {
		providerType string
		wantIsLocal  bool
		wantIsGoogle bool
		wantIsRemote bool
	}{
		{
			providerType: "local",
			wantIsLocal:  true,
			wantIsGoogle: false,
			wantIsRemote: false,
		},
		{
			providerType: "google",
			wantIsLocal:  false,
			wantIsGoogle: true,
			wantIsRemote: false,
		},
		{
			providerType: "remote-hermes",
			wantIsLocal:  false,
			wantIsGoogle: false,
			wantIsRemote: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.providerType, func(t *testing.T) {
			provider := &Provider{Type: tt.providerType}

			if got := provider.IsLocal(); got != tt.wantIsLocal {
				t.Errorf("IsLocal() = %v, want %v", got, tt.wantIsLocal)
			}

			if got := provider.IsGoogle(); got != tt.wantIsGoogle {
				t.Errorf("IsGoogle() = %v, want %v", got, tt.wantIsGoogle)
			}

			if got := provider.IsRemoteHermes(); got != tt.wantIsRemote {
				t.Errorf("IsRemoteHermes() = %v, want %v", got, tt.wantIsRemote)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Test loading the actual test configuration
	config, err := LoadConfig("../../testing/projects.hcl")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify global settings
	if config.Version != "1.0.0-alpha" {
		t.Errorf("Version = %s, want 1.0.0-alpha", config.Version)
	}

	if config.WorkspaceBasePath != "/app/workspaces" {
		t.Errorf("WorkspaceBasePath = %s, want /app/workspaces", config.WorkspaceBasePath)
	}

	// Verify projects are loaded
	if len(config.Projects) == 0 {
		t.Error("No projects loaded")
	}

	// Verify specific projects
	testProject, err := config.GetProject("testing")
	if err != nil {
		t.Errorf("Failed to get testing project: %v", err)
	} else {
		if testProject.ShortName != "TEST" {
			t.Errorf("testing project ShortName = %s, want TEST", testProject.ShortName)
		}
		if testProject.Status != "active" {
			t.Errorf("testing project Status = %s, want active", testProject.Status)
		}
	}

	docsProject, err := config.GetProject("docs")
	if err != nil {
		t.Errorf("Failed to get docs project: %v", err)
	} else {
		if docsProject.ShortName != "DOCS" {
			t.Errorf("docs project ShortName = %s, want DOCS", docsProject.ShortName)
		}
	}
}

func TestMustParseTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantPanic bool
	}{
		{
			name:      "valid RFC3339",
			input:     "2025-10-22T00:00:00Z",
			wantPanic: false,
		},
		{
			name:      "valid date only",
			input:     "2025-10-22",
			wantPanic: false,
		},
		{
			name:      "invalid format",
			input:     "not a date",
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("mustParseTime() panic = %v, wantPanic %v", r != nil, tt.wantPanic)
				}
			}()

			result := mustParseTime(tt.input)
			if !tt.wantPanic && result.IsZero() {
				t.Error("mustParseTime() returned zero time")
			}
		})
	}
}

func TestResolveEnvVars(t *testing.T) {
	// Set a test environment variable
	t.Setenv("TEST_VAR", "test_value")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "env var reference",
			input: `env("TEST_VAR")`,
			want:  "test_value",
		},
		{
			name:  "non-env var",
			input: "plain value",
			want:  "plain value",
		},
		{
			name:  "undefined env var",
			input: `env("UNDEFINED_VAR")`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveEnvVars(tt.input)
			if got != tt.want {
				t.Errorf("ResolveEnvVars() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests

func BenchmarkConfig_GetProject(b *testing.B) {
	config := &Config{
		Projects: map[string]*Project{
			"test-project": {Name: "test-project"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = config.GetProject("test-project")
	}
}

func BenchmarkProject_GetActiveProvider(b *testing.B) {
	project := &Project{
		Name: "test",
		Providers: []*Provider{
			{Type: "local", MigrationStatus: "active"},
			{Type: "google", MigrationStatus: "source"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = project.GetActiveProvider()
	}
}

func BenchmarkProvider_ResolveWorkspacePath(b *testing.B) {
	provider := &Provider{
		WorkspacePath: "testing",
	}
	basePath := "/app/workspaces"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.ResolveWorkspacePath(basePath)
	}
}
