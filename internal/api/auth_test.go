package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/sites"
	dexadapter "github.com/hashicorp-forge/hermes/pkg/auth/adapters/dex"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

func requestForSite(t *testing.T, baseURL string) *http.Request {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "http://example.com/auth/login", nil)
	if baseURL == "" {
		return r
	}

	site := &sites.Site{
		Domain:  domain.MustParse("docs.jrepp.com"),
		BaseURL: baseURL,
	}

	return r.WithContext(sites.NewContext(r.Context(), site))
}

func dexCfg() config.Config {
	return config.Config{
		Dex: &dexadapter.Config{
			IssuerURL:   "https://auth.jrepp.com/dex",
			ClientID:    "hermes",
			RedirectURL: "https://docs.jrepp.com/auth/callback",
		},
	}
}

// TestDexRedirectFollowsTheSite is what makes login work on more than one
// subdomain. OIDC requires the redirect_uri to be identical at authorization
// and at token exchange, and the browser has to return to the host it left --
// so a single configured redirect_url would send every site's login to one
// host, and the user would land on the wrong site holding a session that is
// not valid there.
func TestDexRedirectFollowsTheSite(t *testing.T) {
	t.Parallel()

	got := dexConfigForRequest(dexCfg(), requestForSite(t, "https://notes.jrepp.com"))

	if want := "https://notes.jrepp.com/auth/callback"; got.RedirectURL != want {
		t.Errorf("redirect_url = %q, want %q", got.RedirectURL, want)
	}
}

func TestDexRedirectTrimsTrailingSlash(t *testing.T) {
	t.Parallel()

	got := dexConfigForRequest(dexCfg(), requestForSite(t, "https://notes.jrepp.com/"))

	if want := "https://notes.jrepp.com/auth/callback"; got.RedirectURL != want {
		t.Errorf("redirect_url = %q, want %q", got.RedirectURL, want)
	}
}

func TestDexRedirectFallsBackToConfig(t *testing.T) {
	t.Parallel()

	// No sites configured: the single-tenant deployment keeps whatever the
	// operator wrote.
	got := dexConfigForRequest(dexCfg(), requestForSite(t, ""))

	if want := "https://docs.jrepp.com/auth/callback"; got.RedirectURL != want {
		t.Errorf("redirect_url = %q, want %q", got.RedirectURL, want)
	}
}

// TestDexConfigIsCopied guards against the per-request override leaking into
// the process-wide configuration, which would make the redirect depend on
// whichever site happened to log in last.
func TestDexConfigIsCopied(t *testing.T) {
	t.Parallel()

	cfg := dexCfg()
	original := cfg.Dex.RedirectURL

	_ = dexConfigForRequest(cfg, requestForSite(t, "https://notes.jrepp.com"))

	if cfg.Dex.RedirectURL != original {
		t.Errorf("shared config mutated: redirect_url = %q, want %q",
			cfg.Dex.RedirectURL, original)
	}
}
