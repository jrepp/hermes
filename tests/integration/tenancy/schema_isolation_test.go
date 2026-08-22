//go:build integration

// Package tenancy verifies that two sites served by one Hermes process do not
// share database state.
//
// Everything else about the isolation -- schema derivation, the site registry,
// the routing middleware -- is covered by unit tests. This package covers the
// part that only a real PostgreSQL can answer: whether the search_path
// actually lands on every pooled connection, and whether a query issued
// through one site's pool can see another site's rows.
package tenancy

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	hermesdb "github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/internal/migrate"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/internal/test"
	"github.com/hashicorp-forge/hermes/pkg/domain"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

const (
	docsHost  = "docs.jrepp.com"
	notesHost = "notes.jrepp.com"
)

type stack struct {
	pg      *tcpostgres.PostgresContainer
	pgCfg   config.Postgres
	sites   *sites.Registry
	siteDBs *hermesdb.SiteDBs
}

// newStack brings up PostgreSQL, migrates a schema per site, and opens the
// per-site pools -- the same sequence the server performs at startup.
func newStack(t *testing.T) *stack {
	t.Helper()

	test.RequireDocker(t)

	ctx := context.Background()
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	pg, err := tcpostgres.Run(ctx,
		"pgvector/pgvector:pg17",
		tcpostgres.WithDatabase("hermes"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := pg.Terminate(context.Background()); err != nil {
			t.Logf("terminating postgres: %v", err)
		}
	})

	host, err := pg.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	pgCfg := config.Postgres{
		Host:     host,
		Port:     port.Int(),
		User:     "postgres",
		Password: "postgres",
		DBName:   "hermes",
	}

	cfg := &config.Config{
		Postgres:       &pgCfg,
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites: []*config.Site{
			{Domain: docsHost},
			{Domain: notesHost},
		},
	}

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("building site registry: %v", err)
	}

	if err := hermesdb.MigrateSites(pgCfg, registry, nil); err != nil {
		t.Fatalf("migrating site schemas: %v", err)
	}

	siteDBs, err := hermesdb.NewSiteDBs(pgCfg, registry, nil, nil)
	if err != nil {
		t.Fatalf("opening site databases: %v", err)
	}
	t.Cleanup(func() {
		if err := siteDBs.Close(); err != nil {
			t.Logf("closing site databases: %v", err)
		}
	})

	return &stack{pg: pg, pgCfg: pgCfg, sites: registry, siteDBs: siteDBs}
}

func (s *stack) dbFor(t *testing.T, host string) *gorm.DB {
	t.Helper()

	ctx := domain.NewContext(context.Background(), domain.MustParse(host))
	db, err := s.siteDBs.For(ctx)
	if err != nil {
		t.Fatalf("resolving database for %s: %v", host, err)
	}

	return db
}

// TestWritesDoNotCrossSites is the guarantee in one test: a row written
// through one site's pool must be invisible through another's.
func TestWritesDoNotCrossSites(t *testing.T) {
	s := newStack(t)

	docsDB := s.dbFor(t, docsHost)
	notesDB := s.dbFor(t, notesHost)

	docsProduct := &models.Product{Name: "DocsOnly", Abbreviation: "DOC"}
	if err := docsDB.Create(docsProduct).Error; err != nil {
		t.Fatalf("writing to %s: %v", docsHost, err)
	}

	var seen []models.Product
	if err := notesDB.Find(&seen).Error; err != nil {
		t.Fatalf("reading from %s: %v", notesHost, err)
	}
	if len(seen) != 0 {
		t.Fatalf("%s sees %d product(s) written by %s: %+v",
			notesHost, len(seen), docsHost, seen)
	}

	// And the write is genuinely there, so the test above is not passing
	// because nothing was written at all.
	var mine []models.Product
	if err := docsDB.Find(&mine).Error; err != nil {
		t.Fatalf("reading back from %s: %v", docsHost, err)
	}
	if len(mine) != 1 {
		t.Fatalf("%s sees %d of its own products, want 1", docsHost, len(mine))
	}
}

// TestSameNameInBothSites checks that isolation is not accidentally provided
// by unique constraints. Product.Name is unique, so if both sites shared a
// table the second insert would fail -- and a passing test would prove
// nothing. It succeeding proves the tables are genuinely distinct.
func TestSameNameInBothSites(t *testing.T) {
	s := newStack(t)

	for _, host := range []string{docsHost, notesHost} {
		p := &models.Product{Name: "Shared", Abbreviation: "SHR"}
		if err := s.dbFor(t, host).Create(p).Error; err != nil {
			t.Fatalf("writing the same product name to %s: %v", host, err)
		}
	}
}

// TestSearchPathSurvivesConnectionChurn is the reason search_path is passed in
// the DSN rather than issued as a SET after connecting.
//
// A runtime SET applies to one connection. Under a pool, the next query may
// land on a connection that never received it -- so a fraction of queries
// would silently read the default schema, and the failure would be
// load-dependent and nearly impossible to reproduce. Forcing the pool to open
// many connections at once makes that failure deterministic if it exists.
func TestSearchPathSurvivesConnectionChurn(t *testing.T) {
	s := newStack(t)

	docsDB := s.dbFor(t, docsHost)
	if err := docsDB.Create(&models.Product{Name: "Churn", Abbreviation: "CHN"}).Error; err != nil {
		t.Fatalf("seeding: %v", err)
	}

	const concurrency = 24
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var schema string
			if err := docsDB.Raw("SELECT current_schema()").Scan(&schema).Error; err != nil {
				errs <- fmt.Errorf("current_schema: %w", err)
				return
			}
			want := s.sites.Sites()[0].SchemaName
			if schema != want {
				errs <- fmt.Errorf("connection resolved to schema %q, want %q", schema, want)
				return
			}

			var count int64
			if err := docsDB.Model(&models.Product{}).Count(&count).Error; err != nil {
				errs <- fmt.Errorf("count: %w", err)
				return
			}
			if count != 1 {
				errs <- fmt.Errorf("count = %d, want 1", count)
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

// TestEachSiteHasItsOwnMigrationVersion guards the subtlety that makes adding
// a site to a running deployment work. golang-migrate keeps a version table;
// if it were shared, the first site's completed migration would make every
// later site look already-migrated, and those sites would come up with no
// tables and no error.
func TestEachSiteHasItsOwnMigrationVersion(t *testing.T) {
	s := newStack(t)

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		s.pgCfg.Host, s.pgCfg.Port, s.pgCfg.User, s.pgCfg.Password, s.pgCfg.DBName)

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	for _, site := range s.sites.Sites() {
		var version int64
		err := sqlDB.QueryRow(
			`SELECT version FROM ` + site.SchemaName + `.schema_migrations`,
		).Scan(&version)
		if err != nil {
			t.Errorf("site %s has no migration version table in schema %s: %v",
				site.Domain.String(), site.SchemaName, err)
			continue
		}
		if version == 0 {
			t.Errorf("site %s reports migration version 0", site.Domain.String())
		}
	}

	// The site tables must live in the site schemas, not in public. If they
	// were in public, every query would still work -- via the search_path
	// fallback -- and every site would share them.
	var inPublic int
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name = 'products'`,
	).Scan(&inPublic); err != nil {
		t.Fatalf("querying information_schema: %v", err)
	}
	if inPublic != 0 {
		t.Error("products exists in the public schema; site queries can fall through to it")
	}
}

// TestUnknownSiteHasNoDatabase confirms the fail-closed path against a live
// registry rather than a hand-built map.
func TestUnknownSiteHasNoDatabase(t *testing.T) {
	s := newStack(t)

	ctx := domain.NewContext(context.Background(), domain.MustParse("stranger.example.com"))
	if _, err := s.siteDBs.For(ctx); err == nil {
		t.Fatal("an unconfigured site resolved to a database")
	}
}

// TestSingleTenantMigrationIsUnchanged guards the path every existing
// deployment uses. Adding schema support must not move anyone's tables: with
// no schema named, migrations still land in the connection's own schema.
func TestSingleTenantMigrationIsUnchanged(t *testing.T) {
	test.RequireDocker(t)

	ctx := context.Background()
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	pg, err := tcpostgres.Run(ctx,
		"pgvector/pgvector:pg17",
		tcpostgres.WithDatabase("hermes"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := pg.Terminate(context.Background()); err != nil {
			t.Logf("terminating postgres: %v", err)
		}
	})

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := migrate.RunMigrations(sqlDB, "postgres"); err != nil {
		t.Fatalf("single-tenant migration: %v", err)
	}

	for _, table := range []string{"documents", "products", "users", "schema_migrations"} {
		var found sql.NullString
		if err := sqlDB.QueryRow(
			"SELECT to_regclass($1)::text", "public."+table,
		).Scan(&found); err != nil {
			t.Fatalf("checking public.%s: %v", table, err)
		}
		if !found.Valid {
			t.Errorf("public.%s does not exist after a single-tenant migration", table)
		}
	}

	// Running twice must be a no-op, not an error: the server migrates on
	// every start.
	if err := migrate.RunMigrations(sqlDB, "postgres"); err != nil {
		t.Fatalf("second single-tenant migration: %v", err)
	}

	// The tenancy stamp is a multi-site construct. A deployment with no sites
	// must not acquire a domain column or an identity table it never asked
	// for -- the schema it had before this work is the schema it keeps.
	var extras int
	if err := sqlDB.QueryRow(`
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'domain'`).Scan(&extras); err != nil {
		t.Fatalf("querying columns: %v", err)
	}
	if extras != 0 {
		t.Errorf("single-tenant schema gained %d tenant column(s)", extras)
	}

	if err := sqlDB.QueryRow(`
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'site_identity'`).Scan(&extras); err != nil {
		t.Fatalf("querying tables: %v", err)
	}
	if extras != 0 {
		t.Error("single-tenant schema gained a site_identity table")
	}
}

// TestSitesCoexistWithAPopulatedPublicSchema covers the upgrade path: a
// database that was previously single-tenant already has every Hermes table in
// public, and every site's search_path ends in public so that extension types
// resolve.
//
// The question is whether the site migrations then no-op. They do not:
// PostgreSQL resolves `CREATE TABLE IF NOT EXISTS` against the schema the
// table would be created in -- the first search_path entry -- not against what
// is visible along the whole path. So the site tables are created in the site
// schemas and the public ones are simply shadowed.
//
// That is worth a test rather than a comment, because if it were the other way
// around every site would silently share public's rows with no error anywhere.
func TestSitesCoexistWithAPopulatedPublicSchema(t *testing.T) {
	test.RequireDocker(t)

	ctx := context.Background()
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	pg, err := tcpostgres.Run(ctx,
		"pgvector/pgvector:pg17",
		tcpostgres.WithDatabase("hermes"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := pg.Terminate(context.Background()); err != nil {
			t.Logf("terminating postgres: %v", err)
		}
	})

	host, err := pg.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	pgCfg := config.Postgres{
		Host: host, Port: port.Int(),
		User: "postgres", Password: "postgres", DBName: "hermes",
	}

	// Bring the database up the way a single-tenant deployment would, and put
	// a row in it that no site must ever see.
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := migrate.RunMigrations(sqlDB, "postgres"); err != nil {
		t.Fatalf("single-tenant migration: %v", err)
	}
	if _, err := sqlDB.Exec(
		`INSERT INTO public.products (created_at, updated_at, name, abbreviation)
		 VALUES (now(), now(), 'LegacyPublic', 'LEG')`); err != nil {
		t.Fatalf("seeding public: %v", err)
	}

	// Now enable multi-site hosting on top of it.
	cfg := &config.Config{
		Postgres:       &pgCfg,
		LocalWorkspace: &config.LocalWorkspace{BasePath: t.TempDir()},
		Sites:          []*config.Site{{Domain: docsHost}, {Domain: notesHost}},
	}
	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("building site registry: %v", err)
	}
	if err := hermesdb.MigrateSites(pgCfg, registry, nil); err != nil {
		t.Fatalf("MigrateSites over a populated public schema: %v", err)
	}

	siteDBs, err := hermesdb.NewSiteDBs(pgCfg, registry, nil, nil)
	if err != nil {
		t.Fatalf("opening site databases: %v", err)
	}
	t.Cleanup(func() { _ = siteDBs.Close() })

	// Each site must have its own tables, not public's.
	for _, site := range registry.Sites() {
		var found sql.NullString
		if err := sqlDB.QueryRow(
			"SELECT to_regclass($1)::text", site.SchemaName+".products",
		).Scan(&found); err != nil {
			t.Fatalf("checking %s.products: %v", site.SchemaName, err)
		}
		if !found.Valid {
			t.Fatalf("%s.products does not exist; the site migration no-opped "+
				"against public and this site would share public's rows",
				site.SchemaName)
		}
	}

	// And the legacy row must be invisible from every site.
	for _, host := range []string{docsHost, notesHost} {
		siteDB, err := siteDBs.For(domain.NewContext(ctx, domain.MustParse(host)))
		if err != nil {
			t.Fatalf("resolving %s: %v", host, err)
		}

		var products []models.Product
		if err := siteDB.Find(&products).Error; err != nil {
			t.Fatalf("reading from %s: %v", host, err)
		}
		if len(products) != 0 {
			t.Errorf("%s sees %d row(s) from the public schema: %+v",
				host, len(products), products)
		}
	}
}
