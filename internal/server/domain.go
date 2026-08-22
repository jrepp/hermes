package server

import (
	"context"
	"net/http"
)

// ForDomain returns a copy of the Server whose per-tenant resources are bound
// to the site named in ctx.
//
// Handlers must call this (or ForRequest) before touching the database. The
// Server they are constructed with holds the process-wide defaults, and in a
// multi-site deployment those defaults belong to no tenant in particular.
//
// A request naming a site with no configured resources is an error rather than
// a fallback. Falling back would run one tenant's query against another's
// schema, which is the failure the whole arrangement exists to prevent.
//
// Only the database is scoped here so far. SearchProvider is still the shared
// process-wide one, so search results are not yet isolated between sites.
func (s Server) ForDomain(ctx context.Context) (Server, error) {
	if s.SiteDBs == nil {
		return s, nil
	}

	scopedDB, err := s.SiteDBs.For(ctx)
	if err != nil {
		return Server{}, err
	}
	s.DB = scopedDB

	return s, nil
}

// ForRequest is ForDomain plus the error handling every handler would
// otherwise repeat.
//
// It writes a 500 and returns false when the site cannot be resolved, so the
// caller's whole body is:
//
//	srv, ok := srv.ForRequest(w, r)
//	if !ok {
//		return
//	}
func (s Server) ForRequest(w http.ResponseWriter, r *http.Request) (Server, bool) {
	scoped, err := s.ForDomain(r.Context())
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("could not resolve site for request",
				"error", err, "host", r.Host, "path", r.URL.Path)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)

		return Server{}, false
	}

	return scoped, true
}
