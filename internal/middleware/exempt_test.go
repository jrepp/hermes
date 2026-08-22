package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/middleware"
)

func label(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, name)
	})
}

func TestExemptRoutesTheExactPath(t *testing.T) {
	t.Parallel()

	h := middleware.Exempt("/health", label("health"), label("next"))

	for _, tc := range []struct{ path, want string }{
		{"/health", "health"},
		{"/", "next"},
		{"/healthz", "next"},
		{"/health/", "next"},
		{"/health/sub", "next"},
		{"/api/v2/me", "next"},
	} {
		r := httptest.NewRequest(http.MethodGet, "http://example.com"+tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if got := w.Body.String(); got != tc.want {
			t.Errorf("%s -> %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestExemptIgnoresHost is the point: the exempt path must answer regardless of
// which hostname asked, because a loopback probe sends Host: 127.0.0.1 and no
// deployment configures that as a site.
func TestExemptIgnoresHost(t *testing.T) {
	t.Parallel()

	h := middleware.Exempt("/health", label("health"), label("next"))

	for _, host := range []string{"127.0.0.1:8000", "docs.jrepp.com", "", "[::1]:8000"} {
		r := httptest.NewRequest(http.MethodGet, "http://placeholder/health", nil)
		r.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if got := w.Body.String(); got != "health" {
			t.Errorf("Host %q -> %q, want health", host, got)
		}
	}
}
