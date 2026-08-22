package sites_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/sites"
)

func cfgWith(base string, siteBlocks ...*config.Site) *config.Config {
	return &config.Config{
		Postgres:       &config.Postgres{Host: "db", Port: 5432, DBName: "hermes", User: "u"},
		LocalWorkspace: &config.LocalWorkspace{BasePath: base},
		Sites:          siteBlocks,
	}
}

// TestRejectsSharedSchema is the hazard distinct hostnames do not cover.
// Hostnames are not what isolate tenants -- storage is -- and schema_name is
// overridable, so two sites can be pointed at one schema. Each would connect,
// serve its own hostname, and read the other's rows, with nothing reporting it.
func TestRejectsSharedSchema(t *testing.T) {
	t.Parallel()

	_, err := sites.NewRegistry(cfgWith(t.TempDir(),
		&config.Site{Domain: "docs.jrepp.com", SchemaName: "shared"},
		&config.Site{Domain: "notes.jrepp.com", SchemaName: "shared"},
	))
	if err == nil {
		t.Fatal("two sites were allowed to share one schema")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Errorf("error does not name the cause: %v", err)
	}
}

// TestAllowsSharedSchemaNameInDifferentDatabases is the other half of the
// rule, and the reason the check compares the whole connection rather than the
// schema name. A per-site-database deployment normally has every site called
// "site_x" in a database of its own; rejecting that would be wrong.
func TestAllowsSharedSchemaNameInDifferentDatabases(t *testing.T) {
	t.Parallel()

	cfg := cfgWith(t.TempDir(),
		&config.Site{
			Domain: "docs.jrepp.com", SchemaName: "hermes",
			Database: &config.SiteDatabase{DBName: "hermes_docs"},
		},
		&config.Site{
			Domain: "notes.jrepp.com", SchemaName: "hermes",
			Database: &config.SiteDatabase{DBName: "hermes_notes"},
		},
	)

	if _, err := sites.NewRegistry(cfg); err != nil {
		t.Fatalf("one schema name in two databases was rejected: %v", err)
	}
}

func TestRejectsSharedWorkspacePath(t *testing.T) {
	t.Parallel()

	shared := t.TempDir()

	_, err := sites.NewRegistry(cfgWith(t.TempDir(),
		&config.Site{Domain: "docs.jrepp.com", WorkspacePath: shared},
		&config.Site{Domain: "notes.jrepp.com", WorkspacePath: shared},
	))
	if err == nil {
		t.Fatal("two sites were allowed to share one workspace directory")
	}
	if !strings.Contains(err.Error(), "documents") {
		t.Errorf("error does not name the cause: %v", err)
	}
}

// TestRejectsNestedWorkspacePaths matters as much as the equality case: the
// outer site's tree contains the inner site's, so listing documents for the
// outer one walks into the inner one.
func TestRejectsNestedWorkspacePaths(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	outer := filepath.Join(base, "docs")
	inner := filepath.Join(outer, "notes")

	for _, order := range [][2]string{{outer, inner}, {inner, outer}} {
		_, err := sites.NewRegistry(cfgWith(base,
			&config.Site{Domain: "docs.jrepp.com", WorkspacePath: order[0]},
			&config.Site{Domain: "notes.jrepp.com", WorkspacePath: order[1]},
		))
		if err == nil {
			t.Errorf("nested workspaces accepted (%s inside %s)", order[1], order[0])
		}
	}
}

// TestAllowsAdjacentWorkspacePaths guards the check against being too eager:
// a shared prefix is not containment, and "/data/docs" does not contain
// "/data/docs-notes".
func TestAllowsAdjacentWorkspacePaths(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	cfg := cfgWith(base,
		&config.Site{Domain: "docs.jrepp.com", WorkspacePath: filepath.Join(base, "docs")},
		&config.Site{Domain: "notes.jrepp.com", WorkspacePath: filepath.Join(base, "docs-notes")},
	)

	if _, err := sites.NewRegistry(cfg); err != nil {
		t.Fatalf("adjacent directories were rejected as overlapping: %v", err)
	}
}

// TestDerivedPathsNeverOverlap covers the default: with nothing overridden,
// per-site subdirectories are distinct by construction, and this pins that the
// new check does not reject the ordinary configuration.
func TestDerivedPathsNeverOverlap(t *testing.T) {
	t.Parallel()

	cfg := cfgWith(t.TempDir(),
		&config.Site{Domain: "docs.jrepp.com"},
		&config.Site{Domain: "notes.jrepp.com"},
		&config.Site{Domain: "a.jrepp.com"},
	)

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("derived per-site paths were rejected: %v", err)
	}
	if registry.Len() != 3 {
		t.Errorf("registry has %d sites, want 3", registry.Len())
	}
}

// TestDisabledSiteDoesNotBlockAnother confirms a site taken out of service
// releases its storage claim, so its schema can be reassigned without editing
// two blocks.
func TestDisabledSiteDoesNotBlockAnother(t *testing.T) {
	t.Parallel()

	cfg := cfgWith(t.TempDir(),
		&config.Site{Domain: "old.jrepp.com", SchemaName: "shared", Disabled: true},
		&config.Site{Domain: "new.jrepp.com", SchemaName: "shared"},
	)

	if _, err := sites.NewRegistry(cfg); err != nil {
		t.Fatalf("a disabled site blocked its successor: %v", err)
	}
}
