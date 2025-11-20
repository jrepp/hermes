package projects

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/instance"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

// RegisterProject registers a single project from config with the current instance.
// It creates or updates the workspace project, linking it to the instance and
// calculating a config hash for drift detection.
func RegisterProject(ctx context.Context, db *gorm.DB, cfg *projectconfig.Project, logger hclog.Logger) (*models.WorkspaceProject, error) {
	inst := instance.GetCurrentInstance()
	if inst == nil {
		return nil, fmt.Errorf("instance not initialized - call instance.Initialize first")
	}

	// Check if project already registered for this instance
	project, err := models.GetWorkspaceProjectByInstanceAndName(db, inst.InstanceUUID, cfg.Name)
	if err == nil {
		// Project exists with this instance - check for config changes
		newHash := calculateProjectConfigHash(cfg)
		if project.ConfigHash != nil && *project.ConfigHash != newHash {
			logger.Warn("Project configuration changed",
				"project_name", cfg.Name,
				"old_hash", *project.ConfigHash,
				"new_hash", newHash)

			// Update project config
			if err := updateProjectFromConfig(project, cfg, inst.InstanceUUID, newHash); err != nil {
				return nil, fmt.Errorf("failed to update project config: %w", err)
			}

			if err := db.Save(project).Error; err != nil {
				return nil, fmt.Errorf("failed to save updated project: %w", err)
			}

			logger.Info("Project configuration updated",
				"project_name", cfg.Name,
				"project_uuid", project.ProjectUUID)
		} else {
			logger.Debug("Project configuration unchanged",
				"project_name", cfg.Name,
				"project_uuid", project.ProjectUUID)
		}

		return project, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to query project: %w", err)
	}

	// Project not found with instance - check if it exists without instance (from SyncToDatabase)
	var unclaimedProject models.WorkspaceProject
	err = db.Where("name = ? AND instance_uuid IS NULL", cfg.Name).First(&unclaimedProject).Error
	if err == nil {
		// Found unclaimed project - link it to this instance
		logger.Info("Claiming existing project for instance",
			"project_name", cfg.Name,
			"project_uuid", unclaimedProject.ProjectUUID,
			"instance_uuid", inst.InstanceUUID)

		unclaimedProject.InstanceUUID = &inst.InstanceUUID
		newHash := calculateProjectConfigHash(cfg)
		unclaimedProject.ConfigHash = &newHash

		// Update global project ID
		if unclaimedProject.ProjectUUID != nil {
			globalID := fmt.Sprintf("%s/%s", inst.InstanceUUID.String(), cfg.Name)
			unclaimedProject.GlobalProjectID = &globalID
		}

		if err := db.Save(&unclaimedProject).Error; err != nil {
			return nil, fmt.Errorf("failed to claim project: %w", err)
		}

		logger.Info("Project claimed",
			"project_name", unclaimedProject.Name,
			"project_uuid", unclaimedProject.ProjectUUID,
			"global_project_id", unclaimedProject.GlobalProjectID,
			"instance_uuid", inst.InstanceUUID)

		return &unclaimedProject, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to query unclaimed project: %w", err)
	}

	// Create new project
	newHash := calculateProjectConfigHash(cfg)
	project = &models.WorkspaceProject{
		InstanceUUID: &inst.InstanceUUID,
		Name:         cfg.Name,
		Title:        cfg.Title,
		FriendlyName: cfg.FriendlyName,
		ShortName:    cfg.ShortName,
		Status:       cfg.Status,
		SourceType:   models.WorkspaceProjectSourceHCLFile,
		ConfigHash:   &newHash,
	}

	// Set optional description
	if cfg.Description != "" {
		project.Description = &cfg.Description
	}

	// Serialize providers
	providersData := convertProvidersToData(cfg.Providers)
	if err := project.SetProviders(providersData); err != nil {
		return nil, fmt.Errorf("failed to serialize providers: %w", err)
	}

	// Ensure ProvidersJSON is not empty (PostgreSQL doesn't accept empty string for JSONB)
	if project.ProvidersJSON == "" {
		project.ProvidersJSON = `{"providers":[]}`
	}

	logger.Debug("Creating project",
		"name", cfg.Name,
		"providers_json", project.ProvidersJSON,
		"metadata_json", project.MetadataJSON)

	// Serialize metadata
	if cfg.Metadata != nil {
		metadataData := &models.MetadataData{
			Owner: cfg.Metadata.Owner,
			Tags:  cfg.Metadata.Tags,
			Notes: cfg.Metadata.Notes,
		}
		if !cfg.Metadata.CreatedAt.IsZero() {
			metadataData.CreatedAt = &cfg.Metadata.CreatedAt
		}
		if err := project.SetMetadata(metadataData); err != nil {
			return nil, fmt.Errorf("failed to serialize metadata: %w", err)
		}
	}

	// Ensure MetadataJSON is not empty (PostgreSQL doesn't accept empty string for JSONB)
	if project.MetadataJSON == "" {
		project.MetadataJSON = `{}`
	}

	// Create in database (ProjectUUID and GlobalProjectID will be auto-generated)
	if err := db.Create(project).Error; err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	logger.Info("Project registered",
		"project_name", project.Name,
		"project_uuid", project.ProjectUUID,
		"global_project_id", project.GlobalProjectID,
		"instance_uuid", inst.InstanceUUID)

	return project, nil
}

// RegisterAllProjects registers all projects from config with the current instance.
func RegisterAllProjects(ctx context.Context, db *gorm.DB, cfg *projectconfig.Config, logger hclog.Logger) error {
	if cfg == nil {
		return fmt.Errorf("project config is nil")
	}

	inst := instance.GetCurrentInstance()
	if inst == nil {
		return fmt.Errorf("instance not initialized - call instance.Initialize first")
	}

	logger.Info("Registering workspace projects with instance",
		"instance_id", inst.InstanceID,
		"instance_uuid", inst.InstanceUUID,
		"project_count", len(cfg.Projects))

	successCount := 0
	errorCount := 0
	var firstError error

	for name, project := range cfg.Projects {
		_, err := RegisterProject(ctx, db, project, logger)
		if err != nil {
			errorCount++
			if firstError == nil {
				firstError = err
			}
			logger.Error("Failed to register project",
				"project_name", name,
				"error", err)
			continue
		}
		successCount++
	}

	logger.Info("Project registration complete",
		"success", successCount,
		"errors", errorCount,
		"total", len(cfg.Projects))

	if errorCount > 0 {
		return fmt.Errorf("registered %d/%d projects with %d errors: %w",
			successCount, len(cfg.Projects), errorCount, firstError)
	}

	return nil
}

// calculateProjectConfigHash computes SHA-256 hash of the project configuration.
// This is used for drift detection - if the config changes, the hash changes.
func calculateProjectConfigHash(cfg *projectconfig.Project) string {
	// Create a canonical representation of the config for hashing
	type hashData struct {
		Name         string
		Title        string
		FriendlyName string
		ShortName    string
		Description  string
		Status       string
		Providers    interface{}
		Metadata     interface{}
	}

	data := hashData{
		Name:         cfg.Name,
		Title:        cfg.Title,
		FriendlyName: cfg.FriendlyName,
		ShortName:    cfg.ShortName,
		Description:  cfg.Description,
		Status:       cfg.Status,
		Providers:    cfg.Providers,
		Metadata:     cfg.Metadata,
	}

	// Marshal to JSON for consistent hashing
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		// Use empty hash if marshaling fails
		return ""
	}
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}

// updateProjectFromConfig updates an existing project with new config values
func updateProjectFromConfig(project *models.WorkspaceProject, cfg *projectconfig.Project, instanceUUID uuid.UUID, configHash string) error {
	project.Title = cfg.Title
	project.FriendlyName = cfg.FriendlyName
	project.ShortName = cfg.ShortName
	project.Status = cfg.Status
	project.ConfigHash = &configHash

	if cfg.Description != "" {
		project.Description = &cfg.Description
	} else {
		project.Description = nil
	}

	// Update providers
	providersData := convertProvidersToData(cfg.Providers)
	if err := project.SetProviders(providersData); err != nil {
		return fmt.Errorf("failed to update providers: %w", err)
	}

	// Ensure ProvidersJSON is not empty (PostgreSQL doesn't accept empty string for JSONB)
	if project.ProvidersJSON == "" {
		project.ProvidersJSON = `{"providers":[]}`
	}

	// Update metadata
	if cfg.Metadata != nil {
		metadataData := &models.MetadataData{
			Owner: cfg.Metadata.Owner,
			Tags:  cfg.Metadata.Tags,
			Notes: cfg.Metadata.Notes,
		}
		if !cfg.Metadata.CreatedAt.IsZero() {
			metadataData.CreatedAt = &cfg.Metadata.CreatedAt
		}
		if err := project.SetMetadata(metadataData); err != nil {
			return fmt.Errorf("failed to update metadata: %w", err)
		}
	}

	// Ensure MetadataJSON is not empty (PostgreSQL doesn't accept empty string for JSONB)
	if project.MetadataJSON == "" {
		project.MetadataJSON = `{}`
	}

	return nil
}

// convertProvidersToData converts projectconfig providers to models providers format
func convertProvidersToData(providers []*projectconfig.Provider) *models.ProvidersData {
	data := &models.ProvidersData{
		Providers: make([]models.ProviderData, 0, len(providers)),
	}

	for _, p := range providers {
		providerData := models.ProviderData{
			Type:            p.Type,
			MigrationStatus: p.MigrationStatus,
			Config:          make(map[string]interface{}),
		}

		// Serialize provider-specific config based on type
		switch p.Type {
		case "local":
			if p.WorkspacePath != "" {
				providerData.Config["workspace_path"] = p.WorkspacePath
			}
			if p.Git != nil {
				providerData.Config["git"] = map[string]interface{}{
					"repository": p.Git.Repository,
					"branch":     p.Git.Branch,
				}
			}
			if p.Indexing != nil {
				providerData.Config["indexing"] = map[string]interface{}{
					"enabled":            p.Indexing.Enabled,
					"allowed_extensions": p.Indexing.AllowedExtensions,
					"public_read_access": p.Indexing.PublicReadAccess,
				}
			}

		case "google":
			if p.WorkspaceID != "" {
				providerData.Config["workspace_id"] = p.WorkspaceID
			}
			if p.ServiceAccountEmail != "" {
				providerData.Config["service_account_email"] = p.ServiceAccountEmail
			}
			if p.CredentialsPath != "" {
				providerData.Config["credentials_path"] = p.CredentialsPath
			}
			if len(p.SharedDriveIDs) > 0 {
				providerData.Config["shared_drive_ids"] = p.SharedDriveIDs
			}

		case "remote-hermes":
			if p.HermesURL != "" {
				providerData.Config["hermes_url"] = p.HermesURL
			}
			if p.APIVersion != "" {
				providerData.Config["api_version"] = p.APIVersion
			}
			if p.Authentication != nil {
				providerData.Config["authentication"] = map[string]interface{}{
					"method":         p.Authentication.Method,
					"client_id":      p.Authentication.ClientID,
					"client_secret":  p.Authentication.ClientSecret,
					"token_endpoint": p.Authentication.TokenEndpoint,
				}
			}
			if p.SyncMode != "" {
				providerData.Config["sync_mode"] = p.SyncMode
			}
			if p.CacheTTL > 0 {
				providerData.Config["cache_ttl"] = p.CacheTTL
			}
			if len(p.ProjectFilter) > 0 {
				providerData.Config["project_filter"] = p.ProjectFilter
			}
		}

		data.Providers = append(data.Providers, providerData)
	}

	return data
}
