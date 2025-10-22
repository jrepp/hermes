package projectconfig

import (
	"fmt"
	"time"
)

// Provider state constants
const (
	// ProviderStateActive indicates this provider is actively serving read/write operations
	ProviderStateActive = "active"
	// ProviderStateSource indicates this provider is the source in a migration (read-only)
	ProviderStateSource = "source"
	// ProviderStateTarget indicates this provider is the target in a migration (write destination)
	ProviderStateTarget = "target"
	// ProviderStateArchived indicates this provider is archived (no operations)
	ProviderStateArchived = "archived"
)

// Project status constants
const (
	ProjectStatusActive    = "active"
	ProjectStatusArchived  = "archived"
	ProjectStatusCompleted = "completed"
)

// Provider type constants
const (
	ProviderTypeLocal        = "local"
	ProviderTypeGoogle       = "google"
	ProviderTypeRemoteHermes = "remote-hermes"
)

// Config represents the top-level projects configuration
type Config struct {
	Version           string    `hcl:"version"`
	ConfigDir         string    `hcl:"config_dir"`
	WorkspaceBasePath string    `hcl:"workspace_base_path"`
	Defaults          *Defaults `hcl:"defaults,block"`
	Projects          map[string]*Project
}

// Defaults contains default settings for providers
type Defaults struct {
	Local *LocalDefaults `hcl:"local,block"`
}

// LocalDefaults contains default settings for local providers
type LocalDefaults struct {
	IndexingEnabled bool   `hcl:"indexing_enabled,optional"`
	GitBranch       string `hcl:"git_branch,optional"`
}

// Project represents a single project configuration
type Project struct {
	Name         string      `hcl:"name,label"`
	Title        string      `hcl:"title"`
	FriendlyName string      `hcl:"friendly_name"`
	ShortName    string      `hcl:"short_name"`
	Description  string      `hcl:"description,optional"`
	Status       string      `hcl:"status"`
	Providers    []*Provider `hcl:"provider,block"`
	Metadata     *Metadata   `hcl:"metadata,block"`
}

// Provider represents a workspace provider configuration
type Provider struct {
	Type            string `hcl:"type,label"`
	MigrationStatus string `hcl:"migration_status,optional"`

	// Local provider config
	WorkspacePath string          `hcl:"workspace_path,optional"`
	Git           *GitConfig      `hcl:"git,block"`
	Indexing      *IndexingConfig `hcl:"indexing,block"`

	// Google provider config
	WorkspaceID         string   `hcl:"workspace_id,optional"`
	ServiceAccountEmail string   `hcl:"service_account_email,optional"`
	CredentialsPath     string   `hcl:"credentials_path,optional"`
	SharedDriveIDs      []string `hcl:"shared_drive_ids,optional"`

	// Remote Hermes provider config
	HermesURL      string          `hcl:"hermes_url,optional"`
	APIVersion     string          `hcl:"api_version,optional"`
	Authentication *Authentication `hcl:"authentication,block"`
	SyncMode       string          `hcl:"sync_mode,optional"`
	CacheTTL       int             `hcl:"cache_ttl,optional"`
	ProjectFilter  []string        `hcl:"project_filter,optional"`
}

// GitConfig represents Git repository configuration
type GitConfig struct {
	Repository string `hcl:"repository,optional"`
	Branch     string `hcl:"branch,optional"`
}

// IndexingConfig represents indexing configuration
type IndexingConfig struct {
	Enabled           bool     `hcl:"enabled,optional"`
	AllowedExtensions []string `hcl:"allowed_extensions,optional"`
	PublicReadAccess  bool     `hcl:"public_read_access,optional"`
}

// Authentication represents authentication configuration for remote providers
type Authentication struct {
	Method        string `hcl:"method,optional"`
	ClientID      string `hcl:"client_id,optional"`
	ClientSecret  string `hcl:"client_secret,optional"`
	TokenEndpoint string `hcl:"token_endpoint,optional"`
}

// Metadata represents project metadata
type Metadata struct {
	CreatedAt time.Time `hcl:"created_at,optional"`
	Owner     string    `hcl:"owner,optional"`
	Tags      []string  `hcl:"tags,optional"`
	Notes     string    `hcl:"notes,optional"`
}

// GetProject returns a project by name
func (c *Config) GetProject(name string) (*Project, error) {
	project, ok := c.Projects[name]
	if !ok {
		return nil, fmt.Errorf("project %q not found", name)
	}
	return project, nil
}

// ListProjects returns all project names
func (c *Config) ListProjects() []string {
	names := make([]string, 0, len(c.Projects))
	for name := range c.Projects {
		names = append(names, name)
	}
	return names
}

// GetActiveProjects returns all projects with status "active"
func (c *Config) GetActiveProjects() []*Project {
	active := make([]*Project, 0)
	for _, project := range c.Projects {
		if project.Status == "active" {
			active = append(active, project)
		}
	}
	return active
}

// GetProvider returns a provider by type
func (p *Project) GetProvider(providerType string) (*Provider, error) {
	for _, provider := range p.Providers {
		if provider.Type == providerType {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("provider %q not found in project %q", providerType, p.Name)
}

// GetActiveProvider returns the provider with migration_status "active" (or empty, which defaults to active)
// This is the provider that should be used for all read/write operations in non-migration scenarios.
func (p *Project) GetActiveProvider() (*Provider, error) {
	for _, provider := range p.Providers {
		if provider.MigrationStatus == ProviderStateActive || provider.MigrationStatus == "" {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no active provider found in project %q", p.Name)
}

// GetSourceProvider returns the provider with migration_status "source"
// This is the read-only provider during migration (data is being read from here).
func (p *Project) GetSourceProvider() (*Provider, error) {
	for _, provider := range p.Providers {
		if provider.MigrationStatus == ProviderStateSource {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no source provider found in project %q", p.Name)
}

// GetTargetProvider returns the provider with migration_status "target"
// This is the write destination during migration (data is being written here).
func (p *Project) GetTargetProvider() (*Provider, error) {
	for _, provider := range p.Providers {
		if provider.MigrationStatus == ProviderStateTarget {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no target provider found in project %q", p.Name)
}

// GetProvidersByState returns all providers with the specified migration status
func (p *Project) GetProvidersByState(state string) []*Provider {
	providers := make([]*Provider, 0)
	for _, provider := range p.Providers {
		if provider.MigrationStatus == state {
			providers = append(providers, provider)
		}
	}
	return providers
}

// GetPrimaryProvider returns the provider that should be used for operations.
// During migration, returns the target provider. Otherwise, returns the active provider.
func (p *Project) GetPrimaryProvider() (*Provider, error) {
	// If in migration, use target provider (write destination)
	if p.IsInMigration() {
		targetProvider, err := p.GetTargetProvider()
		if err == nil {
			return targetProvider, nil
		}
	}

	// Otherwise use active provider
	return p.GetActiveProvider()
}

// IsActive returns true if the project status is "active"
func (p *Project) IsActive() bool {
	return p.Status == ProjectStatusActive
}

// IsArchived returns true if the project status is "archived"
func (p *Project) IsArchived() bool {
	return p.Status == ProjectStatusArchived
}

// IsCompleted returns true if the project status is "completed"
func (p *Project) IsCompleted() bool {
	return p.Status == ProjectStatusCompleted
}

// IsInMigration returns true if the project has both a source and target provider
func (p *Project) IsInMigration() bool {
	hasSource := false
	hasTarget := false

	for _, provider := range p.Providers {
		if provider.MigrationStatus == ProviderStateSource {
			hasSource = true
		}
		if provider.MigrationStatus == ProviderStateTarget {
			hasTarget = true
		}
	}

	return hasSource && hasTarget
}

// ProjectSummary represents a sanitized project for API responses
type ProjectSummary struct {
	Name         string             `json:"name"`
	Title        string             `json:"title"`
	FriendlyName string             `json:"friendly_name"`
	ShortName    string             `json:"short_name"`
	Description  string             `json:"description"`
	Status       string             `json:"status"`
	IsActive     bool               `json:"is_active"`
	IsArchived   bool               `json:"is_archived"`
	IsCompleted  bool               `json:"is_completed"`
	InMigration  bool               `json:"in_migration"`
	Providers    []*ProviderSummary `json:"providers"`
	Metadata     *Metadata          `json:"metadata,omitempty"`
}

// ProviderSummary represents a sanitized provider for API responses
type ProviderSummary struct {
	Type              string   `json:"type"`
	State             string   `json:"state"` // active, source, target, archived
	Role              string   `json:"role"`  // Human-readable role description
	WorkspacePath     string   `json:"workspace_path,omitempty"`
	WorkspaceID       string   `json:"workspace_id,omitempty"`
	HermesURL         string   `json:"hermes_url,omitempty"`
	APIVersion        string   `json:"api_version,omitempty"`
	GitRepository     string   `json:"git_repository,omitempty"`
	GitBranch         string   `json:"git_branch,omitempty"`
	IndexingEnabled   bool     `json:"indexing_enabled"`
	HasAuthentication bool     `json:"has_authentication"`
	SharedDriveIDs    []string `json:"shared_drive_ids,omitempty"`
}

// ToSummary returns a sanitized project summary safe for API responses
// All sensitive data (credentials, service accounts, etc.) is excluded
func (p *Project) ToSummary() *ProjectSummary {
	summary := &ProjectSummary{
		Name:         p.Name,
		Title:        p.Title,
		FriendlyName: p.FriendlyName,
		ShortName:    p.ShortName,
		Description:  p.Description,
		Status:       p.Status,
		IsActive:     p.IsActive(),
		IsArchived:   p.IsArchived(),
		IsCompleted:  p.IsCompleted(),
		InMigration:  p.IsInMigration(),
		Metadata:     p.Metadata,
		Providers:    make([]*ProviderSummary, 0, len(p.Providers)),
	}

	for _, provider := range p.Providers {
		providerSummary := &ProviderSummary{
			Type:              provider.Type,
			State:             provider.GetState(),
			Role:              provider.GetRole(),
			HasAuthentication: provider.Authentication != nil,
		}

		// Add non-sensitive provider-specific fields
		if provider.IsLocal() {
			providerSummary.WorkspacePath = provider.WorkspacePath
			if provider.Git != nil {
				providerSummary.GitRepository = provider.Git.Repository
				providerSummary.GitBranch = provider.Git.Branch
			}
			if provider.Indexing != nil {
				providerSummary.IndexingEnabled = provider.Indexing.Enabled
			}
		}

		if provider.IsGoogle() {
			providerSummary.WorkspaceID = provider.WorkspaceID
			providerSummary.SharedDriveIDs = provider.SharedDriveIDs
		}

		if provider.IsRemoteHermes() {
			providerSummary.HermesURL = provider.HermesURL
			providerSummary.APIVersion = provider.APIVersion
		}

		summary.Providers = append(summary.Providers, providerSummary)
	}

	return summary
}

// GetAllProjectSummaries returns sanitized summaries for all projects
func (c *Config) GetAllProjectSummaries() []*ProjectSummary {
	summaries := make([]*ProjectSummary, 0, len(c.Projects))
	for _, project := range c.Projects {
		summaries = append(summaries, project.ToSummary())
	}
	return summaries
}

// GetActiveProjectSummaries returns sanitized summaries for active projects only
func (c *Config) GetActiveProjectSummaries() []*ProjectSummary {
	summaries := make([]*ProjectSummary, 0)
	for _, project := range c.Projects {
		if project.IsActive() {
			summaries = append(summaries, project.ToSummary())
		}
	}
	return summaries
} // ResolveWorkspacePath resolves the full workspace path for a provider
func (p *Provider) ResolveWorkspacePath(basePath string) string {
	if p.WorkspacePath == "" {
		return basePath
	}
	// If workspace_path is relative, join with base path
	if p.WorkspacePath[0] != '/' {
		return fmt.Sprintf("%s/%s", basePath, p.WorkspacePath)
	}
	// If workspace_path is absolute, use as-is
	return p.WorkspacePath
}

// IsLocal returns true if this is a local provider
func (p *Provider) IsLocal() bool {
	return p.Type == ProviderTypeLocal
}

// IsGoogle returns true if this is a Google provider
func (p *Provider) IsGoogle() bool {
	return p.Type == ProviderTypeGoogle
}

// IsRemoteHermes returns true if this is a remote Hermes provider
func (p *Provider) IsRemoteHermes() bool {
	return p.Type == ProviderTypeRemoteHermes
}

// GetState returns the provider's state (active, source, target, archived)
// If migration_status is empty, defaults to "active"
func (p *Provider) GetState() string {
	if p.MigrationStatus == "" {
		return ProviderStateActive
	}
	return p.MigrationStatus
}

// IsActiveState returns true if this provider is in active state
func (p *Provider) IsActiveState() bool {
	return p.GetState() == ProviderStateActive
}

// IsSourceState returns true if this provider is a migration source
func (p *Provider) IsSourceState() bool {
	return p.GetState() == ProviderStateSource
}

// IsTargetState returns true if this provider is a migration target
func (p *Provider) IsTargetState() bool {
	return p.GetState() == ProviderStateTarget
}

// IsArchivedState returns true if this provider is archived
func (p *Provider) IsArchivedState() bool {
	return p.GetState() == ProviderStateArchived
}

// GetRole returns a human-readable description of the provider's role
func (p *Provider) GetRole() string {
	switch p.GetState() {
	case ProviderStateActive:
		return "Active (read/write)"
	case ProviderStateSource:
		return "Migration source (read-only)"
	case ProviderStateTarget:
		return "Migration target (write destination)"
	case ProviderStateArchived:
		return "Archived (no operations)"
	default:
		return "Unknown"
	}
}

// Sanitize returns a copy of the provider with sensitive fields removed
// This is safe to include in API responses
func (p *Provider) Sanitize() *Provider {
	sanitized := &Provider{
		Type:            p.Type,
		MigrationStatus: p.MigrationStatus,
		WorkspacePath:   p.WorkspacePath,
		SyncMode:        p.SyncMode,
		CacheTTL:        p.CacheTTL,
		ProjectFilter:   p.ProjectFilter,
		HermesURL:       p.HermesURL,
		APIVersion:      p.APIVersion,
	}

	// Copy non-sensitive Git config
	if p.Git != nil {
		sanitized.Git = &GitConfig{
			Repository: p.Git.Repository,
			Branch:     p.Git.Branch,
		}
	}

	// Copy indexing config (no secrets)
	if p.Indexing != nil {
		sanitized.Indexing = &IndexingConfig{
			Enabled:           p.Indexing.Enabled,
			AllowedExtensions: p.Indexing.AllowedExtensions,
			PublicReadAccess:  p.Indexing.PublicReadAccess,
		}
	}

	// Indicate presence of authentication without exposing credentials
	if p.Authentication != nil {
		sanitized.Authentication = &Authentication{
			Method: p.Authentication.Method,
			// Do NOT include: ClientID, ClientSecret, TokenEndpoint
		}
	}

	// For Google provider, indicate presence of credentials without exposing them
	if p.IsGoogle() {
		// WorkspaceID is not sensitive, can be included
		sanitized.WorkspaceID = p.WorkspaceID
		// Do NOT include: ServiceAccountEmail, CredentialsPath
		// SharedDriveIDs are not sensitive, can be included
		sanitized.SharedDriveIDs = p.SharedDriveIDs
	}

	return sanitized
}
