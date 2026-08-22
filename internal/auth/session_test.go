package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/middleware"
	"github.com/hashicorp-forge/hermes/internal/session"
	"github.com/hashicorp-forge/hermes/internal/sites"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	dexadapter "github.com/hashicorp-forge/hermes/pkg/auth/adapters/dex"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

const authTestSecret = "auth-test-secret-long-enough-for-hmac-use"

func testSigner(t *testing.T) *session.Signer {
	t.Helper()

	key, err := session.DeriveKey(authTestSecret)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	s, err := session.NewSigner(key, time.Hour)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	return s
}

// echoEmail reports whichever identity made it through authentication, so a
// test can tell "rejected" from "authenticated as somebody else".
func echoEmail() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, _ := pkgauth.GetUserEmail(r.Context())
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, email)
	})
}

func dexConfig() config.Config {
	return config.Config{
		Dex: &dexadapter.Config{
			IssuerURL: "http://localhost:5556/dex",
			ClientID:  "hermes",
			Disabled:  false,
		},
	}
}

// twoSiteStack builds the same handler chain the server does: resolve the
// hostname to a site, then authenticate. Testing them composed is the point --
// the domain binding in a token is only meaningful if the resolver actually
// runs first.
func twoSiteStack(t *testing.T, signer *session.Signer) http.Handler {
	t.Helper()

	cfg := &config.Config{
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{Domain: "docs.jrepp.com", Aliases: []string{"www.jrepp.com"}},
			{Domain: "notes.jrepp.com"},
		},
	}

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	log := hclog.NewNullLogger()
	resolver, err := middleware.NewDomainResolver(registry, nil, log)
	if err != nil {
		t.Fatalf("NewDomainResolver: %v", err)
	}

	return resolver.Middleware(
		AuthenticateRequest(dexConfig(), nil, nil, signer, log, echoEmail()))
}

func get(t *testing.T, h http.Handler, host, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "http://"+host+"/api/v2/me", nil)
	r.Host = host
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: session.CookieName, Value: cookie})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	return w
}

// TestPlaintextEmailCookieIsRejected is the regression test for the bypass
// this work closed. Hermes previously returned the raw cookie value as the
// authenticated identity, so this exact request authenticated as the victim.
func TestPlaintextEmailCookieIsRejected(t *testing.T) {
	h := twoSiteStack(t, testSigner(t))

	w := get(t, h, "docs.jrepp.com", "victim@jrepp.com")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d (%q), want 401; a bare email must not authenticate",
			w.Code, w.Body.String())
	}
}

func TestSignedCookieAuthenticates(t *testing.T) {
	signer := testSigner(t)
	h := twoSiteStack(t, signer)

	token, err := signer.Issue("user@jrepp.com", domain.MustParse("docs.jrepp.com"))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	w := get(t, h, "docs.jrepp.com", token)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%q), want 200", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "user@jrepp.com" {
		t.Fatalf("authenticated as %q, want user@jrepp.com", got)
	}
}

// TestSessionDoesNotCrossSites is the tenant-isolation guarantee end to end.
func TestSessionDoesNotCrossSites(t *testing.T) {
	signer := testSigner(t)
	h := twoSiteStack(t, signer)

	token, err := signer.Issue("user@jrepp.com", domain.MustParse("docs.jrepp.com"))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	w := get(t, h, "notes.jrepp.com", token)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d (%q), want 401; a docs.jrepp.com session must not "+
			"authenticate on notes.jrepp.com", w.Code, w.Body.String())
	}
}

// TestSessionFollowsAliasToCanonicalSite pins the other half of that rule: an
// alias is the same tenant, so its session must keep working. The resolver
// stores the canonical domain, which is what makes this true.
func TestSessionFollowsAliasToCanonicalSite(t *testing.T) {
	signer := testSigner(t)
	h := twoSiteStack(t, signer)

	token, err := signer.Issue("user@jrepp.com", domain.MustParse("docs.jrepp.com"))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	w := get(t, h, "www.jrepp.com", token)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%q), want 200; www.jrepp.com is an alias of the "+
			"site the session was issued for", w.Code, w.Body.String())
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	key, err := session.DeriveKey(authTestSecret)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	// A one-nanosecond lifetime has certainly elapsed by the time the request
	// is served.
	shortLived, err := session.NewSigner(key, time.Nanosecond)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	token, err := shortLived.Issue("user@jrepp.com", domain.MustParse("docs.jrepp.com"))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	h := twoSiteStack(t, testSigner(t))
	if w := get(t, h, "docs.jrepp.com", token); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an expired session", w.Code)
	}
}

func TestMissingCookieIsRejected(t *testing.T) {
	h := twoSiteStack(t, testSigner(t))

	if w := get(t, h, "docs.jrepp.com", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with no cookie", w.Code)
	}
}

// TestNilSignerRefuses covers the fail-closed path. If the server somehow
// reaches request handling without a signer, every request must be
// unauthenticated -- never fall back to believing the cookie.
func TestNilSignerRefuses(t *testing.T) {
	log := hclog.NewNullLogger()
	h := AuthenticateRequest(dexConfig(), nil, nil, nil, log, echoEmail())

	r := httptest.NewRequest(http.MethodGet, "http://docs.jrepp.com/api/v2/me", nil)
	r.AddCookie(&http.Cookie{Name: session.CookieName, Value: "user@jrepp.com"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d (%q), want 401", w.Code, w.Body.String())
	}
}

// TestUnknownHostNeverReachesAuth checks the ordering: a request for a
// hostname this process does not serve is refused before any credential is
// examined, so an unrecognised Host cannot be used to reach the unscoped
// session path.
func TestUnknownHostNeverReachesAuth(t *testing.T) {
	signer := testSigner(t)
	h := twoSiteStack(t, signer)

	token, err := signer.Issue("user@jrepp.com", domain.Name{})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	w := get(t, h, "unknown.example.com", token)

	if w.Code != http.StatusMisdirectedRequest {
		t.Fatalf("status = %d (%q), want 421", w.Code, w.Body.String())
	}
}
