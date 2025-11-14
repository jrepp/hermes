package backends

import (
	"log"
)

// Config holds backend configuration from HCL
type Config struct {
	// Audit backend (always enabled if present)
	Audit *AuditConfig `hcl:"audit,block"`

	// Mail backend configuration
	Mail *MailConfig `hcl:"mail,block"`

	// Ntfy backend configuration
	Ntfy *NtfyConfig `hcl:"ntfy,block"`
}

// AuditConfig configures the audit backend
type AuditConfig struct {
	Enabled bool `hcl:"enabled,optional"`
}

// MailConfig configures the mail backend
type MailConfig struct {
	Enabled bool `hcl:"enabled,optional"`

	SMTPHost     string `hcl:"smtp_host,optional"`
	SMTPPort     string `hcl:"smtp_port,optional"`
	SMTPUsername string `hcl:"smtp_username,optional"`
	SMTPPassword string `hcl:"smtp_password,optional"`
	FromAddress  string `hcl:"from_address,optional"`
	FromName     string `hcl:"from_name,optional"`
	UseTLS       bool   `hcl:"use_tls,optional"`
}

// NtfyConfig configures the ntfy backend
type NtfyConfig struct {
	Enabled bool `hcl:"enabled,optional"`

	ServerURL string `hcl:"server_url,optional"`
	Topic     string `hcl:"topic,optional"`
}

// Registry manages available notification backends
type Registry struct {
	backends map[string]Backend
}

// NewRegistry creates a new backend registry from configuration
func NewRegistry(cfg *Config) (*Registry, error) {
	registry := &Registry{
		backends: make(map[string]Backend),
	}

	if cfg == nil {
		return registry, nil
	}

	// Initialize audit backend
	if cfg.Audit != nil && cfg.Audit.Enabled {
		backend := NewAuditBackend()
		registry.backends["audit"] = backend
		log.Printf("Initialized audit backend")
	}

	// Initialize mail backend
	if cfg.Mail != nil && cfg.Mail.Enabled {
		backend := NewMailBackend(MailBackendConfig{
			SMTPHost:     cfg.Mail.SMTPHost,
			SMTPPort:     cfg.Mail.SMTPPort,
			SMTPUsername: cfg.Mail.SMTPUsername,
			SMTPPassword: cfg.Mail.SMTPPassword,
			FromAddress:  cfg.Mail.FromAddress,
			FromName:     cfg.Mail.FromName,
			UseTLS:       cfg.Mail.UseTLS,
		})
		registry.backends["mail"] = backend
		registry.backends["email"] = backend // Alias
		log.Printf("Initialized mail backend (host=%s, port=%s, from=%s)",
			cfg.Mail.SMTPHost, cfg.Mail.SMTPPort, cfg.Mail.FromAddress)
	}

	// Initialize ntfy backend
	if cfg.Ntfy != nil && cfg.Ntfy.Enabled {
		backend := NewNtfyBackend(NtfyBackendConfig{
			ServerURL: cfg.Ntfy.ServerURL,
			Topic:     cfg.Ntfy.Topic,
		})
		registry.backends["ntfy"] = backend
		serverURL := cfg.Ntfy.ServerURL
		if serverURL == "" {
			serverURL = "https://ntfy.sh (default)"
		}
		log.Printf("Initialized ntfy backend (server=%s, topic=%s)",
			serverURL, cfg.Ntfy.Topic)
	}

	return registry, nil
}

// GetBackend returns a backend by name
func (r *Registry) GetBackend(name string) (Backend, bool) {
	backend, ok := r.backends[name]
	return backend, ok
}

// GetAll returns all registered backends
func (r *Registry) GetAll() []Backend {
	backends := make([]Backend, 0, len(r.backends))
	seen := make(map[string]bool)

	for _, backend := range r.backends {
		// Avoid duplicates (e.g., mail/email alias)
		if !seen[backend.Name()] {
			backends = append(backends, backend)
			seen[backend.Name()] = true
		}
	}
	return backends
}

// GetBackendNames returns the names of all registered backends
func (r *Registry) GetBackendNames() []string {
	names := make([]string, 0, len(r.backends))
	seen := make(map[string]bool)

	for _, backend := range r.backends {
		if !seen[backend.Name()] {
			names = append(names, backend.Name())
			seen[backend.Name()] = true
		}
	}
	return names
}
