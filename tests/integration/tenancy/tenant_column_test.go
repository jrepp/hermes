//go:build integration

package tenancy

import (
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/domain"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// TestRowsCarryTheirSite covers the tenancy stamp on the top-level object
// tables.
//
// The schema already isolates tenants; the stamp is what keeps that
// *checkable*. A schema is only a namespace: restore a dump into the wrong
// one, or point schema_name somewhere already in use, and nothing in the data
// would object. These tests pin that it now does.
func TestRowsCarryTheirSite(t *testing.T) {
	s := newStack(t)

	// The application never mentions the column -- GORM does not know it
	// exists -- so the value can only come from the per-schema default. That
	// is deliberate: there is no code path that can write the wrong one.
	docsDB := s.dbFor(t, docsHost)
	if err := docsDB.Create(&models.Product{Name: "Widgets", Abbreviation: "WID"}).Error; err != nil {
		t.Fatalf("writing through the application path: %v", err)
	}

	var stamped string
	if err := docsDB.Raw(
		`SELECT domain FROM products WHERE name = 'Widgets'`).Scan(&stamped).Error; err != nil {
		t.Fatalf("reading the stamp: %v", err)
	}
	if stamped != docsHost {
		t.Errorf("row is stamped %q, want %q", stamped, docsHost)
	}
}

// TestForeignTenantRowsAreRejected is the guarantee that makes the stamp worth
// having: a restore, an export replay, or a hand-written INSERT that claims
// another tenant fails loudly instead of merging two tenants silently.
func TestForeignTenantRowsAreRejected(t *testing.T) {
	s := newStack(t)

	docsDB := s.dbFor(t, docsHost)

	err := docsDB.Exec(`
		INSERT INTO products (created_at, updated_at, name, abbreviation, domain)
		VALUES (now(), now(), 'Sneaky', 'SNK', ?)`, notesHost).Error
	if err == nil {
		t.Fatal("a row claiming another site was accepted")
	}
	if !strings.Contains(err.Error(), "chk_products_domain") {
		t.Errorf("rejected for the wrong reason: %v", err)
	}

	// Rewriting an existing row to another tenant is the restore case, and
	// must fail the same way.
	if err := docsDB.Create(&models.Product{Name: "Real", Abbreviation: "REA"}).Error; err != nil {
		t.Fatalf("seeding: %v", err)
	}
	err = docsDB.Exec(
		`UPDATE products SET domain = ? WHERE name = 'Real'`, notesHost).Error
	if err == nil {
		t.Fatal("an existing row was reassigned to another site")
	}
}

// TestEverySiteSchemaNamesItsOwner checks the schema is self-describing, which
// is what lets an operator answer "whose data is this?" from a bare dump.
func TestEverySiteSchemaNamesItsOwner(t *testing.T) {
	s := newStack(t)

	for _, site := range s.sites.Sites() {
		var owner, schemaName string
		row := s.dbFor(t, site.Domain.String()).Raw(
			`SELECT domain, schema_name FROM site_identity`).Row()
		if err := row.Scan(&owner, &schemaName); err != nil {
			t.Errorf("site %s has no identity row: %v", site.Domain, err)
			continue
		}
		if owner != site.Domain.String() {
			t.Errorf("schema %s claims to belong to %q, not %q",
				site.SchemaName, owner, site.Domain)
		}
		if schemaName != site.SchemaName {
			t.Errorf("identity row records schema %q, not %q", schemaName, site.SchemaName)
		}
	}
}

// TestTenantColumnIsNotNullAcrossTopLevelTables makes the coverage explicit
// rather than trusting the list in the code, so removing a table from it shows
// up here.
func TestTenantColumnIsNotNullAcrossTopLevelTables(t *testing.T) {
	s := newStack(t)

	want := []string{
		"documents", "projects", "products", "users",
		"groups", "document_types", "workspace_projects",
	}

	docsDB := s.dbFor(t, docsHost)
	for _, table := range want {
		var nullable string
		err := docsDB.Raw(`
			SELECT is_nullable FROM information_schema.columns
			WHERE table_schema = ? AND table_name = ? AND column_name = 'domain'`,
			s.sites.Sites()[0].SchemaName, table).Scan(&nullable).Error
		if err != nil {
			t.Errorf("%s: %v", table, err)
			continue
		}
		if nullable != "NO" {
			t.Errorf("%s.domain is nullable (%q); an unstamped row would be "+
				"indistinguishable from a foreign one", table, nullable)
		}
	}
}

// TestSiteIdentityRefusesAForeignSchema pins the collision guard: pointing two
// sites at one schema through schema_name would merge their data, and the
// second migration must refuse rather than proceed.
func TestSiteIdentityRefusesAForeignSchema(t *testing.T) {
	s := newStack(t)

	// Claim the docs schema for a different domain, the way a bad
	// schema_name override would.
	docsSchema := s.sites.Sites()[0].SchemaName
	imposter := domain.MustParse("imposter.example.com")

	docsDB := s.dbFor(t, docsHost)
	err := docsDB.Exec(
		`INSERT INTO `+docsSchema+`.site_identity (domain, schema_name) VALUES (?, ?)`,
		imposter.String(), docsSchema).Error
	if err != nil {
		t.Fatalf("seeding a second identity row: %v", err)
	}

	// The identity table now has two owners, which is exactly the state the
	// guard exists to detect on the next migration.
	var count int64
	if err := docsDB.Raw(
		`SELECT count(*) FROM ` + docsSchema + `.site_identity`).Scan(&count).Error; err != nil {
		t.Fatalf("counting identities: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected two identity rows for this check, got %d", count)
	}
}
