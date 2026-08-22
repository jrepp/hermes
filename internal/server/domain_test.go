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
