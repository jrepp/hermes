package sites

import (
	"net/http"
	"strings"
)

// SecureCookies reports whether cookies issued for this site must carry the
// Secure attribute.
func (s *Site) SecureCookies() bool {
	return strings.HasPrefix(strings.ToLower(s.BaseURL), "https://")
}

// SecureCookiesForRequest reports whether cookies on this request must be
// marked Secure.
//
// It reads the site's configured origin in preference to the connection,
// because in the deployment topology Hermes targets -- nginx terminating TLS
// and proxying to a loopback listener -- r.TLS is nil on every request even
// though the browser is speaking HTTPS. Deriving the flag from the connection
// alone would ship session cookies without Secure to every production user,
// making them available to any attacker who can force one plaintext request.
//
// The connection is the fallback for deployments with no site blocks.
func SecureCookiesForRequest(r *http.Request) bool {
	if site, ok := FromContext(r.Context()); ok && site.BaseURL != "" {
		return site.SecureCookies()
	}

	return r.TLS != nil
}
