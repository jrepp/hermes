package projectconfig

import (
	"fmt"
	"time"
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

// GetActiveProvider returns the provider with migration_status "active"
func (p *Project) GetActiveProvider() (*Provider, error) {
	for _, provider := range p.Providers {
		if provider.MigrationStatus == "active" || provider.MigrationStatus == "" {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no active provider found in project %q", p.Name)
}

// GetSourceProvider returns the provider with migration_status "source"
func (p *Project) GetSourceProvider() (*Provider, error) {
	for _, provider := range p.Providers {
		if provider.MigrationStatus == "source" {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no source provider found in project %q", p.Name)
}

// GetTargetProvider returns the provider with migration_status "target"
func (p *Project) GetTargetProvider() (*Provider, error) {
	for _, provider := range p.Providers {
		if provider.MigrationStatus == "target" {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no target provider found in project %q", p.Name)
}

// IsActive returns true if the project status is "active"
func (p *Project) IsActive() bool {
	return p.Status == "active"
}

// IsInMigration returns true if the project has multiple providers
func (p *Project) IsInMigration() bool {
	return len(p.Providers) > 1
}

// ResolveWorkspacePath resolves the full workspace path for a provider
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
	return p.Type == "local"
}

// IsGoogle returns true if this is a Google provider
func (p *Provider) IsGoogle() bool {
	return p.Type == "google"
}

// IsRemoteHermes returns true if this is a remote Hermes provider
func (p *Provider) IsRemoteHermes() bool {
	return p.Type == "remote-hermes"
}
