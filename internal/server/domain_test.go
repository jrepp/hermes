package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/pkg/domain"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

func TestForDomainWithoutSiteDBsIsUnchanged(t *testing.T) {
	t.Parallel()

	// A single-tenant deployment configures no sites, so scoping is a no-op
	// and the handler keeps the process-wide database.
	shared := &gorm.DB{}
	srv := Server{DB: shared}

	scoped, err := srv.ForDomain(context.Background())
	if err != nil {
		t.Fatalf("ForDomain: %v", err)
	}
	if scoped.DB != shared {
		t.Error("ForDomain replaced the database in a single-tenant server")
	}
}

func TestForDomainSelectsTheSitePool(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")
	docsDB, notesDB, shared := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}

	srv := Server{
		DB: shared,
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
			docs:  docsDB,
			notes: notesDB,
		}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs:  fakeProvider{name: "docs"},
			notes: fakeProvider{name: "notes"},
		},
		SiteWorkspace: workspacesFor(docs, notes),
	}

	for _, tc := range []struct {
		name domain.Name
		want *gorm.DB
	}{
		{docs, docsDB},
		{notes, notesDB},
	} {
		scoped, err := srv.ForDomain(domain.NewContext(context.Background(), tc.name))
		if err != nil {
			t.Fatalf("ForDomain(%s): %v", tc.name, err)
		}
		if scoped.DB != tc.want {
			t.Errorf("ForDomain(%s) bound the wrong database", tc.name)
		}
	}
}

// TestForDomainDoesNotMutateTheReceiver matters because one Server value is
// shared by every handler and every concurrent request. Scoping must produce a
// copy; mutating in place would let one request's site leak into another's.
func TestForDomainDoesNotMutateTheReceiver(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	docsDB, shared := &gorm.DB{}, &gorm.DB{}

	srv := Server{
		DB:      shared,
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{docs: docsDB}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: "docs"},
		},
		SiteWorkspace: workspacesFor(docs),
	}

	if _, err := srv.ForDomain(domain.NewContext(context.Background(), docs)); err != nil {
		t.Fatalf("ForDomain: %v", err)
	}

	if srv.DB != shared {
		t.Error("ForDomain mutated the Server it was called on")
	}
}

func TestForRequestFailsClosedOnUnknownSite(t *testing.T) {
	t.Parallel()

	known := domain.MustParse("docs.jrepp.com")
	shared := &gorm.DB{}

	srv := Server{
		DB:      shared,
		Logger:  hclog.NewNullLogger(),
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{known: {}}, shared),
	}

	r := httptest.NewRequest(http.MethodGet, "http://other.example.com/api/v2/me", nil)
	r = r.WithContext(domain.NewContext(r.Context(), domain.MustParse("other.example.com")))
	w := httptest.NewRecorder()

	scoped, ok := srv.ForRequest(w, r)
	if ok {
		t.Fatal("ForRequest succeeded for a site with no configured database")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if scoped.DB != nil {
		t.Error("ForRequest returned a usable Server alongside a failure")
	}
}

func TestForRequestSucceedsForAKnownSite(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	docsDB, shared := &gorm.DB{}, &gorm.DB{}

	srv := Server{
		DB:      shared,
		Logger:  hclog.NewNullLogger(),
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{docs: docsDB}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: "docs"},
		},
		SiteWorkspace: workspacesFor(docs),
	}

	r := httptest.NewRequest(http.MethodGet, "http://docs.jrepp.com/api/v2/me", nil)
	r = r.WithContext(domain.NewContext(r.Context(), docs))
	w := httptest.NewRecorder()

	scoped, ok := srv.ForRequest(w, r)
	if !ok {
		t.Fatalf("ForRequest failed: %d %s", w.Code, w.Body.String())
	}
	if scoped.DB != docsDB {
		t.Error("ForRequest bound the wrong database")
	}
}

// fakeProvider is a search.Provider identity marker; ForDomain only routes.
type fakeProvider struct {
	search.Provider
	name string
}

func (f fakeProvider) Name() string { return f.name }

// fakeWorkspace is the same idea for the workspace provider.
type fakeWorkspace struct {
	workspace.WorkspaceProvider
	name string
}

func workspacesFor(names ...domain.Name) map[domain.Name]workspace.WorkspaceProvider {
	m := make(map[domain.Name]workspace.WorkspaceProvider, len(names))
	for _, n := range names {
		m[n] = fakeWorkspace{name: n.String()}
	}

	return m
}

func TestForDomainSelectsTheSiteSearchProvider(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")
	shared := &gorm.DB{}

	srv := Server{
		DB:             shared,
		SearchProvider: fakeProvider{name: "shared"},
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
			docs: {}, notes: {},
		}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs:  fakeProvider{name: "docs"},
			notes: fakeProvider{name: "notes"},
		},
		SiteWorkspace: workspacesFor(docs, notes),
	}

	for _, tc := range []struct {
		name domain.Name
		want string
	}{
		{docs, "docs"},
		{notes, "notes"},
	} {
		scoped, err := srv.ForDomain(domain.NewContext(context.Background(), tc.name))
		if err != nil {
			t.Fatalf("ForDomain(%s): %v", tc.name, err)
		}
		if got := scoped.SearchProvider.Name(); got != tc.want {
			t.Errorf("ForDomain(%s) bound search provider %q, want %q",
				tc.name, got, tc.want)
		}
	}
}

// TestForDomainRefusesASiteWithNoSearchProvider is the fail-closed rule
// applied to search. Falling back to the shared provider would search one
// tenant's index on another tenant's behalf.
func TestForDomainRefusesASiteWithNoSearchProvider(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	orphan := domain.MustParse("orphan.jrepp.com")
	shared := &gorm.DB{}

	srv := Server{
		DB:             shared,
		SearchProvider: fakeProvider{name: "shared"},
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
			docs: {}, orphan: {},
		}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: "docs"},
		},
		SiteWorkspace: workspacesFor(docs),
	}

	if _, err := srv.ForDomain(
		domain.NewContext(context.Background(), orphan)); err == nil {
		t.Fatal("a site with no search provider resolved to the shared one")
	}
}

// TestForDomainKeepsSharedSearchWhenSingleTenant covers the deployment with no
// site blocks, where scoping must change nothing.
func TestForDomainKeepsSharedSearchWhenSingleTenant(t *testing.T) {
	t.Parallel()

	shared := &gorm.DB{}
	srv := Server{DB: shared, SearchProvider: fakeProvider{name: "shared"}}

	scoped, err := srv.ForDomain(context.Background())
	if err != nil {
		t.Fatalf("ForDomain: %v", err)
	}
	if got := scoped.SearchProvider.Name(); got != "shared" {
		t.Errorf("single-tenant search provider = %q, want shared", got)
	}
}

func TestVerifySiteResources(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")
	shared := &gorm.DB{}

	complete := Server{
		DB: shared,
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
			docs: {}, notes: {},
		}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: "docs"}, notes: fakeProvider{name: "notes"},
		},
		SiteWorkspace: workspacesFor(docs, notes),
	}
	if err := complete.VerifySiteResources([]domain.Name{docs, notes}); err != nil {
		t.Fatalf("a fully wired server failed verification: %v", err)
	}

	// No sites: nothing to verify.
	if err := (Server{}).VerifySiteResources(nil); err != nil {
		t.Errorf("single-tenant server failed verification: %v", err)
	}

	missingSearch := complete
	missingSearch.SiteSearch = map[domain.Name]search.Provider{docs: fakeProvider{name: "docs"}}
	if err := missingSearch.VerifySiteResources([]domain.Name{docs, notes}); err == nil {
		t.Error("a site with no search provider passed verification")
	}

	missingDB := complete
	missingDB.SiteDBs = db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{docs: {}}, shared)
	if err := missingDB.VerifySiteResources([]domain.Name{docs, notes}); err == nil {
		t.Error("a site with no database passed verification")
	}

	noPools := complete
	noPools.SiteDBs = nil
	if err := noPools.VerifySiteResources([]domain.Name{docs}); err == nil {
		t.Error("sites configured with no pools at all passed verification")
	}
}

// TestForDomainSelectsTheSiteWorkspace closes the last of the three per-tenant
// resources. Without it the local adapter's domain scoping exists but nothing
// sets it, and every site writes documents into one flat directory.
func TestForDomainSelectsTheSiteWorkspace(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")
	shared := &gorm.DB{}

	srv := Server{
		DB: shared,
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
			docs: {}, notes: {},
		}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: "docs"}, notes: fakeProvider{name: "notes"},
		},
		SiteWorkspace: workspacesFor(docs, notes),
	}

	for _, name := range []domain.Name{docs, notes} {
		scoped, err := srv.ForDomain(domain.NewContext(context.Background(), name))
		if err != nil {
			t.Fatalf("ForDomain(%s): %v", name, err)
		}
		got, ok := scoped.WorkspaceProvider.(fakeWorkspace)
		if !ok {
			t.Fatalf("ForDomain(%s) returned an unexpected workspace type", name)
		}
		if got.name != name.String() {
			t.Errorf("ForDomain(%s) bound workspace %q", name, got.name)
		}
	}
}

func TestForDomainRefusesASiteWithNoWorkspace(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	orphan := domain.MustParse("orphan.jrepp.com")
	shared := &gorm.DB{}

	srv := Server{
		DB: shared,
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
			docs: {}, orphan: {},
		}, shared),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: "docs"}, orphan: fakeProvider{name: "orphan"},
		},
		SiteWorkspace: workspacesFor(docs),
	}

	if _, err := srv.ForDomain(
		domain.NewContext(context.Background(), orphan)); err == nil {
		t.Fatal("a site with no workspace provider resolved to the shared one")
	}
}
