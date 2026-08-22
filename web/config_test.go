package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/middleware"
	"github.com/hashicorp-forge/hermes/internal/sites"
	dexadapter "github.com/hashicorp-forge/hermes/pkg/auth/adapters/dex"
)

func init() { gin.SetMode(gin.TestMode) }

func getConfig(t *testing.T, cfg *config.Config, host string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	h := ConfigHandler(cfg, nil, hclog.NewNullLogger())
	r := httptest.NewRequest(http.MethodGet, "http://"+host+"/api/v2/web/config", nil)
	r.Host = host
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)

	return w, body
}

// TestConfigHandlerWithoutAlgolia is the regression test for an unauthenticated
// nil-pointer dereference.
//
// The handler read cfg.Algolia.DocsIndexName unconditionally, but a
// Meilisearch or Bleve deployment has no algolia block at all -- the shipped
// multi-site example is one. gin.New() installs no recovery middleware, so the
// panic took the connection with it, on the one endpoint the frontend must
// reach before it can do anything else.
func TestConfigHandlerWithoutAlgolia(t *testing.T) {
	cfg := &config.Config{
		BaseURL:   "https://docs.jrepp.com",
		Providers: &config.Providers{Workspace: "local", Search: "meilisearch"},
		Dex:       &dexadapter.Config{IssuerURL: "https://auth.jrepp.com/dex", ClientID: "hermes"},
	}

	w, body := getConfig(t, cfg, "docs.jrepp.com")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if body["auth_provider"] != "dex" {
		t.Errorf("auth_provider = %v, want dex", body["auth_provider"])
	}
}

// TestConfigHandlerIsSiteScoped covers what the frontend does with this
// response: it builds short links from it and starts the OIDC flow with it. A
// global base_url would hand notes.jrepp.com links and a callback belonging to
// docs.jrepp.com -- and OIDC compares redirect_uri exactly, so the login would
// simply fail.
func TestConfigHandlerIsSiteScoped(t *testing.T) {
	cfg := &config.Config{
		BaseURL:   "https://docs.jrepp.com",
		Providers: &config.Providers{Workspace: "local", Search: "meilisearch"},
		Dex: &dexadapter.Config{
			IssuerURL:   "https://auth.jrepp.com/dex",
			ClientID:    "hermes",
			RedirectURL: "https://docs.jrepp.com/auth/callback",
		},
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{Domain: "docs.jrepp.com"},
			{Domain: "notes.jrepp.com"},
		},
	}

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	resolver, err := middleware.NewDomainResolver(registry, nil, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("NewDomainResolver: %v", err)
	}
	h := resolver.Middleware(ConfigHandler(cfg, nil, hclog.NewNullLogger()))

	for _, tc := range []struct{ host, wantBase string }{
		{"docs.jrepp.com", "https://docs.jrepp.com"},
		{"notes.jrepp.com", "https://notes.jrepp.com"},
	} {
		r := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/api/v2/web/config", nil)
		r.Host = tc.host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body %s", tc.host, w.Code, w.Body.String())
		}

		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: decoding response: %v", tc.host, err)
		}

		// No Algolia client, so no /l/ handler and no short links.
		if got := body["short_link_base_url"]; got != "" {
			t.Errorf("%s: short_link_base_url = %v, want empty without a "+
				"redirect handler", tc.host, got)
		}
		if got, want := body["dex_redirect_url"], tc.wantBase+"/auth/callback"; got != want {
			t.Errorf("%s: dex_redirect_url = %v, want %v", tc.host, got, want)
		}
	}
}

// TestConfigHandlerWithoutSitesKeepsGlobalBaseURL covers the single-tenant
// deployment, where scoping must change nothing.
func TestConfigHandlerWithoutSitesKeepsGlobalBaseURL(t *testing.T) {
	cfg := &config.Config{
		BaseURL:   "https://hermes.example.com",
		Providers: &config.Providers{Workspace: "local", Search: "meilisearch"},
	}

	_, body := getConfig(t, cfg, "hermes.example.com")

	if got := body["short_link_base_url"]; got != "" {
		t.Errorf("short_link_base_url = %v, want empty", got)
	}
}

// TestConfigHandlerOmitsShortLinksWithoutARedirectHandler is the point: the /l/
// route is only registered on the Algolia path, so advertising a base URL
// elsewhere hands the frontend a link that loads the single-page app instead
// of the document. The frontend reads an empty value as "no shortener" and
// falls back to the canonical URL.
func TestConfigHandlerOmitsShortLinksWithoutARedirectHandler(t *testing.T) {
	cfg := &config.Config{
		BaseURL:   "https://docs.jrepp.com",
		Providers: &config.Providers{Workspace: "local", Search: "meilisearch"},
	}

	_, body := getConfig(t, cfg, "docs.jrepp.com")

	if got := body["short_link_base_url"]; got != "" {
		t.Errorf("short_link_base_url = %v; there is no /l/ handler to serve it", got)
	}
}

// TestConfigHandlerKeepsAnExplicitShortener covers the operator who runs their
// own: an explicit setting is advertised whatever the search provider is.
func TestConfigHandlerKeepsAnExplicitShortener(t *testing.T) {
	cfg := &config.Config{
		BaseURL:          "https://docs.jrepp.com",
		ShortenerBaseURL: "https://jrepp.link/",
		Providers:        &config.Providers{Workspace: "local", Search: "meilisearch"},
	}

	_, body := getConfig(t, cfg, "docs.jrepp.com")

	if got, want := body["short_link_base_url"], "https://jrepp.link"; got != want {
		t.Errorf("short_link_base_url = %v, want %v", got, want)
	}
}

// TestConfigHandlerHonorsExplicitShortenerBaseURL makes sure the per-site
// default does not override an operator's explicit setting.
func TestConfigHandlerHonorsExplicitShortenerBaseURL(t *testing.T) {
	cfg := &config.Config{
		BaseURL:          "https://docs.jrepp.com",
		ShortenerBaseURL: "https://jrepp.link/",
		Providers:        &config.Providers{Workspace: "local", Search: "meilisearch"},
	}

	_, body := getConfig(t, cfg, "docs.jrepp.com")

	if got, want := body["short_link_base_url"], "https://jrepp.link"; got != want {
		t.Errorf("short_link_base_url = %v, want %v", got, want)
	}
}
