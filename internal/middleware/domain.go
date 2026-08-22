package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// forwardedHostHeader is the de-facto header a reverse proxy uses to preserve
// the hostname the client asked for.
const forwardedHostHeader = "X-Forwarded-Host"

// DomainResolver turns each request's hostname into a canonical domain and
// attaches it to the request context.
//
// This runs ahead of authentication, because which credentials are valid is
// itself domain-scoped: a session issued for one subdomain must not authorize
// a request to another.
type DomainResolver struct {
	registry *sites.Registry
	trusted  []*net.IPNet
	log      hclog.Logger
}

// NewDomainResolver builds the middleware.
//
// trustedProxies is a list of CIDRs. X-Forwarded-Host is honored only when the
// immediate peer's address falls inside one of them; from anywhere else the
// header is attacker-controlled and ignored. An empty list disables the header
// entirely, which is correct when nothing sits in front of Hermes.
func NewDomainResolver(
	registry *sites.Registry, trustedProxies []string, log hclog.Logger,
) (*DomainResolver, error) {
	trusted, err := parseCIDRs(trustedProxies)
	if err != nil {
		return nil, err
	}

	return &DomainResolver{registry: registry, trusted: trusted, log: log}, nil
}

// parseCIDRs accepts CIDR blocks and bare IP addresses, normalizing the latter
// to single-address networks so operators can write "127.0.0.1" directly.
func parseCIDRs(entries []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if _, network, err := net.ParseCIDR(entry); err == nil {
			out = append(out, network)
			continue
		}

		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, &net.ParseError{Type: "trusted proxy CIDR or IP", Text: entry}
		}

		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}

	return out, nil
}

// isTrustedPeer reports whether the request's immediate peer is a proxy we
// allow to rewrite the hostname.
func (d *DomainResolver) isTrustedPeer(remoteAddr string) bool {
	if len(d.trusted) == 0 {
		return false
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, network := range d.trusted {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// requestHost returns the hostname to resolve for r.
//
// r.Host is authoritative. X-Forwarded-Host is consulted only when the peer is
// a trusted proxy, and only its first entry: the header accumulates a
// comma-separated chain as it crosses hops, and the first element is the one
// the original client sent.
func (d *DomainResolver) requestHost(r *http.Request) string {
	if forwarded := r.Header.Get(forwardedHostHeader); forwarded != "" && d.isTrustedPeer(r.RemoteAddr) {
		if first, _, found := strings.Cut(forwarded, ","); found {
			return strings.TrimSpace(first)
		}

		return strings.TrimSpace(forwarded)
	}

	return r.Host
}

// Middleware resolves the domain for each request and stores it in the
// context, rejecting hostnames that map to no configured site.
func (d *DomainResolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := d.requestHost(r)

		name, err := domain.Parse(raw)
		if err != nil {
			d.log.Warn("rejecting request with unparseable host",
				"host", raw, "path", r.URL.Path, "error", err)
			http.Error(w, "Invalid Host header", http.StatusBadRequest)

			return
		}

		site, ok := d.registry.Lookup(name)
		if !ok {
			// 421 tells the client this connection cannot serve the requested
			// authority, which is exactly the situation and prompts well-behaved
			// clients to re-resolve rather than retry blindly.
			d.log.Warn("rejecting request for unconfigured host",
				"host", name.String(), "path", r.URL.Path)
			http.Error(w, "Unknown host", http.StatusMisdirectedRequest)

			return
		}

		// Store the site's canonical domain rather than the requested one, so
		// an alias and its canonical name resolve to a single storage
		// namespace instead of two.
		ctx := domain.NewContext(r.Context(), site.Domain)
		ctx = sites.NewContext(ctx, site)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
