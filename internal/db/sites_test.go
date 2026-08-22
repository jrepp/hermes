package db

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

func TestSiteDBsSelectsByDomain(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")

	// Distinct pointers stand in for distinct pools; For only routes.
	docsDB, notesDB, fallback := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}

	s := NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
		docs:  docsDB,
		notes: notesDB,
	}, fallback)

	for _, tc := range []struct {
		name domain.Name
		want *gorm.DB
	}{
		{docs, docsDB},
		{notes, notesDB},
	} {
		got, err := s.For(domain.NewContext(context.Background(), tc.name))
		if err != nil {
			t.Fatalf("For(%s): %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("For(%s) returned the wrong pool", tc.name)
		}
	}
}

// TestSiteDBsRejectsUnknownSite is the isolation guarantee. Returning the
// fallback here would run the request against the default schema, which in a
// multi-site deployment holds some other tenant's rows.
func TestSiteDBsRejectsUnknownSite(t *testing.T) {
	t.Parallel()

	known := domain.MustParse("docs.jrepp.com")
	fallback := &gorm.DB{}

	s := NewSiteDBsWithPools(map[domain.Name]*gorm.DB{known: {}}, fallback)

	ctx := domain.NewContext(context.Background(), domain.MustParse("other.example.com"))
	got, err := s.For(ctx)
	if err == nil {
		t.Fatalf("For(unknown site) returned a pool (fallback=%v) instead of an error",
			got == fallback)
	}
}

func TestSiteDBsFallsBackWhenNoSiteInContext(t *testing.T) {
	t.Parallel()

	fallback := &gorm.DB{}
	s := NewSiteDBsWithPools(nil, fallback)

	got, err := s.For(context.Background())
	if err != nil {
		t.Fatalf("For(no site): %v", err)
	}
	if got != fallback {
		t.Error("For(no site) did not return the fallback")
	}

	// A zero domain is the same case, not a distinct one.
	got, err = s.For(domain.NewContext(context.Background(), domain.Name{}))
	if err != nil {
		t.Fatalf("For(zero domain): %v", err)
	}
	if got != fallback {
		t.Error("For(zero domain) did not return the fallback")
	}
}

func TestSiteDBsWithoutFallbackRefusesUnscopedRequests(t *testing.T) {
	t.Parallel()

	s := NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
		domain.MustParse("docs.jrepp.com"): {},
	}, nil)

	if _, err := s.For(context.Background()); err == nil {
		t.Fatal("For(no site) succeeded with no fallback configured")
	}
}

func TestSiteDBsLen(t *testing.T) {
	t.Parallel()

	s := NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
		domain.MustParse("a.example.com"): {},
		domain.MustParse("b.example.com"): {},
	}, nil)

	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
}

// TestNewSiteDBsWithPoolsCopies guards against the caller's map being
// retained: a later mutation of it must not silently re-route a tenant.
func TestNewSiteDBsWithPoolsCopies(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	original := &gorm.DB{}
	m := map[domain.Name]*gorm.DB{docs: original}

	s := NewSiteDBsWithPools(m, nil)
	m[docs] = &gorm.DB{}

	got, err := s.For(domain.NewContext(context.Background(), docs))
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if got != original {
		t.Error("mutating the caller's map changed which pool a site resolves to")
	}
}

func TestMigrateSitesWithNoRegistryIsNoop(t *testing.T) {
	t.Parallel()

	// No registry means single-tenant, and the caller runs the ordinary
	// migration path instead. This must not try to connect.
	if err := MigrateSites(postgresConfigThatWouldFail(), nil, nil); err != nil {
		t.Fatalf("MigrateSites(nil registry): %v", err)
	}
}

// postgresConfigThatWouldFail points nowhere on purpose: if MigrateSites ever
// starts dialing for an empty registry, this test fails rather than hangs.
func postgresConfigThatWouldFail() config.Postgres {
	return config.Postgres{
		Host:   "127.0.0.1",
		Port:   1,
		User:   "nobody",
		DBName: "nothing",
	}
}
