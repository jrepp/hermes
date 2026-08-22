package database

import (
	"strings"
	"testing"
)

func TestValidateSchemaNameAccepts(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"public",
		"site_docs_jrepp_com",
		"site_a__b_com", // hyphen encoding from domain.Name.SchemaName
		"_leading_underscore",
		"s",
		strings.Repeat("a", MaxSchemaNameLength),
	} {
		if err := ValidateSchemaName(name); err != nil {
			t.Errorf("ValidateSchemaName(%q) = %v, want nil", name, err)
		}
	}
}

// TestValidateSchemaNameRejects covers the reason this function exists: a
// schema name cannot be a bind parameter, so it is always interpolated into
// SQL or into a connection string somewhere.
func TestValidateSchemaNameRejects(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, why string }{
		{"", "empty"},
		{strings.Repeat("a", MaxSchemaNameLength+1), "over the identifier limit"},
		{"Site_Docs", "uppercase, which PostgreSQL would fold"},
		{"1site", "starts with a digit"},
		{"pg_temp", "reserved prefix"},
		{"site-docs", "hyphen"},
		{"site docs", "space"},
		{`site"; DROP SCHEMA public CASCADE; --`, "injection"},
		{"site';--", "quote"},
		{"site\x00", "NUL byte"},
		{"sïte", "non-ASCII"},
		{"site.docs", "qualified name"},
	} {
		if err := ValidateSchemaName(tc.name); err == nil {
			t.Errorf("ValidateSchemaName(%q) = nil, want an error (%s)", tc.name, tc.why)
		}
	}
}

func TestQuoteSchemaName(t *testing.T) {
	t.Parallel()

	if got, want := QuoteSchemaName("site_docs"), `"site_docs"`; got != want {
		t.Errorf("QuoteSchemaName = %q, want %q", got, want)
	}

	// ValidateSchemaName rejects embedded quotes, but the doubling must still
	// be correct so that relaxing those rules cannot turn into an injection.
	if got, want := QuoteSchemaName(`a"b`), `"a""b"`; got != want {
		t.Errorf("QuoteSchemaName = %q, want %q", got, want)
	}
}

func TestDSNWithoutSchema(t *testing.T) {
	t.Parallel()

	dsn, err := Config{
		Host: "localhost", Port: 5432, User: "u", Password: "p",
		DBName: "hermes", SSLMode: "disable",
	}.DSN()
	if err != nil {
		t.Fatalf("DSN: %v", err)
	}
	if strings.Contains(dsn, "search_path") {
		t.Errorf("DSN with no schema set search_path: %q", dsn)
	}
}

// TestDSNCarriesSchema pins the mechanism. search_path has to arrive as a
// connection option, not as a `SET` after connecting: a pooled connection that
// is recycled or newly opened would not carry the latter, so a fraction of
// queries would silently run against the default schema.
func TestDSNCarriesSchema(t *testing.T) {
	t.Parallel()

	dsn, err := Config{
		Host: "localhost", Port: 5432, User: "u", Password: "p",
		DBName: "hermes", SSLMode: "disable", SchemaName: "site_docs_jrepp_com",
	}.DSN()
	if err != nil {
		t.Fatalf("DSN: %v", err)
	}

	if !strings.Contains(dsn, "search_path=site_docs_jrepp_com,public") {
		t.Errorf("DSN = %q, want it to carry search_path=site_docs_jrepp_com,public", dsn)
	}

	// The site's schema must come first, or its own tables lose to public's.
	sp := dsn[strings.Index(dsn, "search_path="):]
	if !strings.HasPrefix(sp, "search_path=site_docs_jrepp_com,") {
		t.Errorf("search_path is %q; the site schema must be first", sp)
	}
}

func TestDSNRejectsInvalidSchema(t *testing.T) {
	t.Parallel()

	_, err := Config{
		Host: "localhost", Port: 5432, DBName: "hermes",
		SchemaName: `x"; DROP SCHEMA public; --`,
	}.DSN()
	if err == nil {
		t.Fatal("DSN accepted an invalid schema name")
	}
}

func TestDSNSchemasAreDistinctPerSite(t *testing.T) {
	t.Parallel()

	base := Config{Host: "h", Port: 5432, User: "u", DBName: "hermes"}

	a := base
	a.SchemaName = "site_docs_jrepp_com"
	b := base
	b.SchemaName = "site_notes_jrepp_com"

	dsnA, err := a.DSN()
	if err != nil {
		t.Fatalf("DSN: %v", err)
	}
	dsnB, err := b.DSN()
	if err != nil {
		t.Fatalf("DSN: %v", err)
	}
	if dsnA == dsnB {
		t.Fatal("two sites produced the same DSN")
	}
}
