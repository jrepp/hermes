package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/pkg/featureflags"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/internal/version"
	"github.com/hashicorp-forge/hermes/pkg/algolia"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
)

// The `all:` prefix includes dotfiles, so `dist/.gitkeep` alone satisfies the
// pattern. Without it, `go build ./...` fails on a clean checkout with
// "pattern dist: no matching files found" until the frontend is built.
//
//go:embed all:dist
var content embed.FS

// Handler serves the Ember single-page application.
//
// @Summary Serve web application
// @Description Serves the Hermes Ember single-page application and static assets.
// @Tags web
// @Produce html
// @Success 200 {string} string "HTML or static asset content"
// @Failure 405 {string} string "method not allowed"
// @Router / [get]
// @x-rbac {"resource":"web.app","action":"read","authenticated":"deployment-dependent"}
func Handler() http.Handler {
	r := gin.New()
	r.Any("/*path", ginWebHandler(http.FileServer(httpFileSystem())))
	return r
}

func httpFileSystem() http.FileSystem {
	return http.FS(fileSystem())
}

func fileSystem() fs.FS {
	f, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}

	return f
}

// ginWebHandler serves our single-page application from Gin.
func ginWebHandler(next http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		// Serve `/index.html` if there isn't an extension in the URL path.
		// Without this, browser refreshes on SPA routes will 404.
		if ext := strings.LastIndex(c.Request.URL.Path, "."); ext == -1 {
			c.Request.URL.Path = "/"
		}

		next.ServeHTTP(c.Writer, c.Request)
	}
}

type ConfigResponse struct {
	AlgoliaDocsIndexName     string          `json:"algolia_docs_index_name"`
	AlgoliaDraftsIndexName   string          `json:"algolia_drafts_index_name"`
	AlgoliaInternalIndexName string          `json:"algolia_internal_index_name"`
	AlgoliaProjectsIndexName string          `json:"algolia_projects_index_name"`
	AuthProvider             string          `json:"auth_provider"` // "google", "okta", "dex", or "microsoft"
	CreateDocsAsUser         bool            `json:"create_docs_as_user"`
	DexIssuerURL             string          `json:"dex_issuer_url,omitempty"`
	DexClientID              string          `json:"dex_client_id,omitempty"`
	DexRedirectURL           string          `json:"dex_redirect_url,omitempty"`
	FeatureFlags             map[string]bool `json:"feature_flags"`
	GoogleAnalyticsTagID     string          `json:"google_analytics_tag_id"`
	GoogleOAuth2ClientID     string          `json:"google_oauth2_client_id"`
	GoogleOAuth2HD           string          `json:"google_oauth2_hd"`
	GroupApprovals           bool            `json:"group_approvals"`
	JiraURL                  string          `json:"jira_url"`
	ShortLinkBaseURL         string          `json:"short_link_base_url"`
	SimplifiedMode           bool            `json:"simplified_mode"`  // True when running in zero-config simplified mode
	SkipGoogleAuth           bool            `json:"skip_google_auth"` // Deprecated: use auth_provider instead
	SupportLinkURL           string          `json:"support_link_url"`
	ShortRevision            string          `json:"short_revision"`
	Version                  string          `json:"version"`
	WorkspaceProvider        string          `json:"workspace_provider"` // "google", "local", or "sharepoint"
}

// ConfigHandler returns runtime configuration for the Hermes frontend.
//
// @Summary Get web runtime configuration
// @Description Returns frontend runtime configuration, feature flags, auth provider selection, and workspace provider selection.
// @Tags web
// @Produce json
// @Success 200 {object} ConfigResponse
// @Failure 405 {string} string "method not allowed"
// @Failure 500 {string} string "Error encoding web config response"
// @Router /api/v2/web/config [get]
// @x-rbac {"resource":"web.config","action":"read","authenticated":false}
func ConfigHandler(
	cfg *config.Config,
	a *algolia.Client,
	log hclog.Logger,
) http.Handler {
	r := gin.New()
	r.Any("/*path", ginConfigHandler(cfg, a, log))
	return r
}

func ginConfigHandler(
	cfg *config.Config,
	a *algolia.Client,
	log hclog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		// Get user email from auth middleware if available (may be empty if not authenticated)
		userEmail := ""
		if email, ok := c.Request.Context().Value(pkgauth.UserEmailKey).(string); ok {
			userEmail = email
		}

		// Set and toggle the feature flags defined
		// in the configuration
		featureFlags := featureflags.SetAndToggle(
			cfg.FeatureFlags,
			a,
			// Use the "x-amzn-oidc-identity" header if set
			// as id to be hashed and toggle flags.
			c.GetHeader("x-amzn-oidc-identity"),
			// Get user email from value set by auth middleware (may be empty)
			userEmail,
			log,
		)

		// The frontend is served from whichever site the request arrived at, so
		// site-dependent values in this response have to come from that site.
		// A global base_url here would hand notes.jrepp.com short links and an
		// OIDC callback pointing at docs.jrepp.com.
		baseURL := cfg.BaseURL
		if site, ok := sites.FromContext(c.Request.Context()); ok && site.BaseURL != "" {
			baseURL = site.BaseURL
		}

		// Trim last "/"
		shortLinkBaseURL := strings.TrimSuffix(cfg.ShortenerBaseURL, "/")
		// Check if shortener base URL was set, if not
		// use application base URL to create
		// short link base URL.
		if shortLinkBaseURL == "" {
			shortLinkBaseURL = strings.TrimSuffix(baseURL, "/") + "/l"
		}

		// Determine which authentication provider is configured
		authProvider := "google" // Default to Google
		skipGoogleAuth := false  // Legacy compatibility

		if cfg.Dex != nil && !cfg.Dex.Disabled {
			authProvider = "dex"
			skipGoogleAuth = true
		} else if cfg.Okta != nil && !cfg.Okta.Disabled {
			authProvider = "okta"
			skipGoogleAuth = true
		} else if cfg.SharePoint != nil {
			authProvider = "microsoft"
			skipGoogleAuth = true
		}

		// Set CreateDocsAsUser if enabled in the config.
		createDocsAsUser := false
		if cfg.GoogleWorkspace != nil && cfg.GoogleWorkspace.Auth != nil &&
			cfg.GoogleWorkspace.Auth.CreateDocsAsUser {
			createDocsAsUser = true
		}

		// Set GroupApprovals if enabled in the config.
		groupApprovals := false
		if cfg.GoogleWorkspace != nil && cfg.GoogleWorkspace.GroupApprovals != nil &&
			cfg.GoogleWorkspace.GroupApprovals.Enabled {
			groupApprovals = true
		}

		// Set JiraURL if enabled in the config.
		jiraURL := ""
		if cfg.Jira != nil && cfg.Jira.Enabled {
			jiraURL = cfg.Jira.URL
		}

		// Prepare Google OAuth config (only if using Google auth)
		googleOAuth2ClientID := ""
		googleOAuth2HD := ""
		if authProvider == "google" && cfg.GoogleWorkspace != nil {
			googleOAuth2ClientID = cfg.GoogleWorkspace.OAuth2.ClientID
			googleOAuth2HD = cfg.GoogleWorkspace.OAuth2.HD
		}

		// Prepare Dex config (only if using Dex auth)
		dexIssuerURL := ""
		dexClientID := ""
		dexRedirectURL := ""
		if authProvider == "dex" && cfg.Dex != nil {
			dexIssuerURL = cfg.Dex.IssuerURL
			dexClientID = cfg.Dex.ClientID
			dexRedirectURL = cfg.Dex.RedirectURL
			// Must agree with what the login handler sends, or the provider
			// rejects the exchange: OIDC compares redirect_uri exactly.
			if baseURL != "" {
				dexRedirectURL = strings.TrimSuffix(baseURL, "/") + "/auth/callback"
			}
		}

		// Determine which workspace provider is configured
		workspaceProvider := "google" // Default to Google
		if cfg.Providers != nil && cfg.Providers.Workspace != "" {
			workspaceProvider = cfg.Providers.Workspace
		} else if cfg.SharePoint != nil {
			workspaceProvider = "sharepoint"
		} else if cfg.LocalWorkspace != nil {
			workspaceProvider = "local"
		}

		// Absent for any deployment not using Algolia -- Meilisearch and Bleve
		// have no algolia block at all -- so this cannot be dereferenced
		// unconditionally.
		var algoliaDocs, algoliaDrafts, algoliaInternal, algoliaProjects string
		if cfg.Algolia != nil {
			algoliaDocs = cfg.Algolia.DocsIndexName
			algoliaDrafts = cfg.Algolia.DraftsIndexName
			algoliaInternal = cfg.Algolia.InternalIndexName
			algoliaProjects = cfg.Algolia.ProjectsIndexName
		}

		response := &ConfigResponse{
			AlgoliaDocsIndexName:     algoliaDocs,
			AlgoliaDraftsIndexName:   algoliaDrafts,
			AlgoliaInternalIndexName: algoliaInternal,
			AlgoliaProjectsIndexName: algoliaProjects,
			AuthProvider:             authProvider,
			CreateDocsAsUser:         createDocsAsUser,
			DexIssuerURL:             dexIssuerURL,
			DexClientID:              dexClientID,
			DexRedirectURL:           dexRedirectURL,
			FeatureFlags:             featureFlags,
			GoogleAnalyticsTagID:     cfg.GoogleAnalyticsTagID,
			GoogleOAuth2ClientID:     googleOAuth2ClientID,
			GoogleOAuth2HD:           googleOAuth2HD,
			GroupApprovals:           groupApprovals,
			JiraURL:                  jiraURL,
			ShortLinkBaseURL:         shortLinkBaseURL,
			SimplifiedMode:           cfg.SimplifiedMode,
			SkipGoogleAuth:           skipGoogleAuth, // Legacy compatibility
			SupportLinkURL:           cfg.SupportLinkURL,
			ShortRevision:            version.GetShortRevision(),
			Version:                  version.GetVersion(),
			WorkspaceProvider:        workspaceProvider,
		}

		c.JSON(http.StatusOK, response)
	}
}
