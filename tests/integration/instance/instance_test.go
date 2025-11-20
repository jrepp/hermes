//go:build integration
// +build integration

package instance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/instance"
	"github.com/hashicorp-forge/hermes/internal/projects"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

// TestInstanceInitialization validates that instance identity is correctly initialized
func TestInstanceInitialization(t *testing.T) {
	// Setup database (cleans automatically)
	db := setupTestDatabase(t)

	// Reset instance state (important for test isolation)
	instance.ResetForTesting()

	// Create test config
	cfg := &config.Config{
		BaseURL: "http://localhost:8000",
		Server:  &config.Server{},
	}

	logger := hclog.NewNullLogger()
	ctx := context.Background()

	// Initialize instance
	err := instance.Initialize(ctx, db, cfg, logger)
	require.NoError(t, err, "Instance initialization should succeed")

	// Verify instance was created
	inst := instance.GetCurrentInstance()
	require.NotNil(t, inst, "Current instance should not be nil")
	assert.NotEqual(t, uuid.Nil, inst.InstanceUUID, "Instance UUID should be generated")
	assert.Equal(t, "http://localhost:8000", inst.InstanceID, "Instance ID should match BaseURL")
	assert.Equal(t, "development", inst.DeploymentEnv, "Default environment should be development")

	// Verify singleton behavior - second initialization should reuse existing
	err = instance.Initialize(ctx, db, cfg, logger)
	require.NoError(t, err, "Second initialization should succeed")

	inst2 := instance.GetCurrentInstance()
	require.NotNil(t, inst2)
	assert.Equal(t, inst.InstanceUUID, inst2.InstanceUUID, "Instance UUID should not change")
	assert.Equal(t, "http://localhost:8000", inst2.InstanceID, "Instance ID should be same")

	// Verify only one instance in database
	var count int64
	err = db.Model(&models.HermesInstance{}).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Should have exactly one instance")
}

// TestProjectRegistration validates project registration with instance identity
func TestProjectRegistration(t *testing.T) {
	// Setup database (cleans automatically)
	db := setupTestDatabase(t)

	// Reset instance state (important for test isolation)
	instance.ResetForTesting()

	// Initialize instance
	cfg := &config.Config{
		BaseURL: "http://localhost:8000",
		Server:  &config.Server{},
	}
	logger := hclog.NewNullLogger()
	ctx := context.Background()

	err := instance.Initialize(ctx, db, cfg, logger)
	require.NoError(t, err)

	inst := instance.GetCurrentInstance()
	require.NotNil(t, inst)

	// Create a test project config
	projectCfg := &projectconfig.Project{
		Name:         "test-project",
		Title:        "Test Project",
		FriendlyName: "Test Project",
		ShortName:    "TP",
		Description:  "Test project for instance identity",
		Status:       "active",
		Providers: []*projectconfig.Provider{
			{
				Type:          "local",
				WorkspacePath: "/tmp/test",
			},
		},
	}

	// Register project
	project, err := projects.RegisterProject(ctx, db, projectCfg, logger)
	require.NoError(t, err, "Project registration should succeed")
	require.NotNil(t, project)

	// Verify project fields
	assert.Equal(t, "test-project", project.Name)
	assert.NotNil(t, project.InstanceUUID, "Project should have instance UUID")
	assert.Equal(t, inst.InstanceUUID, *project.InstanceUUID, "Project should be linked to instance")
	assert.NotNil(t, project.ProjectUUID, "Project UUID should be generated")
	assert.NotNil(t, project.GlobalProjectID, "Global project ID should be generated")

	// Verify global project ID format: instance_uuid/project_name
	expectedGlobalID := inst.InstanceUUID.String() + "/test-project"
	assert.Equal(t, expectedGlobalID, *project.GlobalProjectID, "Global project ID should have correct format")

	// Verify config hash was calculated
	assert.NotNil(t, project.ConfigHash, "Config hash should be calculated")

	// Verify can retrieve project by composite key
	retrieved, err := models.GetWorkspaceProjectByInstanceAndName(db, inst.InstanceUUID, "test-project")
	require.NoError(t, err)
	assert.Equal(t, project.ProjectUUID, retrieved.ProjectUUID)

	// Verify can retrieve by project UUID
	retrieved2, err := models.GetWorkspaceProjectByUUID(db, *project.ProjectUUID)
	require.NoError(t, err)
	assert.Equal(t, "test-project", retrieved2.Name)

	// Verify can retrieve by global project ID
	retrieved3, err := models.GetWorkspaceProjectByGlobalID(db, *project.GlobalProjectID)
	require.NoError(t, err)
	assert.Equal(t, project.ProjectUUID, retrieved3.ProjectUUID)
}

// TestProjectClaiming validates that unclaimed projects are properly claimed
func TestProjectClaiming(t *testing.T) {
	// Setup database (cleans automatically)
	db := setupTestDatabase(t)

	// Reset instance state (important for test isolation)
	instance.ResetForTesting()

	// Initialize instance
	cfg := &config.Config{
		BaseURL: "http://localhost:8000",
		Server:  &config.Server{},
	}
	logger := hclog.NewNullLogger()
	ctx := context.Background()

	err := instance.Initialize(ctx, db, cfg, logger)
	require.NoError(t, err)

	inst := instance.GetCurrentInstance()
	require.NotNil(t, inst)

	// Create an unclaimed project (simulating SyncToDatabase behavior)
	unclaimedProjectUUID := uuid.New()
	unclaimedProject := &models.WorkspaceProject{
		Name:          "unclaimed-project",
		Title:         "Unclaimed Project",
		FriendlyName:  "Unclaimed",
		ShortName:     "UP",
		Status:        "active",
		ProjectUUID:   &unclaimedProjectUUID,
		ProvidersJSON: `{"providers":[]}`, // Must have valid JSON for JSONB column
		MetadataJSON:  `{}`,               // Must have valid JSON for JSONB column
		// Note: InstanceUUID is nil
	}
	err = db.Create(unclaimedProject).Error
	require.NoError(t, err)

	// Verify project has no instance UUID
	var checkProject models.WorkspaceProject
	err = db.Where("name = ?", "unclaimed-project").First(&checkProject).Error
	require.NoError(t, err)
	assert.Nil(t, checkProject.InstanceUUID, "Project should not have instance UUID initially")
	assert.Nil(t, checkProject.GlobalProjectID, "Project should not have global project ID initially")

	// Now register the same project through RegisterProject
	projectCfg := &projectconfig.Project{
		Name:         "unclaimed-project",
		Title:        "Unclaimed Project Updated",
		FriendlyName: "Unclaimed Updated",
		ShortName:    "UP",
		Status:       "active",
		Providers: []*projectconfig.Provider{
			{
				Type:          "local",
				WorkspacePath: "/tmp/unclaimed",
			},
		},
	}

	// Register should claim the existing project
	claimed, err := projects.RegisterProject(ctx, db, projectCfg, logger)
	require.NoError(t, err, "Project claiming should succeed")
	require.NotNil(t, claimed)

	// Verify project was claimed (not created new)
	assert.Equal(t, unclaimedProjectUUID, *claimed.ProjectUUID, "Should reuse existing project UUID")
	assert.NotNil(t, claimed.InstanceUUID, "Claimed project should have instance UUID")
	assert.Equal(t, inst.InstanceUUID, *claimed.InstanceUUID, "Claimed project should link to current instance")
	assert.NotNil(t, claimed.GlobalProjectID, "Claimed project should have global project ID")

	// Verify only one project exists in database
	var count int64
	err = db.Model(&models.WorkspaceProject{}).Where("name = ?", "unclaimed-project").Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Should have exactly one project (not duplicated)")
}

// TestMultipleProjectRegistration validates registering multiple projects
func TestMultipleProjectRegistration(t *testing.T) {
	// Setup database (cleans automatically)
	db := setupTestDatabase(t)

	// Reset instance state (important for test isolation)
	instance.ResetForTesting()

	// Initialize instance
	cfg := &config.Config{
		BaseURL: "http://localhost:8000",
		Server:  &config.Server{},
	}
	logger := hclog.NewNullLogger()
	ctx := context.Background()

	err := instance.Initialize(ctx, db, cfg, logger)
	require.NoError(t, err)

	inst := instance.GetCurrentInstance()
	require.NotNil(t, inst)

	// Create multiple project configs
	projectConfigs := map[string]*projectconfig.Project{
		"project-a": {
			Name:         "project-a",
			Title:        "Project A",
			FriendlyName: "Project A",
			ShortName:    "PA",
			Status:       "active",
			Providers: []*projectconfig.Provider{
				{Type: "local", WorkspacePath: "/tmp/a"},
			},
		},
		"project-b": {
			Name:         "project-b",
			Title:        "Project B",
			FriendlyName: "Project B",
			ShortName:    "PB",
			Status:       "active",
			Providers: []*projectconfig.Provider{
				{Type: "local", WorkspacePath: "/tmp/b"},
			},
		},
		"project-c": {
			Name:         "project-c",
			Title:        "Project C",
			FriendlyName: "Project C",
			ShortName:    "PC",
			Status:       "active",
			Providers: []*projectconfig.Provider{
				{Type: "local", WorkspacePath: "/tmp/c"},
			},
		},
	}

	projectConfig := &projectconfig.Config{
		Projects: projectConfigs,
	}

	// Register all projects
	err = projects.RegisterAllProjects(ctx, db, projectConfig, logger)
	require.NoError(t, err, "Registering all projects should succeed")

	// Verify all projects were registered
	var registeredProjects []models.WorkspaceProject
	err = db.Where("instance_uuid = ?", inst.InstanceUUID).Find(&registeredProjects).Error
	require.NoError(t, err)
	assert.Equal(t, 3, len(registeredProjects), "Should have 3 registered projects")

	// Verify each project has correct instance linkage
	for _, project := range registeredProjects {
		assert.NotNil(t, project.InstanceUUID)
		assert.Equal(t, inst.InstanceUUID, *project.InstanceUUID)
		assert.NotNil(t, project.ProjectUUID)
		assert.NotNil(t, project.GlobalProjectID)
		assert.Contains(t, *project.GlobalProjectID, inst.InstanceUUID.String())
		assert.Contains(t, *project.GlobalProjectID, project.Name)
	}
}

// setupTestDatabase creates a database connection with fresh migrations
func setupTestDatabase(t *testing.T) *gorm.DB {
	if testDB == nil {
		t.Skip("Database not available, skipping test")
	}

	// Clean tables BEFORE test (important!)
	cleanupTestDatabase(t, testDB)

	// Re-run migrations
	err := testDB.AutoMigrate(
		&models.HermesInstance{},
		&models.WorkspaceProject{},
		&models.Document{},
		&models.DocumentRevision{},
	)
	require.NoError(t, err, "Failed to run migrations")

	return testDB
}

// cleanupTestDatabase cleans up test data
func cleanupTestDatabase(t *testing.T, db *gorm.DB) {
	// Delete in reverse dependency order
	tables := []struct {
		name  string
		model interface{}
	}{
		{"document_revisions", &models.DocumentRevision{}},
		{"documents", &models.Document{}},
		{"workspace_projects", &models.WorkspaceProject{}},
		{"hermes_instances", &models.HermesInstance{}},
	}

	for _, table := range tables {
		result := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(table.model)
		if result.Error != nil {
			t.Logf("Warning: failed to clean %s: %v", table.name, result.Error)
		} else if result.RowsAffected > 0 {
			t.Logf("Cleaned %d rows from %s", result.RowsAffected, table.name)
		}
	}
}
