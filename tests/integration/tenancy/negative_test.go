//go:build integration

package tenancy

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	hermesdb "github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/internal/test"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// The tests here are all about what happens when something is wrong. Every one
// of these failures is silent or misleading without a deliberate check: a site
// pointed at a dead database, a schema that already belongs to somebody else,
// a password that no longer works. The assertions are on the *message* as much
// as the failure, because an error an operator cannot act on is barely better
// than no error.

// TestMigrationRefusesASchemaOwnedByAnotherSite covers the rename hazard.
//
// The registry rejects two configured sites sharing a schema, but it can only
// see the sites in front of it. Renaming a site and keeping its schema_name --
// or reusing the schema of one that was decommissioned -- produces a single
// valid-looking site whose schema already belongs to a different domain. The
// data underneath is the old tenant's, and adopting it silently is how two
// tenants become one.
func TestMigrationRefusesASchemaOwnedByAnotherSite(t *testing.T) {
	s := newStack(t)

	// docs.jrepp.com owns its schema after newStack migrated it.
	docsSchema := s.sites.Sites()[0].SchemaName

	// A different domain now claims that schema.
	cfg := &config.Config{
		Postgres:       &s.pgCfg,
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{Domain: "successor.jrepp.com", SchemaName: docsSchema},
		},
	}
	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("building the registry: %v", err)
	}

	err = hermesdb.MigrateSites(s.pgCfg, registry, nil)
	if err == nil {
		t.Fatal("a site adopted a schema belonging to another domain; " +
			"its data would have been served as the new site's own")
	}
	for _, want := range []string{docsSchema, docsHost, "successor.jrepp.com"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q, so an operator cannot tell "+
				"which two sites collided: %v", want, err)
		}
	}
}

// TestMigrationReportsAnUnreachableDatabase checks that a site pointed at a
// dead server fails by name. With several sites in a config, "connection
// refused" alone leaves the operator to work out which one.
func TestMigrationReportsAnUnreachableDatabase(t *testing.T) {
	s := newStack(t)

	cfg := &config.Config{
		Postgres:       &s.pgCfg,
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{
				Domain: "offline.jrepp.com",
				// Port 1 is reserved and nothing listens there.
				Database: &config.SiteDatabase{Host: "127.0.0.1", Port: 1},
			},
		},
	}
	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("building the registry: %v", err)
	}

	err = hermesdb.MigrateSites(s.pgCfg, registry, nil)
	if err == nil {
		t.Fatal("migrating against an unreachable database succeeded")
	}
	if !strings.Contains(err.Error(), "offline.jrepp.com") {
		t.Errorf("error does not name the site that could not be reached: %v", err)
	}
}

// TestMigrationReportsBadCredentials is the same requirement for the other
// common misconfiguration. A per-site password comes from its own environment
// variable, so getting one wrong while the others work is easy.
func TestMigrationReportsBadCredentials(t *testing.T) {
	s := newStack(t)

	cfg := &config.Config{
		Postgres:       &s.pgCfg,
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{
				Domain:   "wrongpass.jrepp.com",
				Database: &config.SiteDatabase{Password: "not-the-password"},
			},
		},
	}
	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("building the registry: %v", err)
	}

	err = hermesdb.MigrateSites(s.pgCfg, registry, nil)
	if err == nil {
		t.Fatal("migrating with the wrong password succeeded")
	}
	if !strings.Contains(err.Error(), "wrongpass.jrepp.com") {
		t.Errorf("error does not name the site whose credentials failed: %v", err)
	}
	// The password itself must not appear in an error that will be logged.
	if strings.Contains(err.Error(), "not-the-password") {
		t.Errorf("the password is quoted back in the error, which puts it in "+
			"the logs: %v", err)
	}
}

// TestOpeningPoolsReportsTheFailingSite covers the same requirement on the
// other startup path: pools are opened after migration, and a failure there
// must also say which site.
func TestOpeningPoolsReportsTheFailingSite(t *testing.T) {
	s := newStack(t)

	cfg := &config.Config{
		Postgres:       &s.pgCfg,
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{
				Domain:   "nodb.jrepp.com",
				Database: &config.SiteDatabase{DBName: "database_that_does_not_exist"},
			},
		},
	}
	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("building the registry: %v", err)
	}

	_, err = hermesdb.NewSiteDBs(s.pgCfg, registry, nil, nil)
	if err == nil {
		t.Fatal("opening a pool against a nonexistent database succeeded")
	}
	if !strings.Contains(err.Error(), "nodb.jrepp.com") {
		t.Errorf("error does not name the site: %v", err)
	}
}

// TestRequestForAnUnconfiguredSiteFails is the request-time counterpart: the
// resolver refuses a hostname it does not serve, rather than falling back to
// whichever site happens to be first.
func TestRequestForAnUnconfiguredSiteFails(t *testing.T) {
	s := newStack(t)

	for _, host := range []string{
		"stranger.example.com",
		"docs.jrepp.com.evil.example", // suffix that merely looks familiar
		"jrepp.com",                   // the apex, which is not a site
	} {
		ctx := domain.NewContext(context.Background(), domain.MustParse(host))
		if _, err := s.siteDBs.For(ctx); err == nil {
			t.Errorf("%q resolved to a database", host)
		}
	}
}

// TestMigrationIsIdempotent is a negative test in the other direction: running
// it twice must not be an error. The server migrates on every start, so a
// second run that failed would mean the service could never restart.
func TestMigrationIsIdempotent(t *testing.T) {
	s := newStack(t)

	for i := 0; i < 3; i++ {
		if err := hermesdb.MigrateSites(s.pgCfg, s.sites, nil); err != nil {
			t.Fatalf("migration run %d failed: %v", i+2, err)
		}
	}

	// And the identity rows are still one per schema, not one per run.
	for _, site := range s.sites.Sites() {
		var count int64
		if err := s.dbFor(t, site.Domain.String()).
			Raw(`SELECT count(*) FROM site_identity`).Scan(&count).Error; err != nil {
			t.Fatalf("counting identity rows for %s: %v", site.Domain, err)
		}
		if count != 1 {
			t.Errorf("%s has %d identity rows after repeated migration, want 1",
				site.Domain, count)
		}
	}
}

var _ = test.RequireDocker
