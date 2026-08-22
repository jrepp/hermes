package local

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/domain"
)

func TestConfigWithoutDomainKeepsFlatLayout(t *testing.T) {
	t.Parallel()

	// The single-tenant layout must be unchanged, so existing deployments keep
	// finding their documents where they left them.
	c := &Config{BasePath: "/srv/hermes"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if want := "/srv/hermes"; c.Root() != want {
		t.Errorf("Root() = %q, want %q", c.Root(), want)
	}
	if want := filepath.Join("/srv/hermes", "docs"); c.DocsPath != want {
		t.Errorf("DocsPath = %q, want %q", c.DocsPath, want)
	}
}

func TestConfigWithDomainRootsPathsUnderIt(t *testing.T) {
	t.Parallel()

	c := &Config{BasePath: "/srv/hermes", Domain: domain.MustParse("docs.jrepp.com")}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	root := "/srv/hermes/docs.jrepp.com"
	if c.Root() != root {
		t.Errorf("Root() = %q, want %q", c.Root(), root)
	}

	for name, got := range map[string]string{
		"DocsPath":    c.DocsPath,
		"DraftsPath":  c.DraftsPath,
		"FoldersPath": c.FoldersPath,
		"UsersPath":   c.UsersPath,
		"TokensPath":  c.TokensPath,
	} {
		if !strings.HasPrefix(got, root+string(filepath.Separator)) {
			t.Errorf("%s = %q, which is not under the domain root %q", name, got, root)
		}
	}
}

func TestTwoDomainsNeverShareStorage(t *testing.T) {
	t.Parallel()

	// This is the isolation guarantee the whole feature rests on: the same
	// base path, two domains, zero overlap.
	docs := &Config{BasePath: "/srv/hermes", Domain: domain.MustParse("docs.jrepp.com")}
	notes := &Config{BasePath: "/srv/hermes", Domain: domain.MustParse("notes.jrepp.com")}

	for _, c := range []*Config{docs, notes} {
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	}

	pairs := map[string][2]string{
		"root":    {docs.Root(), notes.Root()},
		"docs":    {docs.DocsPath, notes.DocsPath},
		"drafts":  {docs.DraftsPath, notes.DraftsPath},
		"folders": {docs.FoldersPath, notes.FoldersPath},
		"users":   {docs.UsersPath, notes.UsersPath},
		"tokens":  {docs.TokensPath, notes.TokensPath},
	}
	for name, pair := range pairs {
		if pair[0] == pair[1] {
			t.Errorf("%s path is shared between domains: %q", name, pair[0])
		}
	}
}

func TestDomainScopedConfigRejectsEscapingPaths(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*Config){
		"absolute path elsewhere": func(c *Config) { c.DocsPath = "/etc/hermes/docs" },
		"another domain's subtree": func(c *Config) {
			c.DraftsPath = "/srv/hermes/notes.jrepp.com/drafts"
		},
		"parent traversal":     func(c *Config) { c.FoldersPath = "/srv/hermes/docs.jrepp.com/../folders" },
		"escaping users file":  func(c *Config) { c.UsersPath = "/srv/hermes/users.json" },
		"escaping tokens file": func(c *Config) { c.TokensPath = "/srv/tokens.json" },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := &Config{BasePath: "/srv/hermes", Domain: domain.MustParse("docs.jrepp.com")}
			mutate(c)

			// A misconfigured path that reaches into a sibling tenant must fail
			// at startup, not quietly serve the wrong documents.
			if err := c.Validate(); err == nil {
				t.Errorf("Validate accepted an escaping path")
			}
		})
	}
}

func TestDomainScopedConfigAcceptsPathsInsideRoot(t *testing.T) {
	t.Parallel()

	c := &Config{
		BasePath:   "/srv/hermes",
		Domain:     domain.MustParse("docs.jrepp.com"),
		DocsPath:   "/srv/hermes/docs.jrepp.com/published",
		DraftsPath: "/srv/hermes/docs.jrepp.com/wip",
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate rejected paths inside the domain root: %v", err)
	}
	if c.DocsPath != "/srv/hermes/docs.jrepp.com/published" {
		t.Errorf("explicit DocsPath was overwritten: %q", c.DocsPath)
	}
}

func TestUnscopedConfigStillAllowsExplicitPaths(t *testing.T) {
	t.Parallel()

	// Containment applies only to domain-scoped adapters; single-tenant
	// deployments may still point anywhere they like.
	c := &Config{BasePath: "/srv/hermes", DocsPath: "/mnt/other/docs"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if c.DocsPath != "/mnt/other/docs" {
		t.Errorf("DocsPath = %q, want the explicit value", c.DocsPath)
	}
}
