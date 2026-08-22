package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/domain"
	algoliaadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/algolia"
)

func meili() *config.Meilisearch {
	return &config.Meilisearch{
		Host:              "http://127.0.0.1:7700",
		APIKey:            "k",
		DocsIndexName:     "docs",
		DraftsIndexName:   "drafts",
		ProjectsIndexName: "projects",
		LinksIndexName:    "links",
	}
}

func TestMeilisearchWithoutDomainIsUnchanged(t *testing.T) {
	t.Parallel()

	got := meili().ToMeilisearchAdapterConfigForDomain(domain.Name{})

	if got.DocsIndexName != "docs" || got.DraftsIndexName != "drafts" ||
		got.ProjectsIndexName != "projects" || got.LinksIndexName != "links" {
		t.Errorf("zero domain changed the index names: %+v", got)
	}
}

// TestMeilisearchIndexesAreDisjoint is the isolation guarantee for search. Two
// sites sharing an index would return each other's documents, which is the
// same leak the per-site database schema closes on the query side.
func TestMeilisearchIndexesAreDisjoint(t *testing.T) {
	t.Parallel()

	docs := meili().ToMeilisearchAdapterConfigForDomain(domain.MustParse("docs.jrepp.com"))
	notes := meili().ToMeilisearchAdapterConfigForDomain(domain.MustParse("notes.jrepp.com"))

	seen := map[string]string{}
	for label, name := range map[string]string{
		"docs/docs": docs.DocsIndexName, "docs/drafts": docs.DraftsIndexName,
		"docs/projects": docs.ProjectsIndexName, "docs/links": docs.LinksIndexName,
		"notes/docs": notes.DocsIndexName, "notes/drafts": notes.DraftsIndexName,
		"notes/projects": notes.ProjectsIndexName, "notes/links": notes.LinksIndexName,
	} {
		if prev, dup := seen[name]; dup {
			t.Errorf("%s and %s both use index %q", prev, label, name)
		}
		seen[name] = label
	}

	if !strings.HasPrefix(docs.DocsIndexName, "docs_jrepp_com_") {
		t.Errorf("docs index = %q, want a docs.jrepp.com prefix", docs.DocsIndexName)
	}
}

// TestMeilisearchIndexNamesAreLegal guards the reason the slug exists rather
// than the hostname: Meilisearch index UIDs accept only [A-Za-z0-9_-], so a
// dotted hostname could not be used directly.
func TestMeilisearchIndexNamesAreLegal(t *testing.T) {
	t.Parallel()

	got := meili().ToMeilisearchAdapterConfigForDomain(domain.MustParse("a-b.docs.jrepp.com"))

	for _, name := range []string{
		got.DocsIndexName, got.DraftsIndexName,
		got.ProjectsIndexName, got.LinksIndexName,
	} {
		for i := 0; i < len(name); i++ {
			c := name[i]
			ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '_' || c == '-'
			if !ok {
				t.Errorf("index name %q contains %q, which Meilisearch rejects",
					name, string(c))
				break
			}
		}
	}
}

func TestBleveIndexPathIsPerSite(t *testing.T) {
	t.Parallel()

	b := &config.Bleve{IndexPath: "/var/lib/hermes/index"}

	if got := b.IndexPathForDomain(domain.Name{}); got != "/var/lib/hermes/index" {
		t.Errorf("zero domain path = %q, want the configured path", got)
	}

	docs := b.IndexPathForDomain(domain.MustParse("docs.jrepp.com"))
	notes := b.IndexPathForDomain(domain.MustParse("notes.jrepp.com"))

	if docs == notes {
		t.Fatalf("both sites index into %q", docs)
	}
	if want := filepath.Join("/var/lib/hermes/index", "docs.jrepp.com"); docs != want {
		t.Errorf("docs path = %q, want %q", docs, want)
	}
	// A Bleve index is a directory, so one site's index must not sit inside
	// another's.
	if strings.HasPrefix(docs, notes+string(filepath.Separator)) ||
		strings.HasPrefix(notes, docs+string(filepath.Separator)) {
		t.Errorf("one site's index is nested inside the other: %q, %q", docs, notes)
	}
}

func TestAlgoliaIndexesAreDisjoint(t *testing.T) {
	t.Parallel()

	base := &algoliaadapter.Config{
		AppID: "app", WriteAPIKey: "w",
		DocsIndexName: "docs", DraftsIndexName: "drafts",
		InternalIndexName: "internal", LinksIndexName: "links",
		MissingFieldsIndexName: "missing", ProjectsIndexName: "projects",
	}

	docs := config.ScopeAlgoliaForDomain(base, domain.MustParse("docs.jrepp.com"))
	notes := config.ScopeAlgoliaForDomain(base, domain.MustParse("notes.jrepp.com"))

	if docs.DocsIndexName == notes.DocsIndexName {
		t.Errorf("both sites use index %q", docs.DocsIndexName)
	}
	// Credentials are not per-site and must survive the copy.
	if docs.AppID != "app" || docs.WriteAPIKey != "w" {
		t.Errorf("scoping dropped credentials: %+v", docs)
	}
	// The shared config must not be mutated.
	if base.DocsIndexName != "docs" {
		t.Errorf("scoping mutated the shared config: %q", base.DocsIndexName)
	}
}

// TestSearchNamespaceMatchesSchemaEncoding pins that search and the database
// use one encoding. If they diverged, a site's rows and its index could end up
// keyed to different names and nothing would report it.
func TestSearchNamespaceMatchesSchemaEncoding(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"docs.jrepp.com", "a-b.com", "x.y.z.example.com"} {
		n := domain.MustParse(host)
		if got, want := "site_"+n.SearchNamespace(), n.SchemaName(); got != want {
			t.Errorf("%q: search namespace %q does not match schema %q",
				host, n.SearchNamespace(), n.SchemaName())
		}
	}
}
