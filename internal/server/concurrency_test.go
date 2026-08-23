package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/internal/middleware"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/pkg/domain"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// TestConcurrentRequestsStayOnTheirOwnSite is the isolation claim stated as an
// experiment.
//
// Every per-tenant resource is chosen per request by copying one shared Server
// value. That is only safe if the copy is genuinely a copy and nothing writes
// back through it -- and a mistake there does not fail the way most bugs do.
// It fails under load, for a fraction of requests, serving one tenant's data
// to another, and it would not show up in a test that issues requests one at a
// time.
//
// So: many requests, both sites, all at once, each checking that every
// resource it was handed belongs to the host it asked for.
func TestConcurrentRequestsStayOnTheirOwnSite(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 32
		iterations = 40
	)

	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")

	docsDB, notesDB := &gorm.DB{}, &gorm.DB{}
	pools := map[domain.Name]*gorm.DB{docs: docsDB, notes: notesDB}

	base := Server{
		Logger:  hclog.NewNullLogger(),
		DB:      &gorm.DB{},
		Config:  &config.Config{BaseURL: "https://global.example.com"},
		SiteDBs: db.NewSiteDBsWithPools(pools, &gorm.DB{}),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: docs.String()}, notes: fakeProvider{name: notes.String()},
		},
		SiteWorkspace: map[domain.Name]workspace.WorkspaceProvider{
			docs: fakeWorkspace{name: docs.String()}, notes: fakeWorkspace{name: notes.String()},
		},
	}

	registry, err := sites.NewRegistry(&config.Config{
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{Domain: docs.String()}, {Domain: notes.String()},
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	resolver, err := middleware.NewDomainResolver(registry, nil, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("NewDomainResolver: %v", err)
	}

	problems := make(chan string, goroutines*iterations)

	handler := resolver.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv, ok := base.ForRequest(w, r)
		if !ok {
			problems <- "ForRequest failed for " + r.Host

			return
		}

		want, err := domain.Parse(r.Host)
		if err != nil {
			problems <- "unparseable host " + r.Host

			return
		}

		if srv.DB != pools[want] {
			problems <- fmt.Sprintf("%s got another site's database", want)
		}
		if got := srv.SearchProvider.Name(); got != want.String() {
			problems <- fmt.Sprintf("%s got search provider %q", want, got)
		}
		if ws, ok := srv.WorkspaceProvider.(fakeWorkspace); !ok || ws.name != want.String() {
			problems <- fmt.Sprintf("%s got another site's workspace", want)
		}
		if srv.Config.BaseURL != "https://"+want.String() {
			problems <- fmt.Sprintf("%s got base URL %q", want, srv.Config.BaseURL)
		}
	}))

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		host := docs.String()
		if i%2 == 1 {
			host = notes.String()
		}

		wg.Add(1)
		go func(host string) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				r := httptest.NewRequest(http.MethodGet, "http://"+host+"/api/v2/me", nil)
				r.Host = host
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)

				if w.Code != http.StatusOK {
					problems <- fmt.Sprintf("%s: status %d", host, w.Code)
				}
			}
		}(host)
	}
	wg.Wait()
	close(problems)

	seen := map[string]int{}
	for p := range problems {
		seen[p]++
	}
	for p, n := range seen {
		t.Errorf("%s (%d times)", p, n)
	}
}

// TestForDomainDoesNotShareConfigAcrossSites is the narrow version of the same
// worry. ForDomain rewrites Config.BaseURL, and the Server it starts from
// holds a pointer shared by every request; writing through it rather than
// copying would let the site of whichever request ran last leak into all the
// others.
func TestForDomainDoesNotShareConfigAcrossSites(t *testing.T) {
	t.Parallel()

	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")

	base := Server{
		DB:     &gorm.DB{},
		Config: &config.Config{BaseURL: "https://global.example.com"},
		SiteDBs: db.NewSiteDBsWithPools(map[domain.Name]*gorm.DB{
			docs: {}, notes: {},
		}, &gorm.DB{}),
		SiteSearch: map[domain.Name]search.Provider{
			docs: fakeProvider{name: "docs"}, notes: fakeProvider{name: "notes"},
		},
		SiteWorkspace: workspacesFor(docs, notes),
	}

	ctxFor := func(n domain.Name, baseURL string) context.Context {
		ctx := domain.NewContext(context.Background(), n)

		return sites.NewContext(ctx, &sites.Site{Domain: n, BaseURL: baseURL})
	}

	docsSrv, err := base.ForDomain(ctxFor(docs, "https://docs.jrepp.com"))
	if err != nil {
		t.Fatalf("ForDomain(docs): %v", err)
	}
	notesSrv, err := base.ForDomain(ctxFor(notes, "https://notes.jrepp.com"))
	if err != nil {
		t.Fatalf("ForDomain(notes): %v", err)
	}

	// Scoping the second site must not have rewritten the first.
	if docsSrv.Config.BaseURL != "https://docs.jrepp.com" {
		t.Errorf("the first site's config was rewritten by the second: %q",
			docsSrv.Config.BaseURL)
	}
	if notesSrv.Config.BaseURL != "https://notes.jrepp.com" {
		t.Errorf("second site base URL = %q", notesSrv.Config.BaseURL)
	}
	if docsSrv.Config == notesSrv.Config {
		t.Error("both sites share one *config.Config, so one can overwrite the other")
	}
	if base.Config.BaseURL != "https://global.example.com" {
		t.Errorf("the shared config was mutated: %q", base.Config.BaseURL)
	}
}
