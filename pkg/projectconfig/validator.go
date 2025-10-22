package projectconfig

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []*ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// Validator validates project configurations
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// Validate validates the entire configuration
func (v *Validator) Validate(config *Config) error {
	v.errors = make(ValidationErrors, 0)

	// Validate global config
	v.validateConfig(config)

	// Validate each project
	for _, project := range config.Projects {
		v.validateProject(project, config)
	}

	if len(v.errors) > 0 {
		return v.errors
	}

	return nil
}

// validateConfig validates the global configuration
func (v *Validator) validateConfig(config *Config) {
	if config.Version == "" {
		v.addError("version", "version is required")
	}

	if !isValidVersion(config.Version) {
		v.addError("version", "version must be in semver format (e.g., 1.0.0 or 1.0.0-alpha)")
	}

	if config.WorkspaceBasePath == "" {
		v.addError("workspace_base_path", "workspace_base_path is required")
	}
}

// validateProject validates a single project
func (v *Validator) validateProject(project *Project, config *Config) {
	prefix := fmt.Sprintf("project.%s", project.Name)

	// Validate name
	if project.Name == "" {
		v.addError(prefix+".name", "name is required")
	}

	if !isValidProjectName(project.Name) {
		v.addError(prefix+".name", "name must be lowercase alphanumeric with hyphens (kebab-case)")
	}

	// Validate title
	if project.Title == "" {
		v.addError(prefix+".title", "title is required")
	}

	// Validate short name
	if project.ShortName == "" {
		v.addError(prefix+".short_name", "short_name is required")
	}

	if !isValidShortName(project.ShortName) {
		v.addError(prefix+".short_name", "short_name must be 2-10 uppercase characters")
	}

	// Validate status
	if project.Status == "" {
		v.addError(prefix+".status", "status is required")
	}

	if !isValidStatus(project.Status) {
		v.addError(prefix+".status", "status must be one of: active, completed, archived")
	}

	// Validate providers
	if len(project.Providers) == 0 {
		v.addError(prefix+".providers", "at least one provider is required")
	}

	for i, provider := range project.Providers {
		v.validateProvider(provider, project, config, i)
	}

	// Check for migration consistency
	if project.IsInMigration() {
		v.validateMigration(project)
	}
}

// validateProvider validates a provider configuration
func (v *Validator) validateProvider(provider *Provider, project *Project, config *Config, index int) {
	prefix := fmt.Sprintf("project.%s.provider[%d]", project.Name, index)

	// Validate type
	if provider.Type == "" {
		v.addError(prefix+".type", "type is required")
	}

	if !isValidProviderType(provider.Type) {
		v.addError(prefix+".type", "type must be one of: local, google, remote-hermes")
	}

	// Validate migration status
	if provider.MigrationStatus != "" && !isValidMigrationStatus(provider.MigrationStatus) {
		v.addError(prefix+".migration_status", "migration_status must be one of: active, source, target, archived")
	}

	// Type-specific validation
	switch provider.Type {
	case "local":
		v.validateLocalProvider(provider, prefix, config)
	case "google":
		v.validateGoogleProvider(provider, prefix)
	case "remote-hermes":
		v.validateRemoteHermesProvider(provider, prefix)
	}
}

// validateLocalProvider validates a local provider
func (v *Validator) validateLocalProvider(provider *Provider, prefix string, config *Config) {
	if provider.WorkspacePath == "" {
		v.addError(prefix+".workspace_path", "workspace_path is required for local provider")
	}

	// Validate git config if present
	if provider.Git != nil {
		if provider.Git.Repository != "" && !isValidURL(provider.Git.Repository) {
			v.addError(prefix+".git.repository", "repository must be a valid URL")
		}
	}

	// Validate indexing config if present
	if provider.Indexing != nil {
		if len(provider.Indexing.AllowedExtensions) > 0 {
			for _, ext := range provider.Indexing.AllowedExtensions {
				if !isValidFileExtension(ext) {
					v.addError(prefix+".indexing.allowed_extensions", fmt.Sprintf("invalid extension: %s", ext))
				}
			}
		}
	}
}

// validateGoogleProvider validates a Google provider
func (v *Validator) validateGoogleProvider(provider *Provider, prefix string) {
	if provider.WorkspaceID == "" {
		v.addError(prefix+".workspace_id", "workspace_id is required for Google provider")
	}

	if provider.ServiceAccountEmail == "" {
		v.addError(prefix+".service_account_email", "service_account_email is required for Google provider")
	}

	if provider.CredentialsPath == "" {
		v.addError(prefix+".credentials_path", "credentials_path is required for Google provider")
	}

	// Check if credentials use env() function
	if !strings.Contains(provider.WorkspaceID, "env(") {
		v.addWarning(prefix+".workspace_id", "consider using env() function for sensitive data")
	}
}

// validateRemoteHermesProvider validates a remote Hermes provider
func (v *Validator) validateRemoteHermesProvider(provider *Provider, prefix string) {
	if provider.HermesURL == "" {
		v.addError(prefix+".hermes_url", "hermes_url is required for remote-hermes provider")
	}

	if !isValidURL(provider.HermesURL) {
		v.addError(prefix+".hermes_url", "hermes_url must be a valid URL")
	}

	if provider.APIVersion == "" {
		v.addError(prefix+".api_version", "api_version is required for remote-hermes provider")
	}

	if provider.APIVersion != "v1" && provider.APIVersion != "v2" {
		v.addError(prefix+".api_version", "api_version must be v1 or v2")
	}

	if provider.Authentication != nil {
		if provider.Authentication.Method != "" && !isValidAuthMethod(provider.Authentication.Method) {
			v.addError(prefix+".authentication.method", "method must be one of: oidc, bearer, api-key")
		}
	}
}

// validateMigration validates migration consistency
func (v *Validator) validateMigration(project *Project) {
	prefix := fmt.Sprintf("project.%s.migration", project.Name)

	hasSource := false
	hasTarget := false

	for _, provider := range project.Providers {
		if provider.MigrationStatus == "source" {
			hasSource = true
		}
		if provider.MigrationStatus == "target" {
			hasTarget = true
		}
	}

	if hasSource && !hasTarget {
		v.addError(prefix, "migration has source provider but no target provider")
	}

	if hasTarget && !hasSource {
		v.addError(prefix, "migration has target provider but no source provider")
	}
}

// Helper methods

func (v *Validator) addError(field, message string) {
	v.errors = append(v.errors, &ValidationError{
		Field:   field,
		Message: message,
	})
}

func (v *Validator) addWarning(field, message string) {
	// For now, warnings are just logged, not collected
	// In a full implementation, we'd have a separate warnings slice
	_ = field
	_ = message
}

// Validation helper functions

func isValidVersion(version string) bool {
	// Simplified semver check
	pattern := `^\d+\.\d+\.\d+(-[a-z]+)?$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched
}

func isValidProjectName(name string) bool {
	// Must be kebab-case: lowercase alphanumeric with hyphens
	pattern := `^[a-z0-9][a-z0-9-]*[a-z0-9]$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

func isValidShortName(name string) bool {
	// 2-10 uppercase characters
	pattern := `^[A-Z]{2,10}$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

func isValidStatus(status string) bool {
	validStatuses := map[string]bool{
		"active":    true,
		"completed": true,
		"archived":  true,
	}
	return validStatuses[status]
}

func isValidProviderType(providerType string) bool {
	validTypes := map[string]bool{
		"local":         true,
		"google":        true,
		"remote-hermes": true,
	}
	return validTypes[providerType]
}

func isValidMigrationStatus(status string) bool {
	validStatuses := map[string]bool{
		"active":   true,
		"source":   true,
		"target":   true,
		"archived": true,
	}
	return validStatuses[status]
}

func isValidURL(urlStr string) bool {
	// Simplified URL validation
	return strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://")
}

func isValidFileExtension(ext string) bool {
	// Must be alphanumeric, no dots
	pattern := `^[a-z0-9]+$`
	matched, _ := regexp.MatchString(pattern, ext)
	return matched
}

func isValidAuthMethod(method string) bool {
	validMethods := map[string]bool{
		"oidc":    true,
		"bearer":  true,
		"api-key": true,
	}
	return validMethods[method]
}

// ValidateConfig is a convenience function that creates a validator and validates the config
func ValidateConfig(config *Config) error {
	validator := NewValidator()
	return validator.Validate(config)
}
