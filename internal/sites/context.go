package sites

import (
	"context"
	"fmt"
)

// contextKey is unexported so only this package can attach a Site to a
// context, keeping the middleware the single source of request-scoped sites.
type contextKey struct{}

// NewContext returns a copy of ctx carrying site.
func NewContext(ctx context.Context, site *Site) context.Context {
	return context.WithValue(ctx, contextKey{}, site)
}

// FromContext returns the site resolved for this request.
//
// ok is false when the domain-resolution middleware did not run.
func FromContext(ctx context.Context) (*Site, bool) {
	site, ok := ctx.Value(contextKey{}).(*Site)
	if !ok || site == nil {
		return nil, false
	}

	return site, true
}

// RequireFromContext returns the request's site, or an error.
//
// Handlers that reach storage should use this rather than FromContext: a
// missing site must fail the request, never fall back to some other tenant.
func RequireFromContext(ctx context.Context) (*Site, error) {
	site, ok := FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("sites: no site in context (is the domain middleware installed?)")
	}

	return site, nil
}
