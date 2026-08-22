package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/middleware"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// resolvedDomain runs one request through the middleware and reports the
// domain the downstream handler observed, along with the response status.
type resolvedDomain struct {
	status  int
	domain  string
	site    string
	reached bool
}

func newResolver(t *testing.T, cfg *config.Config, trustedProxies []string) *middleware.DomainResolver {
	t.Helper()

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	resolver, err := middleware.NewDomainResolver(registry, trustedProxies, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("NewDomainResolver: %v", err)
	}

	return resolver
}

func run(t *testing.T, resolver *middleware.DomainResolver, req *http.Request) resolvedDomain {
	t.Helper()

	var out resolvedDomain
	handler := resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		out.reached = true
		if name, ok := domain.FromContext(r.Context()); ok {
			out.domain = name.String()
		}
		if site, ok := sites.FromContext(r.Context()); ok {
			out.site = site.Domain.String()
		}
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	out.status = rec.Code

	return out
}

func multiSiteConfig() *config.Config {
	return &config.Config{
		LocalWorkspace: &config.LocalWorkspace{BasePath: "/srv/hermes"},
		Sites: []*config.Site{
			{Domain: "docs.jrepp.com", Aliases: []string{"www.jrepp.com"}},
			{Domain: "notes.jrepp.com"},
		},
	}
}

func TestMiddlewareRoutesByHost(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, multiSiteConfig(), nil)

	cases := map[string]string{
		"docs.jrepp.com":      "docs.jrepp.com",
		"notes.jrepp.com":     "notes.jrepp.com",
		"DOCS.JREPP.COM":      "docs.jrepp.com",
		"docs.jrepp.com:8000": "docs.jrepp.com",
		"docs.jrepp.com.":     "docs.jrepp.com",
		"www.jrepp.com":       "docs.jrepp.com", // alias folds onto its canonical site
	}

	for host, want := range cases {
		t.Run(host, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/v2/me", nil)
			req.Host = host

			got := run(t, resolver, req)
			if !got.reached {
				t.Fatalf("handler not reached, status %d", got.status)
			}
			if got.domain != want {
				t.Errorf("context domain = %q, want %q", got.domain, want)
			}
			// The site's canonical domain, not the requested spelling, is what
			// keys storage; an alias must not open a second namespace.
			if got.site != want {
				t.Errorf("context site = %q, want %q", got.site, want)
			}
		})
	}
}

func TestMiddlewareRejectsUnknownHost(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, multiSiteConfig(), nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "evil.example"

	got := run(t, resolver, req)
	if got.reached {
		t.Fatal("handler ran for an unconfigured host")
	}
	if got.status != http.StatusMisdirectedRequest {
		t.Errorf("status = %d, want %d", got.status, http.StatusMisdirectedRequest)
	}
}

func TestMiddlewareRejectsUnparseableHost(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, multiSiteConfig(), nil)

	for _, host := range []string{"", "not a host", "docs.jrepp.com/../etc", "-bad.example"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host

		got := run(t, resolver, req)
		if got.reached {
			t.Errorf("handler ran for malformed host %q", host)
		}
		if got.status != http.StatusBadRequest {
			t.Errorf("host %q: status = %d, want %d", host, got.status, http.StatusBadRequest)
		}
	}
}

func TestMiddlewareIgnoresForwardedHostFromUntrustedPeer(t *testing.T) {
	t.Parallel()

	// Without a trusted-proxy list the header is entirely attacker-controlled:
	// honoring it would let any client pick which tenant to read.
	resolver := newResolver(t, multiSiteConfig(), nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "docs.jrepp.com"
	req.RemoteAddr = "203.0.113.9:54321"
	req.Header.Set("X-Forwarded-Host", "notes.jrepp.com")

	got := run(t, resolver, req)
	if !got.reached {
		t.Fatalf("handler not reached, status %d", got.status)
	}
	if got.domain != "docs.jrepp.com" {
		t.Errorf("spoofed X-Forwarded-Host was honored: resolved %q", got.domain)
	}
}

func TestMiddlewareHonorsForwardedHostFromTrustedProxy(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, multiSiteConfig(), []string{"127.0.0.1/32", "::1"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:8000" // what nginx presents when it does not rewrite Host
	req.RemoteAddr = "127.0.0.1:41234"
	req.Header.Set("X-Forwarded-Host", "notes.jrepp.com")

	got := run(t, resolver, req)
	if !got.reached {
		t.Fatalf("handler not reached, status %d", got.status)
	}
	if got.domain != "notes.jrepp.com" {
		t.Errorf("resolved %q, want notes.jrepp.com from the trusted proxy header", got.domain)
	}
}

func TestMiddlewareUsesFirstForwardedHostHop(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, multiSiteConfig(), []string{"127.0.0.1/32"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:8000"
	req.RemoteAddr = "127.0.0.1:41234"
	// The header accumulates a chain across hops; the client's original
	// authority is the first entry, and a later hop must not override it.
	req.Header.Set("X-Forwarded-Host", "docs.jrepp.com, notes.jrepp.com")

	got := run(t, resolver, req)
	if got.domain != "docs.jrepp.com" {
		t.Errorf("resolved %q, want the first hop docs.jrepp.com", got.domain)
	}
}

func TestMiddlewareRejectsBadTrustedProxyConfig(t *testing.T) {
	t.Parallel()

	registry, err := sites.NewRegistry(multiSiteConfig())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := middleware.NewDomainResolver(registry, []string{"not-an-ip"}, hclog.NewNullLogger()); err == nil {
		t.Error("NewDomainResolver accepted a malformed trusted proxy entry")
	}
}

func TestMiddlewareIsolatesTenants(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, multiSiteConfig(), nil)

	// The same path on two hostnames must land on two different tenants; this
	// is the property every per-domain resource depends on.
	seen := map[string]string{}
	for _, host := range []string{"docs.jrepp.com", "notes.jrepp.com"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/documents", nil)
		req.Host = host

		got := run(t, resolver, req)
		if !got.reached {
			t.Fatalf("%s: handler not reached", host)
		}
		seen[host] = got.domain
	}

	if seen["docs.jrepp.com"] == seen["notes.jrepp.com"] {
		t.Fatalf("both hosts resolved to the same tenant %q", seen["docs.jrepp.com"])
	}
}
