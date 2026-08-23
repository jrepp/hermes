// Package server provides server functionality.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	_ "github.com/lib/pq" // PostgreSQL driver for migrations
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/api"
	apiv2 "github.com/hashicorp-forge/hermes/internal/api/v2"
	"github.com/hashicorp-forge/hermes/internal/auth"
	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/datadog"
	dbpkg "github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/internal/instance"
	"github.com/hashicorp-forge/hermes/internal/jira"
	"github.com/hashicorp-forge/hermes/internal/middleware"
	"github.com/hashicorp-forge/hermes/internal/migrate"
	"github.com/hashicorp-forge/hermes/internal/openapi"
	"github.com/hashicorp-forge/hermes/internal/otel"
	"github.com/hashicorp-forge/hermes/internal/pkg/doctypes"
	"github.com/hashicorp-forge/hermes/internal/projects"
	"github.com/hashicorp-forge/hermes/internal/pub"
	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/internal/session"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/internal/structs"
	"github.com/hashicorp-forge/hermes/pkg/algolia"
	"github.com/hashicorp-forge/hermes/pkg/domain"
	hcd "github.com/hashicorp-forge/hermes/pkg/hashicorpdocs"
	"github.com/hashicorp-forge/hermes/pkg/indexer/relay"
	"github.com/hashicorp-forge/hermes/pkg/kafka"
	"github.com/hashicorp-forge/hermes/pkg/links"
	"github.com/hashicorp-forge/hermes/pkg/migration"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
	"github.com/hashicorp-forge/hermes/pkg/search"
	searchalgolia "github.com/hashicorp-forge/hermes/pkg/search/adapters/algolia"
	bleveadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/bleve"
	meilisearchadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
	searchoutbox "github.com/hashicorp-forge/hermes/pkg/search/outbox"
	"github.com/hashicorp-forge/hermes/pkg/sharepointhelper"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	gw "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/google"
	localadapter "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/local"
	sharepointadapter "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/sharepoint"
	"github.com/hashicorp-forge/hermes/web"
)

const (
	// Provider names
	providerGoogle     = "google"
	providerSharePoint = "sharepoint"
	providerAlgolia    = "algolia"
)

// Command implements the server CLI command.
type Command struct {
	*base.Command

	flagAddr              string
	flagBaseURL           string
	flagConfig            string
	flagProfile           string
	flagWorkspaceProvider string
	flagSearchProvider    string
	flagAuthProvider      string
	flagOktaAuthServerURL string
	flagOktaClientID      string
	flagTLSEnabled        bool
	flagTLSCert           string
	flagTLSKey            string
}

//nolint:govet // Keep route declaration order aligned with existing positional literals.
type endpoint struct {
	handler http.Handler
	pattern string
}

// Synopsis returns a short description.
func (c *Command) Synopsis() string {
	return "Run the server"
}

// Help returns the full help text.
func (c *Command) Help() string {
	return `Usage: hermes server

  This command runs the Hermes web server.` + c.Flags().Help()
}

// Flags returns the flag set.
func (c *Command) Flags() *base.FlagSet {
	f := base.NewFlagSet(flag.NewFlagSet("server", flag.ExitOnError))

	f.StringVar(
		&c.flagAddr, "addr", "127.0.0.1:8000",
		"[HERMES_SERVER_ADDR] Address to bind to for listening.",
	)
	f.StringVar(
		&c.flagBaseURL, "base-url", "https://localhost:8443",
		"[HERMES_BASE_URL] Base URL used for building links.",
	)
	f.StringVar(
		&c.flagConfig, "config", "", "Path to Hermes config file",
	)
	f.StringVar(
		&c.flagProfile, "profile", "",
		"[HERMES_SERVER_PROFILE] Configuration profile to use (e.g., 'default', 'testing'). "+
			"If empty, uses 'default' profile when profiles exist, or root config for backward compatibility.",
	)
	f.StringVar(
		&c.flagWorkspaceProvider, "workspace-provider", "",
		"[HERMES_WORKSPACE_PROVIDER] Workspace provider to use (e.g., 'google', 'local', 'sharepoint'). "+
			"Overrides the provider specified in the config profile.",
	)
	f.StringVar(
		&c.flagSearchProvider, "search-provider", "",
		"[HERMES_SEARCH_PROVIDER] Search provider to use (e.g., 'algolia', 'meilisearch'). "+
			"Overrides the provider specified in the config profile.",
	)
	f.StringVar(
		&c.flagAuthProvider, "auth-provider", "",
		"[HERMES_AUTH_PROVIDER] Authentication provider to use (e.g., 'dex', 'okta', 'google'). "+
			"Overrides the provider auto-selection based on config. When set to 'dex' or 'okta', "+
			"will disable other providers to force that provider to be used.",
	)
	f.StringVar(
		&c.flagOktaAuthServerURL, "okta-auth-server-url", "",
		"[HERMES_SERVER_OKTA_AUTH_SERVER_URL] URL to the Okta authorization server.",
	)
	f.StringVar(
		&c.flagOktaClientID, "okta-client-id", "",
		"[HERMES_SERVER_OKTA_CLIENT_ID] Okta client ID.",
	)
	f.BoolVar(
		&c.flagTLSEnabled, "tls-enabled", false,
		"[HERMES_SERVER_TLS_ENABLED] Enable TLS/HTTPS for the server.",
	)
	f.StringVar(
		&c.flagTLSCert, "tls-cert", "",
		"[HERMES_SERVER_TLS_CERT] Path to TLS certificate file.",
	)
	f.StringVar(
		&c.flagTLSKey, "tls-key", "",
		"[HERMES_SERVER_TLS_KEY] Path to TLS private key file.",
	)

	return f
}

// Run executes the server command.
//
//nolint:gocognit,gocyclo // Server startup coordinates many optional subsystems in one entrypoint.
func (c *Command) Run(args []string) int {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("error parsing flags: %v", err))
		return 1
	}

	var (
		cfg *config.Config
		err error
	)
	if c.flagConfig != "" {
		// Get profile from flag or environment variable
		profile := c.flagProfile
		if val, ok := os.LookupEnv("HERMES_SERVER_PROFILE"); ok && profile == "" {
			profile = val
		}

		cfg, err = config.NewConfig(c.flagConfig, profile)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error parsing config file: %v: config=%q profile=%q",
				err, c.flagConfig, profile))
			return 1
		}
		// Log configuration loaded successfully without exposing sensitive data
		c.Log.Info("Configuration loaded successfully",
			"config_file", c.flagConfig,
			"log_format", cfg.LogFormat,
			"server_addr", cfg.Server.Addr,
			"base_url", cfg.BaseURL)
	}

	// Get configuration from environment variables if not set on the command
	// line.
	// TODO: make this section more DRY and add tests.
	if val, ok := os.LookupEnv("HERMES_SERVER_ADDR"); ok {
		cfg.Server.Addr = val
	}
	if c.flagAddr != f.Lookup("addr").DefValue {
		cfg.Server.Addr = c.flagAddr
	}
	if val, ok := os.LookupEnv("HERMES_BASE_URL"); ok {
		cfg.BaseURL = val
	}
	if c.flagBaseURL != f.Lookup("base-url").DefValue {
		cfg.BaseURL = c.flagBaseURL
	}
	if cfg.Okta != nil {
		if val, ok := os.LookupEnv("HERMES_SERVER_OKTA_AUTH_SERVER_URL"); ok {
			cfg.Okta.AuthServerURL = val
		}
		if c.flagOktaAuthServerURL != f.Lookup("okta-auth-server-url").DefValue {
			cfg.Okta.AuthServerURL = c.flagOktaAuthServerURL
		}
		if val, ok := os.LookupEnv("HERMES_SERVER_OKTA_CLIENT_ID"); ok {
			cfg.Okta.ClientID = val
		}
		if c.flagOktaClientID != f.Lookup("okta-client-id").DefValue {
			cfg.Okta.ClientID = c.flagOktaClientID
		}
		if val, ok := os.LookupEnv("HERMES_SERVER_OKTA_JWT_SIGNER"); ok {
			cfg.Okta.JWTSigner = val
		}
	}
	if val, ok := os.LookupEnv("HERMES_SERVER_OKTA_DISABLED"); ok {
		if cfg.Okta != nil && val != "" && val != "false" {
			cfg.Okta.Disabled = true
		}
	}

	// Handle TLS configuration
	if val, ok := os.LookupEnv("HERMES_SERVER_TLS_ENABLED"); ok {
		cfg.Server.TLSEnabled = val == "true"
	}
	if c.flagTLSEnabled {
		cfg.Server.TLSEnabled = true
	}
	if val, ok := os.LookupEnv("HERMES_SERVER_TLS_CERT"); ok {
		cfg.Server.TLSCert = val
	}
	if c.flagTLSCert != "" {
		cfg.Server.TLSCert = c.flagTLSCert
	}
	if val, ok := os.LookupEnv("HERMES_SERVER_TLS_KEY"); ok {
		cfg.Server.TLSKey = val
	}
	if c.flagTLSKey != "" {
		cfg.Server.TLSKey = c.flagTLSKey
	}

	// Handle auth provider selection from flag or environment variable
	authProvider := c.flagAuthProvider
	if val, ok := os.LookupEnv("HERMES_AUTH_PROVIDER"); ok && authProvider == "" {
		authProvider = val
	}
	if authProvider != "" {
		// Force the specified provider by disabling others
		switch strings.ToLower(authProvider) {
		case "dex":
			if cfg.Dex != nil {
				cfg.Dex.Disabled = false
			}
			if cfg.Okta != nil {
				cfg.Okta.Disabled = true
			}
			c.Log.Info("auth provider selection", "provider", "dex", "source", "flag/env")
		case "okta":
			if cfg.Dex != nil {
				cfg.Dex.Disabled = true
			}
			if cfg.Okta != nil {
				cfg.Okta.Disabled = false
			}
			c.Log.Info("auth provider selection", "provider", "okta", "source", "flag/env")
		case providerGoogle:
			// Disable both Dex and Okta to fall back to Google
			if cfg.Dex != nil {
				cfg.Dex.Disabled = true
			}
			if cfg.Okta != nil {
				cfg.Okta.Disabled = true
			}
			c.Log.Info("auth provider selection", "provider", providerGoogle, "source", "flag/env")
		default:
			c.UI.Error(fmt.Sprintf("invalid auth provider: %s (valid options: dex, okta, google)", authProvider))
			return 1
		}
	}

	// Validate feature flags defined in configuration
	if cfg.FeatureFlags != nil {
		err := config.ValidateFeatureFlags(cfg.FeatureFlags.FeatureFlag)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing server: %v", err))
			return 1
		}
	}

	// Validate other configuration.
	if cfg.Email != nil && cfg.Email.Enabled {
		if cfg.Email.FromAddress == "" {
			c.UI.Error("email from_address must be set if email is enabled")
			return 1
		}
	}

	switch cfg.LogFormat {
	case "json":
		c.Log = hclog.New(&hclog.LoggerOptions{
			JSONFormat: true,
		})
	case "standard":
	case "":
	default:
		c.UI.Error(fmt.Sprintf("invalid value for log format: %s", cfg.LogFormat))
		return 1
	}

	// Build configuration for Okta ALB authentication.
	if cfg.Okta != nil && !cfg.Okta.Disabled {
		if cfg.Okta.AuthServerURL == "" {
			c.UI.Error("error initializing server: Okta authorization server URL is required")
			return 1
		}
		if cfg.Okta.AWSRegion == "" {
			c.UI.Error("error initializing server: Okta AWS region is required")
			return 1
		}
		if cfg.Okta.ClientID == "" {
			c.UI.Error("error initializing server: Okta client ID is required")
			return 1
		}
		if cfg.Okta.JWTSigner == "" {
			c.UI.Error("error initializing server: Okta JWT signer is required")
			return 1
		}
	}

	// Initialize Datadog.
	dd := datadog.NewConfig(*cfg)
	if dd.Enabled {
		tracerOpts := []tracer.StartOption{
			tracer.WithLogStartup(false),
		}

		if dd.Env != "" {
			tracerOpts = append(tracerOpts, tracer.WithEnv(dd.Env))
		}
		if dd.Service != "" {
			tracerOpts = append(tracerOpts, tracer.WithService(dd.Service))
		}
		if dd.ServiceVersion != "" {
			tracerOpts = append(
				tracerOpts,
				tracer.WithServiceVersion(dd.ServiceVersion),
			)
		}

		tracer.Start(tracerOpts...)
	}

	// Determine which providers to use (from flags, env vars, or config).
	workspaceProviderName := c.flagWorkspaceProvider
	if val, ok := os.LookupEnv("HERMES_WORKSPACE_PROVIDER"); ok && workspaceProviderName == "" {
		workspaceProviderName = val
	}
	if workspaceProviderName == "" && cfg.Providers != nil {
		workspaceProviderName = cfg.Providers.Workspace
	}
	if workspaceProviderName == "" {
		workspaceProviderName = providerGoogle // Default to google for backward compatibility
	}

	searchProviderName := c.flagSearchProvider
	if val, ok := os.LookupEnv("HERMES_SEARCH_PROVIDER"); ok && searchProviderName == "" {
		searchProviderName = val
	}
	if searchProviderName == "" && cfg.Providers != nil {
		searchProviderName = cfg.Providers.Search
	}
	if searchProviderName == "" {
		searchProviderName = providerAlgolia // Default to algolia for backward compatibility
	}

	c.UI.Info(fmt.Sprintf("Using workspace provider: %s", workspaceProviderName))
	c.UI.Info(fmt.Sprintf("Using search provider: %s", searchProviderName))

	// Initialize workspace provider (RFC-084) based on selection.
	var workspaceProvider workspace.WorkspaceProvider
	var goog *gw.Service // Keep for auth that still uses it directly
	var sharepointSvc *sharepointhelper.Service

	// The database password override has to happen before the registry is
	// built: the registry resolves each site's connection from cfg.Postgres,
	// so applying it afterwards would leave every site holding whatever
	// password the config file contained -- usually the placeholder.
	if val, ok := os.LookupEnv("HERMES_SERVER_POSTGRES_PASSWORD"); ok && cfg.Postgres != nil {
		cfg.Postgres.Password = val
	}

	// Resolve the site registry before anything that depends on per-site
	// resources. NewRegistry rejects ambiguous hostname configuration, and it
	// is better to refuse to start than to route requests to whichever tenant
	// a map iteration happened to yield.
	siteRegistry, err := sites.NewRegistry(cfg)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error building site registry: %v", err))
		return 1
	}
	for _, site := range siteRegistry.Sites() {
		c.Log.Info("serving site",
			"domain", site.Domain.String(),
			"aliases", len(site.Aliases),
			"schema", site.SchemaName,
			"workspace", site.WorkspacePath)
	}

	// Providers built below are the process-wide defaults. In a multi-site
	// deployment they belong to no tenant, so they are built for the primary
	// site instead of unscoped: an unscoped provider would create an empty
	// workspace directory and a set of unprefixed search indexes that look
	// like a tenant nobody serves, and any code path that skipped ForRequest
	// would quietly read from them.
	var primaryDomain domain.Name
	if siteRegistry.Len() > 0 {
		primarySite := siteRegistry.Default()
		if primarySite == nil {
			primarySite = siteRegistry.Sites()[0]
		}
		primaryDomain = primarySite.Domain
	}

	switch workspaceProviderName {
	case providerGoogle:
		// Use Google Workspace service user auth if it is defined in the config.
		if cfg.GoogleWorkspace.Auth != nil {
			// Validate temporary drafts folder is configured if creating docs as user.
			if cfg.GoogleWorkspace.Auth.CreateDocsAsUser &&
				cfg.GoogleWorkspace.TemporaryDraftsFolder == "" {
				c.UI.Error(
					"error initializing server: Google Workspace temporary drafts folder is required if create_docs_as_user is true")
				return 1
			}

			goog = gw.NewFromConfig(cfg.GoogleWorkspace.Auth)
		} else {
			// Use OAuth if Google Workspace auth is not defined in the config.
			goog = gw.New()
		}

		reqOpts := map[interface{}]string{
			cfg.BaseURL:                         "Base URL is required",
			cfg.GoogleWorkspace.DocsFolder:      "Google Workspace Docs Folder is required",
			cfg.GoogleWorkspace.Domain:          "Google Workspace Domain is required",
			cfg.GoogleWorkspace.DraftsFolder:    "Google Workspace Drafts Folder is required",
			cfg.GoogleWorkspace.ShortcutsFolder: "Google Workspace Shortcuts Folder is required",
		}
		for r, msg := range reqOpts {
			if r == "" {
				c.UI.Error(fmt.Sprintf("error initializing server: %s", msg))
				return 1
			}
		}

		// Create RFC-084 adapter
		workspaceProvider = gw.NewAdapter(goog)

	case "local":
		if cfg.LocalWorkspace == nil {
			c.UI.Error("error initializing server: local_workspace configuration required when using local workspace provider")
			return 1
		}

		localCfg := cfg.LocalWorkspace.ToLocalAdapterConfigForDomain(primaryDomain)
		adapter, err := localadapter.NewAdapter(localCfg)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing local workspace adapter: %v", err))
			return 1
		}

		// Create RFC-084 adapter
		workspaceProvider = localadapter.NewWorkspaceAdapter(adapter)

		// Note: searchProvider not yet initialized at this point
		// Document indexing will be triggered after search provider is initialized

	case providerSharePoint:
		if cfg.SharePoint == nil {
			c.UI.Error("error initializing server: sharepoint configuration required when using SharePoint workspace provider")
			return 1
		}

		if err := validateSharePointConfig(cfg.SharePoint); err != nil {
			c.UI.Error(fmt.Sprintf("error initializing server: %v", err))
			return 1
		}

		sharepointSvc = sharepointhelper.NewService(cfg.SharePoint, c.Log)
		if _, err := sharepointSvc.GetToken(); err != nil {
			c.UI.Error(fmt.Sprintf("error initializing SharePoint service: %v", err))
			return 1
		}
		workspaceProvider = sharepointadapter.NewAdapter(sharepointSvc)
		c.Log.Info("successfully initialized SharePoint workspace provider")

	default:
		c.UI.Error(fmt.Sprintf("error initializing server: unknown workspace provider %q", workspaceProviderName))
		return 1
	}

	// Initialize search provider based on selection.
	var searchProvider search.Provider
	var algoSearch *algolia.Client       // Keep for legacy proxy handler
	var algoWrite *algolia.Client        // Keep for legacy operations
	var algoliaClientCfg *algolia.Config // Keep for legacy handlers

	switch searchProviderName {
	case "algolia":
		if cfg.Algolia == nil {
			c.UI.Error("error initializing server: algolia configuration required when using algolia search provider")
			return 1
		}

		reqOpts := map[interface{}]string{
			cfg.Algolia.AppID:        "Algolia Application ID is required",
			cfg.Algolia.SearchAPIKey: "Algolia Search API Key is required",
		}
		for r, msg := range reqOpts {
			if r == "" {
				c.UI.Error(fmt.Sprintf("error initializing server: %s", msg))
				return 1
			}
		}

		// Convert search adapter config to legacy algolia config
		algoliaClientCfg = &algolia.Config{
			ApplicationID:          cfg.Algolia.AppID,
			SearchAPIKey:           cfg.Algolia.SearchAPIKey,
			WriteAPIKey:            cfg.Algolia.WriteAPIKey,
			DocsIndexName:          cfg.Algolia.DocsIndexName,
			DraftsIndexName:        cfg.Algolia.DraftsIndexName,
			InternalIndexName:      cfg.Algolia.InternalIndexName,
			LinksIndexName:         cfg.Algolia.LinksIndexName,
			MissingFieldsIndexName: cfg.Algolia.MissingFieldsIndexName,
			ProjectsIndexName:      cfg.Algolia.ProjectsIndexName,
		}

		// Initialize Algolia search client (legacy - still needed for proxy handler).
		algoSearch, err = algolia.NewSearchClient(algoliaClientCfg)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing Algolia search client: %v", err))
			return 1
		}

		// Initialize Algolia write client (legacy - still needed for some operations).
		algoWrite, err = algolia.New(algoliaClientCfg)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing Algolia write client: %v", err))
			return 1
		}

		// Initialize modern search provider adapter.
		searchProvider, err = buildSearchProvider(cfg, providerAlgolia, primaryDomain)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing search provider: %v", err))
			return 1
		}

	case "meilisearch":
		if cfg.Meilisearch == nil {
			c.UI.Error("error initializing server: meilisearch configuration required when using meilisearch search provider")
			return 1
		}

		searchProvider, err = buildSearchProvider(cfg, "meilisearch", primaryDomain)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing meilisearch adapter: %v", err))
			return 1
		}

	case "bleve":
		if cfg.Bleve == nil {
			c.UI.Error("error initializing server: bleve configuration required when using bleve search provider")
			return 1
		}

		searchProvider, err = buildSearchProvider(cfg, "bleve", primaryDomain)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing bleve adapter: %v", err))
			return 1
		}
		c.Log.Info("using Bleve embedded search",
			"index_path", cfg.Bleve.IndexPathForDomain(primaryDomain))

	default:
		c.UI.Error(fmt.Sprintf("error initializing server: unknown search provider %q", searchProviderName))
		return 1
	}

	// Initialize Jira service.
	var jiraSvc *jira.Service
	if cfg.Jira != nil && cfg.Jira.Enabled {
		jiraSvc, err = jira.NewService(*cfg.Jira)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing Jira service: %v", err))
			return 1
		}
	}

	// Initialize database.
	var db *gorm.DB
	if cfg.SimplifiedMode {
		// Simplified mode: use SQLite
		// NOTE: Server binary does not support SQLite to avoid driver conflicts.
		// Use hermes-migrate binary for SQLite databases.
		c.UI.Error("SQLite mode not supported in server binary. Use hermes-migrate for migrations. See docs-internal/memo/SQLITE_DRIVER_CONFLICT.md")
		return 1
	}

	// Migrate. A multi-site deployment keeps no Hermes tables in the public
	// schema at all: every site's search_path ends in public so that extension
	// types resolve, and `CREATE TABLE IF NOT EXISTS` checks the whole path, so
	// tables in public would make every site's migration a silent no-op and
	// leave all of them sharing one set of tables.
	var siteDBs *dbpkg.SiteDBs
	if siteRegistry.Len() > 0 {
		if err := dbpkg.MigrateSites(*cfg.Postgres, siteRegistry, c.Log); err != nil {
			c.UI.Error(fmt.Sprintf("error migrating site schemas: %v", err))
			return 1
		}
	} else {
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
			cfg.Postgres.Host, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.DBName, cfg.Postgres.Port)
		c.Log.Info("running database migrations (PostgreSQL)",
			"host", cfg.Postgres.Host, "dbname", cfg.Postgres.DBName)
		if err := runMigrations("postgres", dsn); err != nil {
			c.UI.Error(fmt.Sprintf("error running PostgreSQL migrations: %v", err))
			return 1
		}

		db, err = dbpkg.NewDB(*cfg.Postgres)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing database: %v", err))
			return 1
		}
	}

	siteDBs, err = dbpkg.NewSiteDBs(*cfg.Postgres, siteRegistry, db, c.Log)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error opening site databases: %v", err))
		return 1
	}
	defer func() {
		if err := siteDBs.Close(); err != nil {
			c.Log.Warn("error closing site databases", "error", err)
		}
	}()

	// Process-level work that carries no site -- the heartbeat, the health
	// probe, the search outbox -- runs against the primary site.
	db = siteDBs.Fallback()

	c.Log.Info("using PostgreSQL database",
		"host", cfg.Postgres.Host, "dbname", cfg.Postgres.DBName,
		"sites", siteRegistry.Len())

	// Initialize instance identity.
	ctx := context.Background()
	instanceLogger := hclog.New(&hclog.LoggerOptions{
		Name:  "instance",
		Level: hclog.Info,
	})
	// Instance identity is per site: each site is an independent logical
	// Hermes with its own schema, so it needs its own row rather than
	// inheriting whichever site happened to initialize first.
	if err := forEachSite(siteDBs, siteRegistry, cfg, db,
		func(siteCfg *config.Config, siteDB *gorm.DB) error {
			return instance.Initialize(ctx, siteDB, siteCfg, instanceLogger)
		}); err != nil {
		c.UI.Error(fmt.Sprintf("error initializing instance identity: %v", err))
		return 1
	}

	// Start instance heartbeat in background
	go instance.StartHeartbeat(ctx, db, 1*time.Minute, instanceLogger)

	// Generate indexer registration token if configured
	// An indexer registers against one site, and its token is stored in that
	// site's schema. A single token written from the primary site would be
	// rejected everywhere else -- so each site gets its own, in its own file
	// named after the site.
	indexerTokenPath := os.Getenv("HERMES_INDEXER_TOKEN_PATH")
	if indexerTokenPath != "" {
		writeToken := func(siteName domain.Name, siteDB *gorm.DB) error {
			path := indexerTokenPath
			if !siteName.IsZero() {
				path = indexerTokenPath + "." + siteName.Slug()
			}

			if err := generateIndexerToken(siteDB, path, c.Log); err != nil {
				return err
			}
			c.UI.Info(fmt.Sprintf("Indexer registration token written to: %s", path))

			return nil
		}

		var tokenErr error
		if siteRegistry.Len() > 0 {
			tokenErr = siteDBs.Each(writeToken)
		} else {
			tokenErr = writeToken(domain.Name{}, db)
		}
		if tokenErr != nil {
			// Not fatal: the server runs without an indexer.
			c.UI.Warn(fmt.Sprintf("error generating indexer token: %v", tokenErr))
		}
	}

	// Register document types.
	// for _, d := range cfg.DocumentTypes.DocumentType {
	// 	if err := models.RegisterDocumentType(*d, db); err != nil {
	// 		c.UI.Error(fmt.Sprintf("error registering document type: %v", err))
	// 		return 1
	// 	}
	// }
	// Document types and products are content configuration, so every site
	// gets its own copy in its own schema.
	if err := forEachSite(siteDBs, siteRegistry, cfg, db,
		func(siteCfg *config.Config, siteDB *gorm.DB) error {
			return registerDocumentTypes(*siteCfg, siteDB)
		}); err != nil {
		c.UI.Error(fmt.Sprintf("error registering document types: %v", err))
		return 1
	}

	// Register products.
	if err := forEachSite(siteDBs, siteRegistry, cfg, db,
		func(siteCfg *config.Config, siteDB *gorm.DB) error {
			return registerProducts(siteCfg, algoWrite, siteDB)
		}); err != nil {
		c.UI.Error(fmt.Sprintf("error registering products: %v", err))
		return 1
	}

	// Register document types.
	// TODO: remove this and use the database for all document type lookups.
	docTypes := map[string]hcd.Doc{
		"frd": &hcd.FRD{},
		"prd": &hcd.PRD{},
		"rfc": &hcd.RFC{},
	}
	for name, dt := range docTypes {
		if err = doctypes.Register(name, dt); err != nil {
			c.UI.Error(fmt.Sprintf("error registering %q doc type: %v", name, err))
			return 1
		}
	}

	// Load workspace project configuration if configured.
	var projectConfig *projectconfig.Config
	if cfg.Providers != nil && cfg.Providers.ProjectsConfigPath != "" {
		c.UI.Info(fmt.Sprintf("Loading workspace projects from: %s", cfg.Providers.ProjectsConfigPath))

		projectConfig, err = projectconfig.LoadConfig(cfg.Providers.ProjectsConfigPath)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error loading workspace projects config: %v", err))
			return 1
		}

		// Validate project configuration
		validator := projectconfig.NewValidator()
		if err := validator.Validate(projectConfig); err != nil {
			c.UI.Error(fmt.Sprintf("invalid workspace projects config: %v", err))
			return 1
		}

		c.UI.Info(fmt.Sprintf("Loaded %d workspace projects from HCL config", len(projectConfig.Projects)))

		// Sync workspace projects into every site.
		//
		// The projects table lives in the site's own schema, and
		// /api/v2/workspace-projects reads whichever site is being served. A
		// sync into the primary site alone would leave every other site
		// reporting no projects at all, with the config file plainly declaring
		// them.
		c.UI.Info("Syncing workspace projects to database...")
		if err := forEachSite(siteDBs, siteRegistry, cfg, db,
			func(_ *config.Config, siteDB *gorm.DB) error {
				return projectConfig.SyncToDatabase(siteDB, cfg.Providers.ProjectsConfigPath)
			}); err != nil {
			c.UI.Error(fmt.Sprintf("error syncing workspace projects to database: %v", err))
			return 1
		}
		c.UI.Info("Workspace projects synced to database successfully")

		// Register projects with instance identity (links projects to this instance)
		c.UI.Info("Registering workspace projects with instance identity...")
		if err := forEachSite(siteDBs, siteRegistry, cfg, db,
			func(_ *config.Config, siteDB *gorm.DB) error {
				return projects.RegisterAllProjects(ctx, siteDB, projectConfig, instanceLogger)
			}); err != nil {
			c.UI.Warn(fmt.Sprintf("error registering projects with instance: %v", err))
			// Non-fatal - projects are still in the database via SyncToDatabase.
		} else {
			c.UI.Info("Workspace projects registered with instance successfully")
		}

		// Log active projects
		activeProjects := projectConfig.GetActiveProjects()
		for _, proj := range activeProjects {
			c.UI.Info(fmt.Sprintf("  - %s (%s): %d provider(s)",
				proj.Name, proj.Title, len(proj.Providers)))

			// Log migration status
			if proj.IsInMigration() {
				sourceProvider, srcErr := proj.GetSourceProvider()
				targetProvider, tgtErr := proj.GetTargetProvider()
				if srcErr == nil && tgtErr == nil {
					c.UI.Info(fmt.Sprintf("    Migration: %s -> %s",
						sourceProvider.Type, targetProvider.Type))
				}
			}
		}
	} else {
		// No HCL config provided, try loading from database.
		//
		// This reads the primary site only. It is used to populate
		// Server.ProjectConfig, which no handler consults today -- handlers go
		// to the database through their own scoped connection -- so the site it
		// comes from does not currently matter. It will the moment anything
		// starts reading that field.
		c.UI.Info("No workspace projects config path provided, loading from database...")
		projectConfig, err = projectconfig.LoadFromDatabase(db)
		if err != nil {
			c.UI.Warn(fmt.Sprintf("No workspace projects found in database: %v", err))
			// projectConfig remains nil, which is handled by the API endpoints
		} else {
			c.UI.Info(fmt.Sprintf("Loaded %d workspace projects from database", len(projectConfig.Projects)))
		}
	}

	// Build one workspace provider per site.
	//
	// The local adapter roots every path at <base_path>/<domain>, so two sites
	// never write documents into the same directory. Without this the domain
	// scoping in the adapter config exists but nothing sets it, and every site
	// shares one flat workspace.
	var siteWorkspace map[domain.Name]workspace.WorkspaceProvider
	if siteRegistry.Len() > 0 {
		if workspaceProviderName != "local" {
			c.UI.Error(fmt.Sprintf(
				"multi-site hosting supports only the local workspace provider, not %q.\n"+
					"Google Workspace and SharePoint address one tenant's storage, so "+
					"every site would share it.\n"+
					"Run one process per site to use them.", workspaceProviderName))
			return 1
		}

		siteWorkspace = make(map[domain.Name]workspace.WorkspaceProvider, siteRegistry.Len())
		for _, site := range siteRegistry.Sites() {
			localCfg := cfg.LocalWorkspace.ToLocalAdapterConfigForDomain(site.Domain)
			adapter, err := localadapter.NewAdapter(localCfg)
			if err != nil {
				c.UI.Error(fmt.Sprintf(
					"error initializing workspace for site %s: %v", site.Domain, err))
				return 1
			}
			siteWorkspace[site.Domain] = localadapter.NewWorkspaceAdapter(adapter)
			c.Log.Info("workspace for site",
				"domain", site.Domain.String(), "path", localCfg.Root())
		}
	}

	// Build one search provider per site.
	//
	// Search indexes are namespaced by the same injective encoding as the
	// database schemas, so two sites can never share an index. A shared index
	// would return one tenant's documents to another's search -- the same leak
	// the per-site schema closes on the query side.
	var siteSearch map[domain.Name]search.Provider
	if siteRegistry.Len() > 0 {
		if searchProviderName == providerAlgolia {
			c.UI.Error(
				"multi-site hosting does not support the Algolia search provider.\n" +
					"The frontend queries Algolia through a proxy that is not site-aware, " +
					"so searches would run against the wrong index.\n" +
					"Use meilisearch or bleve, or run one process per site.")
			return 1
		}

		siteSearch = make(map[domain.Name]search.Provider, siteRegistry.Len())
		for _, site := range siteRegistry.Sites() {
			provider, err := buildSearchProvider(cfg, searchProviderName, site.Domain)
			if err != nil {
				c.UI.Error(fmt.Sprintf(
					"error initializing search for site %s: %v", site.Domain, err))
				return 1
			}
			siteSearch[site.Domain] = provider
			c.Log.Info("search namespace for site",
				"domain", site.Domain.String(),
				"namespace", site.Domain.SearchNamespace())
		}
	}

	// If using the local workspace provider, index its documents into search on
	// startup so the index matches the filesystem.
	//
	// This runs per site. Indexing the process-wide workspace into the
	// process-wide indexes instead would walk one flat directory and write to
	// the unprefixed index names -- so every site's search would be built from
	// the wrong files, or from none.
	if workspaceProviderName == "local" {
		indexOne := func(name domain.Name, ws workspace.WorkspaceProvider, sp search.Provider) {
			wsAdapter, ok := ws.(*localadapter.WorkspaceAdapter)
			if !ok {
				return
			}

			label := "local workspace"
			if !name.IsZero() {
				label = name.String()
			}
			c.UI.Info(fmt.Sprintf("Indexing documents from %s into search provider...", label))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			indexer := localadapter.NewDocumentIndexer(wsAdapter.GetAdapter(), sp, c.Log)
			if err := indexer.IndexAll(ctx); err != nil {
				// Not fatal: the server still works, documents just may not
				// appear in search until reindexed.
				c.UI.Warn(fmt.Sprintf("warning: indexing %s failed: %v", label, err))
				c.UI.Warn("documents may not appear in search results until manually indexed")

				return
			}
			c.UI.Info(fmt.Sprintf("Indexing %s completed successfully", label))
		}

		if siteRegistry.Len() > 0 {
			for _, site := range siteRegistry.Sites() {
				indexOne(site.Domain, siteWorkspace[site.Domain], siteSearch[site.Domain])
			}
		} else {
			indexOne(domain.Name{}, workspaceProvider, searchProvider)
		}
	}

	// Build the session signer.
	var sessionKey, sessionTTL string
	if cfg.Session != nil {
		sessionKey, sessionTTL = cfg.Session.Key, cfg.Session.TTL
	}
	sessionOpts, err := session.Resolve(sessionKey, sessionTTL)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error configuring sessions: %v", err))
		return 1
	}
	if sessionOpts.Ephemeral {
		c.UI.Warn(fmt.Sprintf(
			"warning: no session signing key configured; generated an ephemeral one. "+
				"All users will be logged out when this process restarts, and sessions "+
				"will not work across multiple instances. Set %s to fix this.",
			session.KeyEnvVar))
	}
	sessionSigner, err := session.NewSignerFromOptions(sessionOpts)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error configuring sessions: %v", err))
		return 1
	}

	type serveMux interface {
		Handle(pattern string, handler http.Handler)
		ServeHTTP(http.ResponseWriter, *http.Request)
	}
	var mux serveMux
	if dd.Enabled {
		mux = httptrace.NewServeMux()
	} else {
		mux = http.NewServeMux()
	}

	srv := server.Server{
		SiteDBs:           siteDBs,
		SiteSearch:        siteSearch,
		SiteWorkspace:     siteWorkspace,
		SearchProvider:    searchProvider,
		WorkspaceProvider: workspaceProvider,
		GWService:         goog,
		SharePoint:        sharepointSvc,
		Config:            cfg,
		DB:                db,
		Jira:              jiraSvc,
		Logger:            c.Log,
		ProjectConfig:     projectConfig,
	}

	// Define handlers for authenticated endpoints.
	// All API endpoints use v2.
	authenticatedEndpoints := []endpoint{
		{apiv2.ApprovalsHandler(srv), "/api/v2/approvals/"},
		{apiv2.DocumentTypesHandler(srv), "/api/v2/document-types"},
		{apiv2.DocumentHandler(srv), "/api/v2/documents/"}, // Handles /content suffix too
		{apiv2.DraftsHandler(srv), "/api/v2/drafts"},
		{apiv2.DraftsDocumentHandler(srv), "/api/v2/drafts/"},
		{apiv2.GroupsHandler(srv), "/api/v2/groups"},
		{apiv2.JiraIssueHandler(srv), "/api/v2/jira/issues/"},
		{apiv2.JiraIssuePickerHandler(srv), "/api/v2/jira/issue/picker"},
		{apiv2.MeHandler(srv), "/api/v2/me"},
		{apiv2.MeRecentlyViewedDocsHandler(srv), "/api/v2/me/recently-viewed-docs"},
		{apiv2.MeRecentlyViewedProjectsHandler(srv),
			"/api/v2/me/recently-viewed-projects"},
		{apiv2.MeReviewsHandler(srv), "/api/v2/me/reviews"},
		{apiv2.MeSubscriptionsHandler(srv), "/api/v2/me/subscriptions"},
		{apiv2.MigrationsHandler(srv), "/api/v2/migrations/"},
		{apiv2.PeopleDataHandler(srv), "/api/v2/people"},
		{apiv2.ProductsHandler(srv), "/api/v2/products"},
		{apiv2.ProjectsHandler(srv), "/api/v2/projects"},
		{apiv2.ProjectHandler(srv), "/api/v2/projects/"},
		{apiv2.ProvidersHandler(srv), "/api/v2/providers"},
		{apiv2.ProvidersHandler(srv), "/api/v2/providers/"},
		{apiv2.ReviewsHandler(srv), "/api/v2/reviews/"},
		{apiv2.SearchHandler(srv), "/api/v2/search/"},
		{apiv2.SemanticSearchHandler(srv), "/api/v2/search/semantic"}, // RFC-088: Semantic search
		{apiv2.HybridSearchHandler(srv), "/api/v2/search/hybrid"},     // RFC-088: Hybrid search
		// RFC-088: Similar documents. Registered on the full path rather than
		// on "/api/v2/documents/", which DocumentHandler already owns --
		// http.ServeMux panics on a duplicate pattern, so this took the whole
		// server down at startup rather than merely shadowing the handler.
		// The trailing "/" makes it a subtree match, which is what
		// "/api/v2/documents/{id}/similar" needs.
		{apiv2.SimilarDocumentsHandler(srv), "/api/v2/documents/{documentID}/similar"},
		{apiv2.AnalyticsHandler(srv), "/api/v2/web/analytics"},
		{apiv2.WorkspaceProjectsHandler(srv), "/api/v2/workspace-projects"},
		{apiv2.WorkspaceProjectHandler(srv), "/api/v2/workspace-projects/"},
	}

	// Add Algolia-specific endpoints if using Algolia search provider.
	if searchProviderName == providerAlgolia && algoSearch != nil {
		authenticatedEndpoints = append(authenticatedEndpoints, endpoint{
			algolia.AlgoliaProxyHandler(algoSearch, algoliaClientCfg, c.Log),
			"/1/indexes/",
		})
	}

	// Define handlers for unauthenticated endpoints.
	unauthenticatedEndpoints := []endpoint{
		{healthHandler(db), "/health"},
		{openapi.Handler(), "/openapi.json"},
		{http.StripPrefix("/pub/", pub.Handler()), "/pub/"},
		{apiv2.IndexerHandler(srv), "/api/v2/indexer/"},                                  // Indexer API (handles own token auth)
		{apiv2.EdgeSyncAuthMiddleware(srv, apiv2.EdgeSyncHandler(srv)), "/api/v2/edge/"}, // Edge sync API (token auth)
	}

	// Add Dex OIDC auth endpoints if Dex is configured
	if cfg.Dex != nil && !cfg.Dex.Disabled {
		unauthenticatedEndpoints = append(unauthenticatedEndpoints,
			endpoint{api.LoginHandler(*cfg, c.Log), "/auth/login"},
			endpoint{api.CallbackHandler(*cfg, sessionSigner, c.Log), "/auth/callback"},
			endpoint{api.LogoutHandler(c.Log), "/auth/logout"},
		)
	}

	// Web config and setup endpoints are always unauthenticated so the
	// frontend can load and determine the auth provider before attempting to
	// authenticate. They are registered once, with or without an Algolia
	// client: registering them in each branch of a conditional is how a
	// duplicate pattern gets added without anyone noticing, and http.ServeMux
	// panics on those at startup rather than picking one.
	unauthenticatedEndpoints = append(unauthenticatedEndpoints,
		endpoint{web.ConfigHandler(cfg, algoSearch, c.Log), "/api/v2/web/config"},
		endpoint{apiv2.SetupStatusHandler(c.flagConfig, c.Log), "/api/v2/setup/status"},
		endpoint{apiv2.SetupConfigureHandler(c.Log), "/api/v2/setup/configure"},
		endpoint{apiv2.OllamaValidateHandler(c.Log), "/api/v2/setup/validate-ollama"},
	)

	// The short-link redirector reads from Algolia directly, so it exists only
	// on that path.
	if searchProviderName == providerAlgolia && algoSearch != nil {
		unauthenticatedEndpoints = append(unauthenticatedEndpoints,
			endpoint{links.RedirectHandler(algoSearch, algoliaClientCfg, c.Log), "/l/"},
		)
	}

	// SPA handler - conditionally authenticated based on if Okta or Dex is enabled.
	spaEndpoints := []endpoint{
		{web.Handler(), "/"},
	}

	// If Okta or Dex is enabled, add the SPA handler as an authenticated endpoint.
	if (cfg.Okta != nil && !cfg.Okta.Disabled) || (cfg.Dex != nil && !cfg.Dex.Disabled) {
		authenticatedEndpoints = append(authenticatedEndpoints, spaEndpoints...)
	} else {
		// If both Okta and Dex are disabled, add the SPA handler as an unauthenticated endpoint.
		unauthenticatedEndpoints = append(unauthenticatedEndpoints, spaEndpoints...)
	}
	// Fail before listening if any site is missing a per-tenant resource. The
	// alternative is discovering it one tenant at a time, in production.
	siteNames := make([]domain.Name, 0, siteRegistry.Len())
	for _, site := range siteRegistry.Sites() {
		siteNames = append(siteNames, site.Domain)
	}
	if err := srv.VerifySiteResources(siteNames); err != nil {
		c.UI.Error(fmt.Sprintf("error verifying site resources: %v", err))
		return 1
	}

	// Register handlers.
	for _, e := range authenticatedEndpoints {
		// Note: auth.AuthenticateRequest supports Dex, Okta, or Google authentication.
		// When using non-Google workspace providers with non-Dex authentication,
		// Okta authentication must be enabled.
		if goog == nil && (cfg.Okta == nil || cfg.Okta.Disabled) && (cfg.Dex == nil || cfg.Dex.Disabled) {
			c.UI.Error("error: when using non-Google workspace providers, Okta or Dex authentication must be enabled")
			return 1
		}
		mux.Handle(
			e.pattern,
			auth.AuthenticateRequest(
				*cfg, goog, sharepointSvc, sessionSigner, c.Log, e.handler),
		)
	}
	for _, e := range unauthenticatedEndpoints {
		mux.Handle(e.pattern, e.handler)
	}

	// Resolve the request's site before anything else sees it, so that auth,
	// storage, and redirects all read the same domain from the context. The
	// resolver wraps the whole mux -- unauthenticated endpoints included --
	// because /auth/callback has to know which site it is minting a session
	// for.
	var rootHandler http.Handler = mux
	if siteRegistry.Len() > 0 {
		resolver, err := middleware.NewDomainResolver(
			siteRegistry, cfg.TrustedProxies, c.Log)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error building domain resolver: %v", err))
			return 1
		}
		// The liveness probe answers before hostname routing. A monitor
		// hitting the loopback listener directly sends Host: 127.0.0.1, which
		// is no site, so behind the resolver it would get 421 and the service
		// would look down while being perfectly healthy.
		rootHandler = middleware.Exempt(
			"/health", healthHandler(db), resolver.Middleware(mux))
	}

	ginRouter := gin.New()

	// Turn a handler panic into a 500 and a log line rather than a dropped
	// connection. net/http already keeps a panic from killing the process, but
	// it closes the connection without a response, so the client sees a
	// transport error and the cause appears only in a stack trace on stderr.
	//
	// This is a backstop, not a licence: a panic is still a bug, and the tests
	// that exercise the config endpoint exist because two of them shipped.
	ginRouter.Use(gin.CustomRecoveryWithWriter(nil, func(gc *gin.Context, recovered any) {
		c.Log.Error("panic serving request",
			"error", recovered,
			"method", gc.Request.Method,
			"host", gc.Request.Host,
			"path", gc.Request.URL.Path)
		gc.AbortWithStatus(http.StatusInternalServerError)
	}))

	ginRouter.NoRoute(gin.WrapH(rootHandler))

	httpServer := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           ginRouter,
		ReadHeaderTimeout: 30 * time.Second, // Prevent Slowloris attacks
	}
	go func() {
		if cfg.Server.TLSEnabled {
			c.Log.Info("Starting server with TLS/HTTPS", "addr", cfg.Server.Addr, "tls_enabled", true)
			if err := httpServer.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != http.ErrServerClosed {
				c.Log.Error(fmt.Sprintf("error starting TLS listener: %v", err))
				os.Exit(1)
			}
			return
		}

		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			c.Log.Error(fmt.Sprintf("error starting listener: %v", err))
			os.Exit(1)
		}
	}()

	if cfg.OpenTelemetry != nil && cfg.OpenTelemetry.Enabled {
		otelHandler := otel.Handler(cfg.OpenTelemetry, c.Log.Named("otel"))
		otelServer := &http.Server{
			Addr:              otel.ListenerAddr(cfg.OpenTelemetry),
			Handler:           otelHandler,
			ReadHeaderTimeout: 30 * time.Second,
		}
		go func() {
			c.Log.Info("Starting OpenTelemetry listener",
				"addr", otelServer.Addr,
				"prometheus_url", cfg.OpenTelemetry.PrometheusURL)
			if err := otelServer.ListenAndServe(); err != http.ErrServerClosed {
				c.Log.Error(fmt.Sprintf("error starting OpenTelemetry listener: %v", err))
				os.Exit(1)
			}
		}()
	}

	// One search outbox relay per site.
	//
	// The outbox lives in the site's own schema, so a single relay reading the
	// primary site's database would leave every other site's documents
	// unindexed -- silently, since nothing errors: the rows simply sit there
	// and search returns nothing for that tenant.
	if searchProvider != nil {
		ctx, cancel := context.WithCancel(c.Context)
		defer cancel()

		startRelay := func(name domain.Name, siteDB *gorm.DB, provider search.Provider) error {
			logger := c.Log.Named("search-outbox-relay")
			if !name.IsZero() {
				logger = logger.With("site", name.String())
			}

			searchRelay, err := searchoutbox.New(searchoutbox.Config{
				DB:       siteDB,
				Provider: provider,
				Logger:   logger,
			})
			if err != nil {
				return err
			}

			go func() {
				logger.Info("starting search outbox relay service",
					"search_provider", provider.Name())
				if err := searchRelay.Start(ctx); err != nil && err != context.Canceled {
					logger.Error(fmt.Sprintf("search outbox relay service failed: %v", err))
				}
			}()

			return nil
		}

		if siteRegistry.Len() > 0 {
			if err := siteDBs.Each(func(name domain.Name, siteDB *gorm.DB) error {
				return startRelay(name, siteDB, siteSearch[name])
			}); err != nil {
				c.Log.Error(fmt.Sprintf("failed to create search outbox relay service: %v", err))
				return 1
			}
		} else if db != nil {
			if err := startRelay(domain.Name{}, db, searchProvider); err != nil {
				c.Log.Error(fmt.Sprintf("failed to create search outbox relay service: %v", err))
				return 1
			}
		}
	}

	// RFC-088: Start outbox relay goroutine (publishes outbox events to Redpanda)
	// The relay runs in the main server process to keep database writes transactional
	if cfg.Indexer != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		brokers := kafka.GetBrokers(cfg)
		baseTopic := kafka.GetDocumentRevisionTopic(cfg)

		// One relay per site, each on its own topic.
		//
		// The outbox lives in the site's schema, so a single relay would strand
		// every other site's events. Publishing them all to one topic instead
		// would be worse: the event carries no site, so a consumer could not
		// tell which tenant a document belongs to and would write it back to
		// whichever one it happened to be pointed at.
		startRelay := func(name domain.Name, siteDB *gorm.DB) error {
			topic := baseTopic
			logger := c.Log.Named("outbox-relay")
			if !name.IsZero() {
				topic = baseTopic + "." + name.SearchNamespace()
				logger = logger.With("site", name.String())
			}

			relayService, err := relay.New(relay.Config{
				DB:           siteDB,
				Brokers:      brokers,
				Topic:        topic,
				PollInterval: cfg.Indexer.PollInterval,
				BatchSize:    cfg.Indexer.BatchSize,
				Logger:       logger,
			})
			if err != nil {
				return err
			}

			go func() {
				logger.Info("starting outbox relay service", "topic", topic)
				if err := relayService.Start(ctx); err != nil {
					logger.Error(fmt.Sprintf("outbox relay service failed: %v", err))
				}
			}()

			// Cleanup runs every 24 hours.
			go func() {
				ticker := time.NewTicker(24 * time.Hour)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						if err := relayService.CleanupOldEntries(7 * 24 * time.Hour); err != nil {
							logger.Error(fmt.Sprintf("failed to cleanup old outbox entries: %v", err))
						}
					}
				}
			}()

			return nil
		}

		var relayErr error
		if siteRegistry.Len() > 0 {
			relayErr = siteDBs.Each(startRelay)
		} else if db != nil {
			relayErr = startRelay(domain.Name{}, db)
		}
		if relayErr != nil {
			c.Log.Error(fmt.Sprintf("failed to create outbox relay service: %v", relayErr))
			return 1
		}
	}

	// RFC-089: Start migration worker goroutine (processes migration tasks)
	// The worker runs in the main server process to handle document migrations
	if cfg.Migration != nil && cfg.Migration.Enabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Set defaults for migration config
		pollInterval := 5 * time.Second
		if cfg.Migration.PollInterval > 0 {
			pollInterval = cfg.Migration.PollInterval
		}

		maxConcurrency := 5
		if cfg.Migration.MaxConcurrency > 0 {
			maxConcurrency = cfg.Migration.MaxConcurrency
		}

		workerCfg := &migration.WorkerConfig{
			PollInterval:   pollInterval,
			MaxConcurrency: maxConcurrency,
		}

		// One worker per site.
		//
		// Migration jobs are created through /api/v2/migrations, which is
		// scoped, so a job lands in the schema of the site that asked for it.
		// A single worker reading the primary site's database would leave every
		// other site's jobs queued forever -- no error, just a job that never
		// starts.
		startWorker := func(siteName domain.Name, siteDB *gorm.DB,
			ws workspace.WorkspaceProvider,
		) error {
			sqlDB, err := siteDB.DB()
			if err != nil {
				return fmt.Errorf("getting the SQL handle: %w", err)
			}

			// Providers are per site for the same reason the database is: the
			// worker reads and writes documents, and those live in the site's
			// own workspace.
			providerMap := map[string]workspace.WorkspaceProvider{
				workspaceProviderName: ws,
			}

			logger := c.Log.Named("migration-worker")
			if !siteName.IsZero() {
				logger = logger.With("site", siteName.String())
			}

			worker := migration.NewWorker(sqlDB, providerMap, logger, workerCfg)

			go func() {
				logger.Info("starting migration worker",
					"poll_interval", pollInterval,
					"max_concurrency", maxConcurrency)
				if err := worker.Start(ctx); err != nil && err != context.Canceled {
					logger.Error(fmt.Sprintf("migration worker failed: %v", err))
				}
			}()

			return nil
		}

		var workerErr error
		if siteRegistry.Len() > 0 {
			workerErr = siteDBs.Each(func(name domain.Name, siteDB *gorm.DB) error {
				return startWorker(name, siteDB, siteWorkspace[name])
			})
		} else {
			workerErr = startWorker(domain.Name{}, db, workspaceProvider)
		}
		if workerErr != nil {
			c.Log.Error(fmt.Sprintf("failed to start migration worker: %v", workerErr))
			return 1
		}

		c.Log.Info("RFC-089 migration system enabled",
			"write_strategy", cfg.Migration.WriteStrategy,
			"read_strategy", cfg.Migration.ReadStrategy)

		// Cancel worker context on shutdown
		defer cancel()
	}

	return c.WaitForInterrupt(c.ShutdownServer(httpServer))
}

// healthHandler responds with the health of the service.
func healthHandler(db *gorm.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if db != nil {
			stats, err := searchoutbox.GetStats(db, time.Now())
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status":       "OK",
					"searchOutbox": stats,
				})
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			// Error already logged by HTTP server
			return
		}
	})
}

func validateSharePointConfig(cfg *config.SharePointConfig) error {
	if cfg.ClientID == "" {
		return fmt.Errorf("SharePoint client ID is required")
	}
	if cfg.ClientSecret == "" {
		return fmt.Errorf("SharePoint client secret is required")
	}
	if cfg.TenantID == "" {
		return fmt.Errorf("SharePoint tenant ID is required")
	}
	if cfg.SiteID == "" {
		return fmt.Errorf("SharePoint site ID is required")
	}
	if cfg.DriveID == "" {
		return fmt.Errorf("SharePoint drive ID is required")
	}
	return nil
}

// ShutdownServer gracefully shuts down the HTTP server.
func (c *Command) ShutdownServer(s *http.Server) func() {
	return func() {
		c.Log.Debug("shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			c.Log.Error(fmt.Sprintf("error shutting down HTTP server: %v", err))
		}
	}
}

// registerDocumentTypes registers all products configured in the application
// config in the database.
func registerDocumentTypes(cfg config.Config, db *gorm.DB) error {
	// `template` is a Google file ID, so it is required only when Google
	// Workspace is the workspace provider. It used to be mandatory in the
	// schema, which meant a local deployment had to invent one before the
	// config would even parse -- and the shipped example, which sensibly left
	// it out, could not be loaded at all. Checking it here puts the
	// requirement where it is actually true.
	usingGoogle := cfg.Providers == nil || cfg.Providers.Workspace == "" ||
		cfg.Providers.Workspace == "google"

	for _, d := range cfg.DocumentTypes.DocumentType {
		if usingGoogle && d.Template == "" {
			return fmt.Errorf(
				"document type %q has no template; the Google Workspace provider "+
					"needs a Google Docs file ID there", d.Name)
		}

		// Marshal Checks to JSON.
		checksJSON, err := json.Marshal(d.Checks)
		if err != nil {
			return fmt.Errorf("error marshaling checks to JSON: %w", err)
		}

		// Convert custom fields to model's version.
		var cfs []models.DocumentTypeCustomField
		for _, c := range d.CustomFields {
			cf := models.DocumentTypeCustomField{
				Name:     c.Name,
				ReadOnly: c.ReadOnly,
			}

			// Convert custom field type.
			t := strings.ToLower(c.Type)
			switch t {
			case "string":
				cf.Type = models.StringDocumentTypeCustomFieldType
			case "person":
				cf.Type = models.PersonDocumentTypeCustomFieldType
			case "people":
				cf.Type = models.PeopleDocumentTypeCustomFieldType
			case "":
				return fmt.Errorf("missing document type custom field")
			default:
				return fmt.Errorf("invalid document type custom field: %s", t)
			}

			cfs = append(cfs, cf)
		}

		dt := models.DocumentType{
			Name:         d.Name,
			LongName:     d.LongName,
			Description:  d.Description,
			FlightIcon:   d.FlightIcon,
			Checks:       checksJSON,
			CustomFields: cfs,
		}

		if d.MoreInfoLink != nil {
			dt.MoreInfoLinkText = d.MoreInfoLink.Text
			dt.MoreInfoLinkURL = d.MoreInfoLink.URL
		}

		// Upsert document type.
		if err := dt.Upsert(db); err != nil {
			return fmt.Errorf("error upserting document type: %w", err)
		}
	}

	return nil
}

// registerProducts registers all products configured in the application config
// in the database and Algolia.
// TODO: products are currently needed in Algolia for legacy reasons - remove
// this when possible.
func registerProducts(
	cfg *config.Config, algo *algolia.Client, db *gorm.DB) error {

	productsObj := structs.Products{
		ObjectID: "products",
		Data:     make(map[string]structs.ProductData, 0),
	}

	// A config with no products block leaves this nil. That is an ordinary
	// deployment -- the shipped multi-site example is one -- not a reason to
	// crash at startup.
	if cfg.Products == nil {
		return nil
	}

	for _, p := range cfg.Products.Product {
		// Upsert product in database.
		pm := models.Product{
			Name:         p.Name,
			Abbreviation: p.Abbreviation,
		}
		if err := pm.Upsert(db); err != nil {
			return fmt.Errorf("error upserting product: %w", err)
		}

		// Add product to Algolia products object (only if using Algolia).
		productsObj.Data[p.Name] = structs.ProductData{
			Abbreviation: p.Abbreviation,
		}
	}

	// Save Algolia products object (skip if using different search provider).
	if algo != nil {
		res, err := algo.Internal.SaveObject(&productsObj)
		if err != nil {
			return fmt.Errorf("error saving Algolia products object: %w", err)
		}
		err = res.Wait()
		if err != nil {
			return fmt.Errorf("error saving Algolia products object: %w", err)
		}
	}

	return nil
}

// generateIndexerToken generates a registration token for indexers and writes it to a file.
func generateIndexerToken(db *gorm.DB, tokenPath string, logger hclog.Logger) error {
	// Create parent directory if it doesn't exist
	tokenDir := filepath.Dir(tokenPath)
	//nolint:gosec // tokenDir is derived from config, not user input
	if err := os.MkdirAll(tokenDir, 0o750); err != nil {
		return fmt.Errorf("error creating token directory: %w", err)
	}

	// Generate a registration token
	token, err := models.GenerateToken("registration")
	if err != nil {
		return fmt.Errorf("error generating token: %w", err)
	}

	// Store the token in the database
	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour expiration for registration tokens
	indexerToken := models.IndexerToken{
		TokenType: "registration",
		ExpiresAt: &expiresAt,
	}

	if err := indexerToken.Create(db, token); err != nil {
		return fmt.Errorf("error storing token: %w", err)
	}

	// Write token to file
	//nolint:gosec // G703: tokenPath is constructed from config, not user input
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		return fmt.Errorf("error writing token file: %w", err)
	}

	logger.Info("generated indexer registration token",
		"token_id", indexerToken.ID,
		"expires_at", expiresAt)

	return nil
}

// runMigrations runs database migrations using the internal migrate package.
// This is called automatically during server startup to ensure the database schema is up to date.
func runMigrations(driver, dsn string) error {
	// Open database connection
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := migrate.RunMigrations(sqlDB, driver); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// forEachSite runs fn once per site, or once against the process-wide database
// when no sites are configured.
//
// fn receives a config whose BaseURL is that site's own. That matters more
// than it looks: instance identity keys on BaseURL, so passing the global one
// registered every site under the first site's hostname -- two rows, same
// identity, and nothing to tell them apart afterwards.
//
// Startup work that writes content configuration also has to reach every
// schema; a site whose document types were never registered looks empty rather
// than broken, which is the harder failure to notice.
func forEachSite(
	siteDBs *dbpkg.SiteDBs, registry *sites.Registry, cfg *config.Config,
	fallback *gorm.DB, fn func(*config.Config, *gorm.DB) error,
) error {
	if registry == nil || registry.Len() == 0 {
		return fn(cfg, fallback)
	}

	for _, site := range registry.Sites() {
		siteDB, err := siteDBs.For(server.NewDomainContext(site.Domain))
		if err != nil {
			return fmt.Errorf("site %q: %w", site.Domain, err)
		}

		siteCfg := *cfg
		siteCfg.BaseURL = site.BaseURL

		if err := fn(&siteCfg, siteDB); err != nil {
			return fmt.Errorf("site %q: %w", site.Domain, err)
		}
	}

	return nil
}

// buildSearchProvider constructs a search provider scoped to one site, or to
// the whole process when d is the zero domain.
//
// Index naming is the only thing that differs between the two: Meilisearch and
// Algolia take a name prefix, Bleve a subdirectory, since a Bleve index is a
// directory rather than a name.
func buildSearchProvider(
	cfg *config.Config, providerName string, d domain.Name,
) (search.Provider, error) {
	switch providerName {
	case providerAlgolia:
		if cfg.Algolia == nil {
			return nil, fmt.Errorf("algolia configuration is required")
		}
		scoped := config.ScopeAlgoliaForDomain(cfg.Algolia, d)

		return searchalgolia.NewAdapter(&searchalgolia.Config{
			AppID:           scoped.AppID,
			WriteAPIKey:     scoped.WriteAPIKey,
			DocsIndexName:   scoped.DocsIndexName,
			DraftsIndexName: scoped.DraftsIndexName,
		})

	case "meilisearch":
		if cfg.Meilisearch == nil {
			return nil, fmt.Errorf("meilisearch configuration is required")
		}

		return meilisearchadapter.NewAdapter(
			cfg.Meilisearch.ToMeilisearchAdapterConfigForDomain(d))

	case "bleve":
		if cfg.Bleve == nil {
			return nil, fmt.Errorf("bleve configuration is required")
		}

		return bleveadapter.NewAdapter(&bleveadapter.Config{
			IndexPath: cfg.Bleve.IndexPathForDomain(d),
		})
	}

	return nil, fmt.Errorf("unknown search provider %q", providerName)
}
