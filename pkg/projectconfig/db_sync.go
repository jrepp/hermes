package projectconfig

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// SyncToDatabase synchronizes workspace projects from the HCL configuration
// to the database. This should be called on server startup.
// It performs upsert operations to create or update projects based on their name.
func (c *Config) SyncToDatabase(db *gorm.DB, sourcePath string) error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	now := time.Now()
	syncedCount := 0
	errors := []error{}

	for name, project := range c.Projects {
		wp := &models.WorkspaceProject{
			Name:          name,
			Title:         project.Title,
			FriendlyName:  project.FriendlyName,
			ShortName:     project.ShortName,
			Status:        project.Status,
			SourceType:    models.WorkspaceProjectSourceHCLFile,
			ConfigVersion: c.Version,
		}

		// Set optional description
		if project.Description != "" {
			wp.Description = &project.Description
		}

		// Set source identifier
		if sourcePath != "" {
			wp.SourceIdentifier = &sourcePath
		}

		// Set last synced time
		wp.LastSyncedAt = &now

		// Serialize providers
		if err := wp.SetProviders(convertProvidersToData(project.Providers)); err != nil {
			errors = append(errors, fmt.Errorf("error serializing providers for project %q: %w", name, err))
			continue
		}

		// Serialize metadata
		if project.Metadata != nil {
			metadataData := &models.MetadataData{
				Owner: project.Metadata.Owner,
				Tags:  project.Metadata.Tags,
				Notes: project.Metadata.Notes,
			}
			if !project.Metadata.CreatedAt.IsZero() {
				metadataData.CreatedAt = &project.Metadata.CreatedAt
			}
			if err := wp.SetMetadata(metadataData); err != nil {
				errors = append(errors, fmt.Errorf("error serializing metadata for project %q: %w", name, err))
				continue
			}
		}

		// Upsert the workspace project
		if err := wp.Upsert(db); err != nil {
			errors = append(errors, fmt.Errorf("error upserting project %q: %w", name, err))
			continue
		}

		syncedCount++
	}

	if len(errors) > 0 {
		// Return first error but log that there were multiple
		return fmt.Errorf("synced %d projects with %d errors: %w", syncedCount, len(errors), errors[0])
	}

	return nil
}

// convertProvidersToData converts project providers to the database model format.
func convertProvidersToData(providers []*Provider) *models.ProvidersData {
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
		case ProviderTypeLocal:
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

		case ProviderTypeGoogle:
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

		case ProviderTypeRemoteHermes:
			if p.HermesURL != "" {
				providerData.Config["hermes_url"] = p.HermesURL
			}
			if p.APIVersion != "" {
				providerData.Config["api_version"] = p.APIVersion
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
			if p.Authentication != nil {
				providerData.Config["authentication"] = map[string]interface{}{
					"method":         p.Authentication.Method,
					"client_id":      p.Authentication.ClientID,
					"client_secret":  p.Authentication.ClientSecret,
					"token_endpoint": p.Authentication.TokenEndpoint,
				}
			}
		}

		data.Providers = append(data.Providers, providerData)
	}

	return data
}

// LoadFromDatabase loads workspace projects from the database and converts them
// back to a Config structure. This allows runtime queries to use the same
// projectconfig.Config interface.
func LoadFromDatabase(db *gorm.DB) (*Config, error) {
	projects, err := models.GetAllActiveWorkspaceProjects(db)
	if err != nil {
		return nil, fmt.Errorf("error loading workspace projects from database: %w", err)
	}

	config := &Config{
		Version:  "1.0", // Default version
		Projects: make(map[string]*Project),
	}

	for _, wp := range projects {
		project := &Project{
			Name:         wp.Name,
			Title:        wp.Title,
			FriendlyName: wp.FriendlyName,
			ShortName:    wp.ShortName,
			Status:       wp.Status,
		}

		if wp.Description != nil {
			project.Description = *wp.Description
		}

		// Deserialize providers
		providersData, err := wp.GetProviders()
		if err != nil {
			return nil, fmt.Errorf("error deserializing providers for project %q: %w", wp.Name, err)
		}
		project.Providers = convertDataToProviders(providersData)

		// Deserialize metadata
		metadataData, err := wp.GetMetadata()
		if err != nil {
			return nil, fmt.Errorf("error deserializing metadata for project %q: %w", wp.Name, err)
		}
		if metadataData != nil {
			project.Metadata = &Metadata{
				Owner: metadataData.Owner,
				Tags:  metadataData.Tags,
				Notes: metadataData.Notes,
			}
			if metadataData.CreatedAt != nil {
				project.Metadata.CreatedAt = *metadataData.CreatedAt
			}
		}

		config.Projects[wp.Name] = project
	}

	return config, nil
}

// convertDataToProviders converts database provider data back to Provider structs.
func convertDataToProviders(data *models.ProvidersData) []*Provider {
	providers := make([]*Provider, 0, len(data.Providers))

	for _, pd := range data.Providers {
		provider := &Provider{
			Type:            pd.Type,
			MigrationStatus: pd.MigrationStatus,
		}

		// Deserialize provider-specific config
		switch pd.Type {
		case ProviderTypeLocal:
			if val, ok := pd.Config["workspace_path"].(string); ok {
				provider.WorkspacePath = val
			}
			if gitMap, ok := pd.Config["git"].(map[string]interface{}); ok {
				provider.Git = &GitConfig{}
				if repo, ok := gitMap["repository"].(string); ok {
					provider.Git.Repository = repo
				}
				if branch, ok := gitMap["branch"].(string); ok {
					provider.Git.Branch = branch
				}
			}
			if indexingMap, ok := pd.Config["indexing"].(map[string]interface{}); ok {
				provider.Indexing = &IndexingConfig{}
				if enabled, ok := indexingMap["enabled"].(bool); ok {
					provider.Indexing.Enabled = enabled
				}
				if exts, ok := indexingMap["allowed_extensions"].([]interface{}); ok {
					provider.Indexing.AllowedExtensions = interfaceSliceToStringSlice(exts)
				}
				if publicRead, ok := indexingMap["public_read_access"].(bool); ok {
					provider.Indexing.PublicReadAccess = publicRead
				}
			}

		case ProviderTypeGoogle:
			if val, ok := pd.Config["workspace_id"].(string); ok {
				provider.WorkspaceID = val
			}
			if val, ok := pd.Config["service_account_email"].(string); ok {
				provider.ServiceAccountEmail = val
			}
			if val, ok := pd.Config["credentials_path"].(string); ok {
				provider.CredentialsPath = val
			}
			if drives, ok := pd.Config["shared_drive_ids"].([]interface{}); ok {
				provider.SharedDriveIDs = interfaceSliceToStringSlice(drives)
			}

		case ProviderTypeRemoteHermes:
			if val, ok := pd.Config["hermes_url"].(string); ok {
				provider.HermesURL = val
			}
			if val, ok := pd.Config["api_version"].(string); ok {
				provider.APIVersion = val
			}
			if val, ok := pd.Config["sync_mode"].(string); ok {
				provider.SyncMode = val
			}
			if val, ok := pd.Config["cache_ttl"].(float64); ok {
				provider.CacheTTL = int(val)
			}
			if filter, ok := pd.Config["project_filter"].([]interface{}); ok {
				provider.ProjectFilter = interfaceSliceToStringSlice(filter)
			}
			if authMap, ok := pd.Config["authentication"].(map[string]interface{}); ok {
				provider.Authentication = &Authentication{}
				if method, ok := authMap["method"].(string); ok {
					provider.Authentication.Method = method
				}
				if clientID, ok := authMap["client_id"].(string); ok {
					provider.Authentication.ClientID = clientID
				}
				if clientSecret, ok := authMap["client_secret"].(string); ok {
					provider.Authentication.ClientSecret = clientSecret
				}
				if tokenEndpoint, ok := authMap["token_endpoint"].(string); ok {
					provider.Authentication.TokenEndpoint = tokenEndpoint
				}
			}
		}

		providers = append(providers, provider)
	}

	return providers
}

// interfaceSliceToStringSlice converts []interface{} to []string.
func interfaceSliceToStringSlice(in []interface{}) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// GetWorkspaceProjectSummary returns a simplified summary of a workspace project
// suitable for API responses.
func GetWorkspaceProjectSummary(wp *models.WorkspaceProject) (*ProjectSummary, error) {
	summary := &ProjectSummary{
		Name:         wp.Name,
		Title:        wp.Title,
		FriendlyName: wp.FriendlyName,
		ShortName:    wp.ShortName,
		Status:       wp.Status,
		IsActive:     wp.Status == ProjectStatusActive,
		IsArchived:   wp.Status == ProjectStatusArchived,
		IsCompleted:  wp.Status == ProjectStatusCompleted,
		Providers:    []*ProviderSummary{},
	}

	// Add description if present
	if wp.Description != nil {
		summary.Description = *wp.Description
	}

	// Parse metadata
	metadataData, err := wp.GetMetadata()
	if err != nil {
		return nil, fmt.Errorf("error parsing metadata: %w", err)
	}
	if metadataData != nil {
		summary.Metadata = &Metadata{
			Owner: metadataData.Owner,
			Tags:  metadataData.Tags,
			Notes: metadataData.Notes,
		}
		if metadataData.CreatedAt != nil {
			summary.Metadata.CreatedAt = *metadataData.CreatedAt
		}
	}

	// Parse providers
	providersData, err := wp.GetProviders()
	if err != nil {
		return nil, fmt.Errorf("error parsing providers: %w", err)
	}

	inMigration := false
	for _, pd := range providersData.Providers {
		providerSummary := &ProviderSummary{
			Type:  pd.Type,
			State: determineProviderState(pd.MigrationStatus),
			Role:  describeProviderRole(pd.MigrationStatus),
		}

		// Check if any provider is in migration
		if pd.MigrationStatus == ProviderStateSource || pd.MigrationStatus == ProviderStateTarget {
			inMigration = true
		}

		// Populate type-specific fields (excluding sensitive data)
		switch pd.Type {
		case ProviderTypeLocal:
			if path, ok := pd.Config["workspace_path"].(string); ok {
				providerSummary.WorkspacePath = path
			}
			if gitMap, ok := pd.Config["git"].(map[string]interface{}); ok {
				if repo, ok := gitMap["repository"].(string); ok {
					providerSummary.GitRepository = repo
				}
				if branch, ok := gitMap["branch"].(string); ok {
					providerSummary.GitBranch = branch
				}
			}
			if indexingMap, ok := pd.Config["indexing"].(map[string]interface{}); ok {
				if enabled, ok := indexingMap["enabled"].(bool); ok {
					providerSummary.IndexingEnabled = enabled
				}
			}

		case ProviderTypeGoogle:
			if wsID, ok := pd.Config["workspace_id"].(string); ok {
				providerSummary.WorkspaceID = wsID
			}
			if drives, ok := pd.Config["shared_drive_ids"].([]interface{}); ok {
				providerSummary.SharedDriveIDs = interfaceSliceToStringSlice(drives)
			}
			// Do NOT expose: service_account_email, credentials_path

		case ProviderTypeRemoteHermes:
			if url, ok := pd.Config["hermes_url"].(string); ok {
				providerSummary.HermesURL = url
			}
			if version, ok := pd.Config["api_version"].(string); ok {
				providerSummary.APIVersion = version
			}
			if authMap, ok := pd.Config["authentication"].(map[string]interface{}); ok {
				if method, ok := authMap["method"].(string); ok && method != "" {
					providerSummary.HasAuthentication = true
				}
			}
			// Do NOT expose: authentication credentials
		}

		summary.Providers = append(summary.Providers, providerSummary)
	}

	summary.InMigration = inMigration

	return summary, nil
}

// determineProviderState maps migration_status to provider state
func determineProviderState(migrationStatus string) string {
	if migrationStatus == "" {
		return ProviderStateActive
	}
	return migrationStatus
}

// describeProviderRole returns a human-readable role description
func describeProviderRole(migrationStatus string) string {
	switch migrationStatus {
	case ProviderStateActive:
		return "Active (read/write)"
	case ProviderStateSource:
		return "Migration source (read-only)"
	case ProviderStateTarget:
		return "Migration target (write destination)"
	case ProviderStateArchived:
		return "Archived (no operations)"
	case "":
		return "Active (read/write)"
	default:
		return "Unknown"
	}
}

// GetAllActiveWorkspaceProjectsFromDB is a convenience wrapper for models.GetAllActiveWorkspaceProjects
func GetAllActiveWorkspaceProjectsFromDB(db *gorm.DB) ([]models.WorkspaceProject, error) {
	return models.GetAllActiveWorkspaceProjects(db)
}

// GetAllWorkspaceProjectsFromDB is a convenience wrapper for models.GetAllWorkspaceProjects
func GetAllWorkspaceProjectsFromDB(db *gorm.DB) ([]models.WorkspaceProject, error) {
	return models.GetAllWorkspaceProjects(db)
}

// GetWorkspaceProjectByNameFromDB retrieves a single workspace project by name
func GetWorkspaceProjectByNameFromDB(db *gorm.DB, name string) (*models.WorkspaceProject, error) {
	wp := &models.WorkspaceProject{}
	if err := wp.GetByName(db, name); err != nil {
		return nil, err
	}
	return wp, nil
}
