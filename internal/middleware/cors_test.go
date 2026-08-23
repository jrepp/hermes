package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/middleware"
)

func corsHandler(t *testing.T, origins ...string) http.Handler {
	t.Helper()

	c, err := middleware.NewCORS(origins, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("NewCORS: %v", err)
	}

	return c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func request(t *testing.T, h http.Handler, method, origin string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(method, "http://docs.jrepp.com/api/v2/me", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	return w
}

// TestCORSRejectsSubstringLookalikes is the regression test for the reason
// this was rewritten.
//
// The previous implementation matched origins with strings.Contains and sent
// Access-Control-Allow-Credentials: true. Anyone who could register a hostname
// containing the allowed string -- office.com.example.net, or in its permissive
// mode anything containing "localhost" -- got a credentialed cross-origin
// session with the victim's cookies attached.
func TestCORSRejectsSubstringLookalikes(t *testing.T) {
	t.Parallel()

	h := corsHandler(t, "https://addin.office.com", "http://localhost:8000")

	for _, origin := range []string{
		"https://office.com.evil.example",
		"https://evil.example/?x=https://addin.office.com",
		"https://addin.office.com.evil.example",
		"https://notlocalhost:8000",
		"http://localhost:8000.evil.example",
		"https://localhost.evil.example",
		"http://addin.office.com", // right host, wrong scheme
		"https://addin.office.com:8443",
		"null",
	} {
		w := request(t, h, http.MethodGet, origin)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%q was allowed as %q", origin, got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("%q was granted credentials", origin)
		}
	}
}

func TestCORSAllowsExactOrigins(t *testing.T) {
	t.Parallel()

	h := corsHandler(t, "https://addin.office.com", "http://localhost:8000")

	for _, origin := range []string{"https://addin.office.com", "http://localhost:8000"} {
		w := request(t, h, http.MethodGet, origin)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin %q got Allow-Origin %q", origin, got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("origin %q got Allow-Credentials %q", origin, got)
		}
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Errorf("origin %q got Vary %q; without it a cache can serve one "+
				"origin's response to another", origin, got)
		}
	}
}

// TestCORSNeverSendsWildcardWithCredentials pins the pair that browsers
// reject. Sending "*" alongside credentials looks permissive and breaks every
// request that relies on it.
func TestCORSNeverSendsWildcardWithCredentials(t *testing.T) {
	t.Parallel()

	h := corsHandler(t, "https://addin.office.com")

	for _, origin := range []string{"", "null", "https://other.example"} {
		w := request(t, h, http.MethodGet, origin)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got == "*" {
			t.Errorf("origin %q produced a wildcard alongside credentials", origin)
		}
	}
}

// TestCORSWithNoAllowlistDeniesEverything is the default a same-origin
// deployment behind nginx wants, and the previous default was the opposite.
func TestCORSWithNoAllowlistDeniesEverything(t *testing.T) {
	t.Parallel()

	h := corsHandler(t)

	w := request(t, h, http.MethodGet, "https://anything.example")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("an empty allowlist allowed %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	t.Parallel()

	h := corsHandler(t, "https://addin.office.com")

	allowed := request(t, h, http.MethodOptions, "https://addin.office.com")
	if allowed.Code != http.StatusNoContent {
		t.Errorf("allowed preflight status = %d, want 204", allowed.Code)
	}
	if allowed.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("allowed preflight carries no Allow-Methods")
	}

	denied := request(t, h, http.MethodOptions, "https://evil.example")
	if denied.Code != http.StatusForbidden {
		t.Errorf("denied preflight status = %d, want 403", denied.Code)
	}
	if denied.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("denied preflight still carried Allow-Origin")
	}
}

// TestCORSPassesThroughSameOriginRequests confirms a request with no Origin --
// same-origin, or not from a browser -- is untouched.
func TestCORSPassesThroughSameOriginRequests(t *testing.T) {
	t.Parallel()

	h := corsHandler(t, "https://addin.office.com")

	w := request(t, h, http.MethodGet, "")
	if w.Code != http.StatusOK {
		t.Errorf("same-origin request got %d", w.Code)
	}
	if w.Header().Get("Vary") != "" {
		t.Error("same-origin request got a Vary header it does not need")
	}
}

func TestNewCORSRejectsNonOrigins(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"example.com", "//example.com", "ftp://example.com"} {
		if _, err := middleware.NewCORS([]string{bad}, hclog.NewNullLogger()); err == nil {
			t.Errorf("%q was accepted as an origin; it would never match and "+
				"the misconfiguration would be silent", bad)
		}
	}
}
