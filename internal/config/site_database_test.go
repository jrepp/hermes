package config_test

import (
	"os"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

func globalPostgres() *config.Config {
	return &config.Config{
		Postgres: &config.Postgres{
			Host: "db.internal", Port: 5432,
			DBName: "hermes", User: "hermes", Password: "global",
			SSLMode: "require",
		},
	}
}

// TestSiteInheritsGlobalDatabase is the common case: schema separation inside
// one database, nothing written in the site block.
func TestSiteInheritsGlobalDatabase(t *testing.T) {
	docs := domain.MustParse("docs.jrepp.com")

	pg := globalPostgres().PostgresForSite(docs, nil, "site_docs_jrepp_com")

	if pg.Host != "db.internal" || pg.DBName != "hermes" ||
		pg.User != "hermes" || pg.Password != "global" || pg.Port != 5432 {
		t.Errorf("site did not inherit the global connection: %+v", pg)
	}
	if pg.SSLMode != "require" {
		t.Errorf("sslmode = %q, want require", pg.SSLMode)
	}
	if pg.SchemaName != "site_docs_jrepp_com" {
		t.Errorf("schema = %q", pg.SchemaName)
	}
}

// TestSiteOverridesDatabase covers a site living on a different server
// entirely, which is the point of allowing the override: one tenant's data
// need not share a backup, a failover, or a blast radius with another's.
func TestSiteOverridesDatabase(t *testing.T) {
	notes := domain.MustParse("notes.jrepp.com")

	pg := globalPostgres().PostgresForSite(notes, &config.SiteDatabase{
		Host:    "other.internal",
		Port:    5433,
		DBName:  "hermes_notes",
		SSLMode: "verify-full",
	}, "site_notes_jrepp_com")

	if pg.Host != "other.internal" || pg.Port != 5433 || pg.DBName != "hermes_notes" {
		t.Errorf("override not applied: %+v", pg)
	}
	if pg.SSLMode != "verify-full" {
		t.Errorf("sslmode = %q, want verify-full", pg.SSLMode)
	}
	// Unset fields still come from the global block.
	if pg.User != "hermes" || pg.Password != "global" {
		t.Errorf("unset override fields did not inherit: %+v", pg)
	}
}

// TestSitePasswordFromEnvironment matters because a deployment whose sites are
// on different servers would otherwise have to write every password into the
// config file.
func TestSitePasswordFromEnvironment(t *testing.T) {
	notes := domain.MustParse("notes.jrepp.com")

	varName := config.SitePasswordEnvVar(notes)
	if want := "HERMES_SITE_NOTES_JREPP_COM_POSTGRES_PASSWORD"; varName != want {
		t.Fatalf("env var = %q, want %q", varName, want)
	}
	t.Setenv(varName, "from-env")

	pg := globalPostgres().PostgresForSite(notes,
		&config.SiteDatabase{Password: "from-config"}, "site_notes_jrepp_com")

	if pg.Password != "from-env" {
		t.Errorf("password = %q, want the environment value", pg.Password)
	}
}

// TestSitePasswordEnvVarsAreDistinct guards the naming: two sites must not map
// to one variable, or setting one would silently set the other.
func TestSitePasswordEnvVarsAreDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, host := range []string{
		"docs.jrepp.com", "notes.jrepp.com", "a-b.com", "a.b.com",
	} {
		v := config.SitePasswordEnvVar(domain.MustParse(host))
		if prev, dup := seen[v]; dup {
			t.Errorf("%q and %q both map to %s", prev, host, v)
		}
		seen[v] = host
	}
}

func TestSitePasswordEnvVarUnsetLeavesConfigValue(t *testing.T) {
	docs := domain.MustParse("docs.jrepp.com")

	// Make sure the variable is genuinely absent.
	prev, had := os.LookupEnv(config.SitePasswordEnvVar(docs))
	_ = os.Unsetenv(config.SitePasswordEnvVar(docs))
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(config.SitePasswordEnvVar(docs), prev)
		}
	})

	pg := globalPostgres().PostgresForSite(docs,
		&config.SiteDatabase{Password: "from-config"}, "site_docs_jrepp_com")

	if pg.Password != "from-config" {
		t.Errorf("password = %q, want the config value", pg.Password)
	}
}

func TestEffectiveSSLModeDefaultsToDisable(t *testing.T) {
	if got := (config.Postgres{}).EffectiveSSLMode(); got != "disable" {
		t.Errorf("default sslmode = %q, want disable", got)
	}
	if got := (config.Postgres{SSLMode: "require"}).EffectiveSSLMode(); got != "require" {
		t.Errorf("sslmode = %q, want require", got)
	}
}
