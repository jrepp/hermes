package projectconfig

import (
	"testing"
)

func TestProvider_GetState(t *testing.T) {
	tests := []struct {
		name            string
		migrationStatus string
		expectedState   string
	}{
		{
			name:            "empty defaults to active",
			migrationStatus: "",
			expectedState:   ProviderStateActive,
		},
		{
			name:            "explicit active",
			migrationStatus: ProviderStateActive,
			expectedState:   ProviderStateActive,
		},
		{
			name:            "source state",
			migrationStatus: ProviderStateSource,
			expectedState:   ProviderStateSource,
		},
		{
			name:            "target state",
			migrationStatus: ProviderStateTarget,
			expectedState:   ProviderStateTarget,
		},
		{
			name:            "archived state",
			migrationStatus: ProviderStateArchived,
			expectedState:   ProviderStateArchived,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{
				Type:            ProviderTypeLocal,
				MigrationStatus: tt.migrationStatus,
			}

			state := provider.GetState()
			if state != tt.expectedState {
				t.Errorf("GetState() = %s, want %s", state, tt.expectedState)
			}
		})
	}
}

func TestProvider_StateCheckers(t *testing.T) {
	tests := []struct {
		name            string
		migrationStatus string
		expectActive    bool
		expectSource    bool
		expectTarget    bool
		expectArchived  bool
	}{
		{
			name:            "active state",
			migrationStatus: ProviderStateActive,
			expectActive:    true,
			expectSource:    false,
			expectTarget:    false,
			expectArchived:  false,
		},
		{
			name:            "empty defaults to active",
			migrationStatus: "",
			expectActive:    true,
			expectSource:    false,
			expectTarget:    false,
			expectArchived:  false,
		},
		{
			name:            "source state",
			migrationStatus: ProviderStateSource,
			expectActive:    false,
			expectSource:    true,
			expectTarget:    false,
			expectArchived:  false,
		},
		{
			name:            "target state",
			migrationStatus: ProviderStateTarget,
			expectActive:    false,
			expectSource:    false,
			expectTarget:    true,
			expectArchived:  false,
		},
		{
			name:            "archived state",
			migrationStatus: ProviderStateArchived,
			expectActive:    false,
			expectSource:    false,
			expectTarget:    false,
			expectArchived:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{
				Type:            ProviderTypeLocal,
				MigrationStatus: tt.migrationStatus,
			}

			if provider.IsActiveState() != tt.expectActive {
				t.Errorf("IsActiveState() = %v, want %v", provider.IsActiveState(), tt.expectActive)
			}
			if provider.IsSourceState() != tt.expectSource {
				t.Errorf("IsSourceState() = %v, want %v", provider.IsSourceState(), tt.expectSource)
			}
			if provider.IsTargetState() != tt.expectTarget {
				t.Errorf("IsTargetState() = %v, want %v", provider.IsTargetState(), tt.expectTarget)
			}
			if provider.IsArchivedState() != tt.expectArchived {
				t.Errorf("IsArchivedState() = %v, want %v", provider.IsArchivedState(), tt.expectArchived)
			}
		})
	}
}

func TestProvider_GetRole(t *testing.T) {
	tests := []struct {
		name            string
		migrationStatus string
		expectedRole    string
	}{
		{
			name:            "active role",
			migrationStatus: ProviderStateActive,
			expectedRole:    "Active (read/write)",
		},
		{
			name:            "source role",
			migrationStatus: ProviderStateSource,
			expectedRole:    "Migration source (read-only)",
		},
		{
			name:            "target role",
			migrationStatus: ProviderStateTarget,
			expectedRole:    "Migration target (write destination)",
		},
		{
			name:            "archived role",
			migrationStatus: ProviderStateArchived,
			expectedRole:    "Archived (no operations)",
		},
		{
			name:            "unknown role",
			migrationStatus: "invalid",
			expectedRole:    "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{
				Type:            ProviderTypeLocal,
				MigrationStatus: tt.migrationStatus,
			}

			role := provider.GetRole()
			if role != tt.expectedRole {
				t.Errorf("GetRole() = %s, want %s", role, tt.expectedRole)
			}
		})
	}
}

func TestProject_GetProvidersByState(t *testing.T) {
	project := &Project{
		Name: "test-project",
		Providers: []*Provider{
			{Type: ProviderTypeLocal, MigrationStatus: ProviderStateActive},
			{Type: ProviderTypeGoogle, MigrationStatus: ProviderStateSource},
			{Type: ProviderTypeLocal, MigrationStatus: ProviderStateTarget},
		},
	}

	tests := []struct {
		name          string
		state         string
		expectedCount int
	}{
		{
			name:          "find active providers",
			state:         ProviderStateActive,
			expectedCount: 1,
		},
		{
			name:          "find source providers",
			state:         ProviderStateSource,
			expectedCount: 1,
		},
		{
			name:          "find target providers",
			state:         ProviderStateTarget,
			expectedCount: 1,
		},
		{
			name:          "find archived providers",
			state:         ProviderStateArchived,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := project.GetProvidersByState(tt.state)
			if len(providers) != tt.expectedCount {
				t.Errorf("GetProvidersByState(%s) returned %d providers, want %d", tt.state, len(providers), tt.expectedCount)
			}
		})
	}
}

func TestProject_GetPrimaryProvider(t *testing.T) {
	tests := []struct {
		name          string
		project       *Project
		expectedType  string
		expectedState string
		wantErr       bool
	}{
		{
			name: "single active provider",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateActive},
				},
			},
			expectedType:  ProviderTypeLocal,
			expectedState: ProviderStateActive,
			wantErr:       false,
		},
		{
			name: "migration - returns target",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeGoogle, MigrationStatus: ProviderStateSource},
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateTarget},
				},
			},
			expectedType:  ProviderTypeLocal,
			expectedState: ProviderStateTarget,
			wantErr:       false,
		},
		{
			name: "empty migration_status defaults to active",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ""},
				},
			},
			expectedType:  ProviderTypeLocal,
			expectedState: ProviderStateActive,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := tt.project.GetPrimaryProvider()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPrimaryProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if provider.Type != tt.expectedType {
					t.Errorf("GetPrimaryProvider() type = %s, want %s", provider.Type, tt.expectedType)
				}
				if provider.GetState() != tt.expectedState {
					t.Errorf("GetPrimaryProvider() state = %s, want %s", provider.GetState(), tt.expectedState)
				}
			}
		})
	}
}

func TestProject_StatusCheckers(t *testing.T) {
	tests := []struct {
		name            string
		status          string
		expectActive    bool
		expectArchived  bool
		expectCompleted bool
	}{
		{
			name:            "active project",
			status:          ProjectStatusActive,
			expectActive:    true,
			expectArchived:  false,
			expectCompleted: false,
		},
		{
			name:            "archived project",
			status:          ProjectStatusArchived,
			expectActive:    false,
			expectArchived:  true,
			expectCompleted: false,
		},
		{
			name:            "completed project",
			status:          ProjectStatusCompleted,
			expectActive:    false,
			expectArchived:  false,
			expectCompleted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &Project{
				Name:   "test",
				Status: tt.status,
			}

			if project.IsActive() != tt.expectActive {
				t.Errorf("IsActive() = %v, want %v", project.IsActive(), tt.expectActive)
			}
			if project.IsArchived() != tt.expectArchived {
				t.Errorf("IsArchived() = %v, want %v", project.IsArchived(), tt.expectArchived)
			}
			if project.IsCompleted() != tt.expectCompleted {
				t.Errorf("IsCompleted() = %v, want %v", project.IsCompleted(), tt.expectCompleted)
			}
		})
	}
}

func TestProject_IsInMigration(t *testing.T) {
	tests := []struct {
		name            string
		project         *Project
		expectMigration bool
	}{
		{
			name: "single active provider - not in migration",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateActive},
				},
			},
			expectMigration: false,
		},
		{
			name: "source and target - in migration",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeGoogle, MigrationStatus: ProviderStateSource},
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateTarget},
				},
			},
			expectMigration: true,
		},
		{
			name: "only source - not in migration",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeGoogle, MigrationStatus: ProviderStateSource},
				},
			},
			expectMigration: false,
		},
		{
			name: "only target - not in migration",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateTarget},
				},
			},
			expectMigration: false,
		},
		{
			name: "multiple active providers - not in migration",
			project: &Project{
				Name: "test",
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateActive},
					{Type: ProviderTypeGoogle, MigrationStatus: ProviderStateActive},
				},
			},
			expectMigration: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.project.IsInMigration() != tt.expectMigration {
				t.Errorf("IsInMigration() = %v, want %v", tt.project.IsInMigration(), tt.expectMigration)
			}
		})
	}
}

func TestProvider_Sanitize(t *testing.T) {
	t.Run("local provider sanitization", func(t *testing.T) {
		provider := &Provider{
			Type:            ProviderTypeLocal,
			MigrationStatus: ProviderStateActive,
			WorkspacePath:   "/app/workspaces/test",
			Git: &GitConfig{
				Repository: "https://github.com/example/repo",
				Branch:     "main",
			},
			Indexing: &IndexingConfig{
				Enabled:           true,
				AllowedExtensions: []string{"md", "txt"},
				PublicReadAccess:  false,
			},
		}

		sanitized := provider.Sanitize()

		// Check sanitized fields are present
		if sanitized.Type != provider.Type {
			t.Errorf("Type not preserved: got %s, want %s", sanitized.Type, provider.Type)
		}
		if sanitized.WorkspacePath != provider.WorkspacePath {
			t.Errorf("WorkspacePath not preserved")
		}
		if sanitized.Git == nil || sanitized.Git.Repository != provider.Git.Repository {
			t.Errorf("Git config not preserved")
		}
		if sanitized.Indexing == nil || sanitized.Indexing.Enabled != provider.Indexing.Enabled {
			t.Errorf("Indexing config not preserved")
		}
	})

	t.Run("google provider sanitization", func(t *testing.T) {
		provider := &Provider{
			Type:                ProviderTypeGoogle,
			MigrationStatus:     ProviderStateSource,
			WorkspaceID:         "workspace-123",
			ServiceAccountEmail: "service@example.com", // Should be excluded
			CredentialsPath:     "/secrets/creds.json", // Should be excluded
			SharedDriveIDs:      []string{"drive-1", "drive-2"},
		}

		sanitized := provider.Sanitize()

		// Check non-sensitive fields are present
		if sanitized.WorkspaceID != provider.WorkspaceID {
			t.Errorf("WorkspaceID not preserved")
		}
		if len(sanitized.SharedDriveIDs) != len(provider.SharedDriveIDs) {
			t.Errorf("SharedDriveIDs not preserved")
		}

		// Check sensitive fields are excluded
		if sanitized.ServiceAccountEmail != "" {
			t.Errorf("ServiceAccountEmail should be excluded, got %s", sanitized.ServiceAccountEmail)
		}
		if sanitized.CredentialsPath != "" {
			t.Errorf("CredentialsPath should be excluded, got %s", sanitized.CredentialsPath)
		}
	})

	t.Run("remote-hermes provider with authentication", func(t *testing.T) {
		provider := &Provider{
			Type:            ProviderTypeRemoteHermes,
			MigrationStatus: ProviderStateTarget,
			HermesURL:       "https://hermes.example.com",
			APIVersion:      "v2",
			Authentication: &Authentication{
				Method:       "oauth2",
				ClientID:     "client-123", // Should be excluded
				ClientSecret: "secret-456", // Should be excluded
			},
		}

		sanitized := provider.Sanitize()

		// Check non-sensitive fields are present
		if sanitized.HermesURL != provider.HermesURL {
			t.Errorf("HermesURL not preserved")
		}
		if sanitized.APIVersion != provider.APIVersion {
			t.Errorf("APIVersion not preserved")
		}

		// Check authentication method is preserved but credentials are excluded
		if sanitized.Authentication == nil {
			t.Errorf("Authentication should indicate presence")
		}
		if sanitized.Authentication.Method != provider.Authentication.Method {
			t.Errorf("Authentication method not preserved")
		}
		if sanitized.Authentication.ClientID != "" {
			t.Errorf("ClientID should be excluded")
		}
		if sanitized.Authentication.ClientSecret != "" {
			t.Errorf("ClientSecret should be excluded")
		}
	})
}

func TestProject_ToSummary(t *testing.T) {
	project := &Project{
		Name:         "test-project",
		Title:        "Test Project",
		FriendlyName: "Test",
		ShortName:    "TEST",
		Description:  "A test project",
		Status:       ProjectStatusActive,
		Providers: []*Provider{
			{
				Type:            ProviderTypeLocal,
				MigrationStatus: ProviderStateActive,
				WorkspacePath:   "/app/workspaces/test",
				Git: &GitConfig{
					Repository: "https://github.com/example/repo",
					Branch:     "main",
				},
			},
		},
		Metadata: &Metadata{
			Owner: "team@example.com",
			Tags:  []string{"test", "development"},
		},
	}

	summary := project.ToSummary()

	// Check basic fields
	if summary.Name != project.Name {
		t.Errorf("Name not preserved: got %s, want %s", summary.Name, project.Name)
	}
	if summary.Title != project.Title {
		t.Errorf("Title not preserved")
	}
	if summary.Status != project.Status {
		t.Errorf("Status not preserved")
	}

	// Check computed fields
	if !summary.IsActive {
		t.Errorf("IsActive should be true for active project")
	}
	if summary.IsArchived {
		t.Errorf("IsArchived should be false for active project")
	}
	if summary.InMigration {
		t.Errorf("InMigration should be false for single provider")
	}

	// Check providers
	if len(summary.Providers) != len(project.Providers) {
		t.Errorf("Providers count mismatch: got %d, want %d", len(summary.Providers), len(project.Providers))
	}

	providerSummary := summary.Providers[0]
	if providerSummary.Type != ProviderTypeLocal {
		t.Errorf("Provider type not preserved")
	}
	if providerSummary.State != ProviderStateActive {
		t.Errorf("Provider state not correct")
	}
	if providerSummary.Role != "Active (read/write)" {
		t.Errorf("Provider role not correct: got %s", providerSummary.Role)
	}
	if providerSummary.GitRepository != project.Providers[0].Git.Repository {
		t.Errorf("Git repository not preserved")
	}

	// Check metadata
	if summary.Metadata == nil {
		t.Errorf("Metadata should be present")
	}
}

func TestConfig_GetAllProjectSummaries(t *testing.T) {
	config := &Config{
		Projects: map[string]*Project{
			"project-1": {
				Name:   "project-1",
				Title:  "Project 1",
				Status: ProjectStatusActive,
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateActive},
				},
			},
			"project-2": {
				Name:   "project-2",
				Title:  "Project 2",
				Status: ProjectStatusArchived,
				Providers: []*Provider{
					{Type: ProviderTypeGoogle, MigrationStatus: ProviderStateArchived},
				},
			},
		},
	}

	summaries := config.GetAllProjectSummaries()

	if len(summaries) != 2 {
		t.Errorf("GetAllProjectSummaries() returned %d summaries, want 2", len(summaries))
	}

	// Verify summaries are complete
	for _, summary := range summaries {
		if summary.Name == "" {
			t.Errorf("Summary missing name")
		}
		if len(summary.Providers) == 0 {
			t.Errorf("Summary missing providers")
		}
	}
}

func TestConfig_GetActiveProjectSummaries(t *testing.T) {
	config := &Config{
		Projects: map[string]*Project{
			"active-1": {
				Name:   "active-1",
				Status: ProjectStatusActive,
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateActive},
				},
			},
			"active-2": {
				Name:   "active-2",
				Status: ProjectStatusActive,
				Providers: []*Provider{
					{Type: ProviderTypeGoogle, MigrationStatus: ProviderStateActive},
				},
			},
			"archived": {
				Name:   "archived",
				Status: ProjectStatusArchived,
				Providers: []*Provider{
					{Type: ProviderTypeLocal, MigrationStatus: ProviderStateArchived},
				},
			},
		},
	}

	summaries := config.GetActiveProjectSummaries()

	if len(summaries) != 2 {
		t.Errorf("GetActiveProjectSummaries() returned %d summaries, want 2", len(summaries))
	}

	// Verify all returned summaries are active
	for _, summary := range summaries {
		if !summary.IsActive {
			t.Errorf("Non-active project in active summaries: %s", summary.Name)
		}
	}
}
