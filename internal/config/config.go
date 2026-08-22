// Package config provides config functionality.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	dexadapter "github.com/hashicorp-forge/hermes/pkg/auth/adapters/dex"
	oktaadapter "github.com/hashicorp-forge/hermes/pkg/auth/adapters/okta"
	"github.com/hashicorp-forge/hermes/pkg/domain"
	algoliaadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/algolia"
	meilisearchadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
	gw "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/google"
	localadapter "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/local"
)

const (
	// Database types
	dbTypePostgres = "postgres"
)

// Config contains the Hermes configuration.
type Config struct {
	Meilisearch          *Meilisearch           `hcl:"meilisearch,block"`
	Products             *Products              `hcl:"products,block"`
	Datadog              *Datadog               `hcl:"datadog,block"`
	Dex                  *dexadapter.Config     `hcl:"dex,block"`
	Algolia              *algoliaadapter.Config `hcl:"algolia,block"`
	Email                *Email                 `hcl:"email,block"`
	Notifications        *Notifications         `hcl:"notifications,block"`
	FeatureFlags         *FeatureFlags          `hcl:"feature_flags,block"`
	Server               *Server                `hcl:"server,block"`
	GoogleWorkspace      *GoogleWorkspace       `hcl:"google_workspace,block"`
	SharePoint           *SharePointConfig      `hcl:"sharepoint,block"`
	Indexer              *Indexer               `hcl:"indexer,block"`
	Jira                 *Jira                  `hcl:"jira,block"`
	Providers            *Providers             `hcl:"providers,block"`
	Postgres             *Postgres              `hcl:"postgres,block"`
	DocumentTypes        *DocumentTypes         `hcl:"document_types,block"`
	Migration            *Migration             `hcl:"migration,block"`
	Bleve                *Bleve                 `hcl:"bleve,block"`
	Ollama               *Ollama                `hcl:"ollama,block"`
	OpenTelemetry        *OpenTelemetry         `hcl:"opentelemetry,block"`
	Okta                 *oktaadapter.Config    `hcl:"okta,block"`
	LocalWorkspace       *LocalWorkspace        `hcl:"local_workspace,block"`
	Sites                []*Site                `hcl:"site,block"`
	Session              *Session               `hcl:"session,block"`
	TrustedProxies       []string               `hcl:"trusted_proxies,optional"`
	DefaultSite          string                 `hcl:"default_site,optional"`
	ShortenerBaseURL     string                 `hcl:"shortener_base_url,optional"`
	BaseURL              string                 `hcl:"base_url,optional"`
	GoogleAnalyticsTagID string                 `hcl:"google_analytics_tag_id,optional"`
	LogFormat            string                 `hcl:"log_format,optional"`
	SupportLinkURL       string                 `hcl:"support_link_url,optional"`
	DatabaseType         string
	DBPath               string
	SimplifiedMode       bool
}

// Session configures the signed session cookie.
//
// The key is a secret. Prefer supplying it through HERMES_SESSION_KEY, which
// overrides whatever is written here, and leave the HCL value unset or as a
// placeholder. When neither is set the server generates a key on startup and
// warns: sessions then end at every restart and are not shared between
// instances, which is safe but not what a deployment wants.
type Session struct {
	// Key is the signing secret. At least 32 characters.
	Key string `hcl:"key,optional"`

	// TTL is how long a session stays valid, as a Go duration ("168h").
	// Defaults to 7 days.
	TTL string `hcl:"ttl,optional"`
}

// Datadog configures Hermes to send metrics to Datadog.
type Datadog struct {
	Env            string `hcl:"env,optional"`
	Service        string `hcl:"service,optional"`
	ServiceVersion string `hcl:"service_version,optional"`
	Enabled        bool   `hcl:"enabled,optional"`
}

// DocumentTypes contain available document types.
type DocumentTypes struct {
	// DocumentType defines a document type.
	DocumentType []*DocumentType `hcl:"document_type,block"`
}

// DocumentType is a document type (e.g., "RFC", "PRD").
type DocumentType struct {
	// Name is the name of the document type, which is generally an abbreviation.
	// Example: "RFC"
	Name string `hcl:"name,label" json:"name"`

	// LongName is the longer name for the document type.
	// Example: "Request for Comments"
	LongName string `hcl:"long_name,optional" json:"longName"`

	// Description is the description of the document type.
	// Example: "Create a Request for Comments document to present a proposal to
	//   colleagues for their review and feedback."
	Description string `hcl:"description,optional" json:"description"`

	// FlightIcon is the name of the Helios flight icon.
	// From: https://helios.hashicorp.design/icons/library
	FlightIcon string `hcl:"flight_icon,optional" json:"flightIcon"`

	// Template is the Google file ID for the document template used for this
	// document type.
	//
	// Optional, because it is meaningful only to the Google Workspace
	// provider: a local or SharePoint deployment uses MarkdownTemplate or
	// MSTemplate instead and has no Google file ID to give. Requiring it here
	// meant the shipped example could not be loaded at all without inventing
	// one. registerDocumentTypes enforces it where it actually matters.
	Template string `hcl:"template,optional"`

	// MSTemplate is the Microsoft file path or ID for the document template used for this
	// document type.
	MSTemplate string `hcl:"ms_template,optional"`

	// MoreInfoLink defines a link to more info for the document type.
	// Example: "When should I create an RFC?"
	MoreInfoLink *DocumentTypeLink `hcl:"more_info_link,block" json:"moreInfoLink"`

	// Checks are document type checks, which require acknowledging a check box
	// in order to publish a document.
	Checks []*DocumentTypeCheck `hcl:"check,block" json:"checks"`

	// CustomFields are custom fields specific to the document type.
	CustomFields []*DocumentTypeCustomField `hcl:"custom_field,block" json:"customFields"`
}

// DocumentTypeCheck is a document type check, which require acknowledging a
// check box in order to publish a document.
type DocumentTypeCheck struct {
	// Label is the document type check label.
	Label string `hcl:"label" json:"label"`

	// HelperText contains more details for the document type check.
	HelperText string `hcl:"helper_text,optional" json:"helperText"`

	// Links contain document type check links.
	Links []*DocumentTypeLink `hcl:"link,block" json:"links"`
}

// DocumentTypeCustomField defines a custom field for a document type.
type DocumentTypeCustomField struct {
	Name     string `hcl:"name" json:"name"`
	Type     string `hcl:"type" json:"type"`
	ReadOnly bool   `hcl:"read_only,optional" json:"readOnly"`
}

// DocumentTypeLink is a document type link.
type DocumentTypeLink struct {
	// Text is the displayed text for a document type link.
	Text string `hcl:"text" json:"text"`

	// URL is the URL that the document type link links to.
	URL string `hcl:"url" json:"url"`
}

// Email configures Hermes to send email notifications.
type Email struct {
	Retry       *EmailRetry `hcl:"retry,block"`
	FromAddress string      `hcl:"from_address,optional"`
	Enabled     bool        `hcl:"enabled,optional"`
}

// EmailRetry configures retry behavior for best-effort notification sends.
type EmailRetry struct {
	MaxAttempts         int `hcl:"max_attempts,optional"`
	InitialDelayMinutes int `hcl:"initial_delay_minutes,optional"`
	FinalDelayMinutes   int `hcl:"final_delay_minutes,optional"`
}

// Notifications configures the RFC-087 notification system.
type Notifications struct {
	SMTP          *SMTPConfig `hcl:"smtp,block"`
	Brokers       string      `hcl:"brokers,optional"`
	Topic         string      `hcl:"topic,optional"`
	Backends      string      `hcl:"backends,optional"`
	TemplatesPath string      `hcl:"templates_path,optional"`
	Enabled       bool        `hcl:"enabled,optional"`
}

// SMTPConfig configures SMTP for email notifications.
type SMTPConfig struct {
	// Host is the SMTP server hostname.
	Host string `hcl:"host,optional"`

	// Port is the SMTP server port (typically 587 for TLS, 25 for plaintext).
	Port string `hcl:"port,optional"`

	// Username for SMTP authentication (optional).
	Username string `hcl:"username,optional"`

	// Password for SMTP authentication (optional).
	Password string `hcl:"password,optional"`

	// FromAddress is the "from" email address for notifications.
	FromAddress string `hcl:"from_address,optional"`

	// FromName is the "from" display name for notifications.
	FromName string `hcl:"from_name,optional"`

	// UseTLS enables STARTTLS (recommended for port 587).
	UseTLS bool `hcl:"use_tls,optional"`
}

// FeatureFlags contain available feature flags.
type FeatureFlags struct {
	// FeatureFlag defines a feature flag in Hermes.
	FeatureFlag []*FeatureFlag `hcl:"flag,block"`
}

// FeatureFlag defines a feature flag configuration.
type FeatureFlag struct {
	Enabled    *bool  `hcl:"enabled,optional"`
	Name       string `hcl:"name,label"`
	Percentage int    `hcl:"percentage,optional"`
}

// Indexer contains the configuration for the Hermes indexer.
type Indexer struct {
	Topic                      string           `hcl:"topic,optional"`
	ConsumerGroup              string           `hcl:"consumer_group,optional"`
	RedpandaBrokers            []string         `hcl:"redpanda_brokers,optional"`
	Rulesets                   []IndexerRuleset `hcl:"rulesets,block"`
	MaxParallelDocs            int              `hcl:"max_parallel_docs,optional"`
	PollInterval               time.Duration    `hcl:"poll_interval,optional"`
	BatchSize                  int              `hcl:"batch_size,optional"`
	UpdateDocHeaders           bool             `hcl:"update_doc_headers,optional"`
	UpdateDraftHeaders         bool             `hcl:"update_draft_headers,optional"`
	UseDatabaseForDocumentData bool             `hcl:"use_database_for_document_data,optional"`
}

// IndexerRuleset defines when and how to process a document revision.
type IndexerRuleset struct {
	Conditions map[string]string      `hcl:"conditions,optional"`
	Config     map[string]interface{} `hcl:"config,optional"`
	Name       string                 `hcl:"name,label"`
	Pipeline   []string               `hcl:"pipeline"`
}

// GoogleWorkspace is the configuration to work with Google Workspace.
type GoogleWorkspace struct {
	Auth                  *gw.Config                        `hcl:"auth,block"`
	GroupApprovals        *GoogleWorkspaceGroupApprovals    `hcl:"group_approvals,block"`
	OAuth2                *GoogleWorkspaceOAuth2            `hcl:"oauth2,block"`
	UserNotFoundEmail     *GoogleWorkspaceUserNotFoundEmail `hcl:"user_not_found_email,block"`
	DocsFolder            string                            `hcl:"docs_folder"`
	Domain                string                            `hcl:"domain"`
	DraftsFolder          string                            `hcl:"drafts_folder"`
	ShortcutsFolder       string                            `hcl:"shortcuts_folder"`
	TemporaryDraftsFolder string                            `hcl:"temporary_drafts_folder,optional"`
	CreateDocShortcuts    bool                              `hcl:"create_doc_shortcuts,optional"`
}

// SharePointConfig configures Microsoft SharePoint / Graph workspace access.
type SharePointConfig struct {
	GroupApprovals  *SharePointGroupApprovals `hcl:"group_approvals,block"`
	ClientID        string                    `hcl:"client_id"`
	ClientSecret    string                    `hcl:"client_secret"`
	RedirectURI     string                    `hcl:"redirect_uri"`
	TenantID        string                    `hcl:"tenant_id"`
	SiteID          string                    `hcl:"site_id"`
	DriveID         string                    `hcl:"drive_id"`
	Domain          string                    `hcl:"domain"`
	DocsFolder      string                    `hcl:"docs_folder"`
	DraftsFolder    string                    `hcl:"drafts_folder"`
	ShortcutsFolder string                    `hcl:"shortcuts_folder"`
}

// GoogleWorkspaceGroupApprovals is the configuration for using Google Groups as
// document approvers.
type GoogleWorkspaceGroupApprovals struct {
	SearchPrefix string `hcl:"search_prefix,optional"`
	Enabled      bool   `hcl:"enabled,optional"`
}

// SharePointGroupApprovals is the configuration for using Microsoft distribution lists as
// document approvers.
type SharePointGroupApprovals struct {
	// Enabled enables using Microsoft distribution lists as document approvers.
	Enabled bool `hcl:"enabled,optional"`

	// SearchPrefix is the prefix to use when searching for distribution lists.
	SearchPrefix string `hcl:"search_prefix,optional"`
}

// GoogleWorkspaceOAuth2 is the configuration to use OAuth 2.0 to access Google
// Workspace APIs.
type GoogleWorkspaceOAuth2 struct {
	// ClientID is the client ID obtained from the Google API Console Credentials
	// page.
	ClientID string `hcl:"client_id,optional"`

	// HD is the allowed domain associated with the authenticating user.
	HD string `hcl:"hd,optional"`

	// RedirectURI is an authorized redirect URI for the given client_id as
	// specified in the Google API Console Credentials page.
	RedirectURI string `hcl:"redirect_uri,optional"`
}

// GoogleWorkspaceUserNotFoundEmail is the configuration to send an email when a
// user is not found in Google Workspace.
type GoogleWorkspaceUserNotFoundEmail struct {
	Body    string `hcl:"body,optional"`
	Subject string `hcl:"subject,optional"`
	Enabled bool   `hcl:"enabled,optional"`
}

// Jira is the configuration for Hermes to work with Jira.
type Jira struct {
	APIToken string `hcl:"api_token,optional"`
	URL      string `hcl:"url,optional"`
	User     string `hcl:"user,optional"`
	Enabled  bool   `hcl:"enabled,optional"`
}

// Postgres configures PostgreSQL as the app database.
type Postgres struct {
	DBName   string `hcl:"dbname"`
	Host     string `hcl:"host"`
	Password string `hcl:"password"`
	User     string `hcl:"user"`
	Port     int    `hcl:"port"`
}

// Products contain available products.
type Products struct {
	// Product defines a product.
	Product []*Product `hcl:"product,block"`
}

// Product is a product/area.
type Product struct {
	// Name is the name of the product.
	Name string `hcl:"name,label" json:"name"`

	// Abbreviation is the abbreviation (usually a few uppercase letters).
	Abbreviation string `hcl:"abbreviation" json:"abbreviation"`
}

// Providers specifies which workspace and search providers to use.
type Providers struct {
	// Workspace is the workspace provider name (e.g., "google", "local", "sharepoint").
	Workspace string `hcl:"workspace,optional"`

	// Search is the search provider name (e.g., "algolia", "meilisearch").
	Search string `hcl:"search,optional"`

	// ProjectsConfigPath is the path to the workspace projects HCL configuration file.
	// This enables multi-tenant workspace isolation with different providers per project.
	// Example: "testing/projects.hcl"
	ProjectsConfigPath string `hcl:"projects_config_path,optional"`
}

// LocalWorkspace configures local filesystem workspace storage.
type LocalWorkspace struct {
	SMTP     *LocalWorkspaceSMTP `hcl:"smtp,block"`
	BasePath string              `hcl:"base_path"`

	// The sub-paths are optional. Left unset, the local adapter derives them
	// from base_path — and, when the process serves several sites, from each
	// site's own subdirectory beneath it. A multi-domain deployment must leave
	// them unset, since one shared docs_path would put every tenant's
	// documents in the same directory.
	DocsPath    string `hcl:"docs_path,optional"`
	DraftsPath  string `hcl:"drafts_path,optional"`
	FoldersPath string `hcl:"folders_path,optional"`
	UsersPath   string `hcl:"users_path,optional"`
	TokensPath  string `hcl:"tokens_path,optional"`

	// Domain is the email domain used for generated addresses. It is unrelated
	// to the site hostnames in the `site` blocks.
	Domain string `hcl:"domain,optional"`
}

// LocalWorkspaceSMTP configures SMTP for the local workspace adapter.
type LocalWorkspaceSMTP struct {
	Host     string `hcl:"host,optional"`
	Username string `hcl:"username,optional"`
	Password string `hcl:"password,optional"`
	Port     int    `hcl:"port,optional"`
	Enabled  bool   `hcl:"enabled,optional"`
}

// Meilisearch configures Hermes to work with Meilisearch.
type Meilisearch struct {
	// Host is the Meilisearch server URL (e.g., "http://localhost:7700").
	Host string `hcl:"host"`

	// APIKey is the Meilisearch API key (master key).
	APIKey string `hcl:"api_key"`

	// DocsIndexName is the index name for published documents.
	DocsIndexName string `hcl:"docs_index_name"`

	// DraftsIndexName is the index name for draft documents.
	DraftsIndexName string `hcl:"drafts_index_name"`

	// ProjectsIndexName is the index name for projects.
	ProjectsIndexName string `hcl:"projects_index_name"`

	// LinksIndexName is the index name for links/redirects.
	LinksIndexName string `hcl:"links_index_name"`
}

// Bleve configures Hermes to work with Bleve (embedded full-text search).
type Bleve struct {
	// IndexPath is the directory where Bleve indexes are stored.
	// E.g., "./docs-cms/data/fts.index"
	IndexPath string `hcl:"index_path"`
}

// Migration configures the RFC-089 storage migration system.
type Migration struct {
	WriteStrategy  string        `hcl:"write_strategy,optional"`
	ReadStrategy   string        `hcl:"read_strategy,optional"`
	PollInterval   time.Duration `hcl:"poll_interval,optional"`
	MaxConcurrency int           `hcl:"max_concurrency,optional"`
	Enabled        bool          `hcl:"enabled,optional"`
}

// Ollama configures Hermes to work with Ollama for local AI summarization.
type Ollama struct {
	// URL is the Ollama API URL (e.g., "http://localhost:11434").
	URL string `hcl:"url"`

	// SummarizeModel is the model for document summarization (e.g., "llama3.2").
	SummarizeModel string `hcl:"summarize_model,optional"`

	// EmbeddingModel is the model for vector embeddings (e.g., "nomic-embed-text").
	EmbeddingModel string `hcl:"embedding_model,optional"`
}

// OpenTelemetry configures the auxiliary observability HTTP listener.
type OpenTelemetry struct {
	// Addr is the listener address for OTEL/PromQL routes. It is intentionally
	// separate from server.addr so observability traffic can be exposed on a
	// different port or interface.
	Addr string `hcl:"addr,optional"`

	// PrometheusURL is the base URL for a Prometheus-compatible PromQL backend.
	PrometheusURL string `hcl:"prometheus_url,optional"`

	// Enabled starts the auxiliary OTEL listener when true.
	Enabled bool `hcl:"enabled,optional"`
}

// Site is one hostname served by this Hermes process.
//
// A single listener serves many subdomains. Each site is an isolated tenant:
// its own PostgreSQL schema, its own workspace directory, its own search index
// namespace, and its own session scope. The block label is the canonical
// hostname; see pkg/domain for what "canonical" means here.
type Site struct {
	// Domain is the block label, e.g. site "docs.jrepp.com".
	Domain string `hcl:"domain,label"`

	// Aliases are additional hostnames routed to this same site. An alias
	// shares the site's storage entirely; it is a second name for one tenant,
	// not a second tenant.
	Aliases []string `hcl:"aliases,optional"`

	// BaseURL is the externally reachable origin for this site, used to build
	// absolute links and OAuth redirects. Defaults to https://<domain>.
	BaseURL string `hcl:"base_url,optional"`

	// WorkspacePath overrides where this site's documents live on disk.
	// Defaults to <local_workspace.base_path>/<domain>.
	WorkspacePath string `hcl:"workspace_path,optional"`

	// SchemaName overrides the PostgreSQL schema for this site. Defaults to
	// the value derived by domain.Name.SchemaName. Set this only when
	// adopting an existing schema.
	SchemaName string `hcl:"schema_name,optional"`

	// Disabled takes the site out of service without deleting its
	// configuration or data.
	Disabled bool `hcl:"disabled,optional"`
}

// Server contains the configuration for the Hermes server.
type Server struct {
	// Addr is the address to bind to for listening.
	Addr string `hcl:"addr,optional"`

	TLSEnabled bool   `hcl:"tls_enabled,optional"`
	TLSCert    string `hcl:"tls_cert,optional"`
	TLSKey     string `hcl:"tls_key,optional"`
}

// NewConfig parses an HCL configuration file and returns the Hermes config.
// If profile is non-empty, loads config from profile block with that name.
// If profile is empty and file has profiles, uses "default" profile.
// If profile is empty and file has no profiles, loads from root level (backward compatible).
//
//nolint:gocognit,gocyclo // Config loading preserves legacy/profile compatibility in one place.
func NewConfig(filename, profile string) (*Config, error) {
	// Read and parse file to check if it has profiles
	//nolint:gosec // G304: filename is provided by the caller, typically CLI flag
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse config file: %w", diags)
	}

	// Check if file has any profile blocks
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected HCL body type")
	}
	hasProfiles := false
	for _, block := range body.Blocks {
		if block.Type == "profile" {
			hasProfiles = true
			break
		}
	}

	// If no profiles in file and no profile requested, use root-level config (backward compatible)
	if !hasProfiles && profile == "" {
		c := &Config{
			Algolia:         &algoliaadapter.Config{},
			Email:           &Email{},
			FeatureFlags:    &FeatureFlags{},
			GoogleWorkspace: &GoogleWorkspace{},
			Indexer:         &Indexer{},
			Okta:            &oktaadapter.Config{},
			Server:          &Server{},
		}
		err := hclsimple.DecodeFile(filename, nil, c)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration: %w", err)
		}

		// Detect database type based on config
		// If LocalWorkspace exists and no valid Postgres config, use SQLite
		if c.LocalWorkspace != nil && (c.Postgres == nil || c.Postgres.Password == "") {
			c.DatabaseType = "sqlite"
			c.SimplifiedMode = true
			// Set default DB path if not set
			if c.DBPath == "" {
				c.DBPath = filepath.Join(c.LocalWorkspace.BasePath, "data", "hermes.db")
			}
		} else if c.Postgres != nil && c.Postgres.Host != "" {
			c.DatabaseType = dbTypePostgres
		}

		return c, nil
	}

	// File has profiles - select which one to use
	selectedProfile := profile
	if selectedProfile == "" {
		selectedProfile = "default" // Default profile when file has profiles
	}

	// Find and decode the requested profile.
	for _, block := range body.Blocks {
		if block.Type != "profile" || len(block.Labels) == 0 || block.Labels[0] != selectedProfile {
			continue
		}

		c := &Config{
			Algolia:         &algoliaadapter.Config{},
			Email:           &Email{},
			FeatureFlags:    &FeatureFlags{},
			GoogleWorkspace: &GoogleWorkspace{},
			Indexer:         &Indexer{},
			Okta:            &oktaadapter.Config{},
			Server:          &Server{},
		}
		err := gohcl.DecodeBody(block.Body, nil, c)
		if err != nil {
			return nil, fmt.Errorf("failed to decode profile %q: %w", selectedProfile, err)
		}

		// Detect database type based on config.
		// If LocalWorkspace exists and no valid Postgres config, use SQLite.
		if c.LocalWorkspace != nil && (c.Postgres == nil || c.Postgres.Password == "") {
			c.DatabaseType = "sqlite"
			c.SimplifiedMode = true
			if c.DBPath == "" {
				c.DBPath = filepath.Join(c.LocalWorkspace.BasePath, "data", "hermes.db")
			}
		} else if c.Postgres != nil && c.Postgres.Host != "" {
			c.DatabaseType = dbTypePostgres
		}

		return c, nil
	}

	return nil, fmt.Errorf("profile %q not found in configuration", selectedProfile)
}

// ToLocalAdapterConfig converts LocalWorkspace config to local adapter config.
//
// The result is not domain-scoped; use ToLocalAdapterConfigForDomain when the
// process serves more than one site.
func (lw *LocalWorkspace) ToLocalAdapterConfig() *localadapter.Config {
	return lw.toLocalAdapterConfig(domain.Name{})
}

// ToLocalAdapterConfigForDomain converts LocalWorkspace config to a local
// adapter config scoped to one site.
//
// The per-site paths are left empty so the adapter derives them from the
// domain root itself. Copying the globally configured docs_path and friends
// here would point every site at the same directories, which is precisely the
// cross-tenant leak the domain scoping exists to prevent.
func (lw *LocalWorkspace) ToLocalAdapterConfigForDomain(d domain.Name) *localadapter.Config {
	return lw.toLocalAdapterConfig(d)
}

func (lw *LocalWorkspace) toLocalAdapterConfig(d domain.Name) *localadapter.Config {
	if lw == nil {
		return nil
	}

	if !d.IsZero() {
		cfg := &localadapter.Config{
			BasePath: lw.BasePath,
			Domain:   d,
		}
		cfg.SMTPConfig = lw.smtpAdapterConfig()

		return cfg
	}

	cfg := &localadapter.Config{
		BasePath:    lw.BasePath,
		DocsPath:    lw.DocsPath,
		DraftsPath:  lw.DraftsPath,
		FoldersPath: lw.FoldersPath,
		UsersPath:   lw.UsersPath,
		TokensPath:  lw.TokensPath,
	}

	cfg.SMTPConfig = lw.smtpAdapterConfig()

	return cfg
}

// smtpAdapterConfig returns the adapter SMTP settings, or nil when disabled.
func (lw *LocalWorkspace) smtpAdapterConfig() *localadapter.SMTPConfig {
	if lw.SMTP == nil || !lw.SMTP.Enabled {
		return nil
	}

	return &localadapter.SMTPConfig{
		Host:     lw.SMTP.Host,
		Port:     lw.SMTP.Port,
		Username: lw.SMTP.Username,
		Password: lw.SMTP.Password,
		From:     "hermes@" + lw.Domain,
	}
}

// ToMeilisearchAdapterConfig converts Meilisearch config to meilisearch adapter config.
func (m *Meilisearch) ToMeilisearchAdapterConfig() *meilisearchadapter.Config {
	if m == nil {
		return nil
	}

	return &meilisearchadapter.Config{
		Host:              m.Host,
		APIKey:            m.APIKey,
		DocsIndexName:     m.DocsIndexName,
		DraftsIndexName:   m.DraftsIndexName,
		ProjectsIndexName: m.ProjectsIndexName,
		LinksIndexName:    m.LinksIndexName,
	}
}

// GenerateSimplifiedConfig creates a config for simplified mode with embedded
// database, local workspace, and zero external dependencies.
func GenerateSimplifiedConfig(workspacePath string) *Config {
	return &Config{
		SimplifiedMode: true,
		DatabaseType:   "sqlite",
		DBPath:         filepath.Join(workspacePath, "data", "hermes.db"),
		BaseURL:        "http://localhost:8000",

		Server: &Server{
			Addr: "127.0.0.1:8000",
		},

		Providers: &Providers{
			Workspace: "local",
			Search:    "bleve", // Use embedded Bleve search
		},

		Bleve: &Bleve{
			IndexPath: filepath.Join(workspacePath, "data", "fts.index"),
		},

		LocalWorkspace: &LocalWorkspace{
			BasePath:    workspacePath,
			DocsPath:    filepath.Join(workspacePath, "documents"),
			DraftsPath:  filepath.Join(workspacePath, "drafts"),
			FoldersPath: filepath.Join(workspacePath, "folders"),
			UsersPath:   filepath.Join(workspacePath, "users"),
			TokensPath:  filepath.Join(workspacePath, "tokens"),
			Domain:      "localhost",
			SMTP: &LocalWorkspaceSMTP{
				Enabled: false,
			},
		},

		// Disable all external auth providers (simplified mode uses embedded Dex)
		Okta: &oktaadapter.Config{
			Disabled: true,
		},

		Dex: &dexadapter.Config{
			Disabled: true, // Can be enabled later with local Dex config
		},

		// Minimal indexer config
		Indexer: &Indexer{
			MaxParallelDocs:            5,
			UpdateDocHeaders:           true,
			UpdateDraftHeaders:         true,
			UseDatabaseForDocumentData: true, // Use DB as source of truth
		},

		// Disable external services
		Email: &Email{
			Enabled: false,
		},

		LogFormat: "standard",
	}
}

// WriteConfig writes a Config to an HCL file (for temporary config generation).
// This is a simplified writer that doesn't preserve all fields - used only
// for internal config passing to server command.
func WriteConfig(cfg *Config, path string) error {
	// For now, we'll just create a minimal HCL file with key settings
	// The server command will use the in-memory Config object
	content := fmt.Sprintf(`
# Auto-generated Hermes simplified mode config
# This is a temporary file and should not be edited

server {
  addr = %q
}

providers {
  workspace = "local"
  search    = "bleve"
}

bleve {
  index_path = %q
}

local_workspace {
  base_path    = %q
  docs_path    = %q
  drafts_path  = %q
  folders_path = %q
  users_path   = %q
  tokens_path  = %q
  domain       = "localhost"

  smtp {
    enabled = false
  }
}

indexer {
  max_parallel_docs = 5
  update_doc_headers = true
  update_draft_headers = true
  use_database_for_document_data = true
}

okta {
  disabled = true
}

dex {
  disabled = true
}

postgres {
  # Not used in simplified mode (using SQLite)
  dbname   = "hermes"
  host     = "localhost"
  port     = 5432
  user     = "postgres"
  password = ""
}

email {
  enabled = false
}
`,
		cfg.Server.Addr,
		cfg.Bleve.IndexPath,
		cfg.LocalWorkspace.BasePath,
		cfg.LocalWorkspace.DocsPath,
		cfg.LocalWorkspace.DraftsPath,
		cfg.LocalWorkspace.FoldersPath,
		cfg.LocalWorkspace.UsersPath,
		cfg.LocalWorkspace.TokensPath,
	)

	return os.WriteFile(path, []byte(content), 0o600)
}
