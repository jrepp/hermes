package domain

import (
	"context"
	"fmt"
)

// contextKey is unexported so no other package can write this value into a
// context. The only way a request carries a domain is through NewContext,
// which takes an already-canonical Name.
type contextKey struct{}

// NewContext returns a copy of ctx carrying n.
func NewContext(ctx context.Context, n Name) context.Context {
	return context.WithValue(ctx, contextKey{}, n)
}

// FromContext returns the domain carried by ctx.
//
// ok is false when no domain was resolved, which for a request-scoped context
// means the domain-resolution middleware did not run. Callers that index
// storage by domain must treat that as an error, never as "use the default" —
// falling back would write one tenant's data into another's namespace.
func FromContext(ctx context.Context) (Name, bool) {
	n, ok := ctx.Value(contextKey{}).(Name)
	if !ok || n.IsZero() {
		return Name{}, false
	}

	return n, true
}

// RequireFromContext returns the domain carried by ctx, or an error.
//
// Prefer this over FromContext in handlers and storage code: it turns a
// missing domain into a failed request rather than a silent cross-tenant
// write.
func RequireFromContext(ctx context.Context) (Name, error) {
	n, ok := FromContext(ctx)
	if !ok {
		return Name{}, fmt.Errorf("domain: no domain in context (is the domain middleware installed?)")
	}

	return n, nil
}
