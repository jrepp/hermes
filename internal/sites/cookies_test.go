package sites_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/middleware"
	"github.com/hashicorp-forge/hermes/internal/sites"
)

// TestSecureCookiesBehindTLSTerminatingProxy is the regression test for the
// deployment topology this project targets: nginx terminates TLS and proxies
// to a loopback listener, so r.TLS is nil on every request even though the
// browser is speaking HTTPS.
//
// Code that derives the Secure attribute from the connection therefore ships
// every production session cookie without it. This asserts the flag comes from
// the site's configured origin instead.
func TestSecureCookiesBehindTLSTerminatingProxy(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{Domain: "docs.jrepp.com"}, // derives https://
			{Domain: "dev.localhost", BaseURL: "http://dev.localhost:8000"},
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

	var got bool
	h := resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = sites.SecureCookiesForRequest(r)
	}))

	for _, tc := range []struct {
		host string
		want bool
	}{
		{"docs.jrepp.com", true},
		{"dev.localhost", false},
	} {
		// Deliberately a plaintext request with no TLS state, exactly as the
		// proxy delivers it.
		r := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/", nil)
		r.Host = tc.host
		r.TLS = nil

		h.ServeHTTP(httptest.NewRecorder(), r)

		if got != tc.want {
			t.Errorf("%s: secure = %v, want %v", tc.host, got, tc.want)
		}
	}
}

// TestExampleConfigYieldsSecureCookies checks the shipped example, since a
// base_url written as http:// there would silently disable Secure for anyone
// who copied it.
func TestExampleConfigYieldsSecureCookies(t *testing.T) {
	t.Parallel()

	cfg, err := config.NewConfig("../../local/config.example.hcl", "")
	if err != nil {
		t.Fatalf("local/config.example.hcl does not parse: %v", err)
	}
	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	for _, site := range registry.Sites() {
		if !site.SecureCookies() {
			t.Errorf("example site %q has base_url %q, which yields non-Secure cookies",
				site.Domain.String(), site.BaseURL)
		}
	}
}

func TestSecureCookiesFallsBackToConnection(t *testing.T) {
	t.Parallel()

	// No site in context: the connection is all there is to go on.
	r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	if sites.SecureCookiesForRequest(r) {
		t.Error("plaintext request with no site reported secure")
	}

	r = httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	if !sites.SecureCookiesForRequest(r) {
		t.Error("TLS request with no site reported insecure")
	}
}
