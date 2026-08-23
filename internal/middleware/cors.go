// Package middleware provides HTTP middleware.
package middleware

import (
	"net/http"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// CORS answers cross-origin requests for a fixed set of origins.
//
// Origins are matched exactly. The previous implementation matched by
// substring -- `strings.Contains(origin, "office.com")`, and in its permissive
// mode `strings.Contains(origin, "localhost")` -- while also sending
// Access-Control-Allow-Credentials: true. Those two together are the dangerous
// combination: an attacker who registers office.com.example.net, or any
// hostname containing "localhost", satisfies a substring check, and the
// browser then hands them a credentialed cross-origin session. Nothing in the
// server wired that middleware up, so it was never exploitable here, but it
// was exported and one call away from being so.
//
// It also defaulted to its permissive mode, which meant the safe-looking
// constructor was the unsafe one.
//
// There is no permissive mode now and no default allowlist. A caller that
// wants cross-origin access says exactly which origins, in full, including the
// scheme -- and an empty list means no cross-origin access at all, which is
// what a same-origin deployment behind nginx wants.
type CORS struct {
	allowed map[string]bool
	log     hclog.Logger
}

// NewCORS builds the middleware from an exact-match allowlist.
//
// Each origin must be a full origin: scheme, host, and port if non-default --
// "https://addin.example.com", not "example.com". Anything that does not parse
// as one is rejected here rather than silently never matching.
func NewCORS(origins []string, log hclog.Logger) (*CORS, error) {
	c := &CORS{allowed: make(map[string]bool, len(origins)), log: log}

	for _, origin := range origins {
		trimmed := strings.TrimSpace(strings.TrimSuffix(origin, "/"))
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			return nil, &InvalidOriginError{Origin: origin}
		}
		c.allowed[trimmed] = true
	}

	return c, nil
}

// InvalidOriginError reports an allowlist entry that is not an origin.
type InvalidOriginError struct{ Origin string }

func (e *InvalidOriginError) Error() string {
	return "middleware: " + e.Origin +
		" is not an origin; use a full scheme://host[:port], e.g. https://example.com"
}

// Middleware wraps next with the CORS headers.
func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// A request with no Origin is same-origin or not from a browser.
		// Nothing to allow, and nothing to deny: the browser is not going to
		// consult CORS for it either way.
		if origin == "" {
			next.ServeHTTP(w, r)

			return
		}

		// Vary is set whether or not the origin is allowed. Without it a cache
		// can serve one origin's allowed response to another.
		w.Header().Add("Vary", "Origin")

		if !c.allowed[origin] {
			if c.log != nil {
				c.log.Warn("cross-origin request denied",
					"origin", origin, "method", r.Method, "path", r.URL.Path)
			}
			// No CORS headers: the browser blocks it. A preflight still gets a
			// response, just not a permissive one.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusForbidden)

				return
			}

			next.ServeHTTP(w, r)

			return
		}

		// Echo the exact origin, never "*". A wildcard is invalid alongside
		// credentials and browsers reject the pair, so sending it would only
		// look permissive while breaking the request.
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS")
			// Cookie is a forbidden header name: the browser sets it and a
			// page cannot, so listing it here means nothing. Credentials are
			// carried by Allow-Credentials above.
			w.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)

			return
		}

		next.ServeHTTP(w, r)
	})
}
