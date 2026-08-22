package sites_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

func baseConfig(siteBlocks ...*config.Site) *config.Config {
	return &config.Config{
		LocalWorkspace: &config.LocalWorkspace{BasePath: "/srv/hermes/workspace"},
		Sites:          siteBlocks,
	}
}

func TestRegistryDerivesPerSiteResources(t *testing.T) {
	t.Parallel()

	cfg := baseConfig(
		&config.Site{Domain: "docs.jrepp.com"},
		&config.Site{Domain: "notes.jrepp.com"},
	)

	r, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	docs, ok := r.Lookup(domain.MustParse("docs.jrepp.com"))
	if !ok {
		t.Fatal("docs.jrepp.com did not resolve")
	}
	notes, ok := r.Lookup(domain.MustParse("notes.jrepp.com"))
	if !ok {
		t.Fatal("notes.jrepp.com did not resolve")
	}

	// The whole point of the registry is that two sites never share storage.
	if docs.SchemaName == notes.SchemaName {
		t.Errorf("sites share PostgreSQL schema %q", docs.SchemaName)
	}
	if docs.WorkspacePath == notes.WorkspacePath {
		t.Errorf("sites share workspace path %q", docs.WorkspacePath)
	}

	if want := "/srv/hermes/workspace/docs.jrepp.com"; docs.WorkspacePath != want {
		t.Errorf("docs workspace = %q, want %q", docs.WorkspacePath, want)
	}
	if want := "site_docs_jrepp_com"; docs.SchemaName != want {
		t.Errorf("docs schema = %q, want %q", docs.SchemaName, want)
	}
	if want := "https://docs.jrepp.com"; docs.BaseURL != want {
		t.Errorf("docs base URL = %q, want %q", docs.BaseURL, want)
	}
}

func TestRegistryNormalizesSiteLabels(t *testing.T) {
	t.Parallel()

	// An operator writing the label in mixed case must get the same tenant as
	// the lowercase request that arrives at runtime.
	r, err := sites.NewRegistry(baseConfig(&config.Site{Domain: "DOCS.JREPP.COM"}))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	site, ok := r.Lookup(domain.MustParse("docs.jrepp.com"))
	if !ok {
		t.Fatal("lowercase host did not resolve to the mixed-case site block")
	}
	if got := site.Domain.String(); got != "docs.jrepp.com" {
		t.Errorf("site domain = %q, want canonical form", got)
	}
}

func TestRegistryAliasesShareOneTenant(t *testing.T) {
	t.Parallel()

	r, err := sites.NewRegistry(baseConfig(&config.Site{
		Domain:  "docs.jrepp.com",
		Aliases: []string{"WWW.jrepp.com", "jrepp.com"},
	}))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	canonical, _ := r.Lookup(domain.MustParse("docs.jrepp.com"))
	for _, alias := range []string{"www.jrepp.com", "jrepp.com"} {
		site, ok := r.Lookup(domain.MustParse(alias))
		if !ok {
			t.Fatalf("alias %q did not resolve", alias)
		}
		// An alias is a second name for one tenant, so it must land on the
		// same storage, not a parallel namespace.
		if site != canonical {
			t.Errorf("alias %q resolved to a different site", alias)
		}
		if site.SchemaName != canonical.SchemaName {
			t.Errorf("alias %q uses schema %q, want %q", alias, site.SchemaName, canonical.SchemaName)
		}
	}
}

func TestRegistryRejectsAmbiguousRouting(t *testing.T) {
	t.Parallel()

	cases := map[string]*config.Config{
		"duplicate domain": baseConfig(
			&config.Site{Domain: "docs.jrepp.com"},
			&config.Site{Domain: "docs.jrepp.com"},
		),
		"alias collides with another site's domain": baseConfig(
			&config.Site{Domain: "docs.jrepp.com"},
			&config.Site{Domain: "notes.jrepp.com", Aliases: []string{"docs.jrepp.com"}},
		),
		"alias collides with another site's alias": baseConfig(
			&config.Site{Domain: "a.jrepp.com", Aliases: []string{"shared.jrepp.com"}},
			&config.Site{Domain: "b.jrepp.com", Aliases: []string{"shared.jrepp.com"}},
		),
		"domain differs only in case": baseConfig(
			&config.Site{Domain: "docs.jrepp.com"},
			&config.Site{Domain: "DOCS.JREPP.COM"},
		),
		"self alias": baseConfig(
			&config.Site{Domain: "docs.jrepp.com", Aliases: []string{"docs.jrepp.com"}},
		),
		"invalid domain": baseConfig(
			&config.Site{Domain: "not a hostname"},
		),
		"invalid alias": baseConfig(
			&config.Site{Domain: "docs.jrepp.com", Aliases: []string{"bad_host"}},
		),
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := sites.NewRegistry(cfg); err == nil {
				t.Error("NewRegistry accepted an ambiguous or invalid configuration")
			}
		})
	}
}

func TestRegistryDefaultSite(t *testing.T) {
	t.Parallel()

	t.Run("single site is the implicit default", func(t *testing.T) {
		t.Parallel()

		r, err := sites.NewRegistry(baseConfig(&config.Site{Domain: "docs.jrepp.com"}))
		if err != nil {
			t.Fatalf("NewRegistry: %v", err)
		}
		if _, ok := r.Lookup(domain.MustParse("anything.example")); !ok {
			t.Error("unknown host should fall back to the only configured site")
		}
	})

	t.Run("several sites reject unknown hosts", func(t *testing.T) {
		t.Parallel()

		r, err := sites.NewRegistry(baseConfig(
			&config.Site{Domain: "docs.jrepp.com"},
			&config.Site{Domain: "notes.jrepp.com"},
		))
		if err != nil {
			t.Fatalf("NewRegistry: %v", err)
		}
		// Guessing a tenant would route a stranger's request into somebody's
		// data, so an unknown host must simply fail.
		if _, ok := r.Lookup(domain.MustParse("anything.example")); ok {
			t.Error("unknown host resolved despite no default_site being set")
		}
	})

	t.Run("explicit default is honored", func(t *testing.T) {
		t.Parallel()

		cfg := baseConfig(
			&config.Site{Domain: "docs.jrepp.com"},
			&config.Site{Domain: "notes.jrepp.com"},
		)
		cfg.DefaultSite = "notes.jrepp.com"

		r, err := sites.NewRegistry(cfg)
		if err != nil {
			t.Fatalf("NewRegistry: %v", err)
		}
		site, ok := r.Lookup(domain.MustParse("anything.example"))
		if !ok {
			t.Fatal("unknown host did not fall back to default_site")
		}
		if got := site.Domain.String(); got != "notes.jrepp.com" {
			t.Errorf("default site = %q, want notes.jrepp.com", got)
		}
	})

	t.Run("default naming an unconfigured site is rejected", func(t *testing.T) {
		t.Parallel()

		cfg := baseConfig(&config.Site{Domain: "docs.jrepp.com"})
		cfg.DefaultSite = "absent.jrepp.com"

		if _, err := sites.NewRegistry(cfg); err == nil {
			t.Error("NewRegistry accepted a default_site that matches no block")
		}
	})
}

func TestRegistrySkipsDisabledSites(t *testing.T) {
	t.Parallel()

	r, err := sites.NewRegistry(baseConfig(
		&config.Site{Domain: "docs.jrepp.com"},
		&config.Site{Domain: "retired.jrepp.com", Disabled: true},
	))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if r.Len() != 1 {
		t.Errorf("registry holds %d sites, want 1", r.Len())
	}

	// With one enabled site it becomes the implicit default, so check that the
	// disabled hostname does not resolve to its *own* tenant.
	site, ok := r.Lookup(domain.MustParse("retired.jrepp.com"))
	if ok && site.Domain.String() == "retired.jrepp.com" {
		t.Error("disabled site is still serving its own hostname")
	}
}

func TestRegistryHonorsExplicitOverrides(t *testing.T) {
	t.Parallel()

	r, err := sites.NewRegistry(baseConfig(&config.Site{
		Domain:        "docs.jrepp.com",
		BaseURL:       "https://docs.jrepp.com/",
		SchemaName:    "legacy_docs",
		WorkspacePath: "/mnt/legacy/docs",
	}))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	site, _ := r.Lookup(domain.MustParse("docs.jrepp.com"))
	if site.SchemaName != "legacy_docs" {
		t.Errorf("schema override ignored: got %q", site.SchemaName)
	}
	if site.WorkspacePath != "/mnt/legacy/docs" {
		t.Errorf("workspace override ignored: got %q", site.WorkspacePath)
	}
	// A trailing slash here would produce double slashes in every generated link.
	if strings.HasSuffix(site.BaseURL, "/") {
		t.Errorf("base URL %q retains its trailing slash", site.BaseURL)
	}
}

func TestRegistryRequiresWorkspaceLocation(t *testing.T) {
	t.Parallel()

	// With neither a global base path nor a per-site path there is nowhere to
	// put documents; failing at startup beats failing on first write.
	cfg := &config.Config{Sites: []*config.Site{{Domain: "docs.jrepp.com"}}}
	if _, err := sites.NewRegistry(cfg); err == nil {
		t.Error("NewRegistry accepted a site with no resolvable workspace path")
	}
}

func TestSiteContext(t *testing.T) {
	t.Parallel()

	r, err := sites.NewRegistry(baseConfig(&config.Site{Domain: "docs.jrepp.com"}))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	want, _ := r.Lookup(domain.MustParse("docs.jrepp.com"))

	ctx := sites.NewContext(context.Background(), want)
	got, ok := sites.FromContext(ctx)
	if !ok || got != want {
		t.Error("site did not round-trip through the context")
	}

	if _, ok := sites.FromContext(context.Background()); ok {
		t.Error("FromContext found a site in a bare context")
	}
	if _, err := sites.RequireFromContext(context.Background()); err == nil {
		t.Error("RequireFromContext accepted a context with no site")
	}
}
