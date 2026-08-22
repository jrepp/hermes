package middleware

import "net/http"

// Exempt routes requests for an exact path to exempt, and everything else to
// next.
//
// It exists for the liveness probe. Health checks address the process, not a
// tenant: a monitor hitting the loopback listener directly sends
// `Host: 127.0.0.1`, which is not a configured site, so behind the domain
// resolver it would get 421 and the service would look down while being
// perfectly healthy. Requiring every probe to forge a Host header instead
// would make the check depend on the routing it is supposed to be independent
// of.
//
// The match is exact, so it cannot be widened by a crafted path: "/health/../"
// and "/health/x" both go to next.
func Exempt(path string, exempt, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path {
			exempt.ServeHTTP(w, r)

			return
		}

		next.ServeHTTP(w, r)
	})
}
