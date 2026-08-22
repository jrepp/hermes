package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp-forge/hermes/pkg/domain"
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

func (s Server) ForDomain(ctx context.Context) (Server, error) {
	if s.SiteDBs == nil {
		return s, nil
	}

	scopedDB, err := s.SiteDBs.For(ctx)
	if err != nil {
		return Server{}, err
	}
	s.DB = scopedDB

	// Search and workspace are scoped by the same rule: a request naming a
	// site with no provider is an error, never a fall back to the shared one.
	if name, ok := domain.FromContext(ctx); ok && !name.IsZero() {
		searchProvider, ok := s.SiteSearch[name]
		if !ok {
			return Server{}, fmt.Errorf(
				"server: no search provider configured for site %q", name)
		}
		s.SearchProvider = searchProvider

		workspaceProvider, ok := s.SiteWorkspace[name]
		if !ok {
			return Server{}, fmt.Errorf(
				"server: no workspace provider configured for site %q", name)
		}
		s.WorkspaceProvider = workspaceProvider
	}

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

// VerifySiteResources checks that every configured site has the per-tenant
// resources ForDomain will look for.
//
// The check exists because the failure it catches is a wiring mistake made
// once at startup and paid for at request time, tenant by tenant: a site with
// no pool or no search provider serves 500s, and a site whose resources were
// quietly shared with another serves the wrong data. Both are far cheaper to
// find here.
func (s Server) VerifySiteResources(names []domain.Name) error {
	if len(names) == 0 {
		return nil
	}
	if s.SiteDBs == nil {
		return fmt.Errorf(
			"server: %d site(s) configured but no per-site databases were opened",
			len(names))
	}

	for _, name := range names {
		if _, err := s.SiteDBs.For(NewDomainContext(name)); err != nil {
			return fmt.Errorf("server: site %q has no database: %w", name, err)
		}
		if _, ok := s.SiteSearch[name]; !ok {
			return fmt.Errorf("server: site %q has no search provider", name)
		}
		if _, ok := s.SiteWorkspace[name]; !ok {
			return fmt.Errorf("server: site %q has no workspace provider", name)
		}
	}

	return nil
}

// NewDomainContext is a small helper so callers do not need to construct a
// context just to ask about a domain.
func NewDomainContext(name domain.Name) context.Context {
	return domain.NewContext(context.Background(), name)
}
