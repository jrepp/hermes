// Package sites resolves an incoming request's hostname to the tenant that
// serves it.
//
// One Hermes process listens on one socket and serves many subdomains. The
// registry is the single mapping from hostname to tenant; every per-domain
// resource (database schema, workspace directory, search namespace, session
// scope) is keyed off the Site this package returns.
package sites

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/database"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// Site is a resolved tenant: one canonical domain plus everything derived from
// it. Construct sites only through NewRegistry, so that derivation happens in
// exactly one place.
type Site struct {
	// Domain is the canonical hostname identifying this tenant.
	Domain domain.Name

	// Aliases are the additional hostnames routed here.
	Aliases []domain.Name

	// BaseURL is the externally reachable origin, without a trailing slash.
	BaseURL string

	// SchemaName is the PostgreSQL schema holding this site's tables.
	SchemaName string

	// Postgres is the resolved connection for this site: the global block with
	// any per-site override applied, plus SchemaName. Two sites may point at
	// different servers or different databases entirely.
	Postgres config.Postgres

	// WorkspacePath is the directory holding this site's documents.
	WorkspacePath string
}

// Registry maps every configured hostname, canonical or alias, to its Site.
type Registry struct {
	byHost      map[domain.Name]*Site
	ordered     []*Site
	defaultSite *Site
}

// NewRegistry builds the hostname-to-site mapping from configuration.
//
// It rejects configurations that would make routing ambiguous — a hostname
// claimed by two sites, or a default naming a site that does not exist —
// rather than resolving the ambiguity silently. Two sites sharing a hostname
// would mean requests landing in whichever tenant won a map iteration.
func NewRegistry(cfg *config.Config) (*Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("sites: nil config")
	}

	r := &Registry{byHost: make(map[domain.Name]*Site)}

	for _, sc := range cfg.Sites {
		if sc.Disabled {
			continue
		}

		site, err := buildSite(cfg, sc)
		if err != nil {
			return nil, err
		}

		// Claim the canonical hostname and every alias, failing on any
		// hostname already spoken for.
		for _, host := range append([]domain.Name{site.Domain}, site.Aliases...) {
			if existing, taken := r.byHost[host]; taken {
				return nil, fmt.Errorf(
					"sites: hostname %q is claimed by both site %q and site %q",
					host, existing.Domain, site.Domain)
			}
			r.byHost[host] = site
		}

		r.ordered = append(r.ordered, site)
	}

	if err := r.resolveDefault(cfg); err != nil {
		return nil, err
	}

	return r, nil
}

// buildSite derives a Site from one config block, filling in every value the
// operator left unset.
func buildSite(cfg *config.Config, sc *config.Site) (*Site, error) {
	name, err := domain.Parse(sc.Domain)
	if err != nil {
		return nil, fmt.Errorf("sites: site %q has an invalid domain: %w", sc.Domain, err)
	}

	site := &Site{
		Domain:     name,
		BaseURL:    strings.TrimSuffix(sc.BaseURL, "/"),
		SchemaName: sc.SchemaName,
	}

	for _, raw := range sc.Aliases {
		alias, err := domain.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("sites: site %q has an invalid alias %q: %w", sc.Domain, raw, err)
		}
		if alias == name {
			return nil, fmt.Errorf("sites: site %q lists its own domain as an alias", sc.Domain)
		}
		site.Aliases = append(site.Aliases, alias)
	}

	if site.BaseURL == "" {
		site.BaseURL = "https://" + name.String()
	}
	if _, err := url.Parse(site.BaseURL); err != nil {
		return nil, fmt.Errorf("sites: site %q has an invalid base_url %q: %w", sc.Domain, site.BaseURL, err)
	}

	if site.SchemaName == "" {
		site.SchemaName = name.SchemaName()
	}
	if err := database.ValidateSchemaName(site.SchemaName); err != nil {
		return nil, fmt.Errorf("sites: site %q: %w", sc.Domain, err)
	}

	site.Postgres = cfg.PostgresForSite(name, sc.Database, site.SchemaName)

	site.WorkspacePath = sc.WorkspacePath
	if site.WorkspacePath == "" {
		base := ""
		if cfg.LocalWorkspace != nil {
			base = cfg.LocalWorkspace.BasePath
		}
		if base == "" {
			return nil, fmt.Errorf(
				"sites: site %q needs a workspace_path because local_workspace.base_path is unset",
				sc.Domain)
		}
		// PathSegment is safe to join: Parse guarantees it is a single
		// lowercase DNS label sequence, never "." or ".." or a path.
		site.WorkspacePath = filepath.Join(base, name.PathSegment())
	}

	return site, nil
}

// resolveDefault selects the site used for hostnames that match no block.
func (r *Registry) resolveDefault(cfg *config.Config) error {
	if cfg.DefaultSite == "" {
		// With exactly one site configured, that site is unambiguously the
		// default. With several, requiring an explicit choice avoids routing
		// unknown hosts into an arbitrary tenant.
		if len(r.ordered) == 1 {
			r.defaultSite = r.ordered[0]
		}

		return nil
	}

	name, err := domain.Parse(cfg.DefaultSite)
	if err != nil {
		return fmt.Errorf("sites: default_site %q is not a valid domain: %w", cfg.DefaultSite, err)
	}

	site, ok := r.byHost[name]
	if !ok {
		return fmt.Errorf("sites: default_site %q does not match any configured site", cfg.DefaultSite)
	}
	r.defaultSite = site

	return nil
}

// Lookup returns the site serving host.
//
// Aliases resolve to the same Site as their canonical domain. When host
// matches nothing, the configured default is returned if there is one.
func (r *Registry) Lookup(host domain.Name) (*Site, bool) {
	if site, ok := r.byHost[host]; ok {
		return site, true
	}
	if r.defaultSite != nil {
		return r.defaultSite, true
	}

	return nil, false
}

// Sites returns the configured sites in declaration order.
func (r *Registry) Sites() []*Site {
	out := make([]*Site, len(r.ordered))
	copy(out, r.ordered)

	return out
}

// Default returns the fallback site, or nil when unknown hosts are rejected.
func (r *Registry) Default() *Site {
	return r.defaultSite
}

// Len returns the number of configured sites.
func (r *Registry) Len() int {
	return len(r.ordered)
}
