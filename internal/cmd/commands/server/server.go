package server

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	addin "github.com/hashicorp-forge/hermes/hermes-plugin"
	"github.com/hashicorp-forge/hermes/internal/api"
	apiv2 "github.com/hashicorp-forge/hermes/internal/api/v2"
	"github.com/hashicorp-forge/hermes/internal/auth"
	"github.com/hashicorp-forge/hermes/internal/cmd/base"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/datadog"
	"github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/internal/jira"
	"github.com/hashicorp-forge/hermes/internal/middleware"
	"github.com/hashicorp-forge/hermes/internal/pkg/doctypes"
	"github.com/hashicorp-forge/hermes/internal/pub"
	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/internal/structs"
	"github.com/hashicorp-forge/hermes/pkg/algolia"
	hcd "github.com/hashicorp-forge/hermes/pkg/hashicorpdocs"
	"github.com/hashicorp-forge/hermes/pkg/links"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
	searchalgolia "github.com/hashicorp-forge/hermes/pkg/search/adapters/algolia"
	meilisearchadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	gw "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/google"
	localadapter "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/local"
	"github.com/hashicorp-forge/hermes/web"
	"github.com/hashicorp/go-hclog"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gorm.io/gorm"
)

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
	flagOktaDisabled      bool
}

type endpoint struct {
	pattern string
	handler http.Handler
}

func (c *Command) Synopsis() string {
	return "Run the server"
}

func (c *Command) Help() string {
	return `Usage: hermes server

  This command runs the Hermes web server.` + c.Flags().Help()
}

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
		"[HERMES_WORKSPACE_PROVIDER] Workspace provider to use (e.g., 'google', 'local'). "+
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
		&c.flagOidcClientID, "oidc-client-id", "",
		"[HERMES_SERVER_OIDC_CLIENT_ID] OIDC client ID.",
	)
	f.BoolVar(
		&c.flagOidcDisabled, "oidc-disabled", false,
		"[HERMES_SERVER_OIDC_DISABLED] Disable OIDC authorization.",
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
	if val, ok := os.LookupEnv("HERMES_SERVER_OIDC_AUTH_SERVER_URL"); ok {
		cfg.OidcAlb.AuthServerURL = val
	}
	if c.flagOidcAuthServerURL != f.Lookup("oidc-auth-server-url").DefValue {
		cfg.OidcAlb.AuthServerURL = c.flagOidcAuthServerURL
	}
	if val, ok := os.LookupEnv("HERMES_SERVER_OIDC_CLIENT_ID"); ok {
		cfg.OidcAlb.ClientID = val
	}
	if c.flagOidcClientID != f.Lookup("oidc-client-id").DefValue {
		cfg.OidcAlb.ClientID = c.flagOidcClientID
	}
	if val, ok := os.LookupEnv("HERMES_SERVER_OIDC_DISABLED"); ok {
		if val == "" || val == "false" {
			// Keep OIDC ALB enabled if the env var value is an empty string or "false".
		} else {
			cfg.OidcAlb.Disabled = true
		}
	}
	if val, ok := os.LookupEnv("HERMES_SERVER_OIDC_JWT_SIGNER"); ok {
		cfg.OidcAlb.JWTSigner = val
	}
	if c.flagOidcDisabled {
		cfg.OidcAlb.Disabled = true
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
		case "google":
			// Disable both Dex and Okta to fall back to Google
			if cfg.Dex != nil {
				cfg.Dex.Disabled = true
			}
			if cfg.Okta != nil {
				cfg.Okta.Disabled = true
			}
			c.Log.Info("auth provider selection", "provider", "google", "source", "flag/env")
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

	// Log comprehensive configuration overview
	logInstanceOverview(c.Log, cfg)

	// Build configuration for OIDC ALB authentication.
	if !cfg.OidcAlb.Disabled {
		// Check for required OIDC ALB configuration.
		if cfg.OidcAlb.AuthServerURL == "" {
			c.UI.Error("error initializing server: OIDC ALB authorization server URL is required")
			return 1
		}
		if cfg.OidcAlb.AWSRegion == "" {
			c.UI.Error("error initializing server: OIDC ALB AWS region is required")
			return 1
		}
		if cfg.OidcAlb.ClientID == "" {
			c.UI.Error("error initializing server: OIDC ALB client ID is required")
			return 1
		}
		if cfg.OidcAlb.JWTSigner == "" {
			c.UI.Error("error initializing server: OIDC ALB JWT signer is required")
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
		workspaceProviderName = "google" // Default to google for backward compatibility
	}

	searchProviderName := c.flagSearchProvider
	if val, ok := os.LookupEnv("HERMES_SEARCH_PROVIDER"); ok && searchProviderName == "" {
		searchProviderName = val
	}
	if searchProviderName == "" && cfg.Providers != nil {
		searchProviderName = cfg.Providers.Search
	}
	if searchProviderName == "" {
		searchProviderName = "algolia" // Default to algolia for backward compatibility
	}

	c.UI.Info(fmt.Sprintf("Using workspace provider: %s", workspaceProviderName))
	c.UI.Info(fmt.Sprintf("Using search provider: %s", searchProviderName))

	// Initialize workspace provider based on selection.
	var workspaceProvider workspace.Provider
	var goog *gw.Service // Keep for legacy handlers that still use it directly

	switch workspaceProviderName {
	case "google":
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

		workspaceProvider = gw.NewAdapter(goog)

	case "local":
		if cfg.LocalWorkspace == nil {
			c.UI.Error("error initializing server: local_workspace configuration required when using local workspace provider")
			return 1
		}

		localCfg := cfg.LocalWorkspace.ToLocalAdapterConfig()
		_, err := localadapter.NewAdapter(localCfg)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing local workspace adapter: %v", err))
			return 1
		}
		// TODO: Local adapter needs to implement workspace.Provider interface
		// For now, this is a placeholder to demonstrate the provider selection architecture
		c.UI.Error("error: local workspace provider is not yet fully implemented")
		return 1

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
		searchAdapterCfg := &searchalgolia.Config{
			AppID:           cfg.Algolia.AppID,
			WriteAPIKey:     cfg.Algolia.WriteAPIKey,
			DocsIndexName:   cfg.Algolia.DocsIndexName,
			DraftsIndexName: cfg.Algolia.DraftsIndexName,
		}
		searchProvider, err = searchalgolia.NewAdapter(searchAdapterCfg)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing search provider: %v", err))
			return 1
		}

	case "meilisearch":
		if cfg.Meilisearch == nil {
			c.UI.Error("error initializing server: meilisearch configuration required when using meilisearch search provider")
			return 1
		}

		meilisearchCfg := cfg.Meilisearch.ToMeilisearchAdapterConfig()
		meilisearchAdapter, err := meilisearchadapter.NewAdapter(meilisearchCfg)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error initializing meilisearch adapter: %v", err))
			return 1
		}
		searchProvider = meilisearchAdapter

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
	if val, ok := os.LookupEnv("HERMES_SERVER_POSTGRES_PASSWORD"); ok {
		cfg.Postgres.Password = val
	}
	db, err := db.NewDB(*cfg.Postgres)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error initializing database: %v", err))
		return 1
	}

	// Register document types.
	// for _, d := range cfg.DocumentTypes.DocumentType {
	// 	if err := models.RegisterDocumentType(*d, db); err != nil {
	// 		c.UI.Error(fmt.Sprintf("error registering document type: %v", err))
	// 		return 1
	// 	}
	// }
	if err := registerDocumentTypes(*cfg, db); err != nil {
		c.UI.Error(fmt.Sprintf("error registering document types: %v", err))
		return 1
	}

	// Register products.
	if err := registerProducts(cfg, algoWrite, db); err != nil {
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
		SearchProvider:    searchProvider,
		WorkspaceProvider: workspaceProvider,
		Config:            cfg,
		DB:                db,
		Jira:              jiraSvc,
		Logger:            c.Log,
	}

	// Define handlers for authenticated endpoints.
	// All API endpoints use v2.
	authenticatedEndpoints := []endpoint{
		{"/api/v2/approvals/", apiv2.ApprovalsHandler(srv)},
		{"/api/v2/document-types", apiv2.DocumentTypesHandler(srv)},
		{"/api/v2/documents/", apiv2.DocumentHandler(srv)},
		{"/api/v2/drafts", apiv2.DraftsHandler(srv)},
		{"/api/v2/drafts/", apiv2.DraftsDocumentHandler(srv)},
		{"/api/v2/groups", apiv2.GroupsHandler(srv)},
		{"/api/v2/jira/issues/", apiv2.JiraIssueHandler(srv)},
		{"/api/v2/jira/issue/picker", apiv2.JiraIssuePickerHandler(srv)},
		{"/api/v2/me", apiv2.MeHandler(srv)},
		{"/api/v2/me/recently-viewed-docs", apiv2.MeRecentlyViewedDocsHandler(srv)},
		{"/api/v2/me/recently-viewed-projects",
			apiv2.MeRecentlyViewedProjectsHandler(srv)},
		{"/api/v2/me/subscriptions", apiv2.MeSubscriptionsHandler(srv)},
		{"/api/v2/people", apiv2.PeopleDataHandler(srv)},
		{"/api/v2/products", apiv2.ProductsHandler(srv)},
		{"/api/v2/projects", apiv2.ProjectsHandler(srv)},
		{"/api/v2/projects/", apiv2.ProjectHandler(srv)},
		{"/api/v2/reviews/", apiv2.ReviewsHandler(srv)},
		{"/api/v2/web/analytics", apiv2.AnalyticsHandler(srv)},
	}

	// Add Algolia-specific endpoints if using Algolia search provider.
	if searchProviderName == "algolia" && algoSearch != nil {
		authenticatedEndpoints = append(authenticatedEndpoints, endpoint{
			"/1/indexes/",
			algolia.AlgoliaProxyHandler(algoSearch, algoliaClientCfg, c.Log),
		})
	}

	// Define handlers for unauthenticated endpoints.
	unauthenticatedEndpoints := []endpoint{
		{"/health", healthHandler()},
		{"/pub/", http.StripPrefix("/pub/", pub.Handler())},
	}

	// Add Dex OIDC auth endpoints if Dex is configured
	if cfg.Dex != nil && !cfg.Dex.Disabled {
		unauthenticatedEndpoints = append(unauthenticatedEndpoints,
			endpoint{"/auth/login", api.LoginHandler(*cfg, c.Log)},
			endpoint{"/auth/callback", api.CallbackHandler(*cfg, c.Log)},
			endpoint{"/auth/logout", api.LogoutHandler(c.Log)},
		)
	}

	// Web config endpoints are always unauthenticated so the frontend can load
	// and determine the auth provider before attempting to authenticate.
	if searchProviderName == "algolia" && algoSearch != nil {
		unauthenticatedEndpoints = append(unauthenticatedEndpoints,
			endpoint{"/api/v2/web/config", web.ConfigHandler(cfg, algoSearch, c.Log)},
			endpoint{"/l/", links.RedirectHandler(algoSearch, algoliaClientCfg, c.Log)},
		)
	} else {
		// For non-Algolia search providers, provide minimal config handlers
		// that return configuration without Algolia-specific data
		unauthenticatedEndpoints = append(unauthenticatedEndpoints,
			endpoint{"/api/v2/web/config", web.ConfigHandler(cfg, nil, c.Log)},
		)
	}

	// SPA handler - conditionally authenticated based on if Okta or Dex is enabled.
	spaEndpoints := []endpoint{
		{"/", web.Handler()},
	}

	// If Okta or Dex is enabled, add the SPA handler as an authenticated endpoint.
	if (cfg.Okta != nil && !cfg.Okta.Disabled) || (cfg.Dex != nil && !cfg.Dex.Disabled) {
		authenticatedEndpoints = append(authenticatedEndpoints, spaEndpoints...)
	} else {
		// If both Okta and Dex are disabled, add the SPA handler as an unauthenticated endpoint.
		unauthenticatedEndpoints = append(unauthenticatedEndpoints, spaEndpoints...)
	}
	// Config and redirect endpoints are always unauthenticated.
	unauthenticatedEndpoints = append(unauthenticatedEndpoints, webEndpoints2...)

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
			auth.AuthenticateRequest(*cfg, goog, sharepointSvc, c.Log, e.handler),
		)
	}
	for _, e := range unauthenticatedEndpoints {
		mux.Handle(e.pattern, e.handler)
	}

	// Use dev_mode flag from configuration
	isDevelopment := cfg.Server.DevMode

	if isDevelopment {
		c.Log.Info("Running in development mode - permissive CORS policy enabled")
	} else {
		c.Log.Info("Running in production mode - restrictive CORS policy enabled")
	} // Wrap the entire mux with CORS middleware
	corsHandler := middleware.CorsMiddlewareWithConfig(c.Log, mux, isDevelopment)

	server := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: corsHandler,
	}
	go func() {
		if cfg.Server.TLSEnabled {
			c.Log.Info("Starting server with TLS/HTTPS", "addr", cfg.Server.Addr, "tls_enabled", true)

			if cfg.Server.TLSCert == "" || cfg.Server.TLSKey == "" {
				c.Log.Error("TLS is enabled but certificate or key file path is not specified")
				os.Exit(1)
			}

			if err := server.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != http.ErrServerClosed {
				c.Log.Error("Error starting TLS listener", "error", err, "addr", cfg.Server.Addr)
				os.Exit(1)
			}
		} else {
			c.Log.Info("Starting server", "addr", cfg.Server.Addr, "tls_enabled", false)

			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				c.Log.Error("Error starting listener", "error", err, "addr", cfg.Server.Addr)
				os.Exit(1)
			}
		}
	}()

	return c.WaitForInterrupt(c.ShutdownServer(server))
}

// healthHandler responds with the health of the service.
func healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
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
	for _, d := range cfg.DocumentTypes.DocumentType {
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
				cf.Type = models.DocumentTypeCustomFieldType(
					models.StringDocumentTypeCustomFieldType)
			case "person":
				cf.Type = models.DocumentTypeCustomFieldType(
					models.PersonDocumentTypeCustomFieldType)
			case "people":
				cf.Type = models.DocumentTypeCustomFieldType(
					models.PeopleDocumentTypeCustomFieldType)
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

// logInstanceOverview logs a comprehensive overview of the running instance configuration
// without exposing sensitive data
func logInstanceOverview(log hclog.Logger, cfg *config.Config) {
	// Determine authentication method
	var authMethod string
	var authDetails []interface{}

	if cfg.OidcAlb != nil && !cfg.OidcAlb.Disabled {
		authMethod = "OIDC ALB"
		authDetails = []interface{}{
			"auth_server_configured", cfg.OidcAlb.AuthServerURL != "",
			"client_id_configured", cfg.OidcAlb.ClientID != "",
			"aws_region", cfg.OidcAlb.AWSRegion,
		}
	} else if cfg.Okta != nil && !cfg.Okta.Disabled {
		authMethod = "Okta ALB"
		authDetails = []interface{}{
			"auth_server_configured", cfg.Okta.AuthServerURL != "",
			"client_id_configured", cfg.Okta.ClientID != "",
			"aws_region", cfg.Okta.AWSRegion,
		}
	} else if cfg.SharePoint != nil {
		authMethod = "Microsoft Auth / SharePoint"
		authDetails = []interface{}{
			"sharepoint_configured", true,
		}
		authDetails = append(authDetails,
			"tenant_id_configured", cfg.SharePoint.TenantID != "",
			"site_id_configured", cfg.SharePoint.SiteID != "",
			"drive_id_configured", cfg.SharePoint.DriveID != "",
			"domain", cfg.SharePoint.Domain,
		)
	} else {
		authMethod = "Google OAuth"
		authDetails = []interface{}{
			"google_workspace_configured", cfg.GoogleWorkspace != nil,
		}
		if cfg.GoogleWorkspace != nil {
			authDetails = append(authDetails,
				"gw_domain", cfg.GoogleWorkspace.Domain,
				"oauth2_configured", cfg.GoogleWorkspace.OAuth2 != nil,
			)
		}
	}

	// Determine mode (development vs production)
	mode := "production"
	if cfg.Server != nil && cfg.Server.DevMode {
		mode = "development"
	}

	// Log format determination
	logFormat := "standard"
	if cfg.LogFormat == "json" {
		logFormat = "json"
	} else if cfg.LogFormat != "" {
		logFormat = cfg.LogFormat
	}

	// Main configuration overview
	configFields := []interface{}{
		"instance_mode", mode,
		"server_addr", cfg.Server.Addr,
		"base_url", cfg.BaseURL,
		"shortener_base_url", cfg.ShortenerBaseURL,
		"log_format", logFormat,
		"tls_enabled", cfg.Server != nil && cfg.Server.TLSEnabled,
		"auth_method", authMethod,
	}

	// Add auth-specific details
	configFields = append(configFields, authDetails...)

	// Service configurations
	configFields = append(configFields,
		"algolia_configured", cfg.Algolia != nil && cfg.Algolia.ApplicationID != "",
		"google_workspace_configured", cfg.GoogleWorkspace != nil && cfg.GoogleWorkspace.Domain != "",
		"email_enabled", cfg.Email != nil && cfg.Email.Enabled,
		"jira_enabled", cfg.Jira != nil && cfg.Jira.Enabled,
		"datadog_enabled", cfg.Datadog != nil && cfg.Datadog.Enabled,
	)

	// Google Workspace details (if configured)
	if cfg.GoogleWorkspace != nil {
		configFields = append(configFields,
			"gw_domain", cfg.GoogleWorkspace.Domain,
			"gw_docs_folder", cfg.GoogleWorkspace.DocsFolder,
			"gw_drafts_folder", cfg.GoogleWorkspace.DraftsFolder,
			"gw_shortcuts_enabled", cfg.GoogleWorkspace.CreateDocShortcuts,
		)
	}

	// Document types count
	if cfg.DocumentTypes != nil {
		configFields = append(configFields,
			"document_types_count", len(cfg.DocumentTypes.DocumentType),
		)
	}

	// Products count
	if cfg.Products != nil {
		configFields = append(configFields,
			"products_count", len(cfg.Products.Product),
		)
	}

	log.Info("=== Hermes Instance Configuration Overview ===", configFields...)
}
